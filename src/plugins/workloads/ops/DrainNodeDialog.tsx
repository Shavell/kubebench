import { useRef, useState } from 'react'
import { refreshData, useProviders } from '@/data/DataContext'
import type { DrainResult } from '@/data/types'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { Checkbox, Field, NumberInput, Select } from '@/shared/form'
import { useWorkloadsTranslation } from '../i18n'
import './ops.css'

const TIMEOUTS = [30, 60, 120, 300, 600]
const MAX_GRACE_PERIOD = 3600

/**
 * Asks before draining a node, with the kubectl drain options. While the
 * drain runs (up to the timeout) the dialog shows progress; afterwards
 * `onDrained` gets the outcome of every Pod.
 */
export function DrainNodeDialog({
  node,
  onClose,
  onDrained,
}: {
  node: string
  onClose: () => void
  onDrained: (result: DrainResult) => void
}) {
  const { t } = useWorkloadsTranslation()
  const { workloadOps } = useProviders()
  const [ignoreDaemonSets, setIgnoreDaemonSets] = useState(true)
  const [deleteEmptyDirData, setDeleteEmptyDirData] = useState(false)
  const [force, setForce] = useState(false)
  /** null: each Pod's own grace period. */
  const [gracePeriod, setGracePeriod] = useState<number | null>(null)
  const [timeoutSeconds, setTimeoutSeconds] = useState(120)
  const result = useRef<DrainResult | null>(null)

  const graceError =
    gracePeriod !== null && (!Number.isInteger(gracePeriod) || gracePeriod < 0 || gracePeriod > MAX_GRACE_PERIOD)
      ? t('workloadOps.drain.gracePeriodInvalid', { max: MAX_GRACE_PERIOD })
      : null

  const drain = async () => {
    if (graceError) throw new Error(graceError)
    result.current = await workloadOps.drainNode(node, {
      ignoreDaemonSets,
      deleteEmptyDirData,
      force,
      gracePeriod: gracePeriod ?? -1,
      timeoutSeconds,
    })
    refreshData()
  }

  // ConfirmDialog closes itself once the drain is done; the results take its place.
  const close = () => (result.current ? onDrained(result.current) : onClose())

  return (
    <ConfirmDialog
      open
      danger
      title={t('workloadOps.drain.title', { name: node })}
      message={t('workloadOps.drain.message')}
      confirmLabel={t('workloadOps.drain.confirm')}
      confirmPhrase={node}
      onConfirm={drain}
      onClose={close}
    >
      <div className="ops-options">
        <Checkbox
          checked={ignoreDaemonSets}
          onChange={setIgnoreDaemonSets}
          label={t('workloadOps.drain.ignoreDaemonSets')}
          description={t('workloadOps.drain.ignoreDaemonSetsHelp')}
        />
        <Checkbox
          checked={deleteEmptyDirData}
          onChange={setDeleteEmptyDirData}
          label={t('workloadOps.drain.deleteEmptyDirData')}
          description={t('workloadOps.drain.deleteEmptyDirDataHelp')}
        />
        <Checkbox
          checked={force}
          onChange={setForce}
          label={t('workloadOps.drain.force')}
          description={t('workloadOps.drain.forceHelp')}
        />
        <div className="ops-options__row">
          <Field label={t('workloadOps.drain.gracePeriod')} help={t('workloadOps.drain.gracePeriodHelp')} error={graceError}>
            <NumberInput
              value={gracePeriod}
              onChange={setGracePeriod}
              min={0}
              max={MAX_GRACE_PERIOD}
              step={1}
              unit={t('workloadOps.drain.secondsUnit')}
              placeholder={t('workloadOps.drain.gracePeriodDefault')}
            />
          </Field>
          <Field label={t('workloadOps.drain.timeout')} help={t('workloadOps.drain.timeoutHelp')}>
            <Select
              value={timeoutSeconds}
              onChange={setTimeoutSeconds}
              options={TIMEOUTS.map((seconds) => ({
                value: seconds,
                label: t('workloadOps.drain.seconds', { seconds }),
              }))}
            />
          </Field>
        </div>
      </div>
    </ConfirmDialog>
  )
}
