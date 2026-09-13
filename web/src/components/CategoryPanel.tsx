import { useState } from 'react'
import type { Category } from '../types'
import { ConfirmModal } from './ConfirmModal'

interface Props {
  categories: Category[]
  onCreate: (data: Omit<Category, 'id' | 'created_at' | 'torrent_count'>) => void
  onUpdate: (id: string, data: Omit<Category, 'id' | 'created_at' | 'torrent_count'>) => void
  onDelete: (id: string) => void
  formatBytes: (n: number) => string
}

const PRESET_COLORS = [
  '#6366f1', '#8b5cf6', '#ec4899', '#ef4444',
  '#f97316', '#eab308', '#22c55e', '#06b6d4',
  '#3b82f6', '#14b8a6',
]

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

function UnitSelect({ value, onChange }: { value: number; onChange: (v: number) => void }) {
  return (
    <select value={value} onChange={e => onChange(Number(e.target.value))} style={{ width: 70, marginLeft: 6 }}>
      {unitOptions.map(u => <option key={u.value} value={u.value}>{u.label}</option>)}
    </select>
  )
}

interface CategoryFormProps {
  initial?: Category
  onSave: (data: Omit<Category, 'id' | 'created_at' | 'torrent_count'>) => void
  onCancel?: () => void
  formatBytes: (n: number) => string
}

function CategoryForm({ initial, onSave, onCancel, formatBytes: _ }: CategoryFormProps) {
  const [name, setName] = useState(initial?.name ?? '')
  const [color, setColor] = useState(initial?.color ?? PRESET_COLORS[0])

  const [uploadSpeed, setUploadSpeed] = useState(initial ? toDisplay(initial.upload_speed, bestUnit(initial.upload_speed)) : '0')
  const [uploadUnit, setUploadUnit] = useState(initial ? bestUnit(initial.upload_speed) : 1048576)
  const [downloadSpeed, setDownloadSpeed] = useState(initial ? toDisplay(initial.download_speed, bestUnit(initial.download_speed)) : '0')
  const [downloadUnit, setDownloadUnit] = useState(initial ? bestUnit(initial.download_speed) : 1048576)
  const [variance, setVariance] = useState(initial ? toDisplay(initial.speed_variance, bestUnit(initial.speed_variance)) : '0')
  const [varianceUnit, setVarianceUnit] = useState(initial ? bestUnit(initial.speed_variance) : 1048576)
  const [targetRatio, setTargetRatio] = useState(String(initial?.target_ratio ?? 2.0))

  const handleSave = () => {
    if (!name.trim()) return
    onSave({
      name: name.trim(),
      color,
      upload_speed: Math.round((parseFloat(uploadSpeed) || 0) * uploadUnit),
      download_speed: Math.round((parseFloat(downloadSpeed) || 0) * downloadUnit),
      speed_variance: Math.round((parseFloat(variance) || 0) * varianceUnit),
      target_ratio: parseFloat(targetRatio) || 2.0,
    })
  }

  return (
    <div style={{ background: '#16162a', border: '1px solid #2a2a4a', borderRadius: 10, padding: 16, marginBottom: 12 }}>
      <div style={{ display: 'grid', gridTemplateColumns: '1fr auto', gap: 12, marginBottom: 12 }}>
        <div className="form-group" style={{ margin: 0 }}>
          <label>Nom de la catégorie</label>
          <input
            type="text"
            value={name}
            onChange={e => setName(e.target.value)}
            placeholder="ex: Site A, Privé, Public..."
            autoFocus
          />
        </div>
        <div className="form-group" style={{ margin: 0 }}>
          <label>Couleur</label>
          <div style={{ display: 'flex', gap: 6, flexWrap: 'wrap', marginTop: 6 }}>
            {PRESET_COLORS.map(c => (
              <div
                key={c}
                onClick={() => setColor(c)}
                style={{
                  width: 22, height: 22, borderRadius: '50%', background: c, cursor: 'pointer',
                  border: color === c ? '2px solid #fff' : '2px solid transparent',
                  boxSizing: 'border-box',
                }}
              />
            ))}
          </div>
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr 1fr', gap: 10, marginBottom: 12 }}>
        <div className="form-group" style={{ margin: 0 }}>
          <label>Upload</label>
          <div style={{ display: 'flex', alignItems: 'center' }}>
            <input type="number" value={uploadSpeed} onChange={e => setUploadSpeed(e.target.value)} min="0" step="any" style={{ flex: 1 }} />
            <UnitSelect value={uploadUnit} onChange={setUploadUnit} />
            <span style={{ marginLeft: 4, fontSize: 11, color: '#666' }}>/s</span>
          </div>
        </div>
        <div className="form-group" style={{ margin: 0 }}>
          <label>Download</label>
          <div style={{ display: 'flex', alignItems: 'center' }}>
            <input type="number" value={downloadSpeed} onChange={e => setDownloadSpeed(e.target.value)} min="0" step="any" style={{ flex: 1 }} />
            <UnitSelect value={downloadUnit} onChange={setDownloadUnit} />
            <span style={{ marginLeft: 4, fontSize: 11, color: '#666' }}>/s</span>
          </div>
        </div>
        <div className="form-group" style={{ margin: 0 }}>
          <label>Variance ±</label>
          <div style={{ display: 'flex', alignItems: 'center' }}>
            <input type="number" value={variance} onChange={e => setVariance(e.target.value)} min="0" step="any" style={{ flex: 1 }} />
            <UnitSelect value={varianceUnit} onChange={setVarianceUnit} />
            <span style={{ marginLeft: 4, fontSize: 11, color: '#666' }}>/s</span>
          </div>
        </div>
        <div className="form-group" style={{ margin: 0 }}>
          <label>Ratio cible</label>
          <input type="number" value={targetRatio} onChange={e => setTargetRatio(e.target.value)} step="0.1" min="0" />
        </div>
      </div>

      <div style={{ display: 'flex', gap: 8 }}>
        <button className="btn btn-primary btn-sm" onClick={handleSave} disabled={!name.trim()}>
          💾 {initial ? 'Mettre à jour' : 'Créer'}
        </button>
        {onCancel && (
          <button className="btn btn-ghost btn-sm" onClick={onCancel}>Annuler</button>
        )}
      </div>
    </div>
  )
}

