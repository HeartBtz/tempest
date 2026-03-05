import type { GlobalStats } from '../types'

interface Props {
  stats: GlobalStats
  formatBytes: (n: number) => string
}

export function StatsBar({ stats, formatBytes }: Props) {
  return (
    <div className="stat-grid" style={{ marginBottom: 20 }}>
      <div className="stat-card">
        <div className="value">{stats.total_torrents}</div>
        <div className="label">Total Torrents</div>
      </div>
      <div className="stat-card">
        <div className="value" style={{ color: stats.active_sessions > 0 ? '#22c55e' : '#ffd700' }}>
          {stats.active_sessions}
        </div>
        <div className="label">Active Sessions</div>
      </div>
      <div className="stat-card">
        <div className="value" style={{ color: '#3b82f6' }}>{formatBytes(stats.total_uploaded)}</div>
        <div className="label">Total Uploaded</div>
      </div>
      <div className="stat-card">
        <div className="value" style={{ color: '#a855f7' }}>{formatBytes(stats.total_downloaded)}</div>
        <div className="label">Total Downloaded</div>
      </div>
    </div>
  )
}
