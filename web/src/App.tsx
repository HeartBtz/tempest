import { useState, useEffect, useCallback } from 'react'
import type { Session, Torrent, GlobalStats, LogEntry, ClientProfile, NetworkInterface, Settings, Category } from './types'
import * as api from './api/client'
import { StatsBar } from './components/StatsBar'
import { TorrentUpload } from './components/TorrentUpload'
import { SessionList } from './components/SessionList'
import { LogViewer } from './components/LogViewer'
import { SettingsPanel } from './components/SettingsPanel'
import { CategoryPanel } from './components/CategoryPanel'
import { ConfirmModal } from './components/ConfirmModal'

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
  const [categories, setCategories] = useState<Category[]>([])
  const [stats, setStats] = useState<GlobalStats>({ total_torrents: 0, active_sessions: 0, total_uploaded: 0, total_downloaded: 0 })
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [profiles, setProfiles] = useState<ClientProfile[]>([])
  const [interfaces, setInterfaces] = useState<NetworkInterface[]>([])
  const [settings, setSettings] = useState<Settings | null>(null)
  const [activeTab, setActiveTab] = useState<'sessions' | 'torrents' | 'categories' | 'logs' | 'settings'>('sessions')
  const [error, setError] = useState<string | null>(null)

  // Torrent bulk selection
  const [selectedTorrentIds, setSelectedTorrentIds] = useState<Set<string>>(new Set())
  const [assignTarget, setAssignTarget] = useState<string | null>(null) // category id to assign to
  const [deleteTorrentTarget, setDeleteTorrentTarget] = useState<Torrent | null>(null)

  const refresh = useCallback(async () => {
    try {
      const [s, t, st, l, p, ifaces, cats] = await Promise.all([
        api.listSessions(),
        api.listTorrents(),
        api.getStats(),
        api.getLogs(200),
        api.listProfiles(),
        api.listInterfaces(),
        api.listCategories(),
      ])
      setSessions(s)
      setTorrents(t)
      setStats(st)
      setLogs(l)
      setProfiles(p)
      setInterfaces(ifaces)
      setCategories(cats)
      setError(null)
    } catch (e: any) {
      setError(e.message)
    }
  }, [])

  useEffect(() => {
    refresh()
    api.getSettings().then(setSettings).catch(() => {})
    const interval = setInterval(refresh, 5000)
    return () => clearInterval(interval)
  }, [refresh])

  const handleUpload = async (files: File[]) => {
    await api.uploadTorrents(files)
    await refresh()
  }

  const handleStartSession = async (id: string) => {
    try { await api.startSession(id); await refresh() } catch (e: any) { setError(e.message) }
  }

  const handleStopSession = async (id: string) => {
    try { await api.stopSession(id); await refresh() } catch (e: any) { setError(e.message) }
  }

  const handleDeleteSession = async (id: string) => {
    try { await api.deleteSession(id); await refresh() } catch (e: any) { setError(e.message) }
  }

  const handleDeleteTorrent = async (t: Torrent) => {
    setDeleteTorrentTarget(t)
  }

  const confirmDeleteTorrent = async () => {
    if (!deleteTorrentTarget) return
    try {
      await api.deleteTorrent(deleteTorrentTarget.id)
      setSelectedTorrentIds(prev => { const n = new Set(prev); n.delete(deleteTorrentTarget.id); return n })
      setDeleteTorrentTarget(null)
      await refresh()
    } catch (e: any) { setError(e.message) }
  }

  const handleSaveSettings = async (data: Settings) => {
    try { await api.updateSettings(data); setSettings(data) } catch (e: any) { setError(e.message) }
  }

  // Category handlers
  const handleCreateCategory = async (data: Omit<Category, 'id' | 'created_at' | 'torrent_count'>) => {
    try { await api.createCategory(data); await refresh() } catch (e: any) { setError(e.message) }
  }

  const handleUpdateCategory = async (id: string, data: Omit<Category, 'id' | 'created_at' | 'torrent_count'>) => {
    try { await api.updateCategory(id, data); await refresh() } catch (e: any) { setError(e.message) }
  }

  const handleDeleteCategory = async (id: string) => {
    try { await api.deleteCategory(id); await refresh() } catch (e: any) { setError(e.message) }
  }

  const handleAssignSelected = async (categoryId: string | null) => {
    const ids = Array.from(selectedTorrentIds)
    if (ids.length === 0) return
    try {
      if (categoryId) {
        await api.assignTorrentsToCategory(categoryId, ids)
      } else {
        await api.unassignTorrents(ids)
      }
      setSelectedTorrentIds(new Set())
      setAssignTarget(null)
      await refresh()
    } catch (e: any) { setError(e.message) }
  }

  const toggleTorrentSelect = (id: string) => {
    setSelectedTorrentIds(prev => {
      const n = new Set(prev)
      if (n.has(id)) n.delete(id)
      else n.add(id)
      return n
    })
  }

  const toggleSelectAll = () => {
    if (selectedTorrentIds.size === torrents.length) {
      setSelectedTorrentIds(new Set())
    } else {
      setSelectedTorrentIds(new Set(torrents.map(t => t.id)))
    }
  }

  return (
    <>
      <style>{styles}</style>
      <div className="app">
        <header className="header">
          <h1>⚡ Tempest <span>v0.1.0 — BitTorrent Announce Testing Dashboard</span></h1>
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
          <button className={`tab ${activeTab === 'categories' ? 'active' : ''}`} onClick={() => setActiveTab('categories')}>
            🏷️ Catégories ({categories.length})
          </button>
          <button className={`tab ${activeTab === 'logs' ? 'active' : ''}`} onClick={() => setActiveTab('logs')}>
            📋 Logs
          </button>
          <button className={`tab ${activeTab === 'settings' ? 'active' : ''}`} onClick={() => setActiveTab('settings')}>
            ⚙️ Settings
          </button>
        </div>

        {activeTab === 'sessions' && (
          <div className="card">
            <h2>📡 Sessions actives</h2>
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
            <h2>
              📦 Torrents
              {selectedTorrentIds.size > 0 && (
                <div style={{ marginLeft: 'auto', display: 'flex', gap: 8, alignItems: 'center' }}>
                  <span style={{ fontSize: 12, color: '#888' }}>{selectedTorrentIds.size} sélectionné(s)</span>
                  <select
                    style={{ padding: '4px 8px', borderRadius: 6, background: '#0f0f23', border: '1px solid #2a2a4a', color: '#e0e0e0', fontSize: 12 }}
                    value={assignTarget ?? ''}
                    onChange={e => setAssignTarget(e.target.value || null)}
                  >
                    <option value="">— Assigner à une catégorie —</option>
                    {categories.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}
                    <option value="__none__">Retirer de la catégorie</option>
                  </select>
                  <button
                    className="btn btn-primary btn-sm"
                    disabled={assignTarget === null}
                    onClick={() => handleAssignSelected(assignTarget === '__none__' ? null : assignTarget)}
                  >
                    Appliquer
                  </button>
                  <button className="btn btn-ghost btn-sm" onClick={() => setSelectedTorrentIds(new Set())}>Désélectionner</button>
                </div>
              )}
            </h2>
            <TorrentUpload onUpload={handleUpload} />
            <table style={{ marginTop: 16 }}>
              <thead>
                <tr>
                  <th style={{ width: 32 }}>
                    <input
                      type="checkbox"
                      checked={selectedTorrentIds.size === torrents.length && torrents.length > 0}
                      onChange={toggleSelectAll}
                      style={{ cursor: 'pointer' }}
                    />
                  </th>
                  <th>Nom</th>
                  <th>Catégorie</th>
                  <th>Taille</th>
                  <th>Uploadé</th>
                  <th>Téléchargé</th>
                  <th>Ratio</th>
                  <th>Sessions</th>
                  <th>Trackers</th>
                  <th>Info Hash</th>
                  <th>Ajouté</th>
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
                  const agoStr = ago < 60 ? `${ago}m` : ago < 1440 ? `${Math.floor(ago / 60)}h` : `${Math.floor(ago / 1440)}j`
                  const cat = categories.find(c => c.id === t.category_id)
                  const isSelected = selectedTorrentIds.has(t.id)
                  return (
                    <tr key={t.id} style={isSelected ? { background: '#1e1e3a' } : undefined}>
                      <td>
                        <input
                          type="checkbox"
                          checked={isSelected}
                          onChange={() => toggleTorrentSelect(t.id)}
                          style={{ cursor: 'pointer' }}
                        />
                      </td>
                      <td>
                        <div style={{ fontWeight: 600 }}>{t.name}</div>
                        {t.comment && <div style={{ fontSize: 11, color: '#666', marginTop: 2 }}>{t.comment}</div>}
                      </td>
                      <td>
                        {cat ? (
                          <span style={{
                            display: 'inline-flex', alignItems: 'center', gap: 5,
                            padding: '2px 8px', borderRadius: 12, fontSize: 11, fontWeight: 600,
                            background: cat.color + '22', color: cat.color, border: `1px solid ${cat.color}44`
                          }}>
                            <span style={{ width: 7, height: 7, borderRadius: '50%', background: cat.color }} />
                            {cat.name}
                          </span>
                        ) : (
                          <span style={{ color: '#444', fontSize: 11 }}>—</span>
                        )}
                      </td>
                      <td>{formatBytes(t.size)}</td>
                      <td style={{ color: '#3b82f6' }}>{t.total_uploaded > 0 ? formatBytes(t.total_uploaded) : '—'}</td>
                      <td style={{ color: '#a855f7' }}>{t.total_downloaded > 0 ? formatBytes(t.total_downloaded) : '—'}</td>
                      <td style={{ fontWeight: 700, color: ratio === '—' ? '#555' : '#ffd700' }}>{ratio}</td>
                      <td>
                        {t.active_sessions > 0
                          ? <span style={{ color: '#22c55e' }}>{t.active_sessions} actif(s)</span>
                          : <span style={{ color: '#555' }}>{t.session_count || 0}</span>}
                      </td>
                      <td style={{ fontSize: 12, color: '#888' }}>{trackerCount}</td>
                      <td style={{ fontFamily: 'monospace', fontSize: 11, color: '#888' }}>{t.info_hash.substring(0, 16)}…</td>
                      <td style={{ fontSize: 11, color: '#888' }}>{agoStr} ago</td>
                      <td>
                        <button className="btn btn-danger btn-sm" onClick={() => handleDeleteTorrent(t)}>Suppr.</button>
                      </td>
                    </tr>
                  )
                })}
                {torrents.length === 0 && (
                  <tr><td colSpan={12} style={{ textAlign: 'center', color: '#555', padding: 32 }}>
                    Aucun torrent. Déposez un fichier .torrent ci-dessus pour commencer.
                  </td></tr>
                )}
              </tbody>
            </table>
          </div>
        )}

        {activeTab === 'categories' && (
          <CategoryPanel
            categories={categories}
            onCreate={handleCreateCategory}
            onUpdate={handleUpdateCategory}
            onDelete={handleDeleteCategory}
            formatBytes={formatBytes}
          />
        )}

        {activeTab === 'logs' && (
          <div className="card">
            <h2>📋 Event Log</h2>
            <LogViewer logs={logs} />
          </div>
        )}

        {activeTab === 'settings' && settings && (
          <SettingsPanel
            settings={settings}
            profiles={profiles}
            interfaces={interfaces}
            onSave={handleSaveSettings}
          />
        )}

        {deleteTorrentTarget && (
          <ConfirmModal
            message={<>Voulez-vous vraiment supprimer le torrent <strong>{deleteTorrentTarget.name}</strong> et toutes ses sessions ?</>}
            confirmLabel="Supprimer"
            onConfirm={confirmDeleteTorrent}
            onCancel={() => setDeleteTorrentTarget(null)}
          />
        )}
      </div>
    </>
  )
}

export { formatBytes, formatSpeed }
