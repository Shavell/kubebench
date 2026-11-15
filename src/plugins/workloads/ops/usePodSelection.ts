import { useState } from 'react'
import { usePermission } from '@/core/permissions'
import type { Pod, PodKey } from '@/data/types'
import type { TableSelection } from '@/shared/Table'
import { useWorkloadsTranslation } from '../i18n'

/** The key of a Pod row: "namespace/name", as the Pods table's rowKey. */
export const podKey = (p: PodKey) => `${p.namespace}/${p.name}`

export interface PodSelection {
  selected: ReadonlySet<string>
  clear: () => void
  /** For the Pods table; undefined when the user may not delete Pods. */
  table: TableSelection<Pod> | undefined
}

/** Multi-select state of a Pod list, for bulk delete. */
export function usePodSelection(): PodSelection {
  const { t } = useWorkloadsTranslation()
  const write = usePermission('write')
  const [selected, setSelected] = useState<ReadonlySet<string>>(() => new Set())
  return {
    selected,
    clear: () => setSelected(new Set()),
    table: write.allowed
      ? {
          selected,
          onChange: setSelected,
          rowLabel: (p) => t('workloadOps.pods.selectPod', { name: p.name }),
          allLabel: t('workloadOps.pods.selectAll'),
        }
      : undefined,
  }
}
