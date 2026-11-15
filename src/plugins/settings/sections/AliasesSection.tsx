import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { aliasError, type Alias } from '@/core/palette/kubectl'
import { useAliases } from '@/core/store/aliasStore'
import { Button } from '@/shared/Button'
import { Field, TextInput } from '@/shared/form'
import { Icon } from '@/shared/Icon'
import { SETTINGS_NS } from '../namespace'
import { SettingsSection } from '../SettingsSection'

const EMPTY: Alias = { name: '', expansion: '' }

/**
 * The command palette's aliases (`kgp` → `get pods`), kept in this browser
 * for the signed-in user. An alias replaces the first word of what is
 * typed; the rest is appended (`kgp -n shop` → `get pods -n shop`).
 */
export function AliasesSection() {
  const { t } = useTranslation(SETTINGS_NS)
  const { t: tp } = useTranslation(PLATFORM_NS)
  const { aliases, setAliases } = useAliases()
  const [draft, setDraft] = useState<Alias>(EMPTY)
  /** Name of the alias being edited; null while adding a new one. */
  const [editing, setEditing] = useState<string | null>(null)
  const [submitted, setSubmitted] = useState(false)

  const others = aliases.filter((a) => a.name !== editing)
  const errorKey = aliasError(draft, others)
  const error = submitted && errorKey ? tp(errorKey) : undefined

  const reset = () => {
    setDraft(EMPTY)
    setEditing(null)
    setSubmitted(false)
  }

  const save = () => {
    setSubmitted(true)
    if (errorKey) return
    const alias = { name: draft.name.trim(), expansion: draft.expansion.trim() }
    setAliases(editing === null ? [...aliases, alias] : aliases.map((a) => (a.name === editing ? alias : a)))
    reset()
  }

  return (
    <SettingsSection title={t('sections.aliases')}>
      <p className="settings-aliases__help">{t('aliases.help')}</p>
      {aliases.length === 0 ? (
        <p className="settings-aliases__empty">{t('aliases.empty')}</p>
      ) : (
        <ul className="settings-aliases__list" aria-label={t('sections.aliases')}>
          {aliases.map((alias) => (
            <li key={alias.name} className="settings-aliases__item">
              <code className="settings-aliases__name">{alias.name}</code>
              <Icon name="arrow-right" size={12} />
              <code className="settings-aliases__expansion">{alias.expansion}</code>
              <span className="settings-aliases__actions">
                <Button
                  variant="secondary"
                  icon="edit"
                  aria-label={t('aliases.editLabel', { name: alias.name })}
                  title={t('aliases.edit')}
                  onClick={() => {
                    setDraft(alias)
                    setEditing(alias.name)
                    setSubmitted(false)
                  }}
                />
                <Button
                  variant="secondary"
                  icon="trash"
                  aria-label={t('aliases.removeLabel', { name: alias.name })}
                  title={t('aliases.remove')}
                  onClick={() => {
                    setAliases(aliases.filter((a) => a.name !== alias.name))
                    if (editing === alias.name) reset()
                  }}
                />
              </span>
            </li>
          ))}
        </ul>
      )}
      <form
        className="settings-aliases__form"
        noValidate
        onSubmit={(e) => {
          e.preventDefault()
          save()
        }}
      >
        <Field label={t('aliases.name')} className="settings-aliases__field settings-aliases__field--name">
          <TextInput monospace value={draft.name} placeholder="kgp" onChange={(name) => setDraft({ ...draft, name })} />
        </Field>
        <Field label={t('aliases.expansion')} error={error} className="settings-aliases__field">
          <TextInput
            monospace
            value={draft.expansion}
            placeholder="get pods -n kube-system"
            onChange={(expansion) => setDraft({ ...draft, expansion })}
          />
        </Field>
        <div className="settings-aliases__buttons">
          {editing !== null && (
            <Button variant="secondary" onClick={reset}>
              {t('aliases.cancel')}
            </Button>
          )}
          <Button type="submit" icon={editing === null ? 'add' : 'check'}>
            {editing === null ? t('aliases.add') : t('aliases.save')}
          </Button>
        </div>
      </form>
    </SettingsSection>
  )
}
