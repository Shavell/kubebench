import { useState } from 'react'
import { refreshData, useProviders } from '@/data/DataContext'
import type { DeletePropagation, ObjectRef } from '@/data/types'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { Field, Select } from '@/shared/form'
import { notify } from '@/shared/notify'
import { useWorkloadsTranslation } from '../i18n'
import './editing.css'

/** What deleting a kind cascades to, for the dialog's explanation. */
const CASCADES: Partial<Record<string, string>> = {
  Deployment: 'Deployment → ReplicaSets → Pods',
  StatefulSet: 'StatefulSet → Pods',
  DaemonSet: 'DaemonSet → Pods',
  ReplicaSet: 'ReplicaSet → Pods',
  ReplicationController: 'ReplicationController → Pods',
  Job: 'Job → Pods',
  CronJob: 'CronJob → Jobs → Pods',
}

const POLICIES: DeletePropagation[] = ['Background', 'Foreground', 'Orphan']

/**
 * Deletes any object with a choice of what happens to the objects it owns
 * (propagation policy). Cluster-scoped objects always ask for their name
 * to be typed, as everything does on a production cluster.
 */
export function DeleteResourceDialog({
  target,
  onClose,
  onDeleted,
}: {
  target: ObjectRef
  onClose: () => void
  onDeleted?: () => void
}) {
  const { t } = useWorkloadsTranslation()
  const { cluster } = useProviders()
  const [propagation, setPropagation] = useState<DeletePropagation>('Background')
  const policy = propagation.toLowerCase()
  const cascade = CASCADES[target.kind]
  const clusterScoped = !target.namespace

  const remove = async () => {
    await cluster.deleteResource(target, { propagation })
    notify.success(t('editing.delete.deleted', { kind: target.kind, name: target.name }))
    refreshData()
    onDeleted?.()
  }

  return (
    <ConfirmDialog
      open
      danger
      title={t('editing.delete.title', { kind: target.kind, name: target.name })}
      message={
        clusterScoped
          ? t('editing.delete.messageCluster', { kind: target.kind, name: target.name })
          : t('editing.delete.message', { kind: target.kind, name: target.name, namespace: target.namespace })
      }
      confirmLabel={t('common.delete')}
      confirmPhrase={target.name}
      requirePhrase={clusterScoped}
      onConfirm={remove}
      onClose={onClose}
    >
      <Field label={t('editing.delete.propagation')} help={t(`editing.delete.policyHelp.${policy}`)}>
        <Select
          value={propagation}
          onChange={setPropagation}
          options={POLICIES.map((p) => ({ value: p, label: t(`editing.delete.policy.${p.toLowerCase()}`) }))}
        />
      </Field>
      {cascade && (
        <div className="kb-editing__cascade">
          {t(propagation === 'Orphan' ? 'editing.delete.cascadeOrphan' : 'editing.delete.cascade', { chain: cascade })}
        </div>
      )}
    </ConfirmDialog>
  )
}
