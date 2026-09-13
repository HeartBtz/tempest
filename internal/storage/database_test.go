package storage

import (
	"database/sql"
	"errors"
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

func TestReconcileRunningSessionsAfterCrash(t *testing.T) {
	db := testDatabaseWithSession(t, "running")
	count, err := db.ReconcileRunningSessions()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("reconciled = %d, want 1", count)
	}
	session, err := db.GetSession("session")
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != "stopped" || session.UploadSpeed != 0 || session.DownloadSpeed != 0 || session.SpeedVariance != 0 {
		t.Fatalf("session was not reconciled: %+v", session)
	}
}

func TestSaveSettingsIsTransactional(t *testing.T) {
	db, err := NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	original := DefaultSettings()
	if err := db.SaveSettings(original); err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`CREATE TRIGGER reject_target_ratio BEFORE INSERT ON settings
		WHEN NEW.key = 'target_ratio' BEGIN SELECT RAISE(FAIL, 'rejected'); END`); err != nil {
		t.Fatal(err)
	}
	changed := *original
	changed.ClientProfile = "changed"
	changed.UploadSpeed = 999
	if err := db.SaveSettings(&changed); err == nil {
		t.Fatal("SaveSettings unexpectedly succeeded")
	}
	stored, err := db.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if stored.ClientProfile != original.ClientProfile || stored.UploadSpeed != original.UploadSpeed {
		t.Fatalf("partial settings were committed: %+v", stored)
	}
}

func TestGetSettingsReturnsDatabaseAndParseErrors(t *testing.T) {
	db, err := NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.db.Exec(`INSERT INTO settings (key, value) VALUES ('upload_speed', 'invalid')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetSettings(); err == nil {
		t.Fatal("GetSettings ignored malformed persisted value")
	}
	if _, err := db.db.Exec(`DELETE FROM settings`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetSettings(); err == nil {
		t.Fatal("GetSettings ignored closed database")
	}
}

func TestMutationsReportNotFound(t *testing.T) {
	db, err := NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.DeleteTorrent("missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteTorrent error = %v, want sql.ErrNoRows", err)
	}
	if err := db.DeleteSession("missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteSession error = %v, want sql.ErrNoRows", err)
	}
	if err := db.DeleteCategory("missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("DeleteCategory error = %v, want sql.ErrNoRows", err)
	}
}

func testDatabaseWithSession(t *testing.T, status string) *Database {
	t.Helper()
	db, err := NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	now := time.Now()
	if err := db.CreateTorrent(&Torrent{ID: "torrent", Name: "test", InfoHash: "hash", Trackers: "[]", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateSession(&Session{ID: "session", TorrentID: "torrent", Status: status, UploadSpeed: 1, DownloadSpeed: 2, SpeedVariance: 3, CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	return db
}
