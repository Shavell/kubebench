import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { signOut } from '@/core/session/signOut'
import { useProviders } from '@/data/DataContext'
import { appVersion, useDataMode } from '@/data/mode'
import { Icon } from '@/shared/Icon'
import { useWorkbenchDialog } from '../workbenchDialogs'
import { StatusMenu } from './StatusMenu'
import { useMenuAnchor } from './useMenuAnchor'

/**
 * Demo data: never ambiguous (design §0). In live mode the live-updates
 * item names the connected server instead (LiveItem).
 */
export function DataSourceItem() {
  const { t } = useTranslation(PLATFORM_NS)
  const { mode } = useDataMode()
  if (mode === 'live') return null
  return (
    <div className="status-bar__item" title={t('statusBar.source.mockHint')}>
      <Icon name="beaker" size={13} />
      <span>{t('statusBar.source.mock')}</span>
    </div>
  )
}

/** The version; opens the About dialog. */
export function VersionItem() {
  const { t } = useTranslation(PLATFORM_NS)
  // appVersion() reads the session facts; re-render when they are re-read.
  useDataMode()
  const show = useWorkbenchDialog((s) => s.show)
  return (
    <button
      type="button"
      className="status-bar__item status-bar__item--button"
      title={t('statusBar.version.tooltip')}
      onClick={() => show('about')}
    >
      <span>{t('statusBar.version.label', { version: appVersion() })}</span>
    </button>
  )
}

/** The signed-in user; its menu changes the password or signs out. */
export function UserItem() {
  const { t } = useTranslation(PLATFORM_NS)
  const { user, role, mode } = useDataMode()
  const { session } = useProviders()
  const show = useWorkbenchDialog((s) => s.show)
  const { anchor, toggle, close } = useMenuAnchor()
  if (!user) return null
  const demo = mode === 'mock'

  return (
    <>
      <button
        type="button"
        className="status-bar__item status-bar__item--button"
        title={t('statusBar.user.tooltip', { user })}
        aria-haspopup="menu"
        aria-expanded={Boolean(anchor)}
        onClick={toggle}
      >
        <Icon name="account" size={13} />
        <span>{user}</span>
      </button>
      {anchor && (
        <StatusMenu
          anchor={anchor}
          label={t('statusBar.user.menu')}
          header={t('statusBar.user.signedInAs', {
            user,
            role: t(role === 'admin' ? 'roles.admin' : 'roles.user'),
          })}
          entries={[
            {
              id: 'password',
              label: t('statusBar.user.changePassword'),
              icon: 'key',
              onSelect: () => show('changePassword'),
            },
            { type: 'separator', id: 'sep' },
            {
              id: 'signOut',
              label: t('session.signOut'),
              icon: 'sign-out',
              disabled: demo,
              detail: demo ? t('statusBar.user.demoSignOut') : undefined,
              onSelect: () => void signOut(session),
            },
          ]}
          onClose={close}
        />
      )}
    </>
  )
}
