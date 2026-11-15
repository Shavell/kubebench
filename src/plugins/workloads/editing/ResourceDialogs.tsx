import { useEffect } from 'react'
import { closeResourceDialog, useResourceDialogStore } from './dialogStore'
import { DeleteResourceDialog } from './DeleteResourceDialog'
import { MetadataDialog } from './MetadataDialog'
import { RevealSecretDialog } from './RevealSecretDialog'

/**
 * Renders the resource dialog the page's menus and toolbars opened
 * (dialogStore). Each list and detail page mounts one; the editor area
 * shows one page at a time, and a page that goes away takes its dialog
 * with it.
 */
export function ResourceDialogs() {
  const dialog = useResourceDialogStore((s) => s.dialog)
  useEffect(() => closeResourceDialog, [])

  switch (dialog?.type) {
    case 'delete':
      return <DeleteResourceDialog target={dialog.target} onClose={closeResourceDialog} onDeleted={dialog.onDeleted} />
    case 'metadata':
      return <MetadataDialog target={dialog.target} onClose={closeResourceDialog} />
    case 'reveal':
      return <RevealSecretDialog target={dialog.target} onClose={closeResourceDialog} />
    default:
      return null
  }
}
