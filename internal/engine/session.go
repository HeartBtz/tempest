package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/HeartBtz/tempest/internal/client"
	"github.com/HeartBtz/tempest/internal/config"
	"github.com/HeartBtz/tempest/internal/protocol"
	"github.com/HeartBtz/tempest/internal/storage"
)

type SessionRunner struct {
	session        *storage.Session
	torrent        *storage.Torrent
	profile        client.Profile
	tracker        *protocol.TrackerClient
	randomizer     *client.Randomizer
	infoHash       [20]byte
	trackers       []string
	stopCh         chan struct{}
	done           chan struct{}
	started        bool
	running        bool
	completedSent  bool
	mu             sync.Mutex
	onUpdate       func(*storage.Session)
	onLog          func(string, string)
	speedAllocator func() (uploadSpeed, downloadSpeed, variance int64)
}

func NewSessionRunner(
	session *storage.Session,
	torrent *storage.Torrent,
	onUpdate func(*storage.Session),
	onLog func(string, string),
	speedAllocator func() (int64, int64, int64),
) (*SessionRunner, error) {
	profile, ok := client.GetProfile(session.ClientProfile)
	if !ok {
		profile = client.DefaultProfile()
	}

	// Parse info hash from hex
	var infoHash [20]byte
	if len(torrent.InfoHash) == 40 {
		for i := 0; i < 20; i++ {
			var b byte
			fmt.Sscanf(torrent.InfoHash[i*2:i*2+2], "%02x", &b)
			infoHash[i] = b
		}
	}

	// Parse trackers
	var trackers []string
	json.Unmarshal([]byte(torrent.Trackers), &trackers)

	// Create tracker client with optional interface binding
	var tracker *protocol.TrackerClient
	if session.NetworkInterface != "" {
		tracker = protocol.NewTrackerClientWithInterface(session.NetworkInterface)
	} else {
		tracker = protocol.NewTrackerClient()
	}

	return &SessionRunner{
		session:        cloneSession(session),
		torrent:        torrent,
		profile:        profile,
		tracker:        tracker,
		randomizer:     client.NewRandomizer(config.Get().Engine.EnableRandomization),
		infoHash:       infoHash,
		trackers:       trackers,
		stopCh:         make(chan struct{}),
		done:           make(chan struct{}),
		onUpdate:       onUpdate,
		onLog:          onLog,
		speedAllocator: speedAllocator,
	}, nil
}

func (sr *SessionRunner) Start() error {
	sr.mu.Lock()
	if sr.started {
		sr.mu.Unlock()
		return fmt.Errorf("session already started")
	}
	sr.started = true
	sr.running = true
	sr.session.Status = "running"
	if sr.session.PeerID == "" {
		sr.session.PeerID = sr.profile.GeneratePeerID()
	}
	if sr.session.Key == "" {
		sr.session.Key = sr.profile.GenerateKey()
	}
	snapshot := cloneSession(sr.session)
	sr.mu.Unlock()

	sr.update(snapshot)
	sr.log("info", "Session started for %s", sr.torrent.Name)

	go sr.runLoop()
	return nil
}

func (sr *SessionRunner) applyAllocatedSpeeds() {
	if sr.speedAllocator == nil {
		return
	}
	uploadSpeed, downloadSpeed, variance := sr.speedAllocator()
	sr.mu.Lock()
	sr.session.UploadSpeed = uploadSpeed
	sr.session.DownloadSpeed = downloadSpeed
	sr.session.SpeedVariance = variance
	sr.mu.Unlock()
}

func (sr *SessionRunner) Stop() {
	sr.mu.Lock()
	if !sr.started {
		sr.mu.Unlock()
		return
	}
	if sr.running {
		sr.running = false
		close(sr.stopCh)
	}
	done := sr.done
	sr.mu.Unlock()
	<-done
}

func (sr *SessionRunner) IsRunning() bool {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.running
}

func (sr *SessionRunner) Done() <-chan struct{} {
	return sr.done
}

func (sr *SessionRunner) runLoop() {
	defer close(sr.done)

	// Initial announce with "started" event
	sr.applyAllocatedSpeeds()
	sr.announce(protocol.EventStarted)
	if sr.checkStopConditions() {
		sr.mu.Lock()
		sr.running = false
		sr.mu.Unlock()
		sr.finish("completed", "Session auto-stopped for %s")
		return
	}

	interval := sr.nextInterval()

	timer := time.NewTimer(time.Duration(interval) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-sr.stopCh:
			sr.finish("stopped", "Session stopped for %s")
			return
		case <-timer.C:
			sr.mu.Lock()
			if !sr.running {
				sr.mu.Unlock()
				return
			}
			sr.mu.Unlock()

			// Simulate data transfer between announces
			sr.simulateTransfer(interval)

			// Check if download completed (send completed event only once)
			event := sr.nextEvent()

			sr.announce(event)

			// Check auto-stop conditions (ratio reached, limits hit)
			if sr.checkStopConditions() {
				sr.mu.Lock()
				sr.running = false
				sr.mu.Unlock()
				sr.finish("completed", "Session auto-stopped for %s")
				return
			}

			interval = sr.nextInterval()
			timer.Reset(time.Duration(interval) * time.Second)
		}
	}
}

