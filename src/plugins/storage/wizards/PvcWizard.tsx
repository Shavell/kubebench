import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { TabDescriptor } from '@/core/plugin/types'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { useProviderQuery } from '@/data/DataContext'
import { usePluginQuery } from '@/data/pluginData'
import { Field, recordFromPairs, Select, TextInput, type KeyValuePair } from '@/shared/form'
import { Unavailable } from '@/shared/Unavailable'
import { storageData } from '../data'
import type { DataSourceRef, PvcRequest, PvcSummary, PvSummary, VolumeSnapshot } from '../data/types'
import { NS } from '../i18n'
import { pvcObject } from '../model/build'
import { sizeText, splitSize, type SizeUnit } from '../model/quantity'
import { pvcProblems } from '../model/rules'
import type { PvcWizardProps } from '../tabs'
import { AccessModesField, LabelsField, VolumeModeField } from './fields'
import { SizeInput } from './SizeInput'
import { useWizard } from './useWizard'
import { WizardLayout } from './WizardLayout'

/** Choices of the class select besides the classes themselves. */
const DEFAULT_CLASS = '\u0000default'
const NO_CLASS = '\u0000none'

type SourceMode = 'new' | 'bind' | 'clone' | 'restore'

interface PvcForm {
  namespace: string
  name: string
  storageClass: string
  accessModes: string[]
  amount: string
  unit: SizeUnit
  volumeMode: string
  source: SourceMode
  volumeName: string
  sourcePvc: string
  snapshot: string
  labels: KeyValuePair[]
}

function initialForm(props: PvcWizardProps, globalNamespace: string): PvcForm {
  const source: SourceMode = props.volumeName
    ? 'bind'
    : props.dataSource?.kind === 'PersistentVolumeClaim'
      ? 'clone'
      : props.dataSource?.kind === 'VolumeSnapshot'
        ? 'restore'
        : 'new'
  const className = props.storageClassName
  return {
    namespace: props.namespace || globalNamespace || 'default',
    name: '',
    storageClass: className === undefined ? DEFAULT_CLASS : className === '' ? NO_CLASS : className,
    accessModes: ['ReadWriteOnce'],
    amount: '1',
    unit: 'Gi',
    volumeMode: 'Filesystem',
    source,
    volumeName: props.volumeName ?? '',
    sourcePvc: source === 'clone' ? (props.dataSource?.name ?? '') : '',
    snapshot: source === 'restore' ? (props.dataSource?.name ?? '') : '',
    labels: [],
  }
}

function toRequest(form: PvcForm): PvcRequest {
  const dataSource: DataSourceRef | undefined =
    form.source === 'clone' && form.sourcePvc
      ? { kind: 'PersistentVolumeClaim', name: form.sourcePvc }
      : form.source === 'restore' && form.snapshot
        ? { kind: 'VolumeSnapshot', name: form.snapshot }
        : undefined
  const labels = recordFromPairs(form.labels)
  return {
    namespace: form.namespace.trim(),
    name: form.name.trim(),
    storageClassName:
      form.storageClass === DEFAULT_CLASS ? undefined : form.storageClass === NO_CLASS ? '' : form.storageClass,
    accessModes: form.accessModes,
    size: sizeText(form.amount, form.unit),
    volumeMode: form.volumeMode,
    volumeName: form.source === 'bind' ? form.volumeName || undefined : undefined,
    dataSource,
    labels: Object.keys(labels).length ? labels : undefined,
  }
}

