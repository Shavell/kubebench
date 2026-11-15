import { useEffect, useState } from 'react'
import { usePermission } from '@/core/permissions'
import { refreshData, useProviders } from '@/data/DataContext'
import type { ObjectRef } from '@/data/types'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import {
  Field,
  KeyValueEditor,
  keyValueErrors,
  pairsFromRecord,
  recordFromPairs,
  type KeyValuePair,
} from '@/shared/form'
import { notify } from '@/shared/notify'
import { useWorkloadsTranslation } from '../i18n'
import { changes, keyProblem, LAST_APPLIED, metadataOf, validLabelValue, type ObjectMetadata } from './metadataRules'
import './editing.css'

/**
 * Edits an object's labels and annotations, read fresh from the cluster.
 * Only the keys that changed are sent (a merge patch), so annotations the
 * dialog does not show, such as kubectl's last-applied-configuration, stay
 * as they are.
 */
export function MetadataDialog({ target, onClose }: { target: ObjectRef; onClose: () => void }) {
  const { t } = useWorkloadsTranslation()
  const { cluster } = useProviders()
  const write = usePermission('write')
  const [original, setOriginal] = useState<ObjectMetadata | null>(null)
  const [labels, setLabels] = useState<KeyValuePair[]>([])
  const [annotations, setAnnotations] = useState<KeyValuePair[]>([])
  const [hidesLastApplied, setHidesLastApplied] = useState(false)
  const [loadError, setLoadError] = useState<unknown>(null)
  const [saveError, setSaveError] = useState<unknown>(null)
  const [saving, setSaving] = useState(false)

  // Read once per opening: a refresh while the user edits must not reset the rows.
  useEffect(() => {
    let cancelled = false
    cluster
      .getYaml(target)
      .then((text) => {
        if (cancelled) return
        const meta = metadataOf(text)
        const { [LAST_APPLIED]: lastApplied, ...shown } = meta.annotations
        setOriginal({ labels: meta.labels, annotations: shown })
        setHidesLastApplied(lastApplied !== undefined)
        setLabels(pairsFromRecord(meta.labels))
        setAnnotations(pairsFromRecord(shown))
      })
      .catch((err: unknown) => {
        if (!cancelled) setLoadError(err)
      })
    return () => {
      cancelled = true
    }
  }, [cluster, target])

  const keysValid = [...keyValueErrors(labels, keyProblem), ...keyValueErrors(annotations, keyProblem)].every((e) => !e)
  const badValue = labels.find((pair) => pair.key.trim() && !validLabelValue(pair.value))
  const labelPatch = original ? changes(original.labels, recordFromPairs(labels)) : {}
  const annotationPatch = original ? changes(original.annotations, recordFromPairs(annotations)) : {}
  const changed = Object.keys(labelPatch).length + Object.keys(annotationPatch).length > 0
  const canSave = Boolean(original) && keysValid && !badValue && changed && write.allowed && !saving

  const save = async () => {
    if (!canSave) return
    setSaving(true)
    setSaveError(null)
    try {
      await cluster.patchMetadata(target, {
        labels: Object.keys(labelPatch).length ? labelPatch : undefined,
        annotations: Object.keys(annotationPatch).length ? annotationPatch : undefined,
      })
      notify.success(t('editing.metadata.saved', { kind: target.kind, name: target.name }))
      refreshData()
      onClose()
    } catch (err) {
      setSaveError(err)
      setSaving(false)
    }
  }

  return (
    <Dialog
      open
      size="large"
      icon="tag"
      title={t('editing.metadata.title', { kind: target.kind, name: target.name })}
      description={t('editing.metadata.description')}
      busy={saving}
      onClose={onClose}
      onSubmit={save}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={saving}>
            {t('common.cancel')}
          </Button>
          <Button type="submit" busy={saving} disabled={!canSave} title={write.reason ?? undefined}>
            {t('common.save')}
          </Button>
        </>
      }
    >
      {loadError !== null ? (
        <ErrorNotice error={loadError} />
      ) : !original ? (
        <div className="kb-editing__loading">{t('editing.metadata.loading')}</div>
      ) : (
        <div className="kb-editing__metadata">
          <Field label={t('common.labels')} error={badValue ? t('editing.metadata.valueInvalid', { key: badValue.key }) : undefined}>
            <KeyValueEditor value={labels} onChange={setLabels} validateKey={keyProblem} disabled={saving} addLabel={t('editing.metadata.addLabel')} />
          </Field>
          <Field label={t('common.annotations')} help={hidesLastApplied ? t('editing.metadata.lastAppliedKept') : undefined}>
            <KeyValueEditor
              value={annotations}
              onChange={setAnnotations}
              validateKey={keyProblem}
              disabled={saving}
              addLabel={t('editing.metadata.addAnnotation')}
            />
          </Field>
        </div>
      )}
      <ErrorNotice error={saveError} />
    </Dialog>
  )
}
