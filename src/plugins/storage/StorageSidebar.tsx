import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { usePluginQuery } from '@/data/pluginData'
import { SidebarErrors } from '@/shared/SidebarErrors'
import { Tree, type TreeNode } from '@/shared/Tree'
import { storageData } from './data'
import { NS } from './i18n'
import { listTabId, openClassWizard, openListTab, openPvcWizard, openPvWizard } from './tabs'

type ListNode = 'overview' | 'pvcs' | 'pvs' | 'classes' | 'snapshots'

const LISTS: ListNode[] = ['overview', 'pvcs', 'pvs', 'classes', 'snapshots']

const CREATE_NODES: Record<string, () => void> = {
  'storage-create-pvc': () => openPvcWizard(),
  'storage-create-pv': openPvWizard,
  'storage-create-class': openClassWizard,
}

export function StorageSidebar() {
  const { t } = useTranslation(NS)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const overview = usePluginQuery(() => storageData().overview(), [])
  const data = overview.data

  const list = (id: ListNode, icon: string, count?: number): TreeNode => ({
    id: listTabId(id),
    label: t(`sidebar.${id}`),
    icon,
    count,
  })
  const nodes: TreeNode[] = [
    list('overview', 'database'),
    list('pvcs', 'save', data?.pvcs.total),
    list('pvs', 'save-all', data?.pvs.total),
    list('classes', 'layers', data?.classes.length),
    list('snapshots', 'history'),
    {
      id: 'storage-create',
      label: t('sidebar.create'),
      icon: 'add',
      children: [
        { id: 'storage-create-pvc', label: t('actions.newPvc'), icon: 'save' },
        { id: 'storage-create-pv', label: t('actions.newPv'), icon: 'save-all' },
        { id: 'storage-create-class', label: t('actions.newClass'), icon: 'layers' },
      ],
    },
  ]

  return (
    <>
      <Tree
        nodes={nodes}
        selectedId={activeTabId}
        defaultExpanded={['storage-create']}
        onSelect={(node) => {
          const target = LISTS.find((l) => listTabId(l) === node.id)
          if (target) openListTab(target)
          else CREATE_NODES[node.id]?.()
        }}
      />
      <SidebarErrors errors={[overview.error]} />
    </>
  )
}
