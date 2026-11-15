import { useLayoutStore } from '@/core/store/layoutStore'
import type { Subject } from './data/types'
import { NS, translate } from './i18n'
import { RESOURCE_KEYS, subjectKey, subjectName } from './model/format'

/**
 * Editor tabs of the access plugin. Roles, bindings and ServiceAccounts
 * open in the workbench's generic resource detail (tab kind
 * `generic-detail`), so the plugin never imports the view that renders them.
 */

export const TAB_KINDS = {
  whoCan: 'access-who-can',
  subjects: 'access-subjects',
  risky: 'access-risky',
  graph: 'access-graph',
  subject: 'access-subject',
} as const

export type ListTab = 'whoCan' | 'subjects' | 'risky' | 'graph'

const LIST_TABS: Record<ListTab, { titleKey: string; icon: string }> = {
  whoCan: { titleKey: 'whoCan.title', icon: 'search' },
  subjects: { titleKey: 'subjects.title', icon: 'organization' },
  risky: { titleKey: 'risky.title', icon: 'warning' },
  graph: { titleKey: 'graph.title', icon: 'type-hierarchy' },
}

export const listTabIcon = (tab: ListTab) => LIST_TABS[tab].icon

/** Tab id of a list page; sidebar nodes use the same ids, so the open one is highlighted. */
export const listTabId = (tab: ListTab) => TAB_KINDS[tab]

export function openListTab(tab: ListTab): void {
  const { titleKey, icon } = LIST_TABS[tab]
  useLayoutStore.getState().openTab({
    id: listTabId(tab),
    title: translate(titleKey),
    titleKey,
    titleNs: NS,
    icon,
    kind: TAB_KINDS[tab],
  })
}

const SUBJECT_ICONS: Record<Subject['kind'], string> = {
  User: 'person',
  Group: 'organization',
  ServiceAccount: 'account',
}

export const subjectIcon = (s: Subject) => SUBJECT_ICONS[s.kind]

/** Opens a subject's effective permissions (one tab per subject). */
export function openSubject(subject: Subject): void {
  const value: Subject = subject.namespace
    ? { kind: subject.kind, name: subject.name, namespace: subject.namespace }
    : { kind: subject.kind, name: subject.name }
  useLayoutStore.getState().openTab({
    id: `${TAB_KINDS.subject}:${subjectKey(value)}`,
    title: subjectName(value),
    icon: subjectIcon(value),
    kind: TAB_KINDS.subject,
    props: { ...value },
  })
}

/** Opens a Role, ClusterRole, binding or ServiceAccount in the generic resource detail view. */
export function openResource(kind: string, namespace: string | undefined, name: string): void {
  const target = RESOURCE_KEYS[kind]
  if (!target) return
  useLayoutStore.getState().openTab({
    id: `res:${target.key}:${namespace || '-'}/${name}`,
    title: name,
    icon: target.icon,
    kind: 'generic-detail',
    props: { resourceKey: target.key, namespace: namespace || null, name },
  })
}
