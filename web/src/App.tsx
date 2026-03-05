import { useState, useEffect, useCallback } from 'react'
import type { Session, Torrent, GlobalStats, LogEntry, ClientProfile, NetworkInterface } from './types'
import * as api from './api/client'
import { StatsBar } from './components/StatsBar'
import { TorrentUpload } from './components/TorrentUpload'
import { SessionList } from './components/SessionList'
import { LogViewer } from './components/LogViewer'
import { CreateSessionModal } from './components/CreateSessionModal'

const styles = `
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
    background: #0f0f23;
    color: #e0e0e0;
    min-height: 100vh;
  }
  .app { max-width: 1400px; margin: 0 auto; padding: 20px; }
  .header {
    display: flex; align-items: center; justify-content: space-between;
    padding: 20px 0; border-bottom: 1px solid #1e1e3a;
    margin-bottom: 24px;
  }
  .header h1 { font-size: 28px; color: #ffd700; }
  .header h1 span { color: #888; font-size: 14px; font-weight: normal; margin-left: 8px; }
  .grid { display: grid; grid-template-columns: 1fr; gap: 20px; }
  .card {
    background: #1a1a2e;
    border: 1px solid #2a2a4a;
    border-radius: 12px;
    padding: 20px;
  }
  .card h2 {
    font-size: 16px; color: #aaa; text-transform: uppercase;
    letter-spacing: 1px; margin-bottom: 16px;
    display: flex; align-items: center; gap: 8px;
  }
  .btn {
    padding: 8px 16px; border-radius: 8px; border: none;
    cursor: pointer; font-size: 13px; font-weight: 600;
    transition: all 0.2s;
  }
  .btn-primary { background: #ffd700; color: #0f0f23; }
  .btn-primary:hover { background: #ffed4a; }
  .btn-success { background: #22c55e; color: white; }
  .btn-success:hover { background: #16a34a; }
  .btn-danger { background: #ef4444; color: white; }
  .btn-danger:hover { background: #dc2626; }
  .btn-ghost { background: transparent; color: #aaa; border: 1px solid #333; }
  .btn-ghost:hover { background: #2a2a4a; color: white; }
  .btn-sm { padding: 4px 10px; font-size: 12px; }
  .stat-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(160px, 1fr)); gap: 16px; }
  .stat-card {
    background: #16162a; border: 1px solid #2a2a4a;
    border-radius: 8px; padding: 16px; text-align: center;
  }
  .stat-card .value { font-size: 24px; font-weight: 700; color: #ffd700; }
  .stat-card .label { font-size: 12px; color: #888; margin-top: 4px; }
  table { width: 100%; border-collapse: collapse; }
  th { text-align: left; padding: 10px 12px; color: #888; font-size: 12px;
       text-transform: uppercase; border-bottom: 1px solid #2a2a4a; }
  td { padding: 12px; border-bottom: 1px solid #1e1e3a; font-size: 13px; }
  tr:hover { background: #16162a; }
  .status {
    display: inline-block; padding: 2px 8px; border-radius: 12px;
    font-size: 11px; font-weight: 600; text-transform: uppercase;
  }
  .status-running { background: #22c55e22; color: #22c55e; }
  .status-stopped { background: #ef444422; color: #ef4444; }
  .status-completed { background: #3b82f622; color: #3b82f6; }
  .status-error { background: #f9731622; color: #f97316; }
  .upload { position: relative; }
  .upload-zone {
    border: 2px dashed #2a2a4a; border-radius: 12px;
    padding: 32px; text-align: center; cursor: pointer;
    transition: all 0.2s;
  }
  .upload-zone:hover { border-color: #ffd700; background: #1a1a2e; }
  .upload-zone input { display: none; }
  .log-container {
    max-height: 300px; overflow-y: auto;
    font-family: 'Fira Code', 'Cascadia Code', monospace;
    font-size: 12px; background: #0a0a1a;
    border-radius: 8px; padding: 12px;
  }
  .log-entry { padding: 2px 0; display: flex; gap: 8px; }
  .log-time { color: #555; min-width: 80px; }
  .log-level-info { color: #3b82f6; }
  .log-level-warn { color: #f59e0b; }
  .log-level-error { color: #ef4444; }
  .modal-overlay {
    position: fixed; top: 0; left: 0; right: 0; bottom: 0;
    background: rgba(0,0,0,0.7); display: flex;
    align-items: center; justify-content: center; z-index: 100;
  }
  .modal {
    background: #1a1a2e; border: 1px solid #2a2a4a;
    border-radius: 12px; padding: 24px; width: 480px; max-width: 90vw;
  }
  .modal h3 { margin-bottom: 20px; color: #ffd700; }
  .form-group { margin-bottom: 16px; }
  .form-group label { display: block; font-size: 12px; color: #888; margin-bottom: 6px; }
  .form-group input, .form-group select {
    width: 100%; padding: 8px 12px; border-radius: 8px;
    border: 1px solid #2a2a4a; background: #0f0f23;
    color: #e0e0e0; font-size: 14px;
  }
  .form-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 20px; }
  .tabs { display: flex; gap: 4px; margin-bottom: 20px; }
  .tab {
    padding: 8px 16px; border-radius: 8px 8px 0 0; cursor: pointer;
    background: transparent; color: #888; border: none; font-size: 14px;
  }
  .tab.active { background: #1a1a2e; color: #ffd700; }
`

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatSpeed(bytesPerSec: number): string {
  return formatBytes(bytesPerSec) + '/s'
}

