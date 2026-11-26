import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { refreshData, useProviders } from '@/data/DataContext'
import { CORE_NS } from '@/i18n'
import { Button } from './Button'
import { Dialog } from './Dialog'
import { ErrorNotice } from './ErrorNotice'
import { Field, NumberInput } from './form'
import { Icon } from './Icon'
import { notify } from './notify'
import './ScaleDialog.css'

/** Above this the API server's quota and the scheduler are the limits; it guards against typos. */
const MAX_REPLICAS = 1000

/** Whether n is a replica count the dialog sends. */
function validReplicas(n: number | null): n is number {
  return n !== null && Number.isInteger(n) && n >= 0 && n <= MAX_REPLICAS
}

/**
 * Sets an object's replicas through the scale subresource
 * (workloadOps.scaleWorkload), for the workloads plugin's kinds and, with
 * `apiVersion`, any kind with a scale subresource such as a custom
 * resource. `replicas` is the current desired count (null when unknown);
 * `managedBy` names the owner (e.g. "Deployment/web") that would undo the
 * change, so the dialog can say so. On success it toasts, refreshes the
 * data and closes; failures show their localized reason in the dialog.
 */
export function ScaleDialog({
  kind,
  apiVersion,
  target,
  replicas: current,
  managedBy,
  onClose,
}: {
  kind: string
  apiVersion?: string
  target: { namespace: string; name: string }
  replicas: number | null
  managedBy?: string
  onClose: () => void
}) {
  const { t } = useTranslation(PLATFORM_NS)
  const { workloadOps } = useProviders()
  const [replicas, setReplicas] = useState<number | null>(current ?? 1)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const invalid = !validReplicas(replicas)

  const scale = async () => {
    if (!validReplicas(replicas)) return
    setBusy(true)
    setFailure(null)
    try {
      await workloadOps.scaleWorkload(kind, target.namespace, target.name, replicas, apiVersion)
    } catch (err) {
      setFailure(err)
      setBusy(false)
      return
    }
    notify.success(t('ui.scale.scaled', { kind, name: target.name, count: replicas }))
    refreshData()
    onClose()
  }

  return (
    <Dialog
      open
      size="small"
      icon="arrow-both"
      title={t('ui.scale.title', { kind, name: target.name })}
      busy={busy}
      onClose={onClose}
      onSubmit={() => void scale()}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel', { ns: CORE_NS })}
          </Button>
          <Button type="submit" busy={busy} disabled={invalid || replicas === current}>
            {t('ui.scale.submit')}
          </Button>
        </>
      }
    >
      <Field
        label={t('ui.scale.replicas')}
        help={current === null ? t('ui.scale.currentUnknown') : t('ui.scale.current', { count: current })}
        error={invalid ? t('ui.scale.invalid', { max: MAX_REPLICAS }) : undefined}
        required
      >
        <NumberInput value={replicas} onChange={setReplicas} min={0} max={MAX_REPLICAS} step={1} disabled={busy} />
      </Field>
      {managedBy && (
        <div className="kb-scale__warning" role="note">
          <Icon name="warning" size={14} />
          <span>{t('ui.scale.managedBy', { owner: managedBy })}</span>
        </div>
      )}
      {replicas === 0 && current !== 0 && <p className="kb-scale__muted">{t('ui.scale.toZero')}</p>}
      <ErrorNotice error={failure} />
    </Dialog>
  )
}
