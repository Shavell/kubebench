import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import { Button } from '@/shared/Button'
import { CodeEditor } from '@/shared/editor/CodeEditor'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Toolbar } from '@/shared/Toolbar'
import { NS } from '../i18n'
import type { WizardStatus } from './useWizard'
import '@/shared/tabPage.css'
import '../storage.css'

/**
 * Wizard page: the form on the left, the live read-only YAML of the object
 * it builds on the right, Validate (server-side dry run) and Create below.
 * Validate needs only read access; Create needs write.
 */
export function WizardLayout({
  title,
  yaml,
  status,
  busy,
  onValidate,
  onCreate,
  children,
}: {
  title: string
  yaml: string
  status: WizardStatus
  busy: boolean
  onValidate: () => void
  onCreate: () => void
  children: ReactNode
}) {
  const { t } = useTranslation(NS)
  const write = usePermission('write')
  const validating = status.state === 'busy' && status.dryRun
  const creating = status.state === 'busy' && !status.dryRun

  return (
    <div className="page">
      <Toolbar title={title} />
      <div className="storage-wizard">
        <form
          className="storage-wizard__form"
          aria-label={title}
          onSubmit={(event) => {
            event.preventDefault()
            if (write.allowed) onCreate()
          }}
        >
          {children}
          {/* Enter in a field submits; the visible buttons live in the footer. */}
          <button type="submit" hidden />
        </form>
        <div className="storage-wizard__preview">
          <div className="storage-wizard__preview-title">{t('wizard.preview')}</div>
          <div className="storage-wizard__editor">
            <CodeEditor value={yaml} readOnly ariaLabel={t('wizard.preview')} />
          </div>
        </div>
      </div>
      <div className="storage-wizard__footer">
        <div className="storage-wizard__status" role="status">
          {status.state === 'valid' && (
            <>
              <span className="storage-wizard__status--ok">{t('wizard.valid')}</span>
              {status.warnings.length > 0 && (
                <ul className="storage-warnings">
                  {status.warnings.map((w) => (
                    <li key={w}>{w}</li>
                  ))}
                </ul>
              )}
            </>
          )}
          {status.state === 'failed' && <ErrorNotice error={status.error} />}
        </div>
        <Button variant="secondary" icon="check" busy={validating} disabled={busy} onClick={onValidate}>
          {t('wizard.validate')}
        </Button>
        <Button
          icon="add"
          busy={creating}
          disabled={busy || !write.allowed}
          title={write.reason ?? undefined}
          onClick={onCreate}
        >
          {t('wizard.create')}
        </Button>
      </div>
    </div>
  )
}