/** New PersistentVolumeClaim: a class (or the default, or none), size, modes, and optionally a volume to bind or a source to clone. */
export function PvcWizard({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const globalNamespace = useContextStore((s) => s.namespace)
  const props = (tab.props ?? {}) as PvcWizardProps
  const [start, setStart] = useState(() => initialForm(props, globalNamespace === ALL_NAMESPACES ? '' : globalNamespace))
  const [form, setForm] = useState(start)
  const [prefilled, setPrefilled] = useState(false)
  const set = <K extends keyof PvcForm>(key: K, value: PvcForm[K]) => setForm((f) => ({ ...f, [key]: value }))

  const namespaces = useProviderQuery((p) => p.cluster.listNamespaces(), [])
  const overview = usePluginQuery(() => storageData().overview(), [])
  const volumes = usePluginQuery(() => storageData().listPvs(), [])
  const claims = usePluginQuery(() => storageData().listPvcs(form.namespace), [form.namespace])
  const snapshots = usePluginQuery(
    () => (form.source === 'restore' ? storageData().listSnapshots(form.namespace) : Promise.resolve([])),
    [form.source, form.namespace],
  )

  // A wizard opened for a volume or a source claim takes its size and class
  // once they are loaded (adjusting state during render, not in an effect).
  const snapshotsSettled = form.source !== 'restore' || snapshots.data !== null || snapshots.error !== null
  if (!prefilled && volumes.data && claims.data && snapshotsSettled) {
    const next = prefill(form, volumes.data, claims.data, snapshots.data ?? [])
    setPrefilled(true)
    setForm(next)
    setStart(next)
  }

  const request = useMemo(() => toRequest(form), [form])
  const object = useMemo(() => pvcObject(request), [request])
  const problems = useMemo(() => pvcProblems(request), [request])
  const wizard = useWizard({
    tabId: tab.id,
    object,
    problems,
    dirty: JSON.stringify(form) !== JSON.stringify(start),
    submit: (dryRun) => storageData().createPvc(request, { dryRun }),
  })

  const classes = overview.data?.classes ?? []
  const defaultClass = overview.data?.defaultClass ?? ''
  const classOptions = [
    {
      value: DEFAULT_CLASS,
      label: defaultClass ? t('wizard.pvc.defaultClassNamed', { name: defaultClass }) : t('wizard.pvc.defaultClassNone'),
    },
    ...classes.map((c) => ({
      value: c.name,
      label: c.name === defaultClass ? t('wizard.pvc.classDefaultMarker', { name: c.name }) : c.name,
    })),
    { value: NO_CLASS, label: t('wizard.pvc.noClass') },
  ]
  // A class named in the props may not be listed (yet); keep it selectable.
  if (![DEFAULT_CLASS, NO_CLASS].includes(form.storageClass) && !classes.some((c) => c.name === form.storageClass)) {
    classOptions.splice(1, 0, { value: form.storageClass, label: form.storageClass })
  }

  const available = (volumes.data ?? []).filter((v) => v.phase === 'Available' && !v.claim)
  const pickVolume = (name: string) => {
    const pv = available.find((v) => v.name === name)
    setForm((f) => (pv ? bindTo(f, pv) : { ...f, volumeName: name }))
  }
  const pickSourcePvc = (name: string) => {
    const src = (claims.data ?? []).find((c) => c.name === name)
    setForm((f) => ({
      ...f,
      sourcePvc: name,
      ...(src ? splitSize(src.requested) : {}),
      storageClass: src?.storageClass ? src.storageClass : f.storageClass,
    }))
  }
  const pickSnapshot = (name: string) => {
    const snap = (snapshots.data ?? []).find((s) => s.name === name)
    setForm((f) => ({ ...f, snapshot: name, ...(snap?.restoreSize ? splitSize(snap.restoreSize) : {}) }))
  }

  const namespaceOptions = (namespaces.data ?? []).map((n) => ({ value: n.name, label: n.name }))
  if (!namespaceOptions.some((o) => o.value === form.namespace)) namespaceOptions.unshift({ value: form.namespace, label: form.namespace })
  const pick = (label: string) => ({ value: '', label })

  return (
    <WizardLayout
      title={t('wizard.pvc.title')}
      yaml={wizard.yaml}
      status={wizard.status}
      busy={wizard.busy}
      onValidate={wizard.validate}
      onCreate={wizard.create}
    >
      <div className="storage-wizard__row">
        <Field label={t('fields.namespace')} error={wizard.problemOf('namespace')} required>
          <Select value={form.namespace} onChange={(v) => set('namespace', v)} options={namespaceOptions} />
        </Field>
        <Field label={t('fields.name')} help={t('fields.nameHelp')} error={wizard.problemOf('name')} required>
          <TextInput value={form.name} onChange={(v) => set('name', v)} monospace autoComplete="off" data-autofocus />
        </Field>
      </div>

      <Field label={t('fields.source')}>
        <Select
          value={form.source}
          onChange={(v) => set('source', v)}
          options={(['new', 'bind', 'clone', 'restore'] as const).map((m) => ({
            value: m,
            label: t(`wizard.pvc.sources.${m}`),
            disabled: m === 'restore' && overview.data?.snapshotsAvailable === false,
          }))}
        />
      </Field>

      {form.source === 'bind' && (
        <Field label={t('fields.volume')} help={t('wizard.pvc.bindHelp')} error={wizard.problemOf('volumeName')} required>
          <Select
            value={form.volumeName}
            onChange={pickVolume}
            options={[
              pick(available.length ? t('wizard.pvc.pickVolume') : t('wizard.pvc.noAvailableVolumes')),
              ...available.map((v) => ({
                value: v.name,
                label: `${v.name} · ${v.capacity} · ${v.storageClass || t('wizard.pvc.noClassShort')} · ${v.accessModes.join(', ')}`,
              })),
              ...(form.volumeName && !available.some((v) => v.name === form.volumeName)
                ? [{ value: form.volumeName, label: form.volumeName }]
                : []),
            ]}
          />
        </Field>
      )}
      {form.source === 'clone' && (
        <Field label={t('fields.sourcePvc')} help={t('wizard.pvc.cloneHelp')} error={wizard.problemOf('dataSource.name')} required>
          <Select
            value={form.sourcePvc}
            onChange={pickSourcePvc}
            options={[
              pick(t('wizard.pvc.pickPvc')),
              ...(claims.data ?? []).map((c) => ({ value: c.name, label: `${c.name} · ${c.requested}` })),
              ...(form.sourcePvc && !(claims.data ?? []).some((c) => c.name === form.sourcePvc)
                ? [{ value: form.sourcePvc, label: form.sourcePvc }]
                : []),
            ]}
          />
        </Field>
      )}
      {form.source === 'restore' &&
        (snapshots.error ? (
          <Unavailable compact code={snapshots.errorCode} reason={snapshots.error} title={t('snapshots.unavailable')} />
        ) : (
          <Field label={t('fields.snapshot')} help={t('wizard.pvc.restoreHelp')} error={wizard.problemOf('dataSource.name')} required>
            <Select
              value={form.snapshot}
              onChange={pickSnapshot}
              options={[
                pick(t('wizard.pvc.pickSnapshot')),
                ...(snapshots.data ?? []).map((s) => ({
                  value: s.name,
                  label: `${s.name} · ${s.restoreSize || '—'}${s.readyToUse ? '' : ` · ${t('snapshots.notReady')}`}`,
                })),
                ...(form.snapshot && !(snapshots.data ?? []).some((s) => s.name === form.snapshot)
                  ? [{ value: form.snapshot, label: form.snapshot }]
                  : []),
              ]}
            />
          </Field>
        ))}
      <div className="storage-muted">{wizard.problemOf('dataSource')}</div>

      <Field label={t('fields.storageClass')} help={t('wizard.pvc.classHelp')} error={wizard.problemOf('storageClassName')}>
        <Select value={form.storageClass} onChange={(v) => set('storageClass', v)} options={classOptions} />
      </Field>

      <div className="storage-wizard__row">
        <Field label={t('fields.size')} error={wizard.problemOf('size')} required>
          <SizeInput amount={form.amount} unit={form.unit} onAmount={(v) => set('amount', v)} onUnit={(v) => set('unit', v)} />
        </Field>
        <VolumeModeField value={form.volumeMode} onChange={(v) => set('volumeMode', v)} />
      </div>

      <AccessModesField value={form.accessModes} onChange={(v) => set('accessModes', v)} error={wizard.problemOf('accessModes')} />
      <LabelsField value={form.labels} onChange={(v) => set('labels', v)} error={wizard.problemOf('labels')} />
    </WizardLayout>
  )
}

/** Fills class, size, modes and volume mode from the volume to bind, so the claim matches it. */
function bindTo(form: PvcForm, pv: PvSummary): PvcForm {
  return {
    ...form,
    volumeName: pv.name,
    storageClass: pv.storageClass || NO_CLASS,
    ...splitSize(pv.capacity, 'down'), // the claim must fit the volume
    accessModes: pv.accessModes.slice(0, 1),
    volumeMode: pv.volumeMode,
  }
}

/** The form with what the volume to bind or the claim to clone implies. */
function prefill(
  form: PvcForm,
  volumes: PvSummary[],
  claims: PvcSummary[],
  snapshots: VolumeSnapshot[],
): PvcForm {
  if (form.source === 'bind') {
    const pv = volumes.find((v) => v.name === form.volumeName)
    return pv ? bindTo(form, pv) : form
  }
  if (form.source === 'clone') {
    const src = claims.find((c) => c.name === form.sourcePvc)
    return src ? { ...form, ...splitSize(src.requested), storageClass: src.storageClass || form.storageClass } : form
  }
  if (form.source === 'restore') {
    const snap = snapshots.find((s) => s.name === form.snapshot)
    return snap?.restoreSize ? { ...form, ...splitSize(snap.restoreSize) } : form
  }
  return form
}
