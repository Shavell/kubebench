import { useRef, type ChangeEvent } from 'react'
import { useTranslation } from 'react-i18next'
import type { CloudCredentialProvider } from '@/data/types'
import { Button } from '@/shared/Button'
import { Field, Select, Textarea, TextInput } from '@/shared/form'
import { notify } from '@/shared/notify'
import {
  emptyDraft,
  type CredentialDraft,
  type CredentialField,
  type CredentialMode,
} from './cloudCredentials'
import { CLUSTERS_NS } from './namespace'

/** A service-account key is a few KiB; anything far bigger is the wrong file. */
const MAX_KEY_BYTES = 64 << 10

interface CloudCredentialsFieldsProps {
  draft: CredentialDraft
  onChange: (draft: CredentialDraft) => void
  /** The modes to offer, in order. */
  modes: readonly CredentialMode[]
  /** Field problems to show (after a submit attempt). */
  problems: Partial<Record<CredentialField, string>>
  disabled?: boolean
  /** Edit: the provider of the credentials stored now, for the "keep" label. */
  stored?: CloudCredentialProvider
}

/**
 * Per-cluster cloud credentials for EKS, GKE and AKS kubeconfigs (design
 * §7.12). Secret fields are password inputs, never pre-filled: the server
 * does not return stored credentials.
 */
export function CloudCredentialsFields({ draft, onChange, modes, problems, disabled, stored }: CloudCredentialsFieldsProps) {
  const { t } = useTranslation(CLUSTERS_NS)
  const keyFile = useRef<HTMLInputElement>(null)
  const set = (field: CredentialField) => (value: string) => onChange({ ...draft, [field]: value })

  const loadKey = async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    if (file.size > MAX_KEY_BYTES) {
      notify.error(t('credentials.errors.fileTooLarge', { name: file.name }))
      return
    }
    onChange({ ...draft, serviceAccountJson: await file.text() })
  }

  const text = (field: CredentialField, label: string, options: { secret?: boolean; optional?: boolean; placeholder?: string; help?: string } = {}) => (
    <Field label={label} help={options.help} error={problems[field]} required={!options.optional}>
      <TextInput
        type={options.secret ? 'password' : 'text'}
        autoComplete={options.secret ? 'new-password' : 'off'}
        value={draft[field]}
        onChange={set(field)}
        monospace
        placeholder={options.placeholder}
        disabled={disabled}
      />
    </Field>
  )

  return (
    <>
      <Field label={t('credentials.label')} help={t(`credentials.help.${draft.mode}`, { provider: stored ? t(`credentials.providers.${stored}`) : '' })}>
        <Select
          value={draft.mode}
          // A new mode starts empty: secrets typed for another mode are not carried over.
          onChange={(mode) => onChange(emptyDraft(mode))}
          disabled={disabled}
          options={modes.map((mode) => ({
            value: mode,
            label:
              mode === 'keep' && stored
                ? t('credentials.modes.keepStored', { provider: t(`credentials.providers.${stored}`) })
                : t(`credentials.modes.${mode}`),
          }))}
        />
      </Field>
      {draft.mode === 'aws-keys' && (
        <>
          {text('accessKeyId', t('credentials.accessKeyId'), { placeholder: 'AKIA…' })}
          {text('secretAccessKey', t('credentials.secretAccessKey'), { secret: true })}
          {text('sessionToken', t('credentials.sessionToken'), { secret: true, optional: true, help: t('credentials.sessionTokenHelp') })}
        </>
      )}
      {draft.mode === 'aws-profile' && text('profile', t('credentials.profile'), { placeholder: 'default' })}
      {draft.mode === 'gcp' && (
        <>
          <Field label={t('credentials.serviceAccountJson')} error={problems.serviceAccountJson} required>
            <Textarea
              value={draft.serviceAccountJson}
              onChange={set('serviceAccountJson')}
              monospace
              rows={6}
              placeholder={'{"type": "service_account", …}'}
              disabled={disabled}
            />
          </Field>
          <div>
            <Button variant="secondary" icon="folder-opened" disabled={disabled} onClick={() => keyFile.current?.click()}>
              {t('credentials.loadKey')}
            </Button>
            <input ref={keyFile} type="file" accept=".json,application/json" hidden onChange={(event) => void loadKey(event)} />
          </div>
        </>
      )}
      {draft.mode === 'azure' && (
        <>
          {text('tenantId', t('credentials.tenantId'), { placeholder: '00000000-0000-0000-0000-000000000000' })}
          {text('clientId', t('credentials.clientId'), { placeholder: '00000000-0000-0000-0000-000000000000' })}
          {text('clientSecret', t('credentials.clientSecret'), { secret: true })}
        </>
      )}
    </>
  )
}

