import { usePermission } from '@/core/permissions'
import { useProviders } from '@/data/DataContext'
import type { K8sNode } from '@/data/types'
import { Button } from '@/shared/Button'
import { useWorkloadsTranslation } from '../i18n'
import * as act from './actions'
import { useBusy } from './useBusy'

/** Node detail toolbar: cordon or uncordon, drain, taints. */
export function NodeOpsActions({ node }: { node: K8sNode }) {
  const { t } = useWorkloadsTranslation()
  const write = usePermission('write')
  const { workloadOps } = useProviders()
  const [busy, run] = useBusy()
  const denied = write.reason ?? undefined

  return (
    <>
      <Button
        variant="secondary"
        icon={node.unschedulable ? 'pass' : 'circle-slash'}
        busy={busy}
        disabled={!write.allowed}
        title={denied}
        onClick={() => run(() => act.toggleCordon(workloadOps, node))}
      >
        {t(node.unschedulable ? 'workloadOps.node.uncordon' : 'workloadOps.node.cordon')}
      </Button>
      <Button variant="secondary" icon="debug-step-out" disabled={!write.allowed} title={denied} onClick={() => act.drainNode(node)}>
        {t('workloadOps.node.drain')}
      </Button>
      <Button variant="secondary" icon="tag" disabled={!write.allowed} title={denied} onClick={() => act.editTaints(node)}>
        {t('workloadOps.node.editTaints')}
      </Button>
    </>
  )
}
