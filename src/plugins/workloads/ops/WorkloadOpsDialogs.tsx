import { useEffect } from 'react'
import { ScaleDialog } from '@/shared/ScaleDialog'
import { CreateNamespaceDialog } from './CreateNamespaceDialog'
import { DebugContainerDialog } from './DebugContainerDialog'
import { DeleteNamespaceDialog } from './DeleteNamespaceDialog'
import { DeletePodsDialog } from './DeletePodsDialog'
import { DrainNodeDialog } from './DrainNodeDialog'
import { DrainResultDialog } from './DrainResultDialog'
import { ForceDeletePodDialog } from './ForceDeletePodDialog'
import { closeOpsDialog, openOpsDialog, useOpsDialog } from './opsDialogs'
import { RolloutHistoryDialog } from './RolloutHistoryDialog'
import { TaintsDialog } from './TaintsDialog'

/**
 * Renders the operation dialog requested with openOpsDialog(). Every page
 * that offers operations mounts one; only the active tab's page is
 * mounted, so there is one at a time.
 */
export function WorkloadOpsDialogs() {
  const request = useOpsDialog((s) => s.request)
  // A page that goes away takes its dialog along.
  useEffect(() => closeOpsDialog, [])

  if (!request) return null
  const close = closeOpsDialog
  switch (request.type) {
    case 'drain':
      return (
        <DrainNodeDialog
          node={request.node}
          onClose={close}
          onDrained={(result) => openOpsDialog({ type: 'drainResult', result })}
        />
      )
    case 'drainResult':
      return <DrainResultDialog result={request.result} onClose={close} />
    case 'taints':
      return <TaintsDialog node={request.node} onClose={close} />
    case 'rolloutHistory':
      return <RolloutHistoryDialog deployment={request.deployment} onClose={close} />
    case 'scale':
      return (
        <ScaleDialog
          kind={request.kind}
          target={request.target}
          replicas={request.replicas}
          managedBy={request.managedBy}
          onClose={close}
        />
      )
    case 'forceDeletePod':
      return <ForceDeletePodDialog pod={request.pod} onClose={close} onDeleted={request.onDeleted} />
    case 'deletePods':
      return <DeletePodsDialog pods={request.pods} onClose={close} onDeleted={request.onDeleted} />
    case 'debug':
      return <DebugContainerDialog pod={request.pod} onClose={close} />
    case 'createNamespace':
      return <CreateNamespaceDialog onClose={close} />
    case 'deleteNamespace':
      return <DeleteNamespaceDialog name={request.name} onClose={close} />
  }
}
