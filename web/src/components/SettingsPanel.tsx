import { useState, useEffect } from 'react'
import type { Settings, ClientProfile, NetworkInterface } from '../types'

interface Props {
  settings: Settings
  profiles: ClientProfile[]
  interfaces: NetworkInterface[]
  onSave: (settings: Settings) => Promise<void>
}

const unitOptions = [
  { value: 1024, label: 'KB' },
  { value: 1048576, label: 'MB' },
  { value: 1073741824, label: 'GB' },
]

function bestUnit(bytes: number): number {
  if (bytes >= 1073741824 && bytes % 1073741824 === 0) return 1073741824
  if (bytes >= 1048576 && bytes % 1048576 === 0) return 1048576
  return 1024
}

function toDisplay(bytes: number, unit: number): string {
  if (bytes === 0) return '0'
  return String(bytes / unit)
}

export function SettingsPanel({ settings, profiles, interfaces, onSave }: Props) {
  const [clientProfile, setClientProfile] = useState(settings.client_profile)
  const [uploadSpeed, setUploadSpeed] = useState('')
  const [uploadSpeedUnit, setUploadSpeedUnit] = useState(1024)
  const [downloadSpeed, setDownloadSpeed] = useState('')
  const [downloadSpeedUnit, setDownloadSpeedUnit] = useState(1024)
  const [speedVariance, setSpeedVariance] = useState('')
  const [speedVarianceUnit, setSpeedVarianceUnit] = useState(1024)
  const [targetRatio, setTargetRatio] = useState('')
  const [stopAtRatio, setStopAtRatio] = useState(false)
  const [maxUpload, setMaxUpload] = useState('')
  const [maxUploadUnit, setMaxUploadUnit] = useState(1073741824)
  const [maxDownload, setMaxDownload] = useState('')
  const [maxDownloadUnit, setMaxDownloadUnit] = useState(1073741824)
  const [networkInterface, setNetworkInterface] = useState('')
  const [saved, setSaved] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saveError, setSaveError] = useState<string | null>(null)

  useEffect(() => {
    setClientProfile(settings.client_profile)
    const uU = bestUnit(settings.upload_speed)
    setUploadSpeedUnit(uU)
    setUploadSpeed(toDisplay(settings.upload_speed, uU))
    const dU = bestUnit(settings.download_speed)
    setDownloadSpeedUnit(dU)
    setDownloadSpeed(toDisplay(settings.download_speed, dU))
    const vU = bestUnit(settings.speed_variance)
    setSpeedVarianceUnit(vU)
    setSpeedVariance(toDisplay(settings.speed_variance, vU))
    setTargetRatio(String(settings.target_ratio))
    setStopAtRatio(settings.stop_at_ratio)
    const muU = bestUnit(settings.max_upload)
    setMaxUploadUnit(muU)
    setMaxUpload(toDisplay(settings.max_upload, muU))
    const mdU = bestUnit(settings.max_download)
    setMaxDownloadUnit(mdU)
    setMaxDownload(toDisplay(settings.max_download, mdU))
    setNetworkInterface(settings.network_interface)
  }, [settings])

  const handleSave = async () => {
    setSaving(true)
    setSaved(false)
    setSaveError(null)
    try {
      await onSave({
        client_profile: clientProfile,
        upload_speed: Math.round((parseFloat(uploadSpeed) || 0) * uploadSpeedUnit),
        download_speed: Math.round((parseFloat(downloadSpeed) || 0) * downloadSpeedUnit),
        speed_variance: Math.round((parseFloat(speedVariance) || 0) * speedVarianceUnit),
        target_ratio: parseFloat(targetRatio) || 1.0,
        stop_at_ratio: stopAtRatio,
        max_upload: Math.round((parseFloat(maxUpload) || 0) * maxUploadUnit),
        max_download: Math.round((parseFloat(maxDownload) || 0) * maxDownloadUnit),
        network_interface: networkInterface,
      })
      setSaved(true)
      setTimeout(() => setSaved(false), 2000)
    } catch (error) {
      setSaveError(error instanceof Error ? error.message : 'Failed to save settings')
    } finally {
      setSaving(false)
    }
  }

  const UnitSelect = ({ value, onChange }: { value: number; onChange: (v: number) => void }) => (
    <select value={value} onChange={e => onChange(Number(e.target.value))} style={{ width: 70, marginLeft: 6 }}>
      {unitOptions.map(u => <option key={u.value} value={u.value}>{u.label}</option>)}
    </select>
  )

  return (
    <div className="card">
      <h2>
        ⚙️ Global Settings
        <span style={{ marginLeft: 'auto', fontSize: 12, color: '#666' }}>
          Shared across all running sessions
        </span>
      </h2>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 16 }}>
        <div className="form-group">
          <label>Client Profile</label>
          <select value={clientProfile} onChange={e => setClientProfile(e.target.value)}>
            {profiles.map(p => (
              <option key={p.id} value={p.id}>{p.name} {p.version}</option>
            ))}
          </select>
        </div>

        <div className="form-group">
          <label>Network Interface (optional)</label>
          <select value={networkInterface} onChange={e => setNetworkInterface(e.target.value)}>
            <option value="">Default (system routing)</option>
            {interfaces.map(iface => (
              <option key={iface.name} value={iface.name}>
                {iface.name} — {iface.addresses.join(', ')}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12 }}>
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
        <div className="form-group">
          <label>Speed Variance ±</label>
          <div style={{ display: 'flex', alignItems: 'center' }}>
            <input type="number" value={speedVariance} onChange={e => setSpeedVariance(e.target.value)} min="0" step="any" style={{ flex: 1 }} />
            <UnitSelect value={speedVarianceUnit} onChange={setSpeedVarianceUnit} /><span style={{ marginLeft: 4 }}>/s</span>
          </div>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: 12 }}>
        <div className="form-group">
          <label>Target Ratio</label>
          <input type="number" value={targetRatio} onChange={e => setTargetRatio(e.target.value)} step="0.1" min="0" />
        </div>
        <div className="form-group">
          <label style={{ display: 'flex', alignItems: 'center', gap: 8, marginTop: 22 }}>
            <input type="checkbox" checked={stopAtRatio} onChange={e => setStopAtRatio(e.target.checked)} style={{ width: 'auto' }} />
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

      <div style={{ display: 'flex', alignItems: 'center', gap: 12, marginTop: 16 }}>
        <button className="btn btn-primary" onClick={handleSave} disabled={saving}>{saving ? 'Saving…' : '💾 Save Settings'}</button>
        {saved && <span style={{ color: '#22c55e', fontSize: 13 }}>✓ Settings saved</span>}
        {saveError && <span style={{ color: '#ef4444', fontSize: 13 }}>{saveError}</span>}
      </div>
    </div>
  )
}