func (sr *SessionRunner) simulateTransfer(intervalSec int) {
	sr.applyAllocatedSpeeds()
	sr.mu.Lock()
	defer sr.mu.Unlock()
	uploadSpeed := sr.session.UploadSpeed
	downloadSpeed := sr.session.DownloadSpeed
	variance := sr.session.SpeedVariance

	// Simulate upload
	if uploadSpeed > 0 {
		// Check max upload limit
		if sr.session.MaxUpload > 0 && sr.session.Uploaded >= sr.session.MaxUpload {
			sr.log("info", "Max upload limit reached (%d bytes)", sr.session.MaxUpload)
		} else {
			uploadDelta := sr.randomizer.SimulateUploadDelta(uploadSpeed, variance, intervalSec)
			// Cap at max upload limit
			if sr.session.MaxUpload > 0 {
				remaining := sr.session.MaxUpload - sr.session.Uploaded
				if uploadDelta > remaining {
					uploadDelta = remaining
				}
			}
			sr.session.Uploaded += uploadDelta
		}
	}

	// Simulate download
	if downloadSpeed > 0 && sr.session.Left > 0 {
		// Check max download limit
		if sr.session.MaxDownload > 0 && sr.session.Downloaded >= sr.session.MaxDownload {
			sr.log("info", "Max download limit reached (%d bytes)", sr.session.MaxDownload)
		} else {
			downloadDelta := sr.randomizer.SimulateDownloadDelta(downloadSpeed, variance, intervalSec, sr.session.Left)
			// Cap at max download limit
			if sr.session.MaxDownload > 0 {
				remaining := sr.session.MaxDownload - sr.session.Downloaded
				if downloadDelta > remaining {
					downloadDelta = remaining
				}
			}
			sr.session.Downloaded += downloadDelta
			sr.session.Left -= downloadDelta
			if sr.session.Left < 0 {
				sr.session.Left = 0
			}
		}
	}
}

// checkStopConditions checks if the session should auto-stop based on ratio/limits.
// Returns true if the session should stop.
func (sr *SessionRunner) checkStopConditions() bool {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	// Check stop-at-ratio
	if sr.session.StopAtRatio {
		// For ratio calculation, use Downloaded if available, otherwise use torrent size
		// (for seed-only scenarios where Downloaded stays at 0)
		base := sr.session.Downloaded
		if base == 0 {
			base = sr.torrent.Size
		}
		if base > 0 {
			currentRatio := float64(sr.session.Uploaded) / float64(base)
			if currentRatio >= sr.session.TargetRatio {
				sr.log("info", "Target ratio %.2f reached (current: %.2f) — stopping", sr.session.TargetRatio, currentRatio)
				return true
			}
		}
	}

	// Each configured transfer limit is an independent stop condition.
	uploadDone := sr.session.MaxUpload > 0 && sr.session.Uploaded >= sr.session.MaxUpload
	downloadDone := sr.session.MaxDownload > 0 && sr.session.Downloaded >= sr.session.MaxDownload
	if uploadDone || downloadDone {
		sr.log("info", "Transfer limit reached - stopping")
		return true
	}

	return false
}

