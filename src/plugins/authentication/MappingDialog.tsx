import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { isApiError } from '@/data/apiClient'
import { refreshData, useProviders } from '@/data/DataContext'
import type { ExternalAuthSource, GroupMapping, GroupMappingInput, MappingRole } from '@/data/types'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field, Select, TextInput } from '@/shared/form'
import { notify } from '@/shared/notify'
import { MAPPING_ROLES, roleKey, SOURCES, sourceKey } from './labels'
import { AUTH_NS } from './namespace'

interface MappingDialogProps {
  /** The mapping to edit; a new one when absent. */
  mapping?: GroupMapping
  /** Cluster names of the inventory, offered as suggestions. */
  clusters: string[]
  onClose: () => void
}

/** Creates or edits a group mapping: group → global admin, or group → role on a cluster. */
export function MappingDialog({ mapping, clusters, onClose }: MappingDialogProps) {
  const { t } = useTranslation(AUTH_NS)
  const { identity } = useProviders()
  const listId = useId()
  const [group, setGroup] = useState(mapping?.group ?? '')
  const [source, setSource] = useState<ExternalAuthSource | ''>(mapping?.source ?? '')
  const [role, setRole] = useState<MappingRole>(mapping?.role ?? 'viewer')
  const [cluster, setCluster] = useState(mapping?.cluster ?? '')
  const [attempted, setAttempted] = useState(false)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const input: GroupMappingInput = { group: group.trim(), source, role, cluster: role === 'admin' ? '' : cluster.trim() }
  const problems = {
    group: input.group ? null : t('dialog.errors.group'),
    cluster: role !== 'admin' && !input.cluster ? t('dialog.errors.cluster') : null,
  }
  const errors = attempted ? problems : { group: null, cluster: null }
  const unknownCluster = role !== 'admin' && input.cluster !== '' && !clusters.includes(input.cluster)

  const submit = async () => {
    setAttempted(true)
    if (problems.group || problems.cluster) return
    setBusy(true)
    setFailure(null)
    try {
      if (mapping) await identity.updateMapping(mapping.id, input)
      else await identity.createMapping(input)
    } catch (err) {
      setBusy(false)
      setFailure(isApiError(err, 'conflict') ? new Error(t('dialog.errors.duplicate')) : err)
      return
    }
    refreshData()
    notify.success(t(mapping ? 'dialog.updated' : 'dialog.created', { group: input.group }))
    onClose()
  }

  return (
    <Dialog
      open
      icon="organization"
      title={t(mapping ? 'dialog.editTitle' : 'dialog.createTitle')}
      description={t('dialog.description')}
      busy={busy}
      onClose={onClose}
      onSubmit={() => void submit()}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('actions.cancel')}
          </Button>
          <Button type="submit" busy={busy}>
            {t('dialog.submit')}
          </Button>
        </>
      }
    >
      <Field label={t('dialog.group')} help={t('dialog.groupHelp')} error={errors.group} required>
        <TextInput value={group} onChange={setGroup} monospace autoComplete="off" disabled={busy} />
      </Field>
      <Field label={t('dialog.source')} help={t('dialog.sourceHelp')}>
        <Select
          value={source}
          onChange={setSource}
          disabled={busy}
          options={SOURCES.map((value) => ({ value, label: t(sourceKey(value)) }))}
        />
      </Field>
      <Field label={t('dialog.role')} help={t('dialog.roleHelp')}>
        <Select
          value={role}
          onChange={setRole}
          disabled={busy}
          options={MAPPING_ROLES.map((value) => ({ value, label: t(roleKey(value)) }))}
        />
      </Field>
      {role !== 'admin' && (
        <Field
          label={t('dialog.cluster')}
          help={unknownCluster ? t('dialog.unknownCluster') : t('dialog.clusterHelp')}
          error={errors.cluster}
          required
        >
          <TextInput value={cluster} onChange={setCluster} list={listId} monospace autoComplete="off" disabled={busy} />
        </Field>
      )}
      <datalist id={listId}>
        {clusters.map((name) => (
          <option key={name} value={name} />
        ))}
      </datalist>
      <ErrorNotice error={failure} />
    </Dialog>
  )
}
