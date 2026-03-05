package storage

import (
	"time"
)

type Torrent struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	InfoHash    string    `json:"info_hash"`
	Size        int64     `json:"size"`
	Trackers    string    `json:"trackers"` // JSON array
	Comment     string    `json:"comment"`
	FilePath    string    `json:"file_path"`
	CreatedAt   time.Time `json:"created_at"`
}

type Session struct {
	ID              string    `json:"id"`
	TorrentID       string    `json:"torrent_id"`
	Status          string    `json:"status"` // running, stopped, paused, completed, error
	ClientProfile   string    `json:"client_profile"`
	PeerID          string    `json:"peer_id"`
	Port            int       `json:"port"`
	Key             string    `json:"key"`
	Uploaded        int64     `json:"uploaded"`
	Downloaded      int64     `json:"downloaded"`
	Left            int64     `json:"left"`
	UploadSpeed     int64     `json:"upload_speed"`       // bytes/sec base speed
	DownloadSpeed   int64     `json:"download_speed"`     // bytes/sec base speed
	SpeedVariance   int64     `json:"speed_variance"`     // bytes/sec ± random variance
	TargetRatio     float64   `json:"target_ratio"`
	StopAtRatio     bool      `json:"stop_at_ratio"`      // stop when target ratio reached
	MaxUpload       int64     `json:"max_upload"`         // max total upload bytes (0 = unlimited)
	MaxDownload     int64     `json:"max_download"`       // max total download bytes (0 = unlimited)
	NetworkInterface string   `json:"network_interface"`  // bind to specific interface (empty = default)
	AnnounceInterval int     `json:"announce_interval"`
	Seeders         int       `json:"seeders"`
	Leechers        int       `json:"leechers"`
	LastAnnounce    *time.Time `json:"last_announce"`
	NextAnnounce    *time.Time `json:"next_announce"`
	LastError       string    `json:"last_error"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type GlobalStats struct {
	TotalTorrents    int   `json:"total_torrents"`
	ActiveSessions   int   `json:"active_sessions"`
	TotalUploaded    int64 `json:"total_uploaded"`
	TotalDownloaded  int64 `json:"total_downloaded"`
}

type SessionWithTorrent struct {
	Session
	TorrentName   string `json:"torrent_name"`
	TorrentSize   int64  `json:"torrent_size"`
	InfoHash      string `json:"info_hash"`
}

type TorrentWithStats struct {
	Torrent
	TotalUploaded   int64 `json:"total_uploaded"`
	TotalDownloaded int64 `json:"total_downloaded"`
	SessionCount    int   `json:"session_count"`
	ActiveSessions  int   `json:"active_sessions"`
}

type Settings struct {
	ClientProfile    string  `json:"client_profile"`
	UploadSpeed      int64   `json:"upload_speed"`
	DownloadSpeed    int64   `json:"download_speed"`
	SpeedVariance    int64   `json:"speed_variance"`
	TargetRatio      float64 `json:"target_ratio"`
	StopAtRatio      bool    `json:"stop_at_ratio"`
	MaxUpload        int64   `json:"max_upload"`
	MaxDownload      int64   `json:"max_download"`
	NetworkInterface string  `json:"network_interface"`
}

func DefaultSettings() *Settings {
	return &Settings{
		ClientProfile:    "qbittorrent-4.6.2",
		UploadSpeed:      102400,
		DownloadSpeed:    0,
		SpeedVariance:    10240,
		TargetRatio:      2.0,
		StopAtRatio:      false,
		MaxUpload:        0,
		MaxDownload:      0,
		NetworkInterface: "",
	}
}
