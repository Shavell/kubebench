import { useId, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { TabDescriptor } from '@/core/plugin/types'
import { usePluginQuery } from '@/data/pluginData'
import { Checkbox, Field, KeyValueEditor, recordFromPairs, Select, TextInput, type KeyValuePair } from '@/shared/form'
import { storageData } from '../data'
import type { StorageClassRequest } from '../data/types'
import { NS } from '../i18n'
import { storageClassObject } from '../model/build'
import { provisionerHint, PROVISIONERS, type ParameterHint } from '../model/provisioners'
import { storageClassProblems } from '../model/rules'
import { LabelsField } from './fields'
import { ProvisionerHintBox } from './ProvisionerHintBox'
import { useWizard } from './useWizard'
import { WizardLayout } from './WizardLayout'

interface ClassForm {
  name: string
  provisioner: string
  parameters: KeyValuePair[]
  reclaimPolicy: string
  volumeBindingMode: string
  allowVolumeExpansion: boolean
  isDefault: boolean
  mountOptions: string
  labels: KeyValuePair[]
}

const START: ClassForm = {
  name: '',
  provisioner: '',
  parameters: [],
  reclaimPolicy: 'Delete',
  volumeBindingMode: 'WaitForFirstConsumer',
  allowVolumeExpansion: false,
  isDefault: false,
  mountOptions: '',
  labels: [],
}

function toRequest(form: ClassForm): StorageClassRequest {
  const optional = <T extends object>(value: T) => (Object.keys(value).length ? value : undefined)
  return {
    name: form.name.trim(),
    provisioner: form.provisioner.trim(),
    parameters: optional(recordFromPairs(form.parameters)),
    reclaimPolicy: form.reclaimPolicy,
    volumeBindingMode: form.volumeBindingMode,
    allowVolumeExpansion: form.allowVolumeExpansion || undefined,
    isDefault: form.isDefault || undefined,
    mountOptions: optional(form.mountOptions.split(',').map((o) => o.trim()).filter(Boolean)),
    labels: optional(recordFromPairs(form.labels)),
  }
}

/** New StorageClass, with provisioner-specific parameter suggestions for well-known drivers. */
export function ClassWizard({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const listId = useId()
  const [form, setForm] = useState(START)
  const set = <K extends keyof ClassForm>(key: K, value: ClassForm[K]) => setForm((f) => ({ ...f, [key]: value }))
  const drivers = usePluginQuery(() => storageData().listCsiDrivers(), [])
  const overview = usePluginQuery(() => storageData().overview(), [])

  const request = useMemo(() => toRequest(form), [form])
  const object = useMemo(() => storageClassObject(request), [request])
  const problems = useMemo(() => storageClassProblems(request), [request])
  const wizard = useWizard({
    tabId: tab.id,
    object,
    problems,
    dirty: JSON.stringify(form) !== JSON.stringify(START),
    submit: (dryRun) => storageData().createStorageClass(request, { dryRun }),
  })

  const hint = provisionerHint(form.provisioner)
  const installed = (drivers.data ?? []).map((d) => d.name)
  const suggestions = [...installed, ...PROVISIONERS.map((p) => p.name).filter((n) => !installed.includes(n))]

  /** Picking a known provisioner applies its binding mode and expansion support. */
  const setProvisioner = (value: string) => {
    const known = provisionerHint(value)
    setForm((f) =>
      known
        ? { ...f, provisioner: value, volumeBindingMode: known.bindingMode, allowVolumeExpansion: known.expansion }
        : { ...f, provisioner: value },
    )
  }
  const addParameter = (p: ParameterHint) =>
    setForm((f) =>
      f.parameters.some((pair) => pair.key === p.key)
        ? f
        : { ...f, parameters: [...f.parameters.filter((pair) => pair.key || pair.value), { key: p.key, value: p.example }] },
    )
  const existingDefault = overview.data?.defaultClass

  return (
    <WizardLayout
      title={t('wizard.class.title')}
      yaml={wizard.yaml}
      status={wizard.status}
      busy={wizard.busy}
      onValidate={wizard.validate}
      onCreate={wizard.create}
    >
      <Field label={t('fields.name')} help={t('fields.nameHelp')} error={wizard.problemOf('name')} required>
        <TextInput value={form.name} onChange={(v) => set('name', v)} monospace autoComplete="off" data-autofocus />
      </Field>
      <Field label={t('fields.provisioner')} help={t('wizard.class.provisionerHelp')} error={wizard.problemOf('provisioner')} required>
        <TextInput value={form.provisioner} onChange={setProvisioner} monospace list={`${listId}provisioners`} />
      </Field>
      <datalist id={`${listId}provisioners`}>
        {suggestions.map((name) => (
          <option key={name} value={name} label={provisionerHint(name)?.title} />
        ))}
      </datalist>
      {hint && <ProvisionerHintBox hint={hint} onUseParameter={addParameter} />}

      <Field label={t('fields.parameters')} help={t('wizard.class.parametersHelp')} error={wizard.problemOf('parameters')}>
        <KeyValueEditor value={form.parameters} onChange={(v) => set('parameters', v)} addLabel={t('fields.addParameter')} />
      </Field>
      <div className="storage-wizard__row">
        <Field label={t('fields.reclaimPolicy')} help={t('wizard.class.reclaimHelp')}>
          <Select
            value={form.reclaimPolicy}
            onChange={(v) => set('reclaimPolicy', v)}
            options={['Delete', 'Retain'].map((p) => ({ value: p, label: p }))}
          />
        </Field>
        <Field label={t('fields.bindingMode')} help={t('wizard.class.bindingHelp')}>
          <Select
            value={form.volumeBindingMode}
            onChange={(v) => set('volumeBindingMode', v)}
            options={['WaitForFirstConsumer', 'Immediate'].map((m) => ({ value: m, label: m }))}
          />
        </Field>
      </div>
      <Checkbox
        label={t('fields.allowExpansion')}
        description={hint && !hint.expansion ? t('wizard.class.noExpansionHint', { name: hint.title }) : t('wizard.class.expansionHelp')}
        checked={form.allowVolumeExpansion}
        onChange={(v) => set('allowVolumeExpansion', v)}
      />
      <Checkbox
        label={t('fields.isDefault')}
        description={
          existingDefault ? t('wizard.class.replacesDefault', { name: existingDefault }) : t('wizard.class.defaultHelp')
        }
        checked={form.isDefault}
        onChange={(v) => set('isDefault', v)}
      />
      <Field label={t('fields.mountOptions')} help={t('fields.mountOptionsHelp')} error={wizard.problemOf('mountOptions')}>
        <TextInput value={form.mountOptions} onChange={(v) => set('mountOptions', v)} monospace />
      </Field>
      <LabelsField value={form.labels} onChange={(v) => set('labels', v)} error={wizard.problemOf('labels')} />
    </WizardLayout>
  )
}
