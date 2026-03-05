package engine

import (
	"fmt"
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

type Manager struct {
	db       *storage.Database
	runners  map[string]*SessionRunner
	mu       sync.RWMutex
	logs     []LogEntry
	logMu    sync.RWMutex
	maxLogs  int
	logSubs  map[chan LogEntry]struct{}
	subMu    sync.RWMutex
}

func NewManager(db *storage.Database) *Manager {
	return &Manager{
		db:      db,
		runners: make(map[string]*SessionRunner),
		maxLogs: 1000,
		logSubs: make(map[chan LogEntry]struct{}),
	}
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

	runner, err := NewSessionRunner(session, torrent,
		func(s *storage.Session) {
			m.db.UpdateSession(s)
		},
		func(level, msg string) {
			m.addLog(sessionID, level, msg)
		},
	)
	if err != nil {
		m.mu.Unlock()
		return fmt.Errorf("create runner: %w", err)
	}

	m.runners[sessionID] = runner
	m.mu.Unlock()

	return runner.Start()
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
