import type { ReactNode } from 'react'
import './Toolbar.css'

interface ToolbarProps {
  title: string
  children?: ReactNode
}

/** Header strip above list/detail pages: title left, actions right. */
export function Toolbar({ title, children }: ToolbarProps) {
  return (
    <div className="kb-toolbar">
      <span className="kb-toolbar__title">{title}</span>
      <div className="kb-toolbar__actions">{children}</div>
    </div>
  )
}
