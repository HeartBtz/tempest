package storage

import (
	"database/sql"
	"testing"
	"time"
)

func TestForeignKeysCascadeTorrentSessions(t *testing.T) {
	db, err := NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	now := time.Now()
	if err := db.CreateTorrent(&Torrent{ID: "torrent", Name: "test", InfoHash: "hash", Trackers: "[]", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateSession(&Session{ID: "session", TorrentID: "torrent", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteTorrent("torrent"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetSession("session"); err != sql.ErrNoRows {
		t.Fatalf("GetSession error = %v, want sql.ErrNoRows", err)
	}
}
