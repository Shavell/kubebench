import type { ResourceRef } from '@/core/plugin/types'
import { useNotesQuery } from '../data/useNotesQuery'
import { useNotesTranslation } from '../i18n'
import '../notes.css'

/**
 * The number of notes on an object, on the "Notes" detail-page button.
 * Renders nothing while loading, on errors and when there are none.
 */
export function NoteCountBadge({ resource }: { resource: ResourceRef }) {
  const { t } = useNotesTranslation()
  const counts = useNotesQuery(
    (p) => p.counts({ apiVersion: resource.apiVersion, kind: resource.kind, namespace: resource.namespace ?? '', name: resource.name }),
    [resource.apiVersion, resource.kind, resource.namespace, resource.name],
  )
  const total = (counts.data ?? []).reduce((sum, c) => sum + c.personal + c.shared, 0)
  if (total === 0) return null
  return (
    <span className="kb-notes-count" aria-label={t('resource.count', { count: total })}>
      {total}
    </span>
  )
}