export default function App() {
  const [sessions, setSessions] = useState<Session[]>([])
  const [torrents, setTorrents] = useState<Torrent[]>([])
  const [stats, setStats] = useState<GlobalStats>({ total_torrents: 0, active_sessions: 0, total_uploaded: 0, total_downloaded: 0 })
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [profiles, setProfiles] = useState<ClientProfile[]>([])
  const [interfaces, setInterfaces] = useState<NetworkInterface[]>([])
  const [showCreateSession, setShowCreateSession] = useState(false)
  const [activeTab, setActiveTab] = useState<'sessions' | 'torrents' | 'logs'>('sessions')
  const [error, setError] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const [s, t, st, l, p, ifaces] = await Promise.all([
        api.listSessions(),
        api.listTorrents(),
        api.getStats(),
        api.getLogs(200),
        api.listProfiles(),
        api.listInterfaces(),
      ])
      setSessions(s)
      setTorrents(t)
      setStats(st)
      setLogs(l)
      setProfiles(p)
      setInterfaces(ifaces)
      setError(null)
    } catch (e: any) {
      setError(e.message)
    }
  }, [])

  useEffect(() => {
    refresh()
    const interval = setInterval(refresh, 5000)
    return () => clearInterval(interval)
  }, [refresh])

  const handleUpload = async (file: File) => {
    try {
      await api.uploadTorrent(file)
      await refresh()
    } catch (e: any) {
      setError(e.message)
    }
  }

  const handleStartSession = async (id: string) => {
    try {
      await api.startSession(id)
      await refresh()
    } catch (e: any) {
      setError(e.message)
    }
  }

  const handleStopSession = async (id: string) => {
    try {
      await api.stopSession(id)
      await refresh()
    } catch (e: any) {
      setError(e.message)
    }
  }

  const handleDeleteSession = async (id: string) => {
    try {
      await api.deleteSession(id)
      await refresh()
    } catch (e: any) {
      setError(e.message)
    }
  }

  const handleDeleteTorrent = async (id: string) => {
    try {
      await api.deleteTorrent(id)
      await refresh()
    } catch (e: any) {
      setError(e.message)
    }
  }

  const handleCreateSession = async (data: {
    torrent_id: string;
    client_profile: string;
    upload_speed: number;
    download_speed: number;
    speed_variance: number;
    target_ratio: number;
    stop_at_ratio: boolean;
    max_upload: number;
    max_download: number;
    network_interface: string;
  }) => {
    try {
      await api.createSession(data)
      setShowCreateSession(false)
      await refresh()
    } catch (e: any) {
      setError(e.message)
    }
  }

  return (
    <>
      <style>{styles}</style>
      <div className="app">
        <header className="header">
          <h1>⚡ Tempest <span>v0.1.0 — BitTorrent Tracker Simulator</span></h1>
          <button className="btn btn-primary" onClick={refresh}>↻ Refresh</button>
        </header>

        {error && (
          <div style={{ background: '#ef444422', border: '1px solid #ef4444', borderRadius: 8, padding: 12, marginBottom: 16, color: '#ef4444' }}>
            {error}
            <button onClick={() => setError(null)} style={{ float: 'right', background: 'none', border: 'none', color: '#ef4444', cursor: 'pointer' }}>✕</button>
          </div>
        )}

        <StatsBar stats={stats} formatBytes={formatBytes} />

        <div className="tabs">
          <button className={`tab ${activeTab === 'sessions' ? 'active' : ''}`} onClick={() => setActiveTab('sessions')}>
            📡 Sessions ({sessions.length})
          </button>
          <button className={`tab ${activeTab === 'torrents' ? 'active' : ''}`} onClick={() => setActiveTab('torrents')}>
            📦 Torrents ({torrents.length})
          </button>
          <button className={`tab ${activeTab === 'logs' ? 'active' : ''}`} onClick={() => setActiveTab('logs')}>
            📋 Logs
          </button>
        </div>

        {activeTab === 'sessions' && (
          <div className="card">
            <h2>
              📡 Active Sessions
              <button className="btn btn-primary btn-sm" onClick={() => setShowCreateSession(true)} style={{ marginLeft: 'auto' }}>
                + New Session
              </button>
            </h2>
            <SessionList
              sessions={sessions}
              onStart={handleStartSession}
              onStop={handleStopSession}
              onDelete={handleDeleteSession}
              formatBytes={formatBytes}
              formatSpeed={formatSpeed}
            />
          </div>
        )}

        {activeTab === 'torrents' && (
          <div className="card">
            <h2>📦 Torrents</h2>
            <TorrentUpload onUpload={handleUpload} />
            <table style={{ marginTop: 16 }}>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Size</th>
                  <th>Uploaded</th>
                  <th>Downloaded</th>
                  <th>Ratio</th>
                  <th>Sessions</th>
                  <th>Trackers</th>
                  <th>Info Hash</th>
                  <th>Added</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                {torrents.map(t => {
                  const ratio = t.total_downloaded > 0
                    ? (t.total_uploaded / t.total_downloaded).toFixed(2)
                    : t.total_uploaded > 0 ? '∞' : '—'
                  let trackerCount = 0
                  try { trackerCount = JSON.parse(t.trackers || '[]').length } catch {}
                  const added = new Date(t.created_at)
                  const ago = Math.floor((Date.now() - added.getTime()) / 60000)
                  const agoStr = ago < 60 ? `${ago}m ago` : ago < 1440 ? `${Math.floor(ago / 60)}h ago` : `${Math.floor(ago / 1440)}d ago`
                  return (
                    <tr key={t.id}>
                      <td>
                        <div style={{ fontWeight: 600 }}>{t.name}</div>
                        {t.comment && <div style={{ fontSize: 11, color: '#666', marginTop: 2 }}>{t.comment}</div>}
                      </td>
                      <td>{formatBytes(t.size)}</td>
                      <td style={{ color: '#3b82f6' }}>{t.total_uploaded > 0 ? formatBytes(t.total_uploaded) : '—'}</td>
                      <td style={{ color: '#a855f7' }}>{t.total_downloaded > 0 ? formatBytes(t.total_downloaded) : '—'}</td>
                      <td style={{ fontWeight: 700, color: ratio === '—' ? '#555' : '#ffd700' }}>{ratio}</td>
                      <td>
                        {t.active_sessions > 0 ? (
                          <span style={{ color: '#22c55e' }}>{t.active_sessions} active</span>
                        ) : (
                          <span style={{ color: '#555' }}>{t.session_count || 0}</span>
                        )}
                        {t.session_count > 0 && t.active_sessions !== t.session_count && (
                          <span style={{ color: '#888', fontSize: 11 }}> / {t.session_count} total</span>
                        )}
                      </td>
                      <td style={{ fontSize: 12, color: '#888' }}>{trackerCount}</td>
                      <td style={{ fontFamily: 'monospace', fontSize: 11, color: '#888' }}>{t.info_hash.substring(0, 16)}…</td>
                      <td style={{ fontSize: 11, color: '#888' }}>{agoStr}</td>
                      <td>
                        <button className="btn btn-danger btn-sm" onClick={() => handleDeleteTorrent(t.id)}>Delete</button>
                      </td>
                    </tr>
                  )
                })}
                {torrents.length === 0 && (
                  <tr><td colSpan={10} style={{ textAlign: 'center', color: '#555', padding: 32 }}>No torrents added yet. Upload a .torrent file above.</td></tr>
                )}
              </tbody>
            </table>
          </div>
        )}

        {activeTab === 'logs' && (
          <div className="card">
            <h2>📋 Event Log</h2>
            <LogViewer logs={logs} />
          </div>
        )}

        {showCreateSession && (
          <CreateSessionModal
            torrents={torrents}
            profiles={profiles}
            interfaces={interfaces}
            onClose={() => setShowCreateSession(false)}
            onCreate={handleCreateSession}
          />
        )}
      </div>
    </>
  )
}

export { formatBytes, formatSpeed }
