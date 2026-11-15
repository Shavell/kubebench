import { useId, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { TabDescriptor } from '@/core/plugin/types'
import { useProviderQuery } from '@/data/DataContext'
import { usePluginQuery } from '@/data/pluginData'
import { Checkbox, Field, KeyValueEditor, recordFromPairs, Select, TextInput, type KeyValuePair } from '@/shared/form'
import { storageData } from '../data'
import type { PvRequest, PvSourceKind } from '../data/types'
import { NS } from '../i18n'
import { pvObject } from '../model/build'
import { provisionerHint, PROVISIONERS } from '../model/provisioners'
import { sizeText, type SizeUnit } from '../model/quantity'
import { HOST_PATH_TYPES, pvProblems } from '../model/rules'
import { AccessModesField, CheckboxGroup, LabelsField, VolumeModeField } from './fields'
import { ProvisionerHintBox } from './ProvisionerHintBox'
import { SizeInput } from './SizeInput'
import { useWizard } from './useWizard'
import { WizardLayout } from './WizardLayout'

interface PvForm {
  name: string
  amount: string
  unit: SizeUnit
  accessModes: string[]
  reclaimPolicy: string
  storageClass: string
  volumeMode: string
  mountOptions: string
  source: PvSourceKind
  path: string
  hostPathType: string
  nfsServer: string
  readOnly: boolean
  fsType: string
  driver: string
  volumeHandle: string
  attributes: KeyValuePair[]
  nodes: string[]
  labels: KeyValuePair[]
}

const START: PvForm = {
  name: '',
  amount: '10',
  unit: 'Gi',
  accessModes: ['ReadWriteOnce'],
  reclaimPolicy: 'Retain',
  storageClass: '',
  volumeMode: 'Filesystem',
  mountOptions: '',
  source: 'hostPath',
  path: '',
  hostPathType: 'DirectoryOrCreate',
  nfsServer: '',
  readOnly: false,
  fsType: '',
  driver: '',
  volumeHandle: '',
  attributes: [],
  nodes: [],
  labels: [],
}

const SOURCES: PvSourceKind[] = ['hostPath', 'nfs', 'local', 'csi']

function toRequest(form: PvForm): PvRequest {
  const optional = <T extends object>(value: T) => (Object.keys(value).length ? value : undefined)
  const path = form.path.trim()
  const fsType = form.fsType.trim() || undefined
  return {
    name: form.name.trim(),
    capacity: sizeText(form.amount, form.unit),
    accessModes: form.accessModes,
    reclaimPolicy: form.reclaimPolicy,
    storageClassName: form.storageClass.trim() || undefined,
    volumeMode: form.volumeMode,
    mountOptions: optional(form.mountOptions.split(',').map((o) => o.trim()).filter(Boolean)),
    labels: optional(recordFromPairs(form.labels)),
    hostPath: form.source === 'hostPath' ? { path, type: form.hostPathType || undefined } : undefined,
    nfs: form.source === 'nfs' ? { server: form.nfsServer.trim(), path, readOnly: form.readOnly || undefined } : undefined,
    local: form.source === 'local' ? { path, fsType } : undefined,
    csi:
      form.source === 'csi'
        ? {
            driver: form.driver.trim(),
            volumeHandle: form.volumeHandle.trim(),
            fsType,
            readOnly: form.readOnly || undefined,
            volumeAttributes: optional(recordFromPairs(form.attributes)),
          }
        : undefined,
    // NFS volumes are reachable from every node and take no affinity.
    nodes: form.source !== 'nfs' ? optional(form.nodes) : undefined,
  }
}

/** New PersistentVolume backed by a hostPath, NFS export, local disk or existing CSI volume. */
export function PvWizard({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const listId = useId()
  const [form, setForm] = useState(START)
  const set = <K extends keyof PvForm>(key: K, value: PvForm[K]) => setForm((f) => ({ ...f, [key]: value }))

  const nodes = useProviderQuery((p) => p.cluster.listNodes(), [])
  const overview = usePluginQuery(() => storageData().overview(), [])
  const drivers = usePluginQuery(() => storageData().listCsiDrivers(), [])

  const request = useMemo(() => toRequest(form), [form])
  const object = useMemo(() => pvObject(request), [request])
  const problems = useMemo(() => pvProblems(request), [request])
  const wizard = useWizard({
    tabId: tab.id,
    object,
    problems,
    dirty: JSON.stringify(form) !== JSON.stringify(START),
    submit: (dryRun) => storageData().createPv(request, { dryRun }),
  })

  // Installed drivers first, then the known CSI drivers not installed here.
  const installed = (drivers.data ?? []).map((d) => d.name)
  const driverNames = [...installed, ...PROVISIONERS.filter((p) => p.csi && !installed.includes(p.name)).map((p) => p.name)]
  const hint = form.source === 'csi' ? provisionerHint(form.driver) : undefined
  const pathField = { hostPath: 'hostPath.path', nfs: 'nfs.path', local: 'local.path', csi: '' }[form.source]
  const toggleNode = (name: string, on: boolean) =>
    set('nodes', on ? [...form.nodes, name] : form.nodes.filter((n) => n !== name))

  return (
    <WizardLayout
      title={t('wizard.pv.title')}
      yaml={wizard.yaml}
      status={wizard.status}
      busy={wizard.busy}
      onValidate={wizard.validate}
      onCreate={wizard.create}
    >
      <Field label={t('fields.name')} help={t('fields.nameHelp')} error={wizard.problemOf('name')} required>
        <TextInput value={form.name} onChange={(v) => set('name', v)} monospace autoComplete="off" data-autofocus />
      </Field>
      <div className="storage-wizard__row">
        <Field label={t('fields.capacity')} error={wizard.problemOf('capacity')} required>
          <SizeInput amount={form.amount} unit={form.unit} onAmount={(v) => set('amount', v)} onUnit={(v) => set('unit', v)} />
        </Field>
        <Field label={t('fields.reclaimPolicy')} help={t('fields.reclaimPolicyHelp')}>
          <Select
            value={form.reclaimPolicy}
            onChange={(v) => set('reclaimPolicy', v)}
            options={['Retain', 'Delete'].map((p) => ({ value: p, label: p }))}
          />
        </Field>
      </div>
      <AccessModesField value={form.accessModes} onChange={(v) => set('accessModes', v)} error={wizard.problemOf('accessModes')} />
      <div className="storage-wizard__row">
        <Field label={t('fields.storageClass')} help={t('wizard.pv.classHelp')} error={wizard.problemOf('storageClassName')}>
          <TextInput value={form.storageClass} onChange={(v) => set('storageClass', v)} monospace list={`${listId}classes`} />
        </Field>
        <VolumeModeField value={form.volumeMode} onChange={(v) => set('volumeMode', v)} />
      </div>
      <datalist id={`${listId}classes`}>
        {(overview.data?.classes ?? []).map((c) => (
          <option key={c.name} value={c.name} />
        ))}
      </datalist>
      {wizard.problemOf('volumeMode') && <div className="kb-field__error">{wizard.problemOf('volumeMode')}</div>}

      <div className="storage-wizard__group">
        <Field label={t('fields.sourceType')}>
          <Select value={form.source} onChange={(v) => set('source', v)} options={SOURCES.map((s) => ({ value: s, label: t(`sources.${s}`) }))} />
        </Field>
        {form.source === 'nfs' && (
          <Field label={t('fields.nfsServer')} error={wizard.problemOf('nfs.server')} required>
            <TextInput value={form.nfsServer} onChange={(v) => set('nfsServer', v)} monospace placeholder="nfs.example.internal" />
          </Field>
        )}
        {form.source === 'csi' && (
          <>
            <Field label={t('fields.driver')} help={t('wizard.pv.driverHelp')} error={wizard.problemOf('csi.driver')} required>
              <TextInput value={form.driver} onChange={(v) => set('driver', v)} monospace list={`${listId}drivers`} />
            </Field>
            <datalist id={`${listId}drivers`}>
              {driverNames.map((d) => (
                <option key={d} value={d} />
              ))}
            </datalist>
            {hint && <ProvisionerHintBox hint={hint} />}
            <Field label={t('fields.volumeHandle')} help={t('wizard.pv.volumeHandleHelp')} error={wizard.problemOf('csi.volumeHandle')} required>
              <TextInput value={form.volumeHandle} onChange={(v) => set('volumeHandle', v)} monospace placeholder={hint?.volumeHandle} />
            </Field>
          </>
        )}
        {pathField && (
          <Field label={t(`wizard.pv.path.${form.source}`)} error={wizard.problemOf(pathField)} required>
            <TextInput value={form.path} onChange={(v) => set('path', v)} monospace placeholder={form.source === 'local' ? '/mnt/disks/ssd1' : '/srv/data'} />
          </Field>
        )}
        {form.source === 'hostPath' && (
          <Field label={t('fields.hostPathType')} help={t('wizard.pv.hostPathTypeHelp')}>
            <Select
              value={form.hostPathType}
              onChange={(v) => set('hostPathType', v)}
              options={HOST_PATH_TYPES.map((h) => ({ value: h, label: h || t('wizard.pv.hostPathTypeAny') }))}
            />
          </Field>
        )}
        {(form.source === 'local' || form.source === 'csi') && (
          <Field label={t('fields.fsType')} help={t('wizard.pv.fsTypeHelp')} error={wizard.problemOf(form.source === 'local' ? 'local.fsType' : 'csi.fsType')}>
            <TextInput value={form.fsType} onChange={(v) => set('fsType', v)} monospace placeholder={hint?.fsType ?? 'ext4'} />
          </Field>
        )}
        {(form.source === 'nfs' || form.source === 'csi') && (
          <Checkbox label={t('fields.readOnly')} checked={form.readOnly} onChange={(v) => set('readOnly', v)} />
        )}
        {form.source === 'csi' && (
          <Field label={t('fields.volumeAttributes')}>
            <KeyValueEditor value={form.attributes} onChange={(v) => set('attributes', v)} addLabel={t('fields.addAttribute')} />
          </Field>
        )}
        {form.source !== 'nfs' && (
          <Field
            label={t('fields.nodes')}
            help={form.source === 'local' ? t('wizard.pv.nodesRequired') : t('wizard.pv.nodesOptional')}
            error={wizard.problemOf('nodes')}
            required={form.source === 'local'}
          >
            <CheckboxGroup>
              {(nodes.data ?? []).map((n) => (
                <Checkbox key={n.name} label={n.name} checked={form.nodes.includes(n.name)} onChange={(on) => toggleNode(n.name, on)} />
              ))}
              {nodes.error && <span className="storage-muted">{nodes.error}</span>}
            </CheckboxGroup>
          </Field>
        )}
      </div>

      <Field label={t('fields.mountOptions')} help={t('fields.mountOptionsHelp')} error={wizard.problemOf('mountOptions')}>
        <TextInput value={form.mountOptions} onChange={(v) => set('mountOptions', v)} monospace placeholder="nfsvers=4.1, hard" />
      </Field>
      <LabelsField value={form.labels} onChange={(v) => set('labels', v)} error={wizard.problemOf('labels')} />
    </WizardLayout>
  )
}
