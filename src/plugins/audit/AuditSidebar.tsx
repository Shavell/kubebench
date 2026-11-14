import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { formatDateTime } from '@/i18n/format'
import { Tree, type TreeNode } from '@/shared/Tree'
import { AUDIT_NS } from './namespace'
import { statusIcon } from './status'
import { AUDIT_TAB_KIND, openAuditTab } from './tabs'
import './audit.css'

const RECENT = 25

/** The latest audit entries at a glance; any row opens the full log. */
export function AuditSidebar() {
  const { t } = useTranslation(AUDIT_NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const { data, error } = useProviderQuery((p) => p.audit.listAudit({ limit: RECENT }), [])

  const nodes: TreeNode[] = [
    {
      id: AUDIT_TAB_KIND,
      label: t('sidebar.log'),
      icon: 'history',
      children: (data?.items ?? []).map((e) => ({
        id: `audit:${e.id}`,
        label: t('sidebar.entry', { user: e.user, action: e.action }),
        icon: statusIcon(e.status),
        suffix: <span className="audit-sidebar__time">{formatDateTime(e.ts, { timeStyle: 'short' })}</span>,
      })),
    },
  ]

  return (
    <>
      <Tree nodes={nodes} selectedId={activeTabId} onSelect={openAuditTab} />
      {error && <p className="audit-sidebar__error">{error}</p>}
    </>
  )
}
