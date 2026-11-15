import { PLATFORM_NS } from '@/core/locales'
import type { ClusterProvider } from '@/data/providers'
import type { ObjectRef } from '@/data/types'
import { i18n } from '@/i18n'
import type { ContextMenuItem } from './contextMenu'
import { copyText } from './fileActions'
import { notify } from './notify'

/**
 * "Copy name" and "Copy YAML" for a row menu. The YAML is the object as
 * the YAML tab shows it by default (Secret values stay redacted); it is
 * read when the item is chosen, so the copy is current.
 */
export function copyMenuItems(target: ObjectRef, cluster: Pick<ClusterProvider, 'getYaml'>): ContextMenuItem[] {
  const t = (key: string) => i18n.t(key, { ns: PLATFORM_NS })
  return [
    { label: t('ui.copy.name'), icon: 'copy', onClick: () => void copyText(target.name) },
    {
      label: t('ui.copy.yaml'),
      icon: 'clippy',
      onClick: () => {
        cluster.getYaml(target).then(copyText, (err: unknown) => notify.error(err))
      },
    },
  ]
}
