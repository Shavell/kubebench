import { useLayoutStore } from '@/core/store/layoutStore'
import { i18n } from '@/i18n'
import { AUTH_NS } from './namespace'

export const AUTH_TAB_KIND = 'authentication'

/** Opens Settings > Authentication in its own tab. */
export function openAuthenticationTab(): void {
  useLayoutStore.getState().openTab({
    id: 'authentication',
    title: i18n.t('tab.title', { ns: AUTH_NS }),
    titleKey: 'tab.title',
    titleNs: AUTH_NS,
    icon: 'shield',
    kind: AUTH_TAB_KIND,
  })
}
