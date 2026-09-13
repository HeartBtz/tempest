package engine

import (
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HeartBtz/tempest/internal/config"
	"github.com/HeartBtz/tempest/internal/storage"
)

func TestManagerEnforcesMaximumConcurrentSessions(t *testing.T) {
	cfg := config.Default()
	cfg.Engine.MaxConcurrentSessions = 1
	cfg.Engine.EnableRandomization = false
	config.Set(cfg)

	db, err := storage.NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	now := time.Now()
	for _, id := range []string{"one", "two"} {
		torrentID := "torrent-" + id
		if err := db.CreateTorrent(&storage.Torrent{ID: torrentID, Name: id, InfoHash: "hash-" + id, Trackers: "[]", CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
		if err := db.CreateSession(&storage.Session{ID: id, TorrentID: torrentID, ClientProfile: "qbittorrent-4.6.2", Port: 6881, AnnounceInterval: 1800, CreatedAt: now, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}

	manager := NewManager(db)
	if err := manager.StartSession("one"); err != nil {
		t.Fatal(err)
	}
	defer manager.StopAll()
	if err := manager.StartSession("two"); err == nil || !strings.Contains(err.Error(), "maximum concurrent sessions") {
		t.Fatalf("second StartSession error = %v", err)
	}
}

func TestSessionRunnerUsesConfiguredRandomization(t *testing.T) {
	cfg := config.Default()
	cfg.Engine.EnableRandomization = false
	config.Set(cfg)
	session := &storage.Session{ClientProfile: "qbittorrent-4.6.2"}
	torrent := &storage.Torrent{Trackers: "[]"}
	runner, err := NewSessionRunner(session, torrent, func(*storage.Session) {}, func(string, string) {}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := runner.randomizer.RandomizeInterval(1); got != 1 {
		t.Fatalf("disabled randomization returned %d, want 1", got)
	}

	cfg.Engine.EnableRandomization = true
	runner, err = NewSessionRunner(session, torrent, func(*storage.Session) {}, func(string, string) {}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := runner.randomizer.RandomizeInterval(1); got != 60 {
		t.Fatalf("enabled randomization returned %d, want minimum 60", got)
	}
}

func TestSessionRunnerStopWaitsAndSnapshotsAreImmutable(t *testing.T) {
	cfg := config.Default()
	cfg.Engine.EnableRandomization = false
	config.Set(cfg)

	stoppedUpdateStarted := make(chan struct{})
	releaseStoppedUpdate := make(chan struct{})
	var mu sync.Mutex
	var updates []*storage.Session
	runner, err := NewSessionRunner(
		&storage.Session{ClientProfile: "qbittorrent-4.6.2", AnnounceInterval: 1800},
		&storage.Torrent{Name: "test", Trackers: "[]"},
		func(session *storage.Session) {
			mu.Lock()
			updates = append(updates, session)
			mu.Unlock()
			if session.Status == "stopped" {
				close(stoppedUpdateStarted)
				<-releaseStoppedUpdate
			}
		},
		func(string, string) {},
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := runner.Start(); err != nil {
		t.Fatal(err)
	}

	stopReturned := make(chan struct{})
	go func() {
		runner.Stop()
		close(stopReturned)
	}()
	select {
	case <-stoppedUpdateStarted:
	case <-time.After(time.Second):
		t.Fatal("runner did not begin stopped persistence")
	}
	select {
	case <-stopReturned:
		t.Fatal("Stop returned before runner persistence completed")
	default:
	}
	close(releaseStoppedUpdate)
	select {
	case <-stopReturned:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after runner completion")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(updates) < 2 {
		t.Fatalf("received %d updates, want at least 2", len(updates))
	}
	if updates[0].Status != "running" {
		t.Fatalf("first callback snapshot changed to %q", updates[0].Status)
	}
}

func TestSessionRunnerStopsAtEitherConfiguredLimit(t *testing.T) {
	for _, test := range []struct {
		name    string
		session storage.Session
	}{
		{name: "upload", session: storage.Session{Uploaded: 10, MaxUpload: 10}},
		{name: "download", session: storage.Session{Downloaded: 20, MaxDownload: 20}},
	} {
		t.Run(test.name, func(t *testing.T) {
			runner, err := NewSessionRunner(&test.session, &storage.Torrent{Trackers: "[]"}, func(*storage.Session) {}, func(string, string) {}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if !runner.checkStopConditions() {
				t.Fatal("configured limit did not independently stop session")
			}
		})
	}
}

func TestClampIntervalBeforeDurationArithmetic(t *testing.T) {
	if got := clampInterval(-1); got != 1800 {
		t.Fatalf("negative interval clamped to %d, want 1800", got)
	}
	if got := clampInterval(1); got != 60 {
		t.Fatalf("short interval clamped to %d, want 60", got)
	}
	maxInterval := int(math.MaxInt64 / int64(time.Second))
	if int64(^uint(0)>>1) > int64(maxInterval) {
		if got := clampInterval(maxInterval + 1); got != maxInterval {
			t.Fatalf("large interval clamped to %d, want %d", got, maxInterval)
		}
	}
}

func TestManagerRemovesAutoStoppedRunner(t *testing.T) {
	cfg := config.Default()
	cfg.Engine.EnableRandomization = false
	config.Set(cfg)
	db := newTestDatabaseWithSession(t, &storage.Session{
		ID: "auto", TorrentID: "torrent-auto", ClientProfile: "qbittorrent-4.6.2",
		Uploaded: 10, MaxUpload: 10, AnnounceInterval: 1800,
	})
	manager := NewManager(db)
	if err := manager.StartSession("auto"); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(time.Second)
	for {
		manager.mu.RLock()
		_, exists := manager.runners["auto"]
		manager.mu.RUnlock()
		if !exists {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("auto-stopped runner was not removed")
		}
		time.Sleep(time.Millisecond)
	}
	session, err := db.GetSession("auto")
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != "completed" {
		t.Fatalf("status = %q, want completed", session.Status)
	}
}

func TestManagerUpdateSessionPreservesActiveRuntimeState(t *testing.T) {
	cfg := config.Default()
	cfg.Engine.EnableRandomization = false
	config.Set(cfg)
	db := newTestDatabaseWithSession(t, &storage.Session{
		ID: "active", TorrentID: "torrent-active", ClientProfile: "qbittorrent-4.6.2",
		Uploaded: 42, Left: 100, AnnounceInterval: 1800,
	})
	manager := NewManager(db)
	if err := manager.StartSession("active"); err != nil {
		t.Fatal(err)
	}
	defer manager.StopAll()

	stale := &storage.Session{ID: "active", Uploaded: 0, TargetRatio: 3, StopAtRatio: true, MaxUpload: 100}
	if err := manager.UpdateSession(stale); err != nil {
		t.Fatal(err)
	}
	updated, err := db.GetSession("active")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Uploaded != 42 {
		t.Fatalf("uploaded = %d, want runtime value 42", updated.Uploaded)
	}
	if updated.TargetRatio != 3 || !updated.StopAtRatio || updated.MaxUpload != 100 {
		t.Fatalf("configuration was not applied: %+v", updated)
	}
}

func TestManagerUpdateSessionMergesThroughStoppingRunner(t *testing.T) {
	cfg := config.Default()
	cfg.Engine.EnableRandomization = false
	config.Set(cfg)
	db := newTestDatabaseWithSession(t, &storage.Session{
		ID: "stopping", TorrentID: "torrent-stopping", ClientProfile: "qbittorrent-4.6.2",
		Uploaded: 42, Left: 100, TargetRatio: 1, AnnounceInterval: 1800,
	})
	manager := NewManager(db)
	session, err := db.GetSession("stopping")
	if err != nil {
		t.Fatal(err)
	}
	torrent, err := db.GetTorrent(session.TorrentID)
	if err != nil {
		t.Fatal(err)
	}
	finalUpdateStarted := make(chan struct{})
	releaseFinalUpdate := make(chan struct{})
	var runner *SessionRunner
	runner, err = NewSessionRunner(session, torrent, func(snapshot *storage.Session) {
		if snapshot.Status == "stopped" {
			close(finalUpdateStarted)
			<-releaseFinalUpdate
		}
		manager.persistRunnerUpdate(runner, snapshot)
	}, func(string, string) {}, nil)
	if err != nil {
		t.Fatal(err)
	}
	manager.mu.Lock()
	manager.runners[session.ID] = runner
	manager.mu.Unlock()
	if err := runner.Start(); err != nil {
		t.Fatal(err)
	}
	stopReturned := make(chan struct{})
	go func() {
		runner.Stop()
		close(stopReturned)
	}()
	select {
	case <-finalUpdateStarted:
	case <-time.After(time.Second):
		t.Fatal("runner did not reach final persistence")
	}

	stale := cloneSession(session)
	stale.TargetRatio = 3
	if err := manager.UpdateSession(stale); err != nil {
		t.Fatal(err)
	}
	merged, err := db.GetSession(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if merged.Status != "stopped" || merged.Uploaded != 42 || merged.TargetRatio != 3 {
		t.Fatalf("stopping update was not merged through runner: %+v", merged)
	}

	close(releaseFinalUpdate)
	select {
	case <-stopReturned:
	case <-time.After(time.Second):
		t.Fatal("runner did not finish after final persistence was released")
	}
	final, err := db.GetSession(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if final.Status != "stopped" || final.Uploaded != 42 || final.TargetRatio != 3 {
		t.Fatalf("final persistence restored stale state: %+v", final)
	}
}

func TestRunnerPersistenceAdoptsLegacyActiveEdits(t *testing.T) {
	cfg := config.Default()
	cfg.Engine.EnableRandomization = false
	config.Set(cfg)
	db := newTestDatabaseWithSession(t, &storage.Session{
		ID: "legacy", TorrentID: "torrent-legacy", ClientProfile: "qbittorrent-4.6.2",
		Uploaded: 42, TargetRatio: 1, Left: 100, AnnounceInterval: 1800,
	})
	manager := NewManager(db)
	if err := manager.StartSession("legacy"); err != nil {
		t.Fatal(err)
	}
	defer manager.StopAll()

	edited, err := db.GetSession("legacy")
	if err != nil {
		t.Fatal(err)
	}
	edited.TargetRatio = 5
	if err := db.UpdateSession(edited); err != nil {
		t.Fatal(err)
	}

	manager.mu.RLock()
	runner := manager.runners["legacy"]
	manager.mu.RUnlock()
	runner.mu.Lock()
	staleSnapshot := cloneSession(runner.session)
	runner.mu.Unlock()
	manager.persistRunnerUpdate(runner, staleSnapshot)

	updated, err := db.GetSession("legacy")
	if err != nil {
		t.Fatal(err)
	}
	if updated.TargetRatio != 5 {
		t.Fatalf("target ratio = %v, want legacy edit 5", updated.TargetRatio)
	}
	if updated.Uploaded != 42 {
		t.Fatalf("uploaded = %d, want runtime value 42", updated.Uploaded)
	}
}

func TestCategoryTargetRatioOverridesSessionTarget(t *testing.T) {
	cfg := config.Default()
	cfg.Engine.EnableRandomization = false
	config.Set(cfg)
	db := newTestDatabaseWithSession(t, &storage.Session{
		ID: "category", TorrentID: "torrent-category", ClientProfile: "qbittorrent-4.6.2",
		TargetRatio: 1, AnnounceInterval: 1800,
	})
	categoryID := "category-id"
	if err := db.CreateCategory(&storage.Category{ID: categoryID, Name: "category", TargetRatio: 4, CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := db.SetTorrentCategory("torrent-category", &categoryID); err != nil {
		t.Fatal(err)
	}
	manager := NewManager(db)
	if err := manager.StartSession("category"); err != nil {
		t.Fatal(err)
	}
	defer manager.StopAll()
	session, err := db.GetSession("category")
	if err != nil {
		t.Fatal(err)
	}
	if session.TargetRatio != 4 {
		t.Fatalf("target ratio = %v, want category ratio 4", session.TargetRatio)
	}
}

func newTestDatabaseWithSession(t *testing.T, session *storage.Session) *storage.Database {
	t.Helper()
	db, err := storage.NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	now := time.Now()
	if err := db.CreateTorrent(&storage.Torrent{
		ID: session.TorrentID, Name: session.ID, InfoHash: "", Trackers: "[]", CreatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	session.CreatedAt = now
	session.UpdatedAt = now
	if err := db.CreateSession(session); err != nil {
		t.Fatal(err)
	}
	return db
}
