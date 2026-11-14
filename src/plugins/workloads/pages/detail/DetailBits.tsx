import type { ReactNode } from 'react'
import '@/shared/tabPage.css'

export function DetailRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <>
      <span className="detail__label">{label}</span>
      <span className="detail__value">{value}</span>
    </>
  )
}

export function DetailSection({ title }: { title: string }) {
  return <div className="detail__section">{title}</div>
}
