import type { Torrent, Session, GlobalStats, LogEntry, ClientProfile, NetworkInterface, Settings, Category, UploadResponse } from '../types';

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

export function isUploadResponse(payload: unknown): payload is UploadResponse {
  if (typeof payload !== 'object' || payload === null) return false;
  const results = (payload as { results?: unknown }).results;
  return Array.isArray(results) && results.every(result => {
    if (typeof result !== 'object' || result === null) return false;
    const candidate = result as { filename?: unknown; status?: unknown; error?: unknown };
    return typeof candidate.filename === 'string' &&
      ['started', 'saved', 'skipped', 'error'].includes(String(candidate.status)) &&
      (candidate.error === undefined || typeof candidate.error === 'string');
  });
}

function uploadError(payload: unknown, fallback: string): string {
  if (typeof payload === 'object' && payload !== null &&
      typeof (payload as { error?: unknown }).error === 'string') {
    return (payload as { error: string }).error;
  }
  return fallback;
}

export const uploadTorrents = async (files: File[]): Promise<UploadResponse> => {
  const form = new FormData();
  for (const file of files) {
    form.append('torrent', file);
  }
  const res = await fetch(`${BASE}/torrents`, { method: 'POST', body: form });
  const payload: unknown = await res.json().catch(() => null);
  if (isUploadResponse(payload)) {
    return payload;
  }
  if (!res.ok) throw new Error(uploadError(payload, res.statusText));
  throw new Error('Upload returned an invalid response');
};

export const deleteTorrent = (id: string) =>
  request(`/torrents/${id}`, { method: 'DELETE' });

// Sessions
export const listSessions = () => request<Session[]>('/sessions');

export const createSession = (data: { torrent_id: string }) =>
  request<Session>('/sessions', { method: 'POST', body: JSON.stringify(data) });

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

// Settings
export const getSettings = () => request<Settings>('/settings');

export const updateSettings = (data: Settings) =>
  request<Settings>('/settings', { method: 'PUT', body: JSON.stringify(data) });

// Categories
export const listCategories = () => request<Category[]>('/categories');

export const createCategory = (data: Omit<Category, 'id' | 'created_at' | 'torrent_count'>) =>
  request<Category>('/categories', { method: 'POST', body: JSON.stringify(data) });

export const updateCategory = (id: string, data: Omit<Category, 'id' | 'created_at' | 'torrent_count'>) =>
  request<Category>(`/categories/${id}`, { method: 'PUT', body: JSON.stringify(data) });

export const deleteCategory = (id: string) =>
  request(`/categories/${id}`, { method: 'DELETE' });

export const assignTorrentsToCategory = (categoryId: string, torrentIds: string[]) =>
  request(`/categories/${categoryId}/assign`, {
    method: 'PUT',
    body: JSON.stringify({ torrent_ids: torrentIds }),
  });

export const unassignTorrents = (torrentIds: string[]) =>
  request('/categories/unassign', {
    method: 'PUT',
    body: JSON.stringify({ torrent_ids: torrentIds }),
  });