func (sr *SessionRunner) announce(event protocol.AnnounceEvent) {
	if len(sr.trackers) == 0 {
		sr.log("warn", "No trackers available for %s", sr.torrent.Name)
		return
	}

	sr.mu.Lock()
	session := cloneSession(sr.session)
	sr.mu.Unlock()

	trackerURL := sr.trackers[0]
	sr.log("info", "Announcing to %s (event=%s, up=%d, down=%d, left=%d)",
		protocol.RedactTrackerURL(trackerURL), event, session.Uploaded, session.Downloaded, session.Left)

	req := protocol.AnnounceRequest{
		TrackerURL: trackerURL,
		InfoHash:   sr.infoHash,
		PeerID:     session.PeerID,
		Port:       session.Port,
		Uploaded:   session.Uploaded,
		Downloaded: session.Downloaded,
		Left:       session.Left,
		Event:      event,
		Compact:    sr.profile.SupportsCompact,
		NumWant:    sr.profile.NumWantDefault,
		Key:        session.Key,
		Profile:    sr.profile,
	}

	resp, err := sr.tracker.Announce(req)
	if err != nil {
		lastError := protocol.RedactSensitiveText(err.Error())
		sr.mu.Lock()
		sr.session.LastError = lastError
		snapshot := cloneSession(sr.session)
		sr.mu.Unlock()
		sr.log("error", "Announce failed: %s", lastError)
		sr.update(snapshot)
		return
	}

	if resp.FailureReason != "" {
		lastError := protocol.RedactSensitiveText(resp.FailureReason)
		sr.mu.Lock()
		sr.session.LastError = lastError
		snapshot := cloneSession(sr.session)
		sr.mu.Unlock()
		sr.log("error", "Tracker returned failure: %s", lastError)
		sr.update(snapshot)
		return
	}

	// Update session from tracker response
	now := time.Now()
	sr.mu.Lock()
	sr.session.LastAnnounce = &now
	sr.session.Seeders = resp.Complete
	sr.session.Leechers = resp.Incomplete
	sr.session.LastError = ""

	if resp.Interval > 0 {
		sr.session.AnnounceInterval = clampInterval(resp.Interval)
	}

	interval := clampInterval(sr.session.AnnounceInterval)
	sr.session.AnnounceInterval = interval
	next := now.Add(time.Duration(interval) * time.Second)
	sr.session.NextAnnounce = &next
	snapshot := cloneSession(sr.session)
	sr.mu.Unlock()

	sr.log("info", "Announce OK: interval=%d, seeders=%d, leechers=%d, peers=%d",
		resp.Interval, resp.Complete, resp.Incomplete, len(resp.Peers))

	sr.update(snapshot)
}

func (sr *SessionRunner) nextInterval() int {
	sr.mu.Lock()
	interval := clampInterval(sr.session.AnnounceInterval)
	sr.session.AnnounceInterval = interval
	sr.mu.Unlock()
	return clampInterval(sr.randomizer.RandomizeInterval(interval))
}

func (sr *SessionRunner) nextEvent() protocol.AnnounceEvent {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	if sr.session.Left <= 0 && sr.session.Downloaded > 0 && !sr.completedSent {
		sr.completedSent = true
		return protocol.EventCompleted
	}
	return protocol.EventNone
}

func (sr *SessionRunner) finish(status, message string) {
	sr.mu.Lock()
	sr.session.UploadSpeed = 0
	sr.session.DownloadSpeed = 0
	sr.session.SpeedVariance = 0
	sr.mu.Unlock()
	sr.announce(protocol.EventStopped)

	sr.mu.Lock()
	sr.session.Status = status
	snapshot := cloneSession(sr.session)
	sr.mu.Unlock()
	sr.update(snapshot)
	sr.log("info", message, sr.torrent.Name)
}

func (sr *SessionRunner) updateAllocation(upload, download, variance int64) *storage.Session {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.session.UploadSpeed = upload
	sr.session.DownloadSpeed = download
	sr.session.SpeedVariance = variance
	return cloneSession(sr.session)
}

func (sr *SessionRunner) updateConfiguration(session *storage.Session) *storage.Session {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.session.UploadSpeed = session.UploadSpeed
	sr.session.DownloadSpeed = session.DownloadSpeed
	sr.session.SpeedVariance = session.SpeedVariance
	sr.session.TargetRatio = session.TargetRatio
	sr.session.StopAtRatio = session.StopAtRatio
	sr.session.MaxUpload = session.MaxUpload
	sr.session.MaxDownload = session.MaxDownload
	sr.session.NetworkInterface = session.NetworkInterface
	return cloneSession(sr.session)
}

func (sr *SessionRunner) update(session *storage.Session) {
	if sr.onUpdate != nil {
		sr.onUpdate(session)
	}
}

func cloneSession(session *storage.Session) *storage.Session {
	clone := *session
	if session.LastAnnounce != nil {
		value := *session.LastAnnounce
		clone.LastAnnounce = &value
	}
	if session.NextAnnounce != nil {
		value := *session.NextAnnounce
		clone.NextAnnounce = &value
	}
	return &clone
}

func clampInterval(interval int) int {
	const defaultInterval = 1800
	const minimumInterval = 60
	maxInterval := int64(math.MaxInt64 / int64(time.Second))
	if interval <= 0 {
		return defaultInterval
	}
	if interval < minimumInterval {
		return minimumInterval
	}
	if int64(interval) > maxInterval {
		return int(maxInterval)
	}
	return interval
}

func (sr *SessionRunner) log(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if sr.onLog != nil {
		sr.onLog(level, msg)
	} else {
		log.Printf("[%s] %s", level, msg)
	}
}
