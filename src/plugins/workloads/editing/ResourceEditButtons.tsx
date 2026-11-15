import { usePermission } from '@/core/permissions'
import { openNewResourceTab, openYamlTab } from '@/core/plugin/editorTabs'
import type { ResourceRef } from '@/core/plugin/types'
import { Button } from '@/shared/Button'
import { useWorkloadsTranslation } from '../i18n'
import { deletable, isSecret, openResourceDialog, targetOf } from './dialogStore'

/**
 * Detail page toolbar buttons of resource editing (design §7.2): YAML
 * (view and edit), labels and annotations, Secret reveal and the generic
 * delete (`withDelete: false` where the page has its own, e.g. Pods).
 * Mutations are disabled with the reason when not permitted.
 */
export function ResourceEditButtons({
  resource,
  withDelete = true,
  onDeleted,
}: {
  resource: ResourceRef
  withDelete?: boolean
  /** After the object was deleted, e.g. to close its detail tab. */
  onDeleted?: () => void
}) {
  const { t } = useWorkloadsTranslation()
  const write = usePermission('write')
  const exec = usePermission('exec')
  const target = targetOf(resource)
  if (!target) return null

  return (
    <>
      <Button variant="secondary" icon="file-code" onClick={() => openYamlTab({ ...target, namespace: target.namespace ?? undefined })}>
        {t('actions.yaml')}
      </Button>
      <Button
        variant="secondary"
        icon="tag"
        disabled={!write.allowed}
        title={write.reason ?? t('editing.menu.metadata')}
        onClick={() => openResourceDialog({ type: 'metadata', target })}
      >
        {t('editing.buttons.metadata')}
      </Button>
      {isSecret(target) && (
        <Button
          variant="secondary"
          icon="eye"
          disabled={!exec.allowed}
          title={exec.reason ?? undefined}
          onClick={() => openResourceDialog({ type: 'reveal', target })}
        >
          {t('editing.buttons.reveal')}
        </Button>
      )}
      {withDelete && deletable(target) && (
        <Button
          variant="danger"
          icon="trash"
          disabled={!write.allowed}
          title={write.reason ?? undefined}
          onClick={() => openResourceDialog({ type: 'delete', target, onDeleted })}
        >
          {t('common.delete')}
        </Button>
      )}
    </>
  )
}

/** List page toolbar button: the editor's New resource tab, with this list's kind and namespace. */
export function NewResourceButton({ kind, namespace }: { kind: string; namespace?: string }) {
  const { t } = useWorkloadsTranslation()
  return (
    <Button variant="secondary" icon="add" title={t('editing.buttons.newHint', { kind })} onClick={() => openNewResourceTab({ kind, namespace })}>
      {t('editing.buttons.new')}
    </Button>
  )
}
