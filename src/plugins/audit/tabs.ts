import { useLayoutStore } from '@/core/store/layoutStore'
import { i18n } from '@/i18n'
import { AUDIT_NS } from './namespace'

export const AUDIT_TAB_KIND = 'audit'

export function openAuditTab(): void {
  useLayoutStore.getState().openTab({
    id: AUDIT_TAB_KIND,
    title: i18n.t('tab.title', { ns: AUDIT_NS }),
    titleKey: 'tab.title',
    titleNs: AUDIT_NS,
    icon: 'history',
    kind: AUDIT_TAB_KIND,
  })
}