export function CategoryPanel({ categories, onCreate, onUpdate, onDelete, formatBytes }: Props) {
  const [showCreate, setShowCreate] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Category | null>(null)

  const handleCreate = (data: Omit<Category, 'id' | 'created_at' | 'torrent_count'>) => {
    onCreate(data)
    setShowCreate(false)
  }

  const handleUpdate = (id: string, data: Omit<Category, 'id' | 'created_at' | 'torrent_count'>) => {
    onUpdate(id, data)
    setEditingId(null)
  }

  return (
    <div className="card">
      <h2>
        🏷️ Catégories
        <button
          className="btn btn-primary btn-sm"
          onClick={() => setShowCreate(v => !v)}
          style={{ marginLeft: 'auto' }}
        >
          {showCreate ? '✕ Annuler' : '+ Nouvelle catégorie'}
        </button>
      </h2>

      {showCreate && (
        <CategoryForm
          onSave={handleCreate}
          onCancel={() => setShowCreate(false)}
          formatBytes={formatBytes}
        />
      )}

      {categories.length === 0 && !showCreate && (
        <div style={{ textAlign: 'center', color: '#555', padding: 32 }}>
          Aucune catégorie. Créez-en une pour organiser vos torrents et définir des débits par groupe.
        </div>
      )}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
        {categories.map(cat => (
          <div key={cat.id}>
            {editingId === cat.id ? (
              <CategoryForm
                initial={cat}
                onSave={data => handleUpdate(cat.id, data)}
                onCancel={() => setEditingId(null)}
                formatBytes={formatBytes}
              />
            ) : (
              <div style={{
                display: 'flex', alignItems: 'center', gap: 12,
                padding: '12px 16px', borderRadius: 8,
                background: '#16162a', border: `1px solid ${cat.color}44`,
              }}>
                <div style={{ width: 14, height: 14, borderRadius: '50%', background: cat.color, flexShrink: 0 }} />
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontWeight: 600, color: '#e0e0e0' }}>{cat.name}</div>
                  <div style={{ fontSize: 11, color: '#666', marginTop: 2 }}>
                    {cat.torrent_count ?? 0} torrent{(cat.torrent_count ?? 0) !== 1 ? 's' : ''}
                    {' · '}
                    ↑ {formatBytes(cat.upload_speed)}/s
                    {cat.download_speed > 0 && ` · ↓ ${formatBytes(cat.download_speed)}/s`}
                    {cat.speed_variance > 0 && ` · ±${formatBytes(cat.speed_variance)}/s`}
                    {' · ratio ×'}{cat.target_ratio}
                  </div>
                </div>
                <div style={{ display: 'flex', gap: 6, flexShrink: 0 }}>
                  <button className="btn btn-ghost btn-sm" onClick={() => setEditingId(cat.id)}>✏️</button>
                  <button className="btn btn-danger btn-sm" onClick={() => setDeleteTarget(cat)}>🗑</button>
                </div>
              </div>
            )}
          </div>
        ))}
      </div>

      {deleteTarget && (
        <ConfirmModal
          message={
            <>
              Voulez-vous vraiment supprimer la catégorie <strong style={{ color: deleteTarget.color }}>{deleteTarget.name}</strong> ?
              <br /><br />
              Les {deleteTarget.torrent_count ?? 0} torrent(s) de cette catégorie seront désassignés (non supprimés).
            </>
          }
          confirmLabel="Supprimer la catégorie"
          onConfirm={() => {
            onDelete(deleteTarget.id)
            setDeleteTarget(null)
          }}
          onCancel={() => setDeleteTarget(null)}
        />
      )}
    </div>
  )
}
