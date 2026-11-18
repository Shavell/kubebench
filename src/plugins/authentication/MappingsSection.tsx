import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { refreshData, useProviderQuery, useProviders } from '@/data/DataContext'
import type { GroupMapping } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { EmptyState } from '@/shared/EmptyState'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { Table, type Column } from '@/shared/Table'
import { Unavailable } from '@/shared/Unavailable'
import { roleKey, sourceKey } from './labels'
import { MappingDialog } from './MappingDialog'
import { AUTH_NS } from './namespace'

type DialogState = { kind: 'create' } | { kind: 'edit'; mapping: GroupMapping } | { kind: 'delete'; mapping: GroupMapping }

function GrantCell({ mapping }: { mapping: GroupMapping }) {
  const { t } = useTranslation(AUTH_NS)
  if (mapping.role === 'admin') return <Badge tone="info">{t('mappings.globalAdmin')}</Badge>
  return (
    <span className="auth__grant">
      <span>{t('mappings.onCluster', { role: t(roleKey(mapping.role)), cluster: mapping.cluster })}</span>
      {!mapping.clusterExists && (
        <span className="auth__warning" title={t('mappings.unknownClusterHint')}>
          <Icon name="warning" size={12} />
          {t('mappings.unknownCluster')}
        </span>
      )}
    </span>
  )
}

/**
 * The group mappings. They are applied at every external sign-in and
 * replace the role and cluster access of external users, overriding
 * manual edits in Users.
 */
export function MappingsSection() {
  const { t } = useTranslation(AUTH_NS)
  const { identity } = useProviders()
  const mappings = useProviderQuery((p) => p.identity.listMappings(), [])
  const clusters = useProviderQuery((p) => p.inventory.listClusters(), [])
  const [dialog, setDialog] = useState<DialogState | null>(null)
  const close = () => setDialog(null)

  const remove = async (mapping: GroupMapping) => {
    await identity.deleteMapping(mapping.id)
    refreshData()
    notify.success(t('mappings.deleted', { group: mapping.group }))
  }

  const columns: Column<GroupMapping>[] = [
    {
      key: 'group',
      header: t('mappings.columns.group'),
      sortValue: (m) => m.group.toLowerCase(),
      render: (m) => <code className="auth__value">{m.group}</code>,
    },
    { key: 'source', header: t('mappings.columns.source'), render: (m) => t(sourceKey(m.source)) },
    { key: 'grants', header: t('mappings.columns.grants'), render: (m) => <GrantCell mapping={m} /> },
    {
      key: 'actions',
      header: t('mappings.columns.actions'),
      width: '1%',
      render: (m) => (
        <span className="auth__actions">
          <Button variant="secondary" icon="edit" onClick={() => setDialog({ kind: 'edit', mapping: m })}>
            {t('actions.edit')}
          </Button>
          <Button
            variant="danger"
            icon="trash"
            aria-label={t('actions.deleteNamed', { group: m.group })}
            title={t('actions.deleteNamed', { group: m.group })}
            onClick={() => setDialog({ kind: 'delete', mapping: m })}
          />
        </span>
      ),
    },
  ]

  const clusterNames = (clusters.data ?? []).map((c) => c.name)
  const renderBody = () => {
    if (mappings.error) return <Unavailable compact code={mappings.errorCode} reason={mappings.error} onRetry={refreshData} />
    if (!mappings.data) return <div className="auth__loading">{t('page.loading')}</div>
    if (mappings.data.length === 0) return <EmptyState icon="organization" title={t('mappings.empty')} />
    return <Table columns={columns} rows={mappings.data} rowKey={(m) => String(m.id)} />
  }

  return (
    <>
      <p className="auth__notice">
        <Icon name="info" size={13} />
        {t('mappings.notice')}
      </p>
      <div className="auth__toolbar">
        <Button icon="add" onClick={() => setDialog({ kind: 'create' })}>
          {t('mappings.add')}
        </Button>
      </div>
      {renderBody()}
      {dialog?.kind === 'create' && <MappingDialog clusters={clusterNames} onClose={close} />}
      {dialog?.kind === 'edit' && <MappingDialog mapping={dialog.mapping} clusters={clusterNames} onClose={close} />}
      <ConfirmDialog
        open={dialog?.kind === 'delete'}
        danger
        title={t('mappings.deleteTitle', { group: dialog?.kind === 'delete' ? dialog.mapping.group : '' })}
        message={t('mappings.deleteMessage')}
        confirmLabel={t('actions.delete')}
        onConfirm={() => (dialog?.kind === 'delete' ? remove(dialog.mapping) : undefined)}
        onClose={close}
      />
    </>
  )
}
