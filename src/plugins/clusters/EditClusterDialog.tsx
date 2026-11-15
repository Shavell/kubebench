import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { refreshSession } from '@/core/session/refreshSession'
import { isApiError } from '@/data/apiClient'
import { refreshData, useProviders } from '@/data/DataContext'
import { MAX_CLUSTER_NAME_LENGTH } from '@/data/rules'
import type { ClusterEntry, ClusterEnvironment, ClusterUpdate } from '@/data/types'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Checkbox, Field, Select, TextInput } from '@/shared/form'
import { notify } from '@/shared/notify'
import { CloudCredentialsFields } from './CloudCredentialsFields'
import { credentialProblems, emptyDraft, modesFor, toCredentials } from './cloudCredentials'
import { CLUSTERS_NS } from './namespace'

const ENVIRONMENTS: readonly ClusterEnvironment[] = ['', 'development', 'staging', 'production']

/**
 * Admin: rename a cluster, tag its environment, or make it read-only
 * (PATCH /api/clusters/{id}). Only changed fields are sent. When it is the
 * active cluster, the session is re-read so the status bar and the
 * permission gates follow at once.
 */
export function EditClusterDialog({ cluster, onClose }: { cluster: ClusterEntry & { id: number }; onClose: () => void }) {
  const { t } = useTranslation(CLUSTERS_NS)
  const { inventory, session } = useProviders()
  const [name, setName] = useState(cluster.name)
  const [environment, setEnvironment] = useState<ClusterEnvironment>(cluster.environment ?? '')
  const [readOnly, setReadOnly] = useState(Boolean(cluster.readOnly))
  // Cloud credentials (design §7.12) apply to kubeconfigs that sign in to EKS, GKE or AKS.
  const stored = cluster.cloudCredentials
  const cloudSignIn = Boolean(cluster.cloudAuth || stored)
  const [credentials, setCredentials] = useState(() => emptyDraft(stored ? 'keep' : 'server'))
  const [attempted, setAttempted] = useState(false)
  const [nameTaken, setNameTaken] = useState(false)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const trimmed = name.trim()
  const nameError =
    !trimmed || trimmed.length > MAX_CLUSTER_NAME_LENGTH
      ? t('form.errors.name', { max: MAX_CLUSTER_NAME_LENGTH })
      : nameTaken
        ? t('form.errors.nameTaken')
        : null
  const credentialIssues = credentialProblems(t, credentials)

  const submit = async () => {
    setAttempted(true)
    if (nameError || Object.keys(credentialIssues).length > 0) return
    const update: ClusterUpdate = {}
    if (trimmed !== cluster.name) update.name = trimmed
    if (environment !== (cluster.environment ?? '')) update.environment = environment
    if (readOnly !== Boolean(cluster.readOnly)) update.readOnly = readOnly
    const nextCredentials = toCredentials(credentials)
    // "Server identity" only changes something when credentials are stored.
    if (nextCredentials || (nextCredentials === null && stored)) update.credentials = nextCredentials
    if (Object.keys(update).length === 0) {
      onClose()
      return
    }
    setBusy(true)
    setFailure(null)
    try {
      await inventory.updateCluster(cluster.id, update)
    } catch (err) {
      setBusy(false)
      if (isApiError(err, 'conflict')) setNameTaken(true)
      else setFailure(err)
      return
    }
    refreshData()
    if (cluster.active) await refreshSession(session).catch((err: unknown) => notify.error(err))
    notify.success(t('edit.done', { name: trimmed }))
    onClose()
  }

  return (
    <Dialog
      open
      icon="edit"
      title={t('edit.title', { name: cluster.name })}
      busy={busy}
      onClose={onClose}
      onSubmit={() => void submit()}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('actions.cancel')}
          </Button>
          <Button type="submit" busy={busy} disabled={Boolean(nameError)}>
            {t('edit.submit')}
          </Button>
        </>
      }
    >
      <Field label={t('form.name')} error={nameError} required>
        <TextInput
          value={name}
          onChange={(value) => {
            setName(value)
            setNameTaken(false)
          }}
          monospace
          disabled={busy}
        />
      </Field>
      <Field label={t('form.environment')} help={t('form.environmentHelp')}>
        <Select
          value={environment}
          onChange={setEnvironment}
          disabled={busy}
          options={ENVIRONMENTS.map((value) => ({
            value,
            label: value ? t(`environment.${value}`) : t('environment.none'),
          }))}
        />
      </Field>
      <Checkbox
        checked={readOnly}
        onChange={setReadOnly}
        disabled={busy}
        label={t('form.readOnly')}
        description={t('form.readOnlyHelp')}
      />
      {cloudSignIn && (
        <CloudCredentialsFields
          draft={credentials}
          onChange={setCredentials}
          modes={modesFor(cluster.cloudAuth, stored ? ['keep', 'server'] : ['server'])}
          problems={attempted ? credentialIssues : {}}
          disabled={busy}
          stored={stored}
        />
      )}
      <ErrorNotice error={failure} />
    </Dialog>
  )
}
