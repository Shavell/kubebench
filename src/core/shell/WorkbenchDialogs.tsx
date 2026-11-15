import { AboutDialog } from './AboutDialog'
import { ChangePasswordDialog } from './ChangePasswordDialog'
import { ShortcutsDialog } from './ShortcutsDialog'
import { useWorkbenchDialog } from './workbenchDialogs'

/** Renders the open workbench dialog, if any (mounted once by the shell). */
export function WorkbenchDialogs() {
  const open = useWorkbenchDialog((s) => s.open)
  const close = useWorkbenchDialog((s) => s.close)
  if (open === 'about') return <AboutDialog onClose={close} />
  if (open === 'changePassword') return <ChangePasswordDialog onClose={close} />
  if (open === 'shortcuts') return <ShortcutsDialog onClose={close} />
  return null
}
