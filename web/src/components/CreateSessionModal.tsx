import { useState } from 'react'
import type { Torrent, ClientProfile, NetworkInterface } from '../types'

interface Props {
  torrents: Torrent[]
  profiles: ClientProfile[]
  interfaces: NetworkInterface[]
  onClose: () => void
  onCreate: (data: {
    torrent_id: string
    client_profile: string
    upload_speed: number
    download_speed: number
    speed_variance: number
    target_ratio: number
    stop_at_ratio: boolean
    max_upload: number
    max_download: number
    network_interface: string
  }) => void
}

export function CreateSessionModal({ torrents, profiles, interfaces, onClose, onCreate }: Props) {
  const [torrentId, setTorrentId] = useState(torrents[0]?.id || '')
  const [profileId, setProfileId] = useState('qbittorrent-4.6.2')
  const [uploadSpeed, setUploadSpeed] = useState('100')
  const [uploadSpeedUnit, setUploadSpeedUnit] = useState(1024)
  const [downloadSpeed, setDownloadSpeed] = useState('0')
  const [downloadSpeedUnit, setDownloadSpeedUnit] = useState(1024)
  const [speedVariance, setSpeedVariance] = useState('10')
  const [speedVarianceUnit, setSpeedVarianceUnit] = useState(1024)
  const [targetRatio, setTargetRatio] = useState('2.0')
  const [stopAtRatio, setStopAtRatio] = useState(false)
  const [maxUpload, setMaxUpload] = useState('0')
  const [maxUploadUnit, setMaxUploadUnit] = useState(1073741824)
  const [maxDownload, setMaxDownload] = useState('0')
  const [maxDownloadUnit, setMaxDownloadUnit] = useState(1073741824)
  const [networkInterface, setNetworkInterface] = useState('')

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onCreate({
      torrent_id: torrentId,
      client_profile: profileId,
      upload_speed: Math.round((parseFloat(uploadSpeed) || 0) * uploadSpeedUnit),
      download_speed: Math.round((parseFloat(downloadSpeed) || 0) * downloadSpeedUnit),
      speed_variance: Math.round((parseFloat(speedVariance) || 0) * speedVarianceUnit),
      target_ratio: parseFloat(targetRatio) || 1.0,
      stop_at_ratio: stopAtRatio,
      max_upload: Math.round((parseFloat(maxUpload) || 0) * maxUploadUnit),
      max_download: Math.round((parseFloat(maxDownload) || 0) * maxDownloadUnit),
      network_interface: networkInterface,
    })
  }

  const unitOptions = [
    { value: 1024, label: 'KB' },
    { value: 1048576, label: 'MB' },
    { value: 1073741824, label: 'GB' },
  ]

  const UnitSelect = ({ value, onChange }: { value: number, onChange: (v: number) => void }) => (
    <select value={value} onChange={e => onChange(Number(e.target.value))} style={{ width: 70, marginLeft: 6 }}>
      {unitOptions.map(u => <option key={u.value} value={u.value}>{u.label}</option>)}
    </select>
  )

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()} style={{ width: 560 }}>
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

          <div className="form-group">
            <label>Client Profile</label>
            <select value={profileId} onChange={e => setProfileId(e.target.value)}>
              {profiles.map(p => (
                <option key={p.id} value={p.id}>{p.name} {p.version}</option>
              ))}
            </select>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <div className="form-group">
              <label>Upload Speed</label>
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <input type="number" value={uploadSpeed} onChange={e => setUploadSpeed(e.target.value)} min="0" step="any" style={{ flex: 1 }} />
                <UnitSelect value={uploadSpeedUnit} onChange={setUploadSpeedUnit} /><span style={{ marginLeft: 4 }}>/s</span>
              </div>
            </div>
            <div className="form-group">
              <label>Download Speed</label>
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <input type="number" value={downloadSpeed} onChange={e => setDownloadSpeed(e.target.value)} min="0" step="any" style={{ flex: 1 }} />
                <UnitSelect value={downloadSpeedUnit} onChange={setDownloadSpeedUnit} /><span style={{ marginLeft: 4 }}>/s</span>
              </div>
            </div>
          </div>

          <div className="form-group">
            <label>Speed Variance ±</label>
            <div style={{ display: 'flex', alignItems: 'center' }}>
              <input type="number" value={speedVariance} onChange={e => setSpeedVariance(e.target.value)} min="0" step="any" style={{ flex: 1 }} />
              <UnitSelect value={speedVarianceUnit} onChange={setSpeedVarianceUnit} /><span style={{ marginLeft: 4 }}>/s</span>
            </div>
            <div style={{ fontSize: 11, color: '#666', marginTop: 4 }}>
              Randomizes speed between base ± this value each announce cycle
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <div className="form-group">
              <label>Target Ratio</label>
              <input type="number" value={targetRatio} onChange={e => setTargetRatio(e.target.value)} step="0.1" min="0" />
            </div>
            <div className="form-group">
              <label style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 22 }}>
                <input
                  type="checkbox"
                  checked={stopAtRatio}
                  onChange={e => setStopAtRatio(e.target.checked)}
                  style={{ width: 'auto' }}
                />
                Stop when ratio reached
              </label>
            </div>
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 12 }}>
            <div className="form-group">
              <label>Max Upload (0 = unlimited)</label>
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <input type="number" value={maxUpload} onChange={e => setMaxUpload(e.target.value)} min="0" step="any" style={{ flex: 1 }} />
                <UnitSelect value={maxUploadUnit} onChange={setMaxUploadUnit} />
              </div>
            </div>
            <div className="form-group">
              <label>Max Download (0 = unlimited)</label>
              <div style={{ display: 'flex', alignItems: 'center' }}>
                <input type="number" value={maxDownload} onChange={e => setMaxDownload(e.target.value)} min="0" step="any" style={{ flex: 1 }} />
                <UnitSelect value={maxDownloadUnit} onChange={setMaxDownloadUnit} />
              </div>
            </div>
          </div>

          <div className="form-group">
            <label>Network Interface (for VPN routing)</label>
            <select value={networkInterface} onChange={e => setNetworkInterface(e.target.value)}>
              <option value="">Default (system routing)</option>
              {interfaces.map(iface => (
                <option key={iface.name} value={iface.name}>
                  {iface.name} — {iface.addresses.join(', ')}
                </option>
              ))}
            </select>
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
