package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Database struct {
	db *sql.DB
	mu sync.RWMutex
}

func NewDatabase(dbPath string) (*Database, error) {
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
	}

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	database := &Database{db: db}
	if err := database.migrate(); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return database, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS torrents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			info_hash TEXT NOT NULL UNIQUE,
			size INTEGER NOT NULL DEFAULT 0,
			trackers TEXT NOT NULL DEFAULT '[]',
			comment TEXT DEFAULT '',
			file_path TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			torrent_id TEXT NOT NULL REFERENCES torrents(id) ON DELETE CASCADE,
			status TEXT NOT NULL DEFAULT 'stopped',
			client_profile TEXT NOT NULL DEFAULT 'qbittorrent-4.6.2',
			peer_id TEXT NOT NULL DEFAULT '',
			port INTEGER NOT NULL DEFAULT 6881,
			key TEXT NOT NULL DEFAULT '',
			uploaded INTEGER NOT NULL DEFAULT 0,
			downloaded INTEGER NOT NULL DEFAULT 0,
			left_bytes INTEGER NOT NULL DEFAULT 0,
			upload_speed INTEGER NOT NULL DEFAULT 0,
			download_speed INTEGER NOT NULL DEFAULT 0,
			speed_variance INTEGER NOT NULL DEFAULT 0,
			target_ratio REAL NOT NULL DEFAULT 1.0,
			stop_at_ratio INTEGER NOT NULL DEFAULT 0,
			max_upload INTEGER NOT NULL DEFAULT 0,
			max_download INTEGER NOT NULL DEFAULT 0,
			network_interface TEXT NOT NULL DEFAULT '',
			announce_interval INTEGER NOT NULL DEFAULT 1800,
			seeders INTEGER NOT NULL DEFAULT 0,
			leechers INTEGER NOT NULL DEFAULT 0,
			last_announce DATETIME,
			next_announce DATETIME,
			last_error TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_torrent_id ON sessions(torrent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status)`,
	}

	for _, m := range migrations {
		if _, err := d.db.Exec(m); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	return nil
}

// Torrent operations

func (d *Database) CreateTorrent(t *Torrent) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(
		`INSERT INTO torrents (id, name, info_hash, size, trackers, comment, file_path, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.InfoHash, t.Size, t.Trackers, t.Comment, t.FilePath, t.CreatedAt,
	)
	return err
}

func (d *Database) GetTorrent(id string) (*Torrent, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	t := &Torrent{}
	err := d.db.QueryRow(
		`SELECT id, name, info_hash, size, trackers, comment, file_path, created_at
		 FROM torrents WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.InfoHash, &t.Size, &t.Trackers, &t.Comment, &t.FilePath, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (d *Database) ListTorrents() ([]Torrent, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(
		`SELECT id, name, info_hash, size, trackers, comment, file_path, created_at
		 FROM torrents ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var torrents []Torrent
	for rows.Next() {
		var t Torrent
		if err := rows.Scan(&t.ID, &t.Name, &t.InfoHash, &t.Size, &t.Trackers, &t.Comment, &t.FilePath, &t.CreatedAt); err != nil {
			return nil, err
		}
		torrents = append(torrents, t)
	}
	return torrents, rows.Err()
}

func (d *Database) DeleteTorrent(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`DELETE FROM torrents WHERE id = ?`, id)
	return err
}

// Session operations

