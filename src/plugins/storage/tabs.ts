import { useLayoutStore } from '@/core/store/layoutStore'
import type { DataSourceRef } from './data/types'
import { NS, translate } from './i18n'

/**
 * Editor tabs of the storage plugin. Created and listed objects open in
 * the workbench's generic resource detail (tab kind `generic-detail`), so
 * the plugin never imports the view that renders them.
 */

export const TAB_KINDS = {
  overview: 'storage-overview',
  pvcs: 'storage-pvcs',
  pvs: 'storage-pvs',
  classes: 'storage-classes',
  snapshots: 'storage-snapshots',
  newPvc: 'storage-new-pvc',
  newPv: 'storage-new-pv',
  newClass: 'storage-new-class',
} as const

type ListTab = 'overview' | 'pvcs' | 'pvs' | 'classes' | 'snapshots'

const LIST_TABS: Record<ListTab, { titleKey: string; icon: string }> = {
  overview: { titleKey: 'overview.title', icon: 'database' },
  pvcs: { titleKey: 'pvcs.title', icon: 'save' },
  pvs: { titleKey: 'pvs.title', icon: 'save-all' },
  classes: { titleKey: 'classes.title', icon: 'layers' },
  snapshots: { titleKey: 'snapshots.title', icon: 'history' },
}

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

/** Initial values of the PVC wizard: a namespace, a volume to bind or a source to clone. */
export interface PvcWizardProps {
  namespace?: string
  volumeName?: string
  storageClassName?: string
  dataSource?: DataSourceRef
}

let wizardSeq = 0

/**
 * Opens a wizard in a new tab each time (two drafts may be open at once).
 * Ids only need to be unique within the session, so a counter suffices.
 */
function openWizard(kind: string, titleKey: string, icon: string, props?: Record<string, unknown>) {
  wizardSeq += 1
  useLayoutStore.getState().openTab({
    id: `${kind}:${wizardSeq}`,
    title: translate(titleKey),
    titleKey,
    titleNs: NS,
    icon,
    kind,
    props,
  })
}

export function openPvcWizard(props: PvcWizardProps = {}): void {
  openWizard(TAB_KINDS.newPvc, 'wizard.pvc.title', 'save', { ...props })
}

export function openPvWizard(): void {
  openWizard(TAB_KINDS.newPv, 'wizard.pv.title', 'save-all')
}

export function openClassWizard(): void {
  openWizard(TAB_KINDS.newClass, 'wizard.class.title', 'layers')
}

/** Generic browser keys of the kinds this plugin creates. */
const RESOURCE_KEYS: Record<string, { key: string; icon: string }> = {
  PersistentVolumeClaim: { key: 'persistentvolumeclaims', icon: 'save' },
  PersistentVolume: { key: 'persistentvolumes', icon: 'save-all' },
  StorageClass: { key: 'storageclasses', icon: 'layers' },
}

/** Opens an object in the generic resource detail view (same tab ids as the resource browser). */
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
