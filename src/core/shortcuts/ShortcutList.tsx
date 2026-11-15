import { Fragment } from 'react'
import { useTranslation } from 'react-i18next'
import { allCommands } from '@/core/commands/coreCommands'
import { PLATFORM_NS } from '@/core/locales'
import { useRegistryRevision } from '@/core/plugin/useRegistry'
import { useManifestText } from '@/i18n'
import { shortcutGroups } from './catalog'
import { KeyCaps } from './KeyCaps'
import './shortcuts.css'

/**
 * Every keyboard shortcut, grouped (the `?` cheat sheet and Settings both
 * show it). Plugin commands with keybindings appear as plugins register.
 */
export function ShortcutList() {
  useRegistryRevision()
  const { t } = useTranslation(PLATFORM_NS)
  const { t: tCore } = useTranslation()
  const manifestText = useManifestText()
  const groups = shortcutGroups(t, allCommands(tCore, manifestText))

  return (
    <div className="kb-shortcuts">
      {groups.map((group) => (
        <section key={group.id} className="kb-shortcuts__group" aria-label={group.title}>
          <h3 className="kb-shortcuts__title">{group.title}</h3>
          <dl className="kb-shortcuts__list">
            {group.entries.map((entry) => (
              <div key={entry.id} className="kb-shortcuts__row">
                <dt>{entry.label}</dt>
                <dd>
                  {entry.keys.map((binding, i) => (
                    <Fragment key={binding}>
                      {i > 0 && <span className="kb-shortcuts__or">/</span>}
                      <KeyCaps binding={binding} />
                    </Fragment>
                  ))}
                </dd>
              </div>
            ))}
          </dl>
        </section>
      ))}
    </div>
  )
}
