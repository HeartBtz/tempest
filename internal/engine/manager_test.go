package engine

import (
	"strings"
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
