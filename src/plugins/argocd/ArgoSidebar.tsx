import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { usePluginQuery } from '@/data/pluginData'
import { SidebarErrors } from '@/shared/SidebarErrors'
import { Tree, type TreeNode } from '@/shared/Tree'
import { argoCd } from './data'
import type { ArgoApplicationSummary } from './data/types'
import { NS } from './i18n'
import {
  APPLICATION_SETS_TAB_ID,
  APPLICATIONS_TAB_ID,
  applicationTabId,
  openApplicationSetsTab,
  openApplicationsTab,
  openApplicationTab,
  openProjectsTab,
  PROJECTS_TAB_ID,
} from './tabs'

/** Codicon of an application by health, so problems stand out in the tree. */
function healthIcon(app: ArgoApplicationSummary): string {
  if (app.operating) return 'sync'
  switch (app.health) {
    case 'Healthy':
      return app.sync === 'Synced' ? 'pass' : 'circle-large-outline'
    case 'Degraded':
    case 'Missing':
      return 'error'
    case 'Progressing':
      return 'loading'
    default:
      return 'circle-outline'
  }
}

export function ArgoSidebar() {
  const { t } = useTranslation(NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const apps = usePluginQuery(() => argoCd().listApplications(), [])
  const list = apps.data ?? []

  const projects = [...new Set(list.map((a) => a.project))].sort()
  const appById = new Map(list.map((a) => [applicationTabId(a.namespace, a.name), a]))
  // Node ids equal the ids of the tabs they open, so the open tab is highlighted.
  const nodes: TreeNode[] = [
    {
      id: APPLICATIONS_TAB_ID,
      label: t('sidebar.applications'),
      icon: 'layers',
      count: apps.data?.length,
      children: projects.map((project) => ({
        id: `project:${project}`,
        label: project,
        icon: 'project',
        children: list
          .filter((a) => a.project === project)
          .map((a) => ({
            id: applicationTabId(a.namespace, a.name),
            label: a.name,
            description: a.sync === 'Synced' ? undefined : t(`states.${a.sync}`, { defaultValue: a.sync }),
            icon: healthIcon(a),
          })),
      })),
    },
    { id: PROJECTS_TAB_ID, label: t('sidebar.projects'), icon: 'project' },
    { id: APPLICATION_SETS_TAB_ID, label: t('sidebar.applicationSets'), icon: 'files' },
  ]

  return (
    <>
      {/* Remounted when the projects change, so new project groups start expanded. */}
      <Tree
        key={projects.join('|')}
        nodes={nodes}
        selectedId={activeTabId}
        defaultExpanded={[APPLICATIONS_TAB_ID, ...projects.map((p) => `project:${p}`)]}
        onSelect={(node) => {
          const app = appById.get(node.id)
          if (app) openApplicationTab(app.namespace, app.name)
          else if (node.id === APPLICATIONS_TAB_ID) openApplicationsTab()
          else if (node.id === PROJECTS_TAB_ID) openProjectsTab()
          else if (node.id === APPLICATION_SETS_TAB_ID) openApplicationSetsTab()
        }}
      />
      <SidebarErrors errors={[apps.error]} />
    </>
  )
}
