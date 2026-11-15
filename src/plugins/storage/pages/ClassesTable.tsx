import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { formatBytes, formatNumber } from '@/i18n/format'
import { Icon } from '@/shared/Icon'
import { Table, type Column } from '@/shared/Table'
import type { StorageClassSummary } from '../data/types'
import { NS } from '../i18n'
import { openResource } from '../tabs'

/**
 * StorageClasses with the claims and volumes using them. `defaultClass`
 * is the one that applies; other classes marked default are flagged too.
 */
export function ClassesTable({ classes, defaultClass }: { classes: StorageClassSummary[]; defaultClass: string }) {
  const { t } = useTranslation(NS)
  const columns = useMemo<Column<StorageClassSummary>[]>(
    () => [
      {
        key: 'name',
        header: t('columns.name'),
        sortValue: (c) => c.name,
        render: (c) => (
          <>
            {c.name}
            {c.isDefault && (
              <span
                className="storage-default-marker"
                title={c.name === defaultClass ? t('classes.defaultHint') : t('classes.extraDefaultHint')}
              >
                <Icon name={c.name === defaultClass ? 'star-full' : 'star-empty'} size={12} />
                {t('classes.default')}
              </span>
            )}
          </>
        ),
      },
      {
        key: 'provisioner',
        header: t('columns.provisioner'),
        sortValue: (c) => c.provisioner,
        render: (c) => <span className="storage-mono">{c.provisioner}</span>,
      },
      { key: 'reclaim', header: t('columns.reclaimPolicy'), render: (c) => c.reclaimPolicy },
      { key: 'binding', header: t('columns.bindingMode'), render: (c) => c.volumeBindingMode },
      {
        key: 'expansion',
        header: t('columns.expansion'),
        render: (c) => (c.allowVolumeExpansion ? t('common.yes') : t('common.no')),
      },
      { key: 'pvcs', header: t('columns.pvcs'), sortValue: (c) => c.pvcs, render: (c) => formatNumber(c.pvcs) },
      { key: 'pvs', header: t('columns.pvs'), sortValue: (c) => c.pvs, render: (c) => formatNumber(c.pvs) },
      {
        key: 'requested',
        header: t('columns.requested'),
        sortValue: (c) => c.requestedBytes,
        render: (c) => formatBytes(c.requestedBytes),
      },
      { key: 'age', header: t('columns.age'), render: (c) => c.age },
    ],
    [t, defaultClass],
  )
  return (
    <Table
      columns={columns}
      rows={classes}
      rowKey={(c) => c.name}
      onRowClick={(c) => openResource('StorageClass', undefined, c.name)}
    />
  )
}
