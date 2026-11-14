import { useRef, useState, type ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { isApiError } from '@/data/apiClient'
import { refreshData, useProviders } from '@/data/DataContext'
import { useDataMode } from '@/data/mode'
import { MAX_CLUSTER_NAME_LENGTH } from '@/data/rules'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field, Select, Textarea, TextInput } from '@/shared/form'
import { notify } from '@/shared/notify'
import { CLUSTERS_NS } from './namespace'

type Source = 'content' | 'path' | 'address'

/** Kubeconfigs are small; anything bigger is almost certainly the wrong file. */
const MAX_KUBECONFIG_BYTES = 1 << 20

/**
 * Admin: register a cluster from a pasted (or loaded) kubeconfig, a
 * kubeconfig file on the server (only when the server allows it,
 * `features.pathKubeconfig`), or an API address bookmark.
 */
export function AddClusterDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation(CLUSTERS_NS)
  const { inventory } = useProviders()
  const pathAllowed = Boolean(useDataMode().features?.pathKubeconfig)
  const fileInput = useRef<HTMLInputElement>(null)
  const [source, setSource] = useState<Source>('content')
  const [name, setName] = useState('')
  const [content, setContent] = useState('')
  const [path, setPath] = useState('')
  const [address, setAddress] = useState('')
  const [context, setContext] = useState('')
  const [attempted, setAttempted] = useState(false)
  const [nameTaken, setNameTaken] = useState(false)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const trimmed = name.trim()
  const problems = {
    name:
      !trimmed || trimmed.length > MAX_CLUSTER_NAME_LENGTH
        ? t('form.errors.name', { max: MAX_CLUSTER_NAME_LENGTH })
        : nameTaken
          ? t('form.errors.nameTaken')
          : null,
    content: source === 'content' && !content.trim() ? t('form.errors.content') : null,
    path: source === 'path' && !path.trim() ? t('form.errors.path') : null,
    address: source === 'address' && !/^https?:\/\/\S+$/.test(address.trim()) ? t('form.errors.address') : null,
  }
  const show = (problem: string | null) => (attempted || nameTaken ? problem : null)

  const loadFile = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = '' // the same file can be picked again
    if (!file) return
    if (file.size > MAX_KUBECONFIG_BYTES) {
      notify.error(t('form.errors.fileTooLarge', { name: file.name }))
      return
    }
    setContent(await file.text())
  }

  const submit = async () => {
    setAttempted(true)
    if (Object.values(problems).some(Boolean)) return
    setBusy(true)
    setFailure(null)
    try {
      await inventory.addCluster({
        name: trimmed,
        content: source === 'content' ? content : undefined,
        kubeconfigPath: source === 'path' ? path.trim() : undefined,
        address: source === 'address' ? address.trim() : undefined,
        context: source !== 'address' && context.trim() ? context.trim() : undefined,
      })
    } catch (err) {
      setBusy(false)
      if (isApiError(err, 'conflict')) setNameTaken(true)
      else setFailure(err)
      return
    }
    refreshData()
    notify.success(t('add.done', { name: trimmed }))
    onClose()
  }

  const sourceOptions = [
    { value: 'content' as Source, label: t('add.sources.content') },
    ...(pathAllowed ? [{ value: 'path' as Source, label: t('add.sources.path') }] : []),
    { value: 'address' as Source, label: t('add.sources.address') },
  ]

  return (
    <Dialog
      open
      size="large"
      icon="add"
      title={t('add.title')}
      busy={busy}
      onClose={onClose}
      onSubmit={() => void submit()}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('actions.cancel')}
          </Button>
          <Button type="submit" busy={busy}>
            {t('add.submit')}
          </Button>
        </>
      }
    >
      <Field label={t('form.name')} help={t('add.nameHelp')} error={show(problems.name)} required>
        <TextInput
          value={name}
          onChange={(value) => {
            setName(value)
            setNameTaken(false)
          }}
          monospace
          placeholder="prod-eu-west-1"
          disabled={busy}
        />
      </Field>
      <Field label={t('add.source')} help={t(`add.sourceHelp.${source}`)}>
        <Select value={source} onChange={setSource} options={sourceOptions} disabled={busy} />
      </Field>
      {source === 'content' && (
        <Field label={t('add.kubeconfig')} error={show(problems.content)} required>
          <Textarea
            value={content}
            onChange={setContent}
            monospace
            rows={10}
            placeholder={'apiVersion: v1\nkind: Config\nclusters: …'}
            disabled={busy}
          />
        </Field>
      )}
      {source === 'content' && (
        <div>
          <Button variant="secondary" icon="folder-opened" disabled={busy} onClick={() => fileInput.current?.click()}>
            {t('add.loadFile')}
          </Button>
          <input ref={fileInput} type="file" hidden onChange={(event) => void loadFile(event)} />
        </div>
      )}
      {source === 'path' && (
        <Field label={t('add.path')} error={show(problems.path)} required>
          <TextInput value={path} onChange={setPath} monospace placeholder="/etc/kubebench/prod.kubeconfig" disabled={busy} />
        </Field>
      )}
      {source === 'address' && (
        <Field label={t('add.address')} error={show(problems.address)} required>
          <TextInput
            type="url"
            value={address}
            onChange={setAddress}
            monospace
            placeholder="https://cluster.example.com:6443"
            disabled={busy}
          />
        </Field>
      )}
      {source !== 'address' && (
        <Field label={t('add.context')} help={t('add.contextHelp')}>
          <TextInput value={context} onChange={setContext} monospace disabled={busy} />
        </Field>
      )}
      <ErrorNotice error={failure} />
    </Dialog>
  )
}