func (d *Database) CreateSession(s *Session) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(
		`INSERT INTO sessions (id, torrent_id, status, client_profile, peer_id, port, key,
		 uploaded, downloaded, left_bytes, upload_speed, download_speed, speed_variance,
		 target_ratio, stop_at_ratio, max_upload, max_download, network_interface,
		 announce_interval, seeders, leechers, last_error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.TorrentID, s.Status, s.ClientProfile, s.PeerID, s.Port, s.Key,
		s.Uploaded, s.Downloaded, s.Left, s.UploadSpeed, s.DownloadSpeed, s.SpeedVariance,
		s.TargetRatio, s.StopAtRatio, s.MaxUpload, s.MaxDownload, s.NetworkInterface,
		s.AnnounceInterval, s.Seeders, s.Leechers, s.LastError, s.CreatedAt, s.UpdatedAt,
	)
	return err
}

func (d *Database) GetSession(id string) (*Session, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	s := &Session{}
	err := d.db.QueryRow(
		`SELECT id, torrent_id, status, client_profile, peer_id, port, key,
		 uploaded, downloaded, left_bytes, upload_speed, download_speed, speed_variance,
		 target_ratio, stop_at_ratio, max_upload, max_download, network_interface,
		 announce_interval, seeders, leechers, last_announce, next_announce, last_error,
		 created_at, updated_at FROM sessions WHERE id = ?`, id,
	).Scan(&s.ID, &s.TorrentID, &s.Status, &s.ClientProfile, &s.PeerID, &s.Port, &s.Key,
		&s.Uploaded, &s.Downloaded, &s.Left, &s.UploadSpeed, &s.DownloadSpeed, &s.SpeedVariance,
		&s.TargetRatio, &s.StopAtRatio, &s.MaxUpload, &s.MaxDownload, &s.NetworkInterface,
		&s.AnnounceInterval, &s.Seeders, &s.Leechers, &s.LastAnnounce, &s.NextAnnounce,
		&s.LastError, &s.CreatedAt, &s.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (d *Database) ListSessions() ([]Session, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(
		`SELECT id, torrent_id, status, client_profile, peer_id, port, key,
		 uploaded, downloaded, left_bytes, upload_speed, download_speed, speed_variance,
		 target_ratio, stop_at_ratio, max_upload, max_download, network_interface,
		 announce_interval, seeders, leechers, last_announce, next_announce, last_error,
		 created_at, updated_at FROM sessions ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.TorrentID, &s.Status, &s.ClientProfile, &s.PeerID, &s.Port, &s.Key,
			&s.Uploaded, &s.Downloaded, &s.Left, &s.UploadSpeed, &s.DownloadSpeed, &s.SpeedVariance,
			&s.TargetRatio, &s.StopAtRatio, &s.MaxUpload, &s.MaxDownload, &s.NetworkInterface,
			&s.AnnounceInterval, &s.Seeders, &s.Leechers, &s.LastAnnounce, &s.NextAnnounce,
			&s.LastError, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (d *Database) ListSessionsWithTorrents() ([]SessionWithTorrent, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(
		`SELECT s.id, s.torrent_id, s.status, s.client_profile, s.peer_id, s.port, s.key,
		 s.uploaded, s.downloaded, s.left_bytes, s.upload_speed, s.download_speed, s.speed_variance,
		 s.target_ratio, s.stop_at_ratio, s.max_upload, s.max_download, s.network_interface,
		 s.announce_interval, s.seeders, s.leechers, s.last_announce, s.next_announce, s.last_error,
		 s.created_at, s.updated_at, t.name, t.size, t.info_hash
		 FROM sessions s JOIN torrents t ON s.torrent_id = t.id
		 ORDER BY s.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []SessionWithTorrent
	for rows.Next() {
		var s SessionWithTorrent
		if err := rows.Scan(&s.ID, &s.TorrentID, &s.Status, &s.ClientProfile, &s.PeerID, &s.Port, &s.Key,
			&s.Uploaded, &s.Downloaded, &s.Left, &s.UploadSpeed, &s.DownloadSpeed, &s.SpeedVariance,
			&s.TargetRatio, &s.StopAtRatio, &s.MaxUpload, &s.MaxDownload, &s.NetworkInterface,
			&s.AnnounceInterval, &s.Seeders, &s.Leechers, &s.LastAnnounce, &s.NextAnnounce,
			&s.LastError, &s.CreatedAt, &s.UpdatedAt, &s.TorrentName, &s.TorrentSize, &s.InfoHash); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (d *Database) UpdateSession(s *Session) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	s.UpdatedAt = time.Now()
	_, err := d.db.Exec(
		`UPDATE sessions SET status = ?, uploaded = ?, downloaded = ?, left_bytes = ?,
		 upload_speed = ?, download_speed = ?, speed_variance = ?,
		 target_ratio = ?, stop_at_ratio = ?, max_upload = ?, max_download = ?,
		 network_interface = ?, announce_interval = ?,
		 seeders = ?, leechers = ?, last_announce = ?, next_announce = ?, last_error = ?,
		 updated_at = ? WHERE id = ?`,
		s.Status, s.Uploaded, s.Downloaded, s.Left,
		s.UploadSpeed, s.DownloadSpeed, s.SpeedVariance,
		s.TargetRatio, s.StopAtRatio, s.MaxUpload, s.MaxDownload,
		s.NetworkInterface, s.AnnounceInterval,
		s.Seeders, s.Leechers, s.LastAnnounce, s.NextAnnounce, s.LastError,
		s.UpdatedAt, s.ID,
	)
	return err
}

func (d *Database) DeleteSession(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

func (d *Database) GetGlobalStats() (*GlobalStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := &GlobalStats{}

	d.db.QueryRow(`SELECT COUNT(*) FROM torrents`).Scan(&stats.TotalTorrents)
	d.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE status = 'running'`).Scan(&stats.ActiveSessions)
	d.db.QueryRow(`SELECT COALESCE(SUM(uploaded), 0) FROM sessions`).Scan(&stats.TotalUploaded)
	d.db.QueryRow(`SELECT COALESCE(SUM(downloaded), 0) FROM sessions`).Scan(&stats.TotalDownloaded)

	return stats, nil
}

func (d *Database) ListTorrentsWithStats() ([]TorrentWithStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(
		`SELECT t.id, t.name, t.info_hash, t.size, t.trackers, t.comment, t.file_path, t.created_at,
		 COALESCE(SUM(s.uploaded), 0) AS total_uploaded,
		 COALESCE(SUM(s.downloaded), 0) AS total_downloaded,
		 COUNT(s.id) AS session_count,
		 COALESCE(SUM(CASE WHEN s.status = 'running' THEN 1 ELSE 0 END), 0) AS active_sessions
		 FROM torrents t
		 LEFT JOIN sessions s ON t.id = s.torrent_id
		 GROUP BY t.id
		 ORDER BY t.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var torrents []TorrentWithStats
	for rows.Next() {
		var t TorrentWithStats
		if err := rows.Scan(&t.ID, &t.Name, &t.InfoHash, &t.Size, &t.Trackers, &t.Comment, &t.FilePath, &t.CreatedAt,
			&t.TotalUploaded, &t.TotalDownloaded, &t.SessionCount, &t.ActiveSessions); err != nil {
			return nil, err
		}
		torrents = append(torrents, t)
	}
	return torrents, rows.Err()
}
