export interface Torrent {
  id: string;
  name: string;
  info_hash: string;
  size: number;
  trackers: string;
  comment: string;
  file_path: string;
  created_at: string;
  total_uploaded: number;
  total_downloaded: number;
  session_count: number;
  active_sessions: number;
}

export interface Session {
  id: string;
  torrent_id: string;
  status: string;
  client_profile: string;
  peer_id: string;
  port: number;
  key: string;
  uploaded: number;
  downloaded: number;
  left: number;
  upload_speed: number;
  download_speed: number;
  speed_variance: number;
  target_ratio: number;
  stop_at_ratio: boolean;
  max_upload: number;
  max_download: number;
  network_interface: string;
  announce_interval: number;
  seeders: number;
  leechers: number;
  last_announce: string | null;
  next_announce: string | null;
  last_error: string;
  created_at: string;
  updated_at: string;
  torrent_name?: string;
  torrent_size?: number;
  info_hash?: string;
}

export interface GlobalStats {
  total_torrents: number;
  active_sessions: number;
  total_uploaded: number;
  total_downloaded: number;
}

export interface LogEntry {
  timestamp: string;
  session_id: string;
  level: string;
  message: string;
}

export interface ClientProfile {
  id: string;
  name: string;
  version: string;
  user_agent: string;
}

export interface NetworkInterface {
  name: string;
  addresses: string[];
  flags: string;
  mtu: number;
}

export interface Settings {
  client_profile: string;
  upload_speed: number;
  download_speed: number;
  speed_variance: number;
  target_ratio: number;
  stop_at_ratio: boolean;
  max_upload: number;
  max_download: number;
  network_interface: string;
}
