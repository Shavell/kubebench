import { useState } from 'react'
import { refreshData, useProviders } from '@/data/DataContext'
import type { ScalableKind } from '@/data/types'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field, NumberInput } from '@/shared/form'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { useWorkloadsTranslation } from '../i18n'
import type { ObjectKey } from './opsDialogs'
import './ops.css'

/** Above this the API server's quota and the scheduler are the limits; it guards against typos. */
const MAX_REPLICAS = 1000

/**
 * Sets the replicas of a StatefulSet or ReplicaSet through the scale
 * subresource. `managedBy` names the owner (e.g. "Deployment/web") that
 * would undo the change, so the dialog can say so.
 */
export function ScaleDialog({
  kind,
  target,
  replicas: current,
  managedBy,
  onClose,
}: {
  kind: ScalableKind
  target: ObjectKey
  replicas: number
  managedBy?: string
  onClose: () => void
}) {
  const { t } = useWorkloadsTranslation()
  const { workloadOps } = useProviders()
  const [replicas, setReplicas] = useState<number | null>(current)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const invalid = replicas === null || !Number.isInteger(replicas) || replicas < 0 || replicas > MAX_REPLICAS

  const scale = async () => {
    if (invalid || replicas === null) return
    setBusy(true)
    setFailure(null)
    try {
      await workloadOps.scaleWorkload(kind, target.namespace, target.name, replicas)
    } catch (err) {
      setFailure(err)
      setBusy(false)
      return
    }
    notify.success(t('workloadOps.workload.scaled', { kind, name: target.name, count: replicas }))
    refreshData()
    onClose()
  }

  return (
    <Dialog
      open
      size="small"
      icon="arrow-both"
      title={t('workloadOps.workload.scaleTitle', { kind, name: target.name })}
      busy={busy}
      onClose={onClose}
      onSubmit={() => void scale()}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" busy={busy} disabled={invalid || replicas === current}>
            {t('workloadOps.workload.scaleSubmit')}
          </Button>
        </>
      }
    >
      <Field
        label={t('workloadOps.workload.replicas')}
        help={t('workloadOps.workload.currentReplicas', { count: current })}
        error={invalid ? t('workloadOps.workload.replicasInvalid', { max: MAX_REPLICAS }) : undefined}
        required
      >
        <NumberInput value={replicas} onChange={setReplicas} min={0} max={MAX_REPLICAS} step={1} disabled={busy} />
      </Field>
      {managedBy && (
        <div className="ops-warning" role="note">
          <Icon name="warning" size={14} />
          <span>{t('workloadOps.workload.managedBy', { owner: managedBy })}</span>
        </div>
      )}
      {replicas === 0 && current > 0 && <p className="ops-muted">{t('workloadOps.workload.scaleToZero')}</p>}
      <ErrorNotice error={failure} />
    </Dialog>
  )
}
