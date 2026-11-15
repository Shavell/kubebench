import { usePermission } from '@/core/permissions'
import { useProviders } from '@/data/DataContext'
import type { KubeObject } from '@/data/types'
import { Button } from '@/shared/Button'
import { useWorkloadsTranslation } from '../i18n'
import * as act from './actions'
import { isRestartableObject, isScalableObject, isSuspended } from './targets'
import { useBusy } from './useBusy'

/**
 * Generic detail toolbar: restart (StatefulSet, DaemonSet), scale
 * (StatefulSet, ReplicaSet), trigger and suspend (CronJob). Other kinds
 * get nothing.
 */
export function ObjectOpsActions({ kind, obj }: { kind: string; obj: KubeObject }) {
  const { t } = useWorkloadsTranslation()
  const write = usePermission('write')
  const { workloadOps } = useProviders()
  const [busy, run] = useBusy()
  const common = { variant: 'secondary' as const, disabled: !write.allowed || busy, title: write.reason ?? undefined }

  if (kind === 'CronJob') {
    const suspended = isSuspended(obj)
    return (
      <>
        <Button {...common} icon="play" onClick={() => run(() => act.triggerCronJob(workloadOps, obj))}>
          {t('workloadOps.cronjob.trigger')}
        </Button>
        <Button
          {...common}
          icon={suspended ? 'debug-continue' : 'debug-pause'}
          onClick={() => run(() => act.toggleSuspend(workloadOps, obj))}
        >
          {t(suspended ? 'workloadOps.cronjob.resume' : 'workloadOps.cronjob.suspend')}
        </Button>
      </>
    )
  }
  return (
    <>
      {isRestartableObject(kind) && (
        <Button {...common} icon="sync" onClick={() => run(() => act.restartWorkload(workloadOps, kind, obj))}>
          {t('common.restart')}
        </Button>
      )}
      {isScalableObject(kind) && (
        <Button {...common} icon="arrow-both" onClick={() => act.scaleWorkload(kind, obj)}>
          {t('workloadOps.workload.scale')}
        </Button>
      )}
    </>
  )
}
