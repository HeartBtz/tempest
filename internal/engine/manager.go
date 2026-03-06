package engine

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tempest-bt/tempest/internal/storage"
)

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	SessionID string    `json:"session_id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

type speedAllocation struct {
	upload   int64
	download int64
	variance int64
}

type Manager struct {
	db            *storage.Database
	runners       map[string]*SessionRunner
	allocations   map[string]speedAllocation
	allocationKey string
	rng           *rand.Rand
	mu            sync.RWMutex
	logs          []LogEntry
	logMu         sync.RWMutex
	maxLogs       int
	logSubs       map[chan LogEntry]struct{}
	subMu         sync.RWMutex
}

func NewManager(db *storage.Database) *Manager {
	return &Manager{
		db:          db,
		runners:     make(map[string]*SessionRunner),
		allocations: make(map[string]speedAllocation),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		maxLogs:     1000,
		logSubs:     make(map[chan LogEntry]struct{}),
	}
}

func (m *Manager) activeSessionIDs() []string {
	ids := make([]string, 0, len(m.runners))
	for id, r := range m.runners {
		if r.IsRunning() {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func allocationStateKey(settings *storage.Settings, activeIDs []string) string {
	return fmt.Sprintf("%d:%d:%d:%s",
		settings.UploadSpeed,
		settings.DownloadSpeed,
		settings.SpeedVariance,
		strings.Join(activeIDs, ","),
	)
}

func distributeTotal(total int64, activeIDs []string, weights map[string]float64) map[string]int64 {
	allocation := make(map[string]int64, len(activeIDs))
	if len(activeIDs) == 0 || total <= 0 {
		for _, id := range activeIDs {
			allocation[id] = 0
		}
		return allocation
	}

	var totalWeight float64
	for _, id := range activeIDs {
		totalWeight += weights[id]
	}
	if totalWeight <= 0 {
		base := total / int64(len(activeIDs))
		remainder := total % int64(len(activeIDs))
		for i, id := range activeIDs {
			allocation[id] = base
			if int64(i) < remainder {
				allocation[id]++
			}
		}
		return allocation
	}

	type fractionalShare struct {
		id   string
		frac float64
	}

	remainder := total
	fractions := make([]fractionalShare, 0, len(activeIDs))
	for _, id := range activeIDs {
		exact := (float64(total) * weights[id]) / totalWeight
		whole := int64(exact)
		allocation[id] = whole
		remainder -= whole
		fractions = append(fractions, fractionalShare{
			id:   id,
			frac: exact - float64(whole),
		})
	}

	sort.SliceStable(fractions, func(i, j int) bool {
		if fractions[i].frac == fractions[j].frac {
			return fractions[i].id < fractions[j].id
		}
		return fractions[i].frac > fractions[j].frac
	})
	for i := int64(0); i < remainder; i++ {
		allocation[fractions[int(i)%len(fractions)].id]++
	}
	return allocation
}

func (m *Manager) rebalanceAllocations(settings *storage.Settings, activeIDs []string) {
	weights := make(map[string]float64, len(activeIDs))
	for _, id := range activeIDs {
		weights[id] = 0.9 + m.rng.Float64()*0.2
	}

	uploads := distributeTotal(settings.UploadSpeed, activeIDs, weights)
	downloads := distributeTotal(settings.DownloadSpeed, activeIDs, weights)
	variances := distributeTotal(settings.SpeedVariance, activeIDs, weights)

	m.allocations = make(map[string]speedAllocation, len(activeIDs))
	for _, id := range activeIDs {
		m.allocations[id] = speedAllocation{
			upload:   uploads[id],
			download: downloads[id],
			variance: variances[id],
		}
	}
}

func (m *Manager) syncRunnerAllocationsLocked(activeIDs []string) {
	activeSet := make(map[string]struct{}, len(activeIDs))
	for _, id := range activeIDs {
		activeSet[id] = struct{}{}
	}

	for id, runner := range m.runners {
		allocation, ok := m.allocations[id]
		if !ok {
			if _, isActive := activeSet[id]; !isActive {
				continue
			}
			allocation = speedAllocation{}
		}

		runner.session.UploadSpeed = allocation.upload
		runner.session.DownloadSpeed = allocation.download
		runner.session.SpeedVariance = allocation.variance
		_ = m.db.UpdateSession(runner.session)
	}
}

func (m *Manager) refreshRunnerAllocations() {
	settings, err := m.db.GetSettings()
	if err != nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	activeIDs := m.activeSessionIDs()
	if len(activeIDs) == 0 {
		m.allocations = make(map[string]speedAllocation)
		m.allocationKey = ""
		return
	}

	m.rebalanceAllocations(settings, activeIDs)
	m.allocationKey = allocationStateKey(settings, activeIDs)
	m.syncRunnerAllocationsLocked(activeIDs)
}

func (m *Manager) RefreshRunnerAllocations() {
	m.refreshRunnerAllocations()
}

func (m *Manager) getSpeedAllocation(sessionID string) (int64, int64, int64) {
	settings, err := m.db.GetSettings()
	if err != nil {
		return 0, 0, 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	activeIDs := m.activeSessionIDs()
	if len(activeIDs) == 0 {
		m.allocations = make(map[string]speedAllocation)
		m.allocationKey = ""
		return 0, 0, 0
	}

	key := allocationStateKey(settings, activeIDs)
	if key != m.allocationKey {
		m.rebalanceAllocations(settings, activeIDs)
		m.allocationKey = key
		m.syncRunnerAllocationsLocked(activeIDs)
	}

	allocation, ok := m.allocations[sessionID]
	if !ok {
		m.rebalanceAllocations(settings, activeIDs)
		m.allocationKey = key
		m.syncRunnerAllocationsLocked(activeIDs)
		allocation = m.allocations[sessionID]
	}

	return allocation.upload, allocation.download, allocation.variance
}

func (m *Manager) StartSession(sessionID string) error {
	m.mu.Lock()

	// Check if already running
	if runner, ok := m.runners[sessionID]; ok && runner.IsRunning() {
		m.mu.Unlock()
		return fmt.Errorf("session %s is already running", sessionID)
	}

	session, err := m.db.GetSession(sessionID)
	if err != nil {
		m.mu.Unlock()
		return fmt.Errorf("get session: %w", err)
	}

	torrent, err := m.db.GetTorrent(session.TorrentID)
	if err != nil {
		m.mu.Unlock()
		return fmt.Errorf("get torrent: %w", err)
	}

	// Speed allocator: shared across all active sessions with slight variation,
	// while preserving the configured global total.
	speedAllocator := func() (int64, int64, int64) {
		return m.getSpeedAllocation(sessionID)
	}

	runner, err := NewSessionRunner(session, torrent,
		func(s *storage.Session) {
			m.db.UpdateSession(s)
		},
		func(level, msg string) {
			m.addLog(sessionID, level, msg)
		},
		speedAllocator,
	)
	if err != nil {
		m.mu.Unlock()
		return fmt.Errorf("create runner: %w", err)
	}

	m.runners[sessionID] = runner
	m.mu.Unlock()

	if err := runner.Start(); err != nil {
		return err
	}
	m.refreshRunnerAllocations()
	return nil
}

func (m *Manager) StopSession(sessionID string) error {
	m.mu.Lock()
	runner, ok := m.runners[sessionID]
	m.mu.Unlock()

	if !ok || !runner.IsRunning() {
		// Update status in DB even if runner not found
		session, err := m.db.GetSession(sessionID)
		if err == nil {
			session.Status = "stopped"
			m.db.UpdateSession(session)
		}
		return nil
	}

	runner.Stop()
	m.refreshRunnerAllocations()
	return nil
}

func (m *Manager) StopAll() {
	m.mu.RLock()
	runners := make([]*SessionRunner, 0, len(m.runners))
	for _, r := range m.runners {
		runners = append(runners, r)
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	for _, r := range runners {
		if r.IsRunning() {
			wg.Add(1)
			go func(runner *SessionRunner) {
				defer wg.Done()
				runner.Stop()
			}(r)
		}
	}
	wg.Wait()
}

func (m *Manager) IsRunning(sessionID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if runner, ok := m.runners[sessionID]; ok {
		return runner.IsRunning()
	}
	return false
}

func (m *Manager) addLog(sessionID, level, msg string) {
	entry := LogEntry{
		Timestamp: time.Now(),
		SessionID: sessionID,
		Level:     level,
		Message:   msg,
	}

	m.logMu.Lock()
	m.logs = append(m.logs, entry)
	if len(m.logs) > m.maxLogs {
		m.logs = m.logs[len(m.logs)-m.maxLogs:]
	}
	m.logMu.Unlock()

	// Notify subscribers
	m.subMu.RLock()
	for ch := range m.logSubs {
		select {
		case ch <- entry:
		default:
		}
	}
	m.subMu.RUnlock()
}

func (m *Manager) GetLogs(limit int) []LogEntry {
	m.logMu.RLock()
	defer m.logMu.RUnlock()

	if limit <= 0 || limit > len(m.logs) {
		limit = len(m.logs)
	}

	start := len(m.logs) - limit
	result := make([]LogEntry, limit)
	copy(result, m.logs[start:])
	return result
}

func (m *Manager) SubscribeLogs() chan LogEntry {
	ch := make(chan LogEntry, 100)
	m.subMu.Lock()
	m.logSubs[ch] = struct{}{}
	m.subMu.Unlock()
	return ch
}

func (m *Manager) UnsubscribeLogs(ch chan LogEntry) {
	m.subMu.Lock()
	delete(m.logSubs, ch)
	m.subMu.Unlock()
	close(ch)
}
