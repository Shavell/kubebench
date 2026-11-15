import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import { errorMessage } from '@/data/apiClient'
import { useProviders } from '@/data/DataContext'
import type { UpdateMode, UpdateStatus } from '@/data/types'
import { formatDateTime } from '@/i18n/format'
import { Button } from '@/shared/Button'
import { Field, Select } from '@/shared/form'
import { notify } from '@/shared/notify'
import { Unavailable } from '@/shared/Unavailable'
import { UPDATES_NS } from './namespace'
import { runUpdateCheck } from './runUpdateCheck'
import { useUpdatesStore } from './updatesStore'
import './updates.css'

const MODES: UpdateMode[] = ['off', 'stable', 'prerelease']

/** Settings → Updates: the mode (admins), the running and latest version, the last check. */
export function UpdatesSettingsSection() {
  const { t } = useTranslation(UPDATES_NS)
  const { updates } = useProviders()
  const admin = usePermission('admin')
  const status = useUpdatesStore((s) => s.status)
  const loadError = useUpdatesStore((s) => s.loadError)
  const busy = useUpdatesStore((s) => s.busy)
  const store = useUpdatesStore.getState

  useEffect(() => {
    void store().load(updates)
  }, [store, updates])

  if (!status) {
    return loadError ? (
      <Unavailable compact error={loadError} title={t('settings.loadFailed')} onRetry={() => void store().load(updates)} />
    ) : null
  }

  const changeMode = async (mode: UpdateMode) => {
    try {
      await store().setMode(updates, mode)
      notify.success(t('notices.modeSaved', { mode: t(`modes.${mode}`) }))
    } catch (err) {
      notify.error(t('notices.saveFailed'), { detail: errorMessage(err) })
    }
  }

  const off = status.mode === 'off'
  return (
    <>
      <Field
        layout="inline"
        label={t('settings.mode.label')}
        help={`${t('settings.mode.help')} ${t('settings.mode.serverDefault', { mode: t(`modes.${status.defaultMode}`) })}`}
      >
        <Select
          value={status.mode}
          options={MODES.map((mode) => ({ value: mode, label: t(`modes.${mode}`) }))}
          disabled={!admin.allowed || busy}
          title={admin.reason ?? undefined}
          onChange={(mode) => void changeMode(mode)}
        />
      </Field>
      <dl className="kb-update-settings">
        <dt>{t('settings.current.label')}</dt>
        <dd>{status.current === 'unknown' ? t('settings.current.unknown') : `v${status.current}`}</dd>
        <dt>{t('settings.latest.label')}</dt>
        <dd>
          <LatestRelease status={status} />
          {status.available && admin.allowed && (
            <Button variant="secondary" className="kb-update-settings__notes" onClick={() => store().showAvailable()}>
              {t('settings.showNotes')}
            </Button>
          )}
        </dd>
        {!off && (
          <>
            <dt>{t('settings.checked.label')}</dt>
            <dd>{status.checkedAt ? formatDateTime(status.checkedAt) : t('settings.checked.never')}</dd>
          </>
        )}
        {!off && status.error && (
          <>
            <dt>{t('settings.error.label')}</dt>
            <dd className="kb-update-settings__error">{status.error}</dd>
          </>
        )}
      </dl>
      {admin.allowed && (
        <div className="kb-update-settings__actions">
          <Button
            variant="secondary"
            icon="refresh"
            busy={busy}
            disabled={off}
            title={off ? t('settings.checkOff') : undefined}
            onClick={() => void runUpdateCheck(updates, { openDialog: false })}
          >
            {busy ? t('settings.checking') : t('settings.checkNow')}
          </Button>
        </div>
      )}
    </>
  )
}

function LatestRelease({ status }: { status: UpdateStatus }) {
  const { t } = useTranslation(UPDATES_NS)
  if (status.mode === 'off') return <>{t('settings.latest.off')}</>
  if (!status.latest) return <>{status.checkedAt ? t('settings.latest.none') : t('settings.latest.pending')}</>
  if (status.current === 'unknown') return <>{t('settings.latest.plain', { version: status.latest })}</>
  return (
    <>
      {status.available
        ? t('settings.latest.available', { version: status.latest })
        : t('settings.latest.upToDate', { version: status.latest })}
    </>
  )
}
