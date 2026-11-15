import { useState } from 'react'
import { usePermission } from '@/core/permissions'
import { useProviders } from '@/data/DataContext'
import type { KubeObject, ObjectRef } from '@/data/types'
import { formatBytes } from '@/i18n/format'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { copyText } from '@/shared/fileActions'
import { Icon } from '@/shared/Icon'
import { useWorkloadsTranslation } from '../i18n'
import './editing.css'

/** One revealed key: its text, or null for binary content (then only base64 is offered). */
interface RevealedValue {
  key: string
  text: string | null
  base64: string
  bytes: number
}

function decode(key: string, base64: string): RevealedValue {
  const binary = atob(base64)
  const bytes = Uint8Array.from(binary, (c) => c.charCodeAt(0))
  try {
    return { key, text: new TextDecoder('utf-8', { fatal: true }).decode(bytes), base64, bytes: bytes.length }
  } catch {
    return { key, text: null, base64, bytes: bytes.length }
  }
}

function revealedValues(secret: KubeObject): RevealedValue[] {
  const data = typeof secret.data === 'object' && secret.data !== null ? (secret.data as Record<string, unknown>) : {}
  return Object.entries(data)
    .filter((e): e is [string, string] => typeof e[1] === 'string')
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([key, value]) => decode(key, value))
}

/**
 * Shows a Secret's decoded values after a warning (exec permission; every
 * reveal is audited). The values live in this dialog's state only and are
 * gone when it closes.
 */
export function RevealSecretDialog({ target, onClose }: { target: ObjectRef; onClose: () => void }) {
  const { t } = useWorkloadsTranslation()
  const { cluster } = useProviders()
  const exec = usePermission('exec')
  const [values, setValues] = useState<RevealedValue[] | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)

  const reveal = async () => {
    if (!exec.allowed || busy) return
    setBusy(true)
    setError(null)
    try {
      setValues(revealedValues(await cluster.revealSecret(target.namespace ?? '', target.name)))
    } catch (err) {
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open
      size={values ? 'large' : 'small'}
      icon={values ? 'unlock' : 'warning'}
      role={values ? 'dialog' : 'alertdialog'}
      title={t(values ? 'editing.reveal.titleShown' : 'editing.reveal.title', { name: target.name })}
      description={values ? t('editing.reveal.shown') : t('editing.reveal.warning', { namespace: target.namespace })}
      busy={busy}
      onClose={onClose}
      onSubmit={values ? undefined : reveal}
      footer={
        values ? (
          <Button onClick={onClose}>{t('common.close')}</Button>
        ) : (
          <>
            <Button variant="secondary" onClick={onClose} disabled={busy} data-autofocus="">
              {t('common.cancel')}
            </Button>
            <Button type="submit" variant="danger" icon="eye" busy={busy} disabled={!exec.allowed} title={exec.reason ?? undefined}>
              {t('editing.reveal.confirm')}
            </Button>
          </>
        )
      }
    >
      <ErrorNotice error={error} />
      {values && values.length === 0 && <div className="kb-editing__loading">{t('editing.reveal.empty')}</div>}
      {values && values.length > 0 && (
        <table className="kb-table kb-editing__secret">
          <thead>
            <tr>
              <th>{t('common.key')}</th>
              <th>{t('common.value')}</th>
              <th aria-label={t('common.actions')} />
            </tr>
          </thead>
          <tbody>
            {values.map((v) => (
              <tr key={v.key}>
                <td>
                  <code>{v.key}</code>
                </td>
                <td>
                  {v.text !== null ? (
                    <pre className="kb-editing__value">{v.text}</pre>
                  ) : (
                    <span className="kb-editing__binary">
                      <Icon name="file-binary" size={14} /> {t('editing.reveal.binary', { size: formatBytes(v.bytes) })}
                    </span>
                  )}
                </td>
                <td className="kb-editing__copy">
                  <Button
                    variant="secondary"
                    icon="copy"
                    title={v.text !== null ? t('editing.reveal.copy', { key: v.key }) : t('editing.reveal.copyBase64', { key: v.key })}
                    aria-label={v.text !== null ? t('editing.reveal.copy', { key: v.key }) : t('editing.reveal.copyBase64', { key: v.key })}
                    onClick={() => void copyText(v.text ?? v.base64)}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </Dialog>
  )
}
