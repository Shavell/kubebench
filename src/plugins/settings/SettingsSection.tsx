import { useId, type ReactNode } from 'react'

/**
 * A titled group of settings. Rows are inline Fields
 * (`<Field layout="inline">` from '@/shared/form'), which get separators.
 */
export function SettingsSection({ title, children }: { title: string; children: ReactNode }) {
  const titleId = useId()
  return (
    <section className="settings-section" aria-labelledby={titleId}>
      <h2 id={titleId} className="settings-section__title">
        {title}
      </h2>
      <div className="settings-section__body">{children}</div>
    </section>
  )
}
