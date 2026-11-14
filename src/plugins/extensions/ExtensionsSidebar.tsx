import clsx from 'clsx'
import { useTranslation } from 'react-i18next'
import { usePluginStatusStore } from '@/core/plugin/pluginStatusStore'
import { useLayoutStore } from '@/core/store/layoutStore'
import { Icon } from '@/shared/Icon'
import { useExtensions, type Extension, type ExtensionGroup } from './catalog'
import { ExtensionBadges } from './ExtensionBadges'
import { EXTENSIONS_NS } from './namespace'
import { extensionTabId, openExtensionTab } from './tabs'
import './extensions.css'

const GROUPS: ExtensionGroup[] = ['core', 'builtin', 'external']

function ExtensionRow({ extension, selected }: { extension: Extension; selected: boolean }) {
  const inactive = !extension.status.enabled || !extension.status.available
  return (
    <button
      type="button"
      className={clsx('ext-list__row', selected && 'ext-list__row--selected', inactive && 'ext-list__row--inactive')}
      aria-current={selected || undefined}
      onClick={() => openExtensionTab(extension.id, extension.name, extension.manifest?.name)}
    >
      <Icon name={extension.icon} size={20} className="ext-list__icon" />
      <span className="ext-list__text">
        <span className="ext-list__title">
          <span className="ext-list__name">{extension.name}</span>
          {extension.version && <span className="ext-list__version">{extension.version}</span>}
        </span>
        {extension.description && <span className="ext-list__description">{extension.description}</span>}
        <span className="ext-list__badges">
          <ExtensionBadges extension={extension} />
        </span>
      </span>
    </button>
  )
}

/** Every plugin, grouped into core, built-in and external; a click opens its details. */
export function ExtensionsSidebar() {
  const { t } = useTranslation(EXTENSIONS_NS)
  const extensions = useExtensions()
  const loadError = usePluginStatusStore((s) => s.loadError)
  const activeTabId = useLayoutStore((s) => s.activeTabId)

  return (
    <div className="ext-list">
      {loadError && (
        <div className="ext-list__notice" role="alert">
          <Icon name="warning" size={13} />
          <span>{t('sidebar.catalogFailed', { error: loadError })}</span>
        </div>
      )}
      {GROUPS.map((group) => {
        const members = extensions.filter((e) => e.group === group)
        if (members.length === 0) return null
        return (
          <section key={group} aria-label={t(`groups.${group}`)}>
            <h3 className="ext-list__group">
              {t(`groups.${group}`)}
              <span className="ext-list__count">{members.length}</span>
            </h3>
            {members.map((extension) => (
              <ExtensionRow
                key={extension.id}
                extension={extension}
                selected={activeTabId === extensionTabId(extension.id)}
              />
            ))}
          </section>
        )
      })}
    </div>
  )
}
