import clsx from 'clsx'
import type { ReactNode } from 'react'
import { Toolbar } from './Toolbar'
import './Page.css'

interface PageProps {
  title: string
  /** Toolbar actions, right-aligned. */
  actions?: ReactNode
  children: ReactNode
  className?: string
}

/** An editor-tab page: toolbar on top, scrollable body below. */
export function Page({ title, actions, children, className }: PageProps) {
  return (
    <div className={clsx('kb-page', className)}>
      <Toolbar title={title}>{actions}</Toolbar>
      <div className="kb-page__body">{children}</div>
    </div>
  )
}

/** A titled block inside a Page body. */
export function PageSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="kb-page__section">
      <h2 className="kb-page__section-title">{title}</h2>
      {children}
    </section>
  )
}
