package engine

import (
	"encoding/json"
	"fmt"
	"log"
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
		session:        session,
		torrent:        torrent,
		profile:        profile,
		tracker:        tracker,
		randomizer:     client.NewRandomizer(config.Get().Engine.EnableRandomization),
		infoHash:       infoHash,
		trackers:       trackers,
		stopCh:         make(chan struct{}),
		onUpdate:       onUpdate,
		onLog:          onLog,
		speedAllocator: speedAllocator,
	}, nil
}

func (sr *SessionRunner) Start() error {
	sr.mu.Lock()
	if sr.running {
		sr.mu.Unlock()
		return fmt.Errorf("session already running")
	}
	sr.running = true
	sr.session.Status = "running"
	sr.mu.Unlock()

	// Generate peer ID and key if not set
	if sr.session.PeerID == "" {
		sr.session.PeerID = sr.profile.GeneratePeerID()
	}
	if sr.session.Key == "" {
		sr.session.Key = sr.profile.GenerateKey()
	}

	sr.applyAllocatedSpeeds()
	sr.onUpdate(sr.session)
	sr.log("info", "Session started for %s", sr.torrent.Name)

	go sr.runLoop()
	return nil
}

func (sr *SessionRunner) applyAllocatedSpeeds() {
	if sr.speedAllocator == nil {
		return
	}
	uploadSpeed, downloadSpeed, variance := sr.speedAllocator()
	sr.session.UploadSpeed = uploadSpeed
	sr.session.DownloadSpeed = downloadSpeed
	sr.session.SpeedVariance = variance
}

func (sr *SessionRunner) Stop() {
	sr.mu.Lock()
	if !sr.running {
		sr.mu.Unlock()
		return
	}
	sr.running = false
	sr.mu.Unlock()

	close(sr.stopCh)

	// Send stopped event
	sr.session.UploadSpeed = 0
	sr.session.DownloadSpeed = 0
	sr.session.SpeedVariance = 0
	sr.announce(protocol.EventStopped)

	sr.session.Status = "stopped"
	sr.onUpdate(sr.session)
	sr.log("info", "Session stopped for %s", sr.torrent.Name)
}

func (sr *SessionRunner) IsRunning() bool {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.running
}

func (sr *SessionRunner) runLoop() {
	// Initial announce with "started" event
	sr.applyAllocatedSpeeds()
	sr.announce(protocol.EventStarted)

	interval := sr.session.AnnounceInterval
	if interval <= 0 {
		interval = 1800
	}
	interval = sr.randomizer.RandomizeInterval(interval)

	timer := time.NewTimer(time.Duration(interval) * time.Second)
	defer timer.Stop()

	for {
		select {
		case <-sr.stopCh:
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
			event := protocol.EventNone
			if sr.session.Left <= 0 && sr.session.Downloaded > 0 && !sr.completedSent {
				event = protocol.EventCompleted
				sr.completedSent = true
			}

			sr.announce(event)

			// Check auto-stop conditions (ratio reached, limits hit)
			if sr.checkStopConditions() {
				sr.mu.Lock()
				sr.running = false
				sr.mu.Unlock()
				sr.session.UploadSpeed = 0
				sr.session.DownloadSpeed = 0
				sr.session.SpeedVariance = 0
				sr.announce(protocol.EventStopped)
				sr.session.Status = "completed"
				sr.onUpdate(sr.session)
				sr.log("info", "Session auto-stopped for %s", sr.torrent.Name)
				return
			}

			// Use interval from tracker response, or default
			interval = sr.session.AnnounceInterval
			if interval <= 0 {
				interval = 1800
			}
			interval = sr.randomizer.RandomizeInterval(interval)
			timer.Reset(time.Duration(interval) * time.Second)
		}
	}
}

func (sr *SessionRunner) simulateTransfer(intervalSec int) {
	sr.applyAllocatedSpeeds()
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

	// Check if both upload and download hit their max limits
	uploadDone := sr.session.MaxUpload > 0 && sr.session.Uploaded >= sr.session.MaxUpload
	downloadDone := sr.session.MaxDownload > 0 && sr.session.Downloaded >= sr.session.MaxDownload
	if sr.session.MaxUpload > 0 && sr.session.MaxDownload > 0 && uploadDone && downloadDone {
		sr.log("info", "Both upload and download limits reached — stopping")
		return true
	}

	return false
}

func (sr *SessionRunner) announce(event protocol.AnnounceEvent) {
	if len(sr.trackers) == 0 {
		sr.log("warn", "No trackers available for %s", sr.torrent.Name)
		return
	}

	trackerURL := sr.trackers[0]
	sr.log("info", "Announcing to %s (event=%s, up=%d, down=%d, left=%d)",
		protocol.RedactTrackerURL(trackerURL), event, sr.session.Uploaded, sr.session.Downloaded, sr.session.Left)

	req := protocol.AnnounceRequest{
		TrackerURL: trackerURL,
		InfoHash:   sr.infoHash,
		PeerID:     sr.session.PeerID,
		Port:       sr.session.Port,
		Uploaded:   sr.session.Uploaded,
		Downloaded: sr.session.Downloaded,
		Left:       sr.session.Left,
		Event:      event,
		Compact:    sr.profile.SupportsCompact,
		NumWant:    sr.profile.NumWantDefault,
		Key:        sr.session.Key,
		Profile:    sr.profile,
	}

	resp, err := sr.tracker.Announce(req)
	if err != nil {
		sr.session.LastError = protocol.RedactSensitiveText(err.Error())
		sr.log("error", "Announce failed: %s", sr.session.LastError)
		sr.onUpdate(sr.session)
		return
	}

	if resp.FailureReason != "" {
		sr.session.LastError = protocol.RedactSensitiveText(resp.FailureReason)
		sr.log("error", "Tracker returned failure: %s", sr.session.LastError)
		sr.onUpdate(sr.session)
		return
	}

	// Update session from tracker response
	now := time.Now()
	sr.session.LastAnnounce = &now
	sr.session.Seeders = resp.Complete
	sr.session.Leechers = resp.Incomplete
	sr.session.LastError = ""

	if resp.Interval > 0 {
		sr.session.AnnounceInterval = resp.Interval
	}

	next := now.Add(time.Duration(sr.session.AnnounceInterval) * time.Second)
	sr.session.NextAnnounce = &next

	sr.log("info", "Announce OK: interval=%d, seeders=%d, leechers=%d, peers=%d",
		resp.Interval, resp.Complete, resp.Incomplete, len(resp.Peers))

	sr.onUpdate(sr.session)
}

func (sr *SessionRunner) log(level, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if sr.onLog != nil {
		sr.onLog(level, msg)
	} else {
		log.Printf("[%s] %s", level, msg)
	}
}
