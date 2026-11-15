import { usePermission } from '@/core/permissions'
import { useProviders } from '@/data/DataContext'
import type { Deployment } from '@/data/types'
import { Button } from '@/shared/Button'
import { useWorkloadsTranslation } from '../i18n'
import * as act from './actions'
import { useBusy } from './useBusy'

/** Deployment detail toolbar: pause or resume the rollout. */
export function DeploymentOpsActions({ deployment: d }: { deployment: Deployment }) {
  const { t } = useWorkloadsTranslation()
  const write = usePermission('write')
  const { workloadOps } = useProviders()
  const [busy, run] = useBusy()

  return (
    <Button
      variant="secondary"
      icon={d.paused ? 'debug-continue' : 'debug-pause'}
      busy={busy}
      disabled={!write.allowed}
      title={write.reason ?? undefined}
      onClick={() => run(() => act.togglePause(workloadOps, d))}
    >
      {t(d.paused ? 'workloadOps.rollout.resume' : 'workloadOps.rollout.pause')}
    </Button>
  )
}
