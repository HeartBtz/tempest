import type { LogEntry } from '../types'

interface Props {
  logs: LogEntry[]
}

export function LogViewer({ logs }: Props) {
  if (logs.length === 0) {
    return (
      <div className="log-container" style={{ textAlign: 'center', color: '#555', padding: 32 }}>
        No logs yet. Start a session to see activity.
      </div>
    )
  }

  return (
    <div className="log-container">
      {logs.slice().reverse().map((log, i) => (
        <div key={i} className="log-entry">
          <span className="log-time">
            {new Date(log.timestamp).toLocaleTimeString()}
          </span>
          <span className={`log-level-${log.level}`} style={{ minWidth: 48 }}>
            [{log.level.toUpperCase()}]
          </span>
          <span style={{ color: '#555', minWidth: 80 }}>
            {log.session_id.substring(0, 8)}
          </span>
          <span>{log.message}</span>
        </div>
      ))}
    </div>
  )
}
