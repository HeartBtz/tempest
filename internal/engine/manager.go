package engine

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/HeartBtz/tempest/internal/config"
	"github.com/HeartBtz/tempest/internal/storage"
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
	maxSessions   int
}

func NewManager(db *storage.Database) *Manager {
	return &Manager{
		db:          db,
		runners:     make(map[string]*SessionRunner),
		allocations: make(map[string]speedAllocation),
		rng:         rand.New(rand.NewSource(time.Now().UnixNano())),
		maxLogs:     1000,
		logSubs:     make(map[chan LogEntry]struct{}),
		maxSessions: config.Get().Engine.MaxConcurrentSessions,
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

// categorySettings holds the speed budget for a group of sessions sharing the same category.
type categoryBudget struct {
	uploadSpeed   int64
	downloadSpeed int64
	speedVariance int64
	sessionIDs    []string
}

// buildCategoryGroups groups active session IDs by their torrent category.
// Sessions with no category (or DB error) fall into the "" bucket and use global settings.
func (m *Manager) buildCategoryGroups(globalSettings *storage.Settings, activeIDs []string) map[string]*categoryBudget {
	groups := make(map[string]*categoryBudget)

	for _, id := range activeIDs {
		cat, err := m.db.GetCategoryForSession(id)
		var key string
		var budget *categoryBudget

		if err != nil || cat == nil {
			// Uncategorized: use global settings
			key = ""
			if _, ok := groups[key]; !ok {
				groups[key] = &categoryBudget{
					uploadSpeed:   globalSettings.UploadSpeed,
					downloadSpeed: globalSettings.DownloadSpeed,
					speedVariance: globalSettings.SpeedVariance,
				}
			}
		} else {
			key = cat.ID
			if _, ok := groups[key]; !ok {
				groups[key] = &categoryBudget{
					uploadSpeed:   cat.UploadSpeed,
					downloadSpeed: cat.DownloadSpeed,
					speedVariance: cat.SpeedVariance,
				}
			}
		}

		budget = groups[key]
		budget.sessionIDs = append(budget.sessionIDs, id)
	}

	return groups
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
	groups := m.buildCategoryGroups(settings, activeIDs)

	m.allocations = make(map[string]speedAllocation, len(activeIDs))

	for _, group := range groups {
		if len(group.sessionIDs) == 0 {
			continue
		}

		weights := make(map[string]float64, len(group.sessionIDs))
		for _, id := range group.sessionIDs {
			weights[id] = 0.9 + m.rng.Float64()*0.2
		}

		uploads := distributeTotal(group.uploadSpeed, group.sessionIDs, weights)
		downloads := distributeTotal(group.downloadSpeed, group.sessionIDs, weights)
		variances := distributeTotal(group.speedVariance, group.sessionIDs, weights)

		for _, id := range group.sessionIDs {
			m.allocations[id] = speedAllocation{
				upload:   uploads[id],
				download: downloads[id],
				variance: variances[id],
			}
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

		snapshot := runner.updateAllocation(allocation.upload, allocation.download, allocation.variance)
		_ = m.db.UpdateSession(snapshot)
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

	// A runner remains registered from creation until its goroutine exits, so
	// map membership also reserves a concurrency slot while Start is in flight.
	if _, ok := m.runners[sessionID]; ok {
		m.mu.Unlock()
		return fmt.Errorf("session %s is already running or stopping", sessionID)
	}
	if m.maxSessions > 0 && len(m.runners) >= m.maxSessions {
		m.mu.Unlock()
		return fmt.Errorf("maximum concurrent sessions reached (%d)", m.maxSessions)
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
	if category, categoryErr := m.db.GetCategoryForSession(sessionID); categoryErr == nil && category.TargetRatio > 0 {
		session.TargetRatio = category.TargetRatio
	}

	// Speed allocator: shared across all active sessions with slight variation,
	// while preserving the configured global total.
	speedAllocator := func() (int64, int64, int64) {
		return m.getSpeedAllocation(sessionID)
	}

	var runner *SessionRunner
	runner, err = NewSessionRunner(session, torrent,
		func(s *storage.Session) {
			m.persistRunnerUpdate(runner, s)
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
	if err := runner.Start(); err != nil {
		delete(m.runners, sessionID)
		m.mu.Unlock()
		return err
	}
	m.mu.Unlock()
	go m.removeRunnerWhenDone(sessionID, runner)
	m.refreshRunnerAllocations()
	return nil
}

func (m *Manager) persistRunnerUpdate(runner *SessionRunner, snapshot *storage.Session) {
	current, err := m.db.GetSession(snapshot.ID)
	if err == nil && !current.UpdatedAt.Equal(snapshot.UpdatedAt) && sessionConfigurationChanged(current, snapshot) {
		// Adopt edits made through legacy direct database callers rather than
		// replacing them with a stale runner snapshot.
		snapshot = runner.updateConfiguration(current)
	}
	_ = m.db.UpdateSession(snapshot)
}

func sessionConfigurationChanged(a, b *storage.Session) bool {
	return a.UploadSpeed != b.UploadSpeed ||
		a.DownloadSpeed != b.DownloadSpeed ||
		a.SpeedVariance != b.SpeedVariance ||
		a.TargetRatio != b.TargetRatio ||
		a.StopAtRatio != b.StopAtRatio ||
		a.MaxUpload != b.MaxUpload ||
		a.MaxDownload != b.MaxDownload ||
		a.NetworkInterface != b.NetworkInterface
}

func (m *Manager) removeRunnerWhenDone(sessionID string, runner *SessionRunner) {
	<-runner.Done()
	m.mu.Lock()
	if m.runners[sessionID] == runner {
		delete(m.runners, sessionID)
		delete(m.allocations, sessionID)
		m.allocationKey = ""
	}
	m.mu.Unlock()
	m.refreshRunnerAllocations()
}

// UpdateSession safely applies editable session configuration. Runtime-owned
// counters and tracker state are preserved when the session is active.
func (m *Manager) UpdateSession(session *storage.Session) error {
	m.mu.RLock()
	runner := m.runners[session.ID]
	m.mu.RUnlock()
	if runner == nil {
		return m.db.UpdateSession(cloneSession(session))
	}

	snapshot := runner.updateConfiguration(session)
	return m.db.UpdateSession(snapshot)
}

func (m *Manager) StopSession(sessionID string) error {
	m.mu.Lock()
	runner, ok := m.runners[sessionID]
	m.mu.Unlock()

	if !ok {
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
		wg.Add(1)
		go func(runner *SessionRunner) {
			defer wg.Done()
			runner.Stop()
		}(r)
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
