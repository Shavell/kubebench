import { useId, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { useProviderQuery } from '@/data/DataContext'
import { Button } from '@/shared/Button'
import { Field, Select, TextInput } from '@/shared/form'
import { NS } from '../i18n'
import type { RequestDraft } from '../model/request'

const VERBS = ['get', 'list', 'watch', 'create', 'update', 'patch', 'delete', 'deletecollection', 'impersonate', 'bind', 'escalate', '*']
const RESOURCES = ['pods', 'secrets', 'configmaps', 'services', 'deployments', 'nodes', 'namespaces', 'serviceaccounts', 'roles', 'rolebindings', 'clusterroles', 'clusterrolebindings', 'persistentvolumes', '*']
const SUBRESOURCES = ['exec', 'log', 'attach', 'portforward', 'scale', 'status', 'token', 'proxy', 'eviction']
const GROUPS = ['apps', 'batch', 'rbac.authorization.k8s.io', 'networking.k8s.io', 'storage.k8s.io', 'admissionregistration.k8s.io', 'certificates.k8s.io', '*']

interface AccessRequestFormProps {
  draft: RequestDraft
  onChange: (draft: RequestDraft) => void
  onSubmit: () => void
  submitLabel: string
  busy?: boolean
}

/** Verb, resource (or URL), namespace and name of an access question. */
export function AccessRequestForm({ draft, onChange, onSubmit, submitLabel, busy }: AccessRequestFormProps) {
  const { t } = useTranslation(NS)
  const listId = useId()
  const namespaces = useProviderQuery((p) => p.cluster.listNamespaces(), [])
  const set = <K extends keyof RequestDraft>(key: K, value: RequestDraft[K]) => onChange({ ...draft, [key]: value })
  const url = draft.mode === 'url'
  const incomplete = !draft.verb.trim() || (url ? !draft.path.trim().startsWith('/') : !draft.resource.trim())

  const submit = (event: FormEvent) => {
    event.preventDefault()
    if (!incomplete) onSubmit()
  }

  const datalist = (id: string, values: string[]) => (
    <datalist id={`${listId}${id}`}>
      {values.map((v) => (
        <option key={v} value={v} />
      ))}
    </datalist>
  )

  return (
    <form className="access-request-form" onSubmit={submit}>
      <Field label={t('request.mode')}>
        <Select
          value={draft.mode}
          onChange={(mode) => set('mode', mode)}
          options={[
            { value: 'resource', label: t('request.modeResource') },
            { value: 'url', label: t('request.modeUrl') },
          ]}
        />
      </Field>
      <Field label={t('request.verb')} required>
        <TextInput value={draft.verb} onChange={(v) => set('verb', v)} monospace list={`${listId}verbs`} />
      </Field>
      {url ? (
        <Field label={t('request.path')} required>
          <TextInput value={draft.path} onChange={(v) => set('path', v)} monospace placeholder="/metrics" />
        </Field>
      ) : (
        <>
          <Field label={t('request.group')}>
            <TextInput
              value={draft.group}
              onChange={(v) => set('group', v)}
              monospace
              list={`${listId}groups`}
              placeholder={t('request.groupPlaceholder')}
            />
          </Field>
          <Field label={t('request.resource')} required>
            <TextInput value={draft.resource} onChange={(v) => set('resource', v)} monospace list={`${listId}resources`} />
          </Field>
          <Field label={t('request.subresource')}>
            <TextInput value={draft.subresource} onChange={(v) => set('subresource', v)} monospace list={`${listId}subresources`} />
          </Field>
          <Field label={t('request.namespace')}>
            <Select
              value={draft.namespace}
              onChange={(ns) => set('namespace', ns)}
              options={[
                { value: '', label: t('request.clusterWide') },
                ...(namespaces.data ?? []).map((n) => ({ value: n.name, label: n.name })),
              ]}
            />
          </Field>
          <Field label={t('request.name')}>
            <TextInput value={draft.name} onChange={(v) => set('name', v)} monospace placeholder={t('request.namePlaceholder')} />
          </Field>
        </>
      )}
      <div className="access-request-form__submit">
        <Button type="submit" icon="search" busy={busy} disabled={incomplete}>
          {submitLabel}
        </Button>
      </div>
      <p className="access-request-form__hint access-muted">{t(url ? 'request.pathHelp' : 'request.hint')}</p>
      {datalist('verbs', VERBS)}
      {datalist('groups', GROUPS)}
      {datalist('resources', RESOURCES)}
      {datalist('subresources', SUBRESOURCES)}
    </form>
  )
}
