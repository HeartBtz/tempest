import { useRef, useState } from 'react'

interface UploadStatus {
  name: string
  status: 'uploading' | 'done' | 'error' | 'skipped'
  error?: string
}

interface Props {
  onUpload: (files: File[]) => Promise<void>
}

export function TorrentUpload({ onUpload }: Props) {
  const fileRef = useRef<HTMLInputElement>(null)
  const [statuses, setStatuses] = useState<UploadStatus[]>([])
  const [dragging, setDragging] = useState(false)

  const processFiles = async (files: File[]) => {
    const torrentFiles = files.filter(f => f.name.endsWith('.torrent'))
    if (torrentFiles.length === 0) return

    setStatuses(torrentFiles.map(f => ({ name: f.name, status: 'uploading' })))
    try {
      await onUpload(torrentFiles)
      setStatuses(torrentFiles.map(f => ({ name: f.name, status: 'done' })))
    } catch (e: any) {
      setStatuses(torrentFiles.map(f => ({ name: f.name, status: 'error', error: e.message })))
    }
    setTimeout(() => setStatuses([]), 4000)
  }

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    setDragging(false)
    processFiles(Array.from(e.dataTransfer.files))
  }

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || [])
    if (files.length > 0) {
      processFiles(files)
      e.target.value = ''
    }
  }

  return (
    <div className="upload">
      <div
        className="upload-zone"
        onClick={() => fileRef.current?.click()}
        onDrop={handleDrop}
        onDragOver={e => { e.preventDefault(); setDragging(true) }}
        onDragLeave={() => setDragging(false)}
        style={dragging ? { borderColor: '#ffd700', background: '#16162a' } : undefined}
      >
        <div style={{ fontSize: 32, marginBottom: 8 }}>📁</div>
        <div style={{ color: '#888' }}>
          Drop <strong>.torrent</strong> files here or click to browse
          <span style={{ color: '#555', fontSize: 12, display: 'block', marginTop: 4 }}>
            Multiple files supported — sessions start automatically
          </span>
        </div>
        <input ref={fileRef} type="file" accept=".torrent" multiple onChange={handleChange} />
      </div>

      {statuses.length > 0 && (
        <div style={{ marginTop: 8, display: 'flex', flexDirection: 'column', gap: 4 }}>
          {statuses.map((s, i) => (
            <div key={i} style={{
              display: 'flex', alignItems: 'center', gap: 8,
              fontSize: 12, padding: '4px 8px', borderRadius: 6,
              background: s.status === 'done' ? '#22c55e11' : s.status === 'error' ? '#ef444411' : '#ffd70011',
              border: `1px solid ${s.status === 'done' ? '#22c55e33' : s.status === 'error' ? '#ef444433' : '#ffd70033'}`,
            }}>
              <span>{s.status === 'uploading' ? '⏳' : s.status === 'done' ? '✓' : s.status === 'skipped' ? '⚠' : '✕'}</span>
              <span style={{ color: '#ccc', flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{s.name}</span>
              {s.error && <span style={{ color: '#ef4444' }}>{s.error}</span>}
              {s.status === 'done' && <span style={{ color: '#22c55e' }}>Started</span>}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
