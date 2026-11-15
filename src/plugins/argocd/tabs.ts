import { openYamlTab } from '@/core/plugin/editorTabs'
import { useLayoutStore } from '@/core/store/layoutStore'
import type { ArgoResource } from './data/types'
import { NS, translate } from './i18n'

/**
 * Editor tabs of the Argo CD plugin, and the tab kinds of other plugins it
 * opens managed resources in (design §7 tab kind contract), so it never
 * imports them.
 */

export const APPLICATIONS_TAB_ID = 'argocd-applications'
export const PROJECTS_TAB_ID = 'argocd-projects'
export const APPLICATION_SETS_TAB_ID = 'argocd-applicationsets'

function openListTab(id: string, titleKey: string, icon: string): void {
  useLayoutStore.getState().openTab({ id, title: translate(titleKey), titleKey, titleNs: NS, icon, kind: id })
}

export const openApplicationsTab = () => openListTab(APPLICATIONS_TAB_ID, 'applications.title', 'layers')
export const openProjectsTab = () => openListTab(PROJECTS_TAB_ID, 'projects.title', 'project')
export const openApplicationSetsTab = () => openListTab(APPLICATION_SETS_TAB_ID, 'applicationSets.title', 'files')

/** Applications are unique per namespace: the namespace is part of the id. */
export function applicationTabId(namespace: string, name: string): string {
  return `argocd-app:${namespace}/${name}`
}

/** Detail tab of one application (`argocd-application`, props `{namespace, name}`). */
export function openApplicationTab(namespace: string, name: string): void {
  useLayoutStore.getState().openTab({
    id: applicationTabId(namespace, name),
    title: name,
    icon: 'layers',
    kind: 'argocd-application',
    props: { namespace, name },
  })
}

/** Kinds with a rich detail page in the workloads plugin (`workloads-detail`). */
const RICH_KINDS: Record<string, string> = { '/Pod': 'Pod', '/Service': 'Service', '/Node': 'Node', 'apps/Deployment': 'Deployment' }

/**
 * Opens a managed resource in KubeBench: the rich detail page for pods,
 * deployments, services and nodes, the generic detail page for kinds in
 * the resource catalog, else its YAML.
 */
export function openManagedResource(r: ArgoResource): void {
  const namespace = r.namespace || null
  const rich = RICH_KINDS[`${r.group}/${r.kind}`]
  const openTab = useLayoutStore.getState().openTab
  if (rich) {
    openTab({
      id: `${rich.toLowerCase()}:${namespace ?? '-'}/${r.name}`,
      title: r.name,
      icon: 'symbol-field',
      kind: 'workloads-detail',
      props: { kind: rich, namespace, name: r.name },
    })
  } else if (r.catalogKey) {
    openTab({
      id: `res:${r.catalogKey}:${namespace ?? '-'}/${r.name}`,
      title: r.name,
      icon: 'symbol-field',
      kind: 'generic-detail',
      props: { resourceKey: r.catalogKey, namespace, name: r.name },
    })
  } else {
    openManagedYaml(r)
  }
}

/** Opens a managed resource's YAML in the resource editor (`yaml` tab kind). */
export function openManagedYaml(r: ArgoResource): void {
  openYamlTab({
    apiVersion: r.group ? `${r.group}/${r.version}` : r.version,
    kind: r.kind,
    namespace: r.namespace || undefined,
    name: r.name,
  })
}
