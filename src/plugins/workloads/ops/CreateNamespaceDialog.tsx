import { useState } from 'react'
import { refreshData, useProviders } from '@/data/DataContext'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field, KeyValueEditor, keyValueErrors, recordFromPairs, TextInput, type KeyValuePair } from '@/shared/form'
import { notify } from '@/shared/notify'
import { useWorkloadsTranslation } from '../i18n'
import { labelValueError, namespaceNameError, qualifiedNameError } from './validation'

/** A new namespace: a DNS-1123 label name and optional labels. */
export function CreateNamespaceDialog({ onClose }: { onClose: () => void }) {
  const { t } = useWorkloadsTranslation()
  const { workloadOps } = useProviders()
  const [name, setName] = useState('')
  const [labels, setLabels] = useState<KeyValuePair[]>([])
  const [attempted, setAttempted] = useState(false)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const trimmed = name.trim()
  const nameError = namespaceNameError(trimmed)
  const valueErrors = labels.map((pair) => (pair.key.trim() ? labelValueError(pair.value) : null))
  const labelsInvalid = keyValueErrors(labels, qualifiedNameError).some(Boolean) || valueErrors.some(Boolean)

  const create = async () => {
    setAttempted(true)
    if (nameError || labelsInvalid) return
    setBusy(true)
    setFailure(null)
    try {
      await workloadOps.createNamespace(trimmed, recordFromPairs(labels))
    } catch (err) {
      setFailure(err)
      setBusy(false)
      return
    }
    notify.success(t('workloadOps.namespace.created', { name: trimmed }))
    refreshData()
    onClose()
  }

  return (
    <Dialog
      open
      icon="folder"
      title={t('workloadOps.namespace.createTitle')}
      busy={busy}
      onClose={onClose}
      onSubmit={() => void create()}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" busy={busy}>
            {t('common.create')}
          </Button>
        </>
      }
    >
      <Field
        label={t('workloadOps.namespace.name')}
        help={t('workloadOps.namespace.nameHelp')}
        error={attempted ? nameError : null}
        required
      >
        <TextInput value={name} onChange={setName} monospace autoComplete="off" disabled={busy} />
      </Field>
      <Field
        label={t('workloadOps.namespace.labels')}
        error={attempted ? valueErrors.find(Boolean) : null}
      >
        <KeyValueEditor
          value={labels}
          onChange={setLabels}
          validateKey={qualifiedNameError}
          addLabel={t('workloadOps.namespace.addLabel')}
          disabled={busy}
        />
      </Field>
      <ErrorNotice error={failure} />
    </Dialog>
  )
}
