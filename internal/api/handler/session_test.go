package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HeartBtz/tempest/internal/config"
	"github.com/HeartBtz/tempest/internal/engine"
	"github.com/HeartBtz/tempest/internal/storage"
)

func TestCreateSessionUsesConfiguredAnnouncePort(t *testing.T) {
	cfg := config.Default()
	cfg.Engine.DefaultAnnouncePort = 51413
	config.Set(cfg)
	db, err := storage.NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.CreateTorrent(&storage.Torrent{ID: "torrent", Name: "test", InfoHash: "hash", Trackers: "[]", CreatedAt: time.Now()}); err != nil {
		t.Fatal(err)
	}

	h := NewSessionHandler(db, engine.NewManager(db))
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", strings.NewReader(`{"torrent_id":"torrent"}`))
	h.Create(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d; body=%s", rr.Code, rr.Body.String())
	}
	sessions, err := db.ListSessions()
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions = %v, err = %v", sessions, err)
	}
	if sessions[0].Port != 51413 {
		t.Fatalf("port = %d, want 51413", sessions[0].Port)
	}
}

func TestSessionUpdateRejectsInvalidNumericValues(t *testing.T) {
	db := newHandlerTestDatabase(t)
	h := NewSessionHandler(db, engine.NewManager(db))
	tests := []string{
		`{"upload_speed":-1}`,
		`{"speed_variance":1099511627777}`,
		`{"target_ratio":0}`,
		`{"target_ratio":1e309}`,
		`{"max_download":9007199254740992}`,
	}
	for _, body := range tests {
		rr := httptest.NewRecorder()
		h.Update(rr, httptest.NewRequest(http.MethodPut, "/api/sessions/session", strings.NewReader(body)), "session")
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400; response=%s", body, rr.Code, rr.Body.String())
		}
	}
}

func TestSessionUpdateUsesManagerForActiveSession(t *testing.T) {
	cfg := config.Default()
	cfg.Engine.EnableRandomization = false
	config.Set(cfg)
	db := newHandlerTestDatabase(t)
	manager := engine.NewManager(db)
	if err := manager.StartSession("session"); err != nil {
		t.Fatal(err)
	}
	defer manager.StopAll()

	h := NewSessionHandler(db, manager)
	rr := httptest.NewRecorder()
	h.Update(rr, httptest.NewRequest(http.MethodPut, "/api/sessions/session", strings.NewReader(`{"target_ratio":3}`)), "session")
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rr.Code, rr.Body.String())
	}
	updated, err := db.GetSession("session")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Uploaded != 42 || updated.TargetRatio != 3 {
		t.Fatalf("runtime state/configuration mismatch: %+v", updated)
	}
}

func newHandlerTestDatabase(t *testing.T) *storage.Database {
	t.Helper()
	db, err := storage.NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Now()
	if err := db.CreateTorrent(&storage.Torrent{ID: "torrent", Name: "test", InfoHash: "hash", Trackers: "[]", Size: 100, CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateSession(&storage.Session{ID: "session", TorrentID: "torrent", Status: "stopped", ClientProfile: "qbittorrent-4.6.2", Port: 6881, Uploaded: 42, Left: 100, TargetRatio: 1, AnnounceInterval: 1800, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	return db
}
