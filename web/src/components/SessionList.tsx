import type { Session } from '../types'

interface Props {
  sessions: Session[]
  onStart: (id: string) => void
  onStop: (id: string) => void
  onDelete: (id: string) => void
  formatBytes: (n: number) => string
  formatSpeed: (n: number) => string
}

export function SessionList({ sessions, onStart, onStop, onDelete, formatBytes, formatSpeed }: Props) {
  if (sessions.length === 0) {
    return (
      <div style={{ textAlign: 'center', color: '#555', padding: 32 }}>
        No sessions yet. Add a torrent first, then create a session.
      </div>
    )
  }

  const ratio = (s: Session) => {
    if (s.downloaded === 0) return s.uploaded > 0 ? '∞' : '0.00'
    return (s.uploaded / s.downloaded).toFixed(2)
  }

  return (
    <div style={{ overflowX: 'auto' }}>
      <table>
        <thead>
          <tr>
            <th>Torrent</th>
            <th>Status</th>
            <th>Client</th>
            <th>Uploaded</th>
            <th>Downloaded</th>
            <th>Ratio</th>
            <th>Allocated ↑/↓</th>
            <th>Variance</th>
            <th>Limits</th>
            <th>S/L</th>
            <th>Interface</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {sessions.map(s => (
            <tr key={s.id}>
              <td>
                <div style={{ fontWeight: 600 }}>{s.torrent_name || s.torrent_id}</div>
                {s.info_hash && (
                  <div style={{ fontFamily: 'monospace', fontSize: 10, color: '#555' }}>
                    {s.info_hash.substring(0, 16)}...
                  </div>
                )}
              </td>
              <td>
                <span className={`status status-${s.status}`}>{s.status}</span>
                {s.stop_at_ratio && (
                  <div style={{ fontSize: 10, color: '#f59e0b', marginTop: 2 }}>
                    stop@{s.target_ratio}x
                  </div>
                )}
                {s.last_error && (
                  <div style={{ fontSize: 10, color: '#ef4444', marginTop: 2, maxWidth: 120, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {s.last_error}
                  </div>
                )}
              </td>
              <td style={{ fontSize: 12, color: '#888' }}>{s.client_profile}</td>
              <td style={{ color: '#3b82f6' }}>{formatBytes(s.uploaded)}</td>
              <td style={{ color: '#a855f7' }}>{formatBytes(s.downloaded)}</td>
              <td style={{ fontWeight: 700, color: '#ffd700' }}>{ratio(s)}</td>
              <td style={{ fontSize: 12 }}>
                <span style={{ color: '#3b82f6' }}>↑{formatSpeed(s.upload_speed)}</span>
                <br />
                <span style={{ color: '#a855f7' }}>↓{formatSpeed(s.download_speed)}</span>
              </td>
              <td style={{ fontSize: 11, color: '#888' }}>
                {s.speed_variance > 0 ? `±${formatSpeed(s.speed_variance)}` : '—'}
              </td>
              <td style={{ fontSize: 11, color: '#888' }}>
                {s.max_upload > 0 || s.max_download > 0 ? (
                  <>
                    {s.max_upload > 0 && <div>↑{formatBytes(s.max_upload)}</div>}
                    {s.max_download > 0 && <div>↓{formatBytes(s.max_download)}</div>}
                  </>
                ) : '∞'}
              </td>
              <td style={{ fontSize: 12 }}>
                <span style={{ color: '#22c55e' }}>{s.seeders}</span>
                {'/'}
                <span style={{ color: '#ef4444' }}>{s.leechers}</span>
              </td>
              <td style={{ fontSize: 11, color: '#888', fontFamily: 'monospace' }}>
                {s.network_interface || 'default'}
              </td>
              <td>
                <div style={{ display: 'flex', gap: 4 }}>
                  {s.status === 'running' ? (
                    <button className="btn btn-danger btn-sm" onClick={() => onStop(s.id)}>■ Stop</button>
                  ) : (
                    <button className="btn btn-success btn-sm" onClick={() => onStart(s.id)}>▶ Start</button>
                  )}
                  <button className="btn btn-ghost btn-sm" onClick={() => onDelete(s.id)}>✕</button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
