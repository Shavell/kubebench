import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { refreshData } from '@/data/DataContext'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Checkbox, Field, TextInput } from '@/shared/form'
import { notify } from '@/shared/notify'
import { argoCd } from '../data'
import type { ArgoApplicationDetail, ArgoResource, ArgoSyncRequest } from '../data/types'
import { NS } from '../i18n'
import { StateBadge } from './StatusBadges'

/** Sync options offered as checkboxes, besides the ones the application sets. */
const COMMON_OPTIONS = [
  'CreateNamespace=true',
  'ServerSideApply=true',
  'ApplyOutOfSyncOnly=true',
  'PruneLast=true',
  'Replace=true',
]

const resourceKey = (r: ArgoResource) => `${r.group}/${r.kind}/${r.namespace}/${r.name}`

interface SyncDialogProps {
  app: ArgoApplicationDetail
  onClose: () => void
}

/**
 * Sync request (design §7.5): revision, prune, dry run, sync options
 * (preset from the application, sent only when changed) and an optional
 * selection of resources.
 */
export function SyncDialog({ app, onClose }: SyncDialogProps) {
  const { t } = useTranslation(NS)
  const appOptions = app.syncPolicy.syncOptions
  const [revision, setRevision] = useState('')
  const [prune, setPrune] = useState(false)
  const [dryRun, setDryRun] = useState(false)
  const [options, setOptions] = useState<ReadonlySet<string>>(() => new Set(appOptions))
  const [selective, setSelective] = useState(false)
  const [selected, setSelected] = useState<ReadonlySet<string>>(() => new Set(app.resources.map(resourceKey)))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)

  const multiSource = app.sourceCount > 1
  const allOptions = [...new Set([...appOptions, ...COMMON_OPTIONS])]
  const toggle = (set: ReadonlySet<string>, value: string, on: boolean) => {
    const next = new Set(set)
    if (on) next.add(value)
    else next.delete(value)
    return next
  }
  const optionsChanged = options.size !== appOptions.length || appOptions.some((o) => !options.has(o))
  const nothingSelected = selective && selected.size === 0

  const submit = async () => {
    if (nothingSelected) return
    const request: ArgoSyncRequest = {
      revision: revision.trim() || undefined,
      prune,
      dryRun,
      // Keep the application's options unless the user changed them (the backend copies them in).
      syncOptions: optionsChanged ? allOptions.filter((o) => options.has(o)) : undefined,
      resources: selective
        ? app.resources
            .filter((r) => selected.has(resourceKey(r)))
            .map((r) => ({ group: r.group, kind: r.kind, namespace: r.namespace, name: r.name }))
        : undefined,
    }
    setBusy(true)
    setError(null)
    try {
      await argoCd().syncApplication(app.namespace, app.name, request)
    } catch (err) {
      setError(err)
      setBusy(false)
      return
    }
    notify.success(t(dryRun ? 'sync.startedDryRun' : 'sync.started', { name: app.name }))
    refreshData()
    onClose()
  }

  return (
    <Dialog
      open
      size="large"
      icon="sync"
      title={t('sync.title', { name: app.name })}
      description={t('sync.description')}
      busy={busy}
      onClose={onClose}
      onSubmit={submit}
      footer={
        <>
          <Button variant="secondary" disabled={busy} onClick={onClose}>
            {t('sync.cancel')}
          </Button>
          <Button type="submit" icon="sync" busy={busy} disabled={nothingSelected}>
            {dryRun ? t('sync.submitDryRun') : t('sync.submit')}
          </Button>
        </>
      }
    >
      <Field
        label={t('sync.revision')}
        help={multiSource ? t('sync.revisionMultiSource') : t('sync.revisionHelp', { target: app.targetRevision || 'HEAD' })}
      >
        <TextInput
          value={revision}
          onChange={setRevision}
          monospace
          placeholder={app.targetRevision || 'HEAD'}
          disabled={multiSource || busy}
        />
      </Field>
      <div className="argocd-dialog-row">
        <Checkbox checked={prune} onChange={setPrune} disabled={busy} label={t('sync.prune')} description={t('sync.pruneHelp')} />
        <Checkbox checked={dryRun} onChange={setDryRun} disabled={busy} label={t('sync.dryRun')} description={t('sync.dryRunHelp')} />
      </div>
      <div className="detail__section">{t('sync.options')}</div>
      <div className="argocd-options">
        {allOptions.map((option) => (
          <Checkbox
            key={option}
            checked={options.has(option)}
            onChange={(on) => setOptions((s) => toggle(s, option, on))}
            disabled={busy}
            label={<code>{option}</code>}
          />
        ))}
      </div>
      <div className="detail__section">{t('sync.resources')}</div>
      <Checkbox
        checked={selective}
        onChange={setSelective}
        disabled={busy || app.resources.length === 0}
        label={t('sync.selective')}
        description={t('sync.selectiveHelp')}
      />
      {selective && (
        <div className="argocd-resource-pick" role="group" aria-label={t('sync.resources')}>
          {app.resources.map((r) => (
            <Checkbox
              key={resourceKey(r)}
              checked={selected.has(resourceKey(r))}
              onChange={(on) => setSelected((s) => toggle(s, resourceKey(r), on))}
              disabled={busy}
              label={
                <span className="argocd-resource-pick__label">
                  <span>{`${r.kind} ${r.namespace ? `${r.namespace}/` : ''}${r.name}`}</span>
                  <StateBadge value={r.status} />
                </span>
              }
            />
          ))}
          {nothingSelected && <div className="argocd-muted">{t('sync.noneSelected')}</div>}
        </div>
      )}
      <ErrorNotice error={error} />
    </Dialog>
  )
}
