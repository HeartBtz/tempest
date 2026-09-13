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
