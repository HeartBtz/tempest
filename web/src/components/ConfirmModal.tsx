import type { ReactNode } from 'react'

interface Props {
  message: ReactNode
  onConfirm: () => void
  onCancel: () => void
  confirmLabel?: string
  confirmClass?: string
}

export function ConfirmModal({ message, onConfirm, onCancel, confirmLabel = 'Confirm', confirmClass = 'btn-danger' }: Props) {
  return (
    <div className="modal-overlay" onClick={e => { if (e.target === e.currentTarget) onCancel() }}>
      <div className="modal" style={{ width: 400 }}>
        <h3 style={{ color: '#f97316' }}>⚠️ Confirmation</h3>
        <p style={{ color: '#ccc', margin: '16px 0 24px', lineHeight: 1.6 }}>{message}</p>
        <div className="form-actions">
          <button className="btn btn-ghost" onClick={onCancel}>Annuler</button>
          <button className={`btn ${confirmClass}`} onClick={onConfirm}>{confirmLabel}</button>
        </div>
      </div>
    </div>
  )
}
