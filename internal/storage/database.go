package storage

import (
	"database/sql"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
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

	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	database := &Database{db: db}
	if err := database.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	return database, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS categories (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			color TEXT NOT NULL DEFAULT '#6366f1',
			upload_speed INTEGER NOT NULL DEFAULT 0,
			download_speed INTEGER NOT NULL DEFAULT 0,
			speed_variance INTEGER NOT NULL DEFAULT 0,
			target_ratio REAL NOT NULL DEFAULT 2.0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS torrents (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			info_hash TEXT NOT NULL UNIQUE,
			size INTEGER NOT NULL DEFAULT 0,
			trackers TEXT NOT NULL DEFAULT '[]',
			comment TEXT DEFAULT '',
			file_path TEXT DEFAULT '',
			category_id TEXT DEFAULT NULL REFERENCES categories(id) ON DELETE SET NULL,
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
		`CREATE TABLE IF NOT EXISTS settings (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
	}

	for _, m := range migrations {
		if _, err := d.db.Exec(m); err != nil {
			return fmt.Errorf("execute migration: %w", err)
		}
	}

	// Idempotent ALTER TABLE for existing databases that predate category support
	if err := d.addColumnIfNotExists("torrents", "category_id",
		"TEXT DEFAULT NULL REFERENCES categories(id) ON DELETE SET NULL"); err != nil {
		return fmt.Errorf("alter torrents add category_id: %w", err)
	}

	return nil
}

// addColumnIfNotExists adds a column to a table only if it doesn't already exist.
func (d *Database) addColumnIfNotExists(table, column, definition string) error {
	rows, err := d.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt interface{}
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil // already exists
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = d.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	return err
}

// Torrent operations

func (d *Database) CreateTorrent(t *Torrent) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(
		`INSERT INTO torrents (id, name, info_hash, size, trackers, comment, file_path, category_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.InfoHash, t.Size, t.Trackers, t.Comment, t.FilePath, t.CategoryID, t.CreatedAt,
	)
	return requireAffected(result, err)
}

func (d *Database) GetTorrent(id string) (*Torrent, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	t := &Torrent{}
	err := d.db.QueryRow(
		`SELECT id, name, info_hash, size, trackers, comment, file_path, category_id, created_at
		 FROM torrents WHERE id = ?`, id,
	).Scan(&t.ID, &t.Name, &t.InfoHash, &t.Size, &t.Trackers, &t.Comment, &t.FilePath, &t.CategoryID, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (d *Database) ListTorrents() ([]Torrent, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(
		`SELECT id, name, info_hash, size, trackers, comment, file_path, category_id, created_at
		 FROM torrents ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var torrents []Torrent
	for rows.Next() {
		var t Torrent
		if err := rows.Scan(&t.ID, &t.Name, &t.InfoHash, &t.Size, &t.Trackers, &t.Comment, &t.FilePath, &t.CategoryID, &t.CreatedAt); err != nil {
			return nil, err
		}
		torrents = append(torrents, t)
	}
	return torrents, rows.Err()
}

func (d *Database) DeleteTorrent(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(`DELETE FROM torrents WHERE id = ?`, id)
	return requireAffected(result, err)
}

func (d *Database) SetTorrentCategory(torrentID string, categoryID *string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(`UPDATE torrents SET category_id = ? WHERE id = ?`, categoryID, torrentID)
	return requireAffected(result, err)
}

func (d *Database) AssignTorrentsToCategory(torrentIDs []string, categoryID *string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`UPDATE torrents SET category_id = ? WHERE id = ?`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for _, id := range torrentIDs {
		result, err := stmt.Exec(categoryID, id)
		if err := requireAffected(result, err); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// Session operations

func (d *Database) CreateSession(s *Session) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(
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
	return requireAffected(result, err)
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
		 s.created_at, s.updated_at, t.name, t.size, t.info_hash, t.category_id, c.name
		 FROM sessions s
		 JOIN torrents t ON s.torrent_id = t.id
		 LEFT JOIN categories c ON t.category_id = c.id
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
			&s.LastError, &s.CreatedAt, &s.UpdatedAt, &s.TorrentName, &s.TorrentSize, &s.InfoHash,
			&s.CategoryID, &s.CategoryName); err != nil {
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
	result, err := d.db.Exec(
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
	return requireAffected(result, err)
}

func (d *Database) DeleteSession(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return requireAffected(result, err)
}

// ReconcileRunningSessions clears statuses that can only be backed by an
// in-memory runner. They are stale after an unclean process exit.
func (d *Database) ReconcileRunningSessions() (int64, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(
		`UPDATE sessions SET status = 'stopped', upload_speed = 0, download_speed = 0,
		 speed_variance = 0, updated_at = ? WHERE status = 'running'`, time.Now(),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (d *Database) GetGlobalStats() (*GlobalStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := &GlobalStats{}

	queries := []struct {
		query string
		dest  any
	}{
		{`SELECT COUNT(*) FROM torrents`, &stats.TotalTorrents},
		{`SELECT COUNT(*) FROM sessions WHERE status = 'running'`, &stats.ActiveSessions},
		{`SELECT COALESCE(SUM(uploaded), 0) FROM sessions`, &stats.TotalUploaded},
		{`SELECT COALESCE(SUM(downloaded), 0) FROM sessions`, &stats.TotalDownloaded},
	}
	for _, query := range queries {
		if err := d.db.QueryRow(query.query).Scan(query.dest); err != nil {
			return nil, err
		}
	}

	return stats, nil
}

func (d *Database) ListTorrentsWithStats() ([]TorrentWithStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(
		`SELECT t.id, t.name, t.info_hash, t.size, t.trackers, t.comment, t.file_path, t.category_id, t.created_at,
		 COALESCE(SUM(s.uploaded), 0) AS total_uploaded,
		 COALESCE(SUM(s.downloaded), 0) AS total_downloaded,
		 COUNT(s.id) AS session_count,
		 COALESCE(SUM(CASE WHEN s.status = 'running' THEN 1 ELSE 0 END), 0) AS active_sessions,
		 c.name AS category_name
		 FROM torrents t
		 LEFT JOIN sessions s ON t.id = s.torrent_id
		 LEFT JOIN categories c ON t.category_id = c.id
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
		if err := rows.Scan(&t.ID, &t.Name, &t.InfoHash, &t.Size, &t.Trackers, &t.Comment, &t.FilePath, &t.CategoryID, &t.CreatedAt,
			&t.TotalUploaded, &t.TotalDownloaded, &t.SessionCount, &t.ActiveSessions, &t.CategoryName); err != nil {
			return nil, err
		}
		torrents = append(torrents, t)
	}
	return torrents, rows.Err()
}

// Category operations

func (d *Database) CreateCategory(c *Category) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(
		`INSERT INTO categories (id, name, color, upload_speed, download_speed, speed_variance, target_ratio, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, c.Color, c.UploadSpeed, c.DownloadSpeed, c.SpeedVariance, c.TargetRatio, c.CreatedAt,
	)
	return requireAffected(result, err)
}

func (d *Database) GetCategory(id string) (*Category, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	c := &Category{}
	err := d.db.QueryRow(
		`SELECT id, name, color, upload_speed, download_speed, speed_variance, target_ratio, created_at
		 FROM categories WHERE id = ?`, id,
	).Scan(&c.ID, &c.Name, &c.Color, &c.UploadSpeed, &c.DownloadSpeed, &c.SpeedVariance, &c.TargetRatio, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (d *Database) ListCategories() ([]CategoryWithStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(
		`SELECT c.id, c.name, c.color, c.upload_speed, c.download_speed, c.speed_variance, c.target_ratio, c.created_at,
		 COUNT(t.id) AS torrent_count
		 FROM categories c
		 LEFT JOIN torrents t ON t.category_id = c.id
		 GROUP BY c.id
		 ORDER BY c.created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []CategoryWithStats
	for rows.Next() {
		var c CategoryWithStats
		if err := rows.Scan(&c.ID, &c.Name, &c.Color, &c.UploadSpeed, &c.DownloadSpeed, &c.SpeedVariance, &c.TargetRatio, &c.CreatedAt, &c.TorrentCount); err != nil {
			return nil, err
		}
		categories = append(categories, c)
	}
	return categories, rows.Err()
}

func (d *Database) UpdateCategory(c *Category) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(
		`UPDATE categories SET name = ?, color = ?, upload_speed = ?, download_speed = ?, speed_variance = ?, target_ratio = ?
		 WHERE id = ?`,
		c.Name, c.Color, c.UploadSpeed, c.DownloadSpeed, c.SpeedVariance, c.TargetRatio, c.ID,
	)
	return requireAffected(result, err)
}

func (d *Database) DeleteCategory(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Torrents in this category will have category_id set to NULL (ON DELETE SET NULL)
	result, err := d.db.Exec(`DELETE FROM categories WHERE id = ?`, id)
	return requireAffected(result, err)
}

// GetCategoryForTorrent returns the category for a given torrent (nil if uncategorized).
func (d *Database) GetCategoryForTorrent(torrentID string) (*Category, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	c := &Category{}
	err := d.db.QueryRow(
		`SELECT c.id, c.name, c.color, c.upload_speed, c.download_speed, c.speed_variance, c.target_ratio, c.created_at
		 FROM categories c JOIN torrents t ON t.category_id = c.id
		 WHERE t.id = ?`, torrentID,
	).Scan(&c.ID, &c.Name, &c.Color, &c.UploadSpeed, &c.DownloadSpeed, &c.SpeedVariance, &c.TargetRatio, &c.CreatedAt)
	if err != nil {
		return nil, err // sql.ErrNoRows if uncategorized
	}
	return c, nil
}

// GetCategoryForSession returns the category for the torrent attached to a session.
func (d *Database) GetCategoryForSession(sessionID string) (*Category, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	c := &Category{}
	err := d.db.QueryRow(
		`SELECT c.id, c.name, c.color, c.upload_speed, c.download_speed, c.speed_variance, c.target_ratio, c.created_at
		 FROM categories c
		 JOIN torrents t ON t.category_id = c.id
		 JOIN sessions s ON s.torrent_id = t.id
		 WHERE s.id = ?`, sessionID,
	).Scan(&c.ID, &c.Name, &c.Color, &c.UploadSpeed, &c.DownloadSpeed, &c.SpeedVariance, &c.TargetRatio, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Settings operations

func (d *Database) GetSettings() (*Settings, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	s := DefaultSettings()

	rows, err := d.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		switch k {
		case "client_profile":
			s.ClientProfile = v
		case "upload_speed":
			if s.UploadSpeed, err = strconv.ParseInt(v, 10, 64); err != nil {
				return nil, fmt.Errorf("parse setting %s: %w", k, err)
			}
		case "download_speed":
			if s.DownloadSpeed, err = strconv.ParseInt(v, 10, 64); err != nil {
				return nil, fmt.Errorf("parse setting %s: %w", k, err)
			}
		case "speed_variance":
			if s.SpeedVariance, err = strconv.ParseInt(v, 10, 64); err != nil {
				return nil, fmt.Errorf("parse setting %s: %w", k, err)
			}
		case "target_ratio":
			if s.TargetRatio, err = strconv.ParseFloat(v, 64); err != nil {
				return nil, fmt.Errorf("parse setting %s: %w", k, err)
			}
			if math.IsNaN(s.TargetRatio) || math.IsInf(s.TargetRatio, 0) {
				return nil, fmt.Errorf("parse setting %s: value must be finite", k)
			}
		case "stop_at_ratio":
			if v != "0" && v != "1" {
				return nil, fmt.Errorf("parse setting %s: invalid boolean %q", k, v)
			}
			s.StopAtRatio = v == "1"
		case "max_upload":
			if s.MaxUpload, err = strconv.ParseInt(v, 10, 64); err != nil {
				return nil, fmt.Errorf("parse setting %s: %w", k, err)
			}
		case "max_download":
			if s.MaxDownload, err = strconv.ParseInt(v, 10, 64); err != nil {
				return nil, fmt.Errorf("parse setting %s: %w", k, err)
			}
		case "network_interface":
			s.NetworkInterface = v
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return s, nil
}

func (d *Database) SaveSettings(s *Settings) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	stopAtRatio := "0"
	if s.StopAtRatio {
		stopAtRatio = "1"
	}

	pairs := []struct {
		key   string
		value string
	}{
		{"client_profile", s.ClientProfile},
		{"upload_speed", strconv.FormatInt(s.UploadSpeed, 10)},
		{"download_speed", strconv.FormatInt(s.DownloadSpeed, 10)},
		{"speed_variance", strconv.FormatInt(s.SpeedVariance, 10)},
		{"target_ratio", strconv.FormatFloat(s.TargetRatio, 'g', -1, 64)},
		{"stop_at_ratio", stopAtRatio},
		{"max_upload", strconv.FormatInt(s.MaxUpload, 10)},
		{"max_download", strconv.FormatInt(s.MaxDownload, 10)},
		{"network_interface", s.NetworkInterface},
	}

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, pair := range pairs {
		if _, err := tx.Exec(
			`INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`, pair.key, pair.value,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func requireAffected(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}
