import { usePermission } from '@/core/permissions'
import { MAX_BULK_DELETE, type Pod } from '@/data/types'
import { Button } from '@/shared/Button'
import { useWorkloadsTranslation } from '../i18n'
import * as act from './actions'
import { openOpsDialog } from './opsDialogs'
import { podKey, type PodSelection } from './usePodSelection'

/**
 * Deletes the selected Pods that the list shows (filtered-out ones are never
 * touched), at most MAX_BULK_DELETE at once, as the API takes them.
 */
function DeleteSelectedPods({ pods, selection }: { pods: Pod[]; selection: PodSelection }) {
  const { t } = useWorkloadsTranslation()
  const chosen = pods.filter((p) => selection.selected.has(podKey(p)))
  if (chosen.length === 0) return null
  const tooMany = chosen.length > MAX_BULK_DELETE
  return (
    <Button
      variant="danger"
      icon="trash"
      disabled={tooMany}
      title={
        tooMany
          ? t('workloadOps.pods.deleteTooMany', { count: chosen.length, max: MAX_BULK_DELETE })
          : t('workloadOps.pods.deleteSelectedTitle')
      }
      onClick={() =>
        openOpsDialog({
          type: 'deletePods',
          pods: chosen.map((p) => ({ namespace: p.namespace, name: p.name })),
          onDeleted: selection.clear,
        })
      }
    >
      {t('workloadOps.pods.deleteSelected', { count: chosen.length })}
    </Button>
  )
}

function CreateNamespaceButton() {
  const { t } = useWorkloadsTranslation()
  const write = usePermission('write')
  return (
    <Button variant="secondary" icon="add" disabled={!write.allowed} title={write.reason ?? undefined} onClick={act.createNamespace}>
      {t('workloadOps.namespace.create')}
    </Button>
  )
}

/** List toolbar actions: bulk delete for Pods, a new namespace for Namespaces. */
export function ListOpsActions({ resource, rows, selection }: { resource: string; rows: unknown[]; selection: PodSelection }) {
  if (resource === 'Pods') return <DeleteSelectedPods pods={rows as Pod[]} selection={selection} />
  if (resource === 'Namespaces') return <CreateNamespaceButton />
  return null
}
