import { openYamlTab } from '@/core/plugin/editorTabs'
import { useLayoutStore } from '@/core/store/layoutStore'
import type { SecurityResource } from './data/types'
import { NS, translate } from './i18n'
import type { FindingsFilter } from './model/filters'
import { useSecurityStore } from './store'

/**
 * Editor tabs of the security plugin. Objects open in the workbench's
 * views through their tab kinds (`workloads-detail`, `generic-detail`,
 * `yaml`), so the plugin never imports the plugins that render them.
 */

export const TAB_KINDS = {
  overview: 'security-overview',
  findings: 'security-findings',
  checks: 'security-checks',
  accepted: 'security-accepted',
} as const

export type SecurityTab = keyof typeof TAB_KINDS

const TABS: Record<SecurityTab, { titleKey: string; icon: string }> = {
  overview: { titleKey: 'overview.title', icon: 'shield' },
  findings: { titleKey: 'findings.title', icon: 'warning' },
  checks: { titleKey: 'catalog.title', icon: 'checklist' },
  accepted: { titleKey: 'accepted.title', icon: 'pass' },
}

/** Tab id of a page; sidebar nodes use the same ids, so the open one is highlighted. */
export const tabId = (tab: SecurityTab) => TAB_KINDS[tab]

export function openSecurityTab(tab: SecurityTab): void {
  const { titleKey, icon } = TABS[tab]
  useLayoutStore.getState().openTab({
    id: tabId(tab),
    title: translate(titleKey),
    titleKey,
    titleNs: NS,
    icon,
    kind: TAB_KINDS[tab],
  })
}

/** Opens the findings page with the default filter plus `filter`. */
export function openFindings(filter: Partial<FindingsFilter> = {}): void {
  useSecurityStore.getState().resetFilter(filter)
  openSecurityTab('findings')
}

/** Kinds with a rich detail page in the workloads plugin. */
const RICH_KINDS = new Set(['Pod', 'Deployment'])

/** Generic resource browser keys of the other kinds findings are about. */
const CATALOG_KEYS: Record<string, string> = {
  StatefulSet: 'statefulsets',
  DaemonSet: 'daemonsets',
  ReplicaSet: 'replicasets',
  Job: 'jobs',
  CronJob: 'cronjobs',
  ServiceAccount: 'serviceaccounts',
  Role: 'roles',
  ClusterRole: 'clusterroles',
  RoleBinding: 'rolebindings',
  ClusterRoleBinding: 'clusterrolebindings',
}

/**
 * Opens the object of a finding: the rich detail page for pods and
 * deployments, the generic detail page for kinds in the resource
 * catalog, else (namespaces) its YAML.
 */
export function openResource(r: SecurityResource): void {
  const namespace = r.namespace || null
  const openTab = useLayoutStore.getState().openTab
  if (RICH_KINDS.has(r.kind)) {
    openTab({
      id: `${r.kind.toLowerCase()}:${namespace ?? '-'}/${r.name}`,
      title: r.name,
      icon: 'symbol-field',
      kind: 'workloads-detail',
      props: { kind: r.kind, namespace, name: r.name },
    })
  } else if (CATALOG_KEYS[r.kind]) {
    const key = CATALOG_KEYS[r.kind]
    openTab({
      id: `res:${key}:${namespace ?? '-'}/${r.name}`,
      title: r.name,
      icon: 'symbol-field',
      kind: 'generic-detail',
      props: { resourceKey: key, namespace, name: r.name },
    })
  } else {
    openYamlTab({ apiVersion: r.apiVersion, kind: r.kind, namespace: r.namespace, name: r.name })
  }
}
