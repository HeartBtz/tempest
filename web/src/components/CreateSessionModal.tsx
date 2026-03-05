import { useState } from 'react'
import type { Torrent } from '../types'

interface Props {
  torrents: Torrent[]
  onClose: () => void
  onCreate: (data: { torrent_id: string }) => void
}

export function CreateSessionModal({ torrents, onClose, onCreate }: Props) {
  const [torrentId, setTorrentId] = useState(torrents[0]?.id || '')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onCreate({ torrent_id: torrentId })
  }

  return (
    <div className="modal-overlay" onClick={e => { if (e.target === e.currentTarget) onClose() }}>
      <div className="modal">
        <h3>⚡ New Simulation Session</h3>
        <form onSubmit={handleSubmit}>
          <div className="form-group">
            <label>Torrent</label>
            <select value={torrentId} onChange={e => setTorrentId(e.target.value)}>
              {torrents.map(t => (
                <option key={t.id} value={t.id}>{t.name}</option>
              ))}
            </select>
          </div>
          <div style={{ fontSize: 12, color: '#888', marginBottom: 16 }}>
            Speed, ratio, client and network settings are configured in the ⚙️ Settings tab.
          </div>
          <div className="form-actions">
            <button type="button" className="btn btn-ghost" onClick={onClose}>Cancel</button>
            <button type="submit" className="btn btn-primary" disabled={!torrentId}>Create Session</button>
          </div>
        </form>
      </div>
    </div>
  )
}
