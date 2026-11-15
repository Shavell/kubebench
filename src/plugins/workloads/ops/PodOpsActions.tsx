import { usePermission } from '@/core/permissions'
import type { Pod } from '@/data/types'
import { Button } from '@/shared/Button'
import { useWorkloadsTranslation } from '../i18n'
import * as act from './actions'

/** Pod detail toolbar: debug container (exec) and force delete (write). */
export function PodOpsActions({ pod, onDeleted }: { pod: Pod; onDeleted?: () => void }) {
  const { t } = useWorkloadsTranslation()
  const exec = usePermission('exec')
  const write = usePermission('write')

  return (
    <>
      <Button
        variant="secondary"
        icon="debug"
        disabled={!exec.allowed}
        title={exec.reason ?? undefined}
        onClick={() => act.debugPod(pod)}
      >
        {t('workloadOps.debug.button')}
      </Button>
      <Button
        variant="danger"
        icon="trash"
        disabled={!write.allowed}
        title={write.reason ?? undefined}
        onClick={() => act.forceDeletePod(pod, onDeleted)}
      >
        {t('workloadOps.pods.forceDelete')}
      </Button>
    </>
  )
}
