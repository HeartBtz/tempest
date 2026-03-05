import type { Torrent, Session, GlobalStats, LogEntry, ClientProfile, NetworkInterface } from '../types';

const BASE = '/api';

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
}

// Torrents
export const listTorrents = () => request<Torrent[]>('/torrents');

export const getTorrent = (id: string) => request<Torrent>(`/torrents/${id}`);

export const uploadTorrent = async (file: File): Promise<Torrent> => {
  const form = new FormData();
  form.append('torrent', file);
  const res = await fetch(`${BASE}/torrents`, { method: 'POST', body: form });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  return res.json();
};

export const deleteTorrent = (id: string) =>
  request(`/torrents/${id}`, { method: 'DELETE' });

// Sessions
export const listSessions = () => request<Session[]>('/sessions');

export const createSession = (data: {
  torrent_id: string;
  client_profile?: string;
  upload_speed?: number;
  download_speed?: number;
  speed_variance?: number;
  target_ratio?: number;
  stop_at_ratio?: boolean;
  max_upload?: number;
  max_download?: number;
  network_interface?: string;
  port?: number;
}) => request<Session>('/sessions', { method: 'POST', body: JSON.stringify(data) });

export const startSession = (id: string) =>
  request(`/sessions/${id}/start`, { method: 'POST' });

export const stopSession = (id: string) =>
  request(`/sessions/${id}/stop`, { method: 'POST' });

export const deleteSession = (id: string) =>
  request(`/sessions/${id}`, { method: 'DELETE' });

export const updateSession = (id: string, data: {
  upload_speed?: number;
  download_speed?: number;
  speed_variance?: number;
  target_ratio?: number;
  stop_at_ratio?: boolean;
  max_upload?: number;
  max_download?: number;
  network_interface?: string;
}) => request<Session>(`/sessions/${id}`, { method: 'PUT', body: JSON.stringify(data) });

// Stats
export const getStats = () => request<GlobalStats>('/stats');

// Logs
export const getLogs = (limit = 100) => request<LogEntry[]>(`/logs?limit=${limit}`);

// Profiles
export const listProfiles = () => request<ClientProfile[]>('/profiles');

// Network interfaces
export const listInterfaces = () => request<NetworkInterface[]>('/interfaces');
