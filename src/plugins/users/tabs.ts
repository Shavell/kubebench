import { useLayoutStore } from '@/core/store/layoutStore'
import { i18n } from '@/i18n'
import { USERS_NS } from './namespace'

export const USERS_TAB_KIND = 'users'

export function openUsersTab(): void {
  useLayoutStore.getState().openTab({
    id: 'users',
    title: i18n.t('tab.title', { ns: USERS_NS }),
    titleKey: 'tab.title',
    titleNs: USERS_NS,
    icon: 'organization',
    kind: USERS_TAB_KIND,
  })
}
