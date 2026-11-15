import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field, TextInput } from '@/shared/form'
import { helmData, type HelmRepo } from '../data'
import { NS } from '../i18n'

const REPO_NAME = /^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$/

/**
 * Adds a chart repository (admins). The server downloads its index first,
 * so a wrong URL or credentials fail here; the credentials are stored on
 * the server and never shown again.
 */
export function AddRepoDialog({ onAdded, onClose }: { onAdded: (repo: HelmRepo) => void; onClose: () => void }) {
  const { t } = useTranslation(NS)
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const nameError = name && !REPO_NAME.test(name) ? t('repos.add.nameInvalid') : undefined
  const urlError = url && !/^https?:\/\/[^\s/]+/.test(url) ? t('repos.add.urlInvalid') : undefined
  const credentialsError = Boolean(username) !== Boolean(password) ? t('repos.add.credentialsBoth') : undefined
  const ready = name && url && !nameError && !urlError && !credentialsError

  const submit = async () => {
    if (!ready || busy) return
    setBusy(true)
    setFailure(null)
    try {
      onAdded(await helmData().addRepo({ name, url, username: username || undefined, password: password || undefined }))
      onClose()
    } catch (err) {
      setFailure(err)
      setBusy(false)
    }
  }

  return (
    <Dialog
      open
      title={t('repos.add.title')}
      description={t('repos.add.description')}
      icon="repo"
      busy={busy}
      onClose={onClose}
      onSubmit={() => void submit()}
      footer={
        <>
          <Button variant="secondary" disabled={busy} onClick={onClose}>
            {t('repos.add.cancel')}
          </Button>
          <Button type="submit" busy={busy} disabled={!ready}>
            {t('repos.add.submit')}
          </Button>
        </>
      }
    >
      <Field label={t('repos.add.name')} required error={nameError}>
        <TextInput value={name} onChange={setName} placeholder="podinfo" monospace autoComplete="off" />
      </Field>
      <Field label={t('repos.add.url')} required error={urlError}>
        <TextInput value={url} onChange={setUrl} placeholder="https://stefanprodan.github.io/podinfo" type="url" monospace />
      </Field>
      <Field label={t('repos.add.username')} help={t('repos.add.credentialsHelp')} error={credentialsError}>
        <TextInput value={username} onChange={setUsername} autoComplete="off" />
      </Field>
      <Field label={t('repos.add.password')}>
        <TextInput value={password} onChange={setPassword} type="password" autoComplete="new-password" />
      </Field>
      <ErrorNotice error={failure} />
    </Dialog>
  )
}
