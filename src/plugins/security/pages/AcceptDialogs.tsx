import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { refreshData } from '@/data/DataContext'
import { Button } from '@/shared/Button'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field, Select, Textarea } from '@/shared/form'
import { notify } from '@/shared/notify'
import { securityData } from '../data'
import type { SecurityResource } from '../data/types'
import { NS } from '../i18n'
import { resourceLabel } from './texts'

/** Expiry choices, in days; 0 never expires. */
const EXPIRIES = [30, 90, 180, 365, 0] as const
type Expiry = (typeof EXPIRIES)[number]

const MAX_REASON = 500

/** What the accept and un-accept dialogs act on. */
export interface RiskTarget {
  key: string
  /** Translated check title. */
  title: string
  resource: SecurityResource
}

/**
 * Records a finding as an accepted risk for every user of the cluster,
 * with the reason and an optional expiry, after which it counts again.
 */
export function AcceptDialog({ target, onClose }: { target: RiskTarget; onClose: () => void }) {
  const { t } = useTranslation(NS)
  const [reason, setReason] = useState('')
  const [expiry, setExpiry] = useState<Expiry>(90)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)
  const trimmed = reason.trim()
  const tooLong = trimmed.length > MAX_REASON

  const submit = async () => {
    if (!trimmed || tooLong) return
    setBusy(true)
    setFailure(null)
    // The expiry is a moment the server compares with its own clock.
    const expiresAt = expiry ? new Date(Date.now() + expiry * 86_400_000).toISOString() : undefined
    try {
      await securityData().accept(target.key, trimmed, expiresAt)
    } catch (err) {
      setFailure(err)
      setBusy(false)
      return
    }
    notify.success(t('accept.done', { title: target.title }))
    refreshData()
    onClose()
  }

  return (
    <Dialog
      open
      icon="pass"
      title={t('accept.title')}
      description={t('accept.description', { title: target.title, resource: resourceLabel(target.resource) })}
      busy={busy}
      onClose={onClose}
      onSubmit={() => void submit()}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" busy={busy} disabled={!trimmed || tooLong}>
            {t('accept.submit')}
          </Button>
        </>
      }
    >
      <Field
        label={t('accept.reason')}
        help={t('accept.reasonHelp')}
        error={tooLong ? t('accept.reasonTooLong', { max: MAX_REASON }) : null}
        required
      >
        <Textarea value={reason} onChange={setReason} rows={3} disabled={busy} />
      </Field>
      <Field label={t('accept.expiry')} help={t('accept.expiryHelp')}>
        <Select
          value={expiry}
          onChange={setExpiry}
          disabled={busy}
          options={EXPIRIES.map((days) => ({
            value: days,
            label: days ? t('accept.expiryDays', { count: days }) : t('accept.expiryNever'),
          }))}
        />
      </Field>
      <ErrorNotice error={failure} />
    </Dialog>
  )
}

/** Asks before removing an accepted risk (on production clusters, the object's name must be typed). */
export function UnacceptDialog({ target, onClose }: { target: RiskTarget; onClose: () => void }) {
  const { t } = useTranslation(NS)
  return (
    <ConfirmDialog
      open
      title={t('unaccept.title')}
      message={t('unaccept.message', { title: target.title, resource: resourceLabel(target.resource) })}
      confirmLabel={t('unaccept.confirm')}
      confirmPhrase={target.resource.name}
      onConfirm={async () => {
        await securityData().unaccept(target.key)
        notify.success(t('unaccept.done', { title: target.title }))
        refreshData()
      }}
      onClose={onClose}
    />
  )
}
