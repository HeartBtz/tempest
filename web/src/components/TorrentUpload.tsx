import { useRef } from 'react'

interface Props {
  onUpload: (file: File) => void
}

export function TorrentUpload({ onUpload }: Props) {
  const fileRef = useRef<HTMLInputElement>(null)

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault()
    const file = e.dataTransfer.files[0]
    if (file && file.name.endsWith('.torrent')) {
      onUpload(file)
    }
  }

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (file) {
      onUpload(file)
      e.target.value = ''
    }
  }

  return (
    <div className="upload">
      <div
        className="upload-zone"
        onClick={() => fileRef.current?.click()}
        onDrop={handleDrop}
        onDragOver={e => e.preventDefault()}
      >
        <div style={{ fontSize: 32, marginBottom: 8 }}>📁</div>
        <div style={{ color: '#888' }}>Drop a <strong>.torrent</strong> file here or click to browse</div>
        <input ref={fileRef} type="file" accept=".torrent" onChange={handleChange} />
      </div>
    </div>
  )
}
