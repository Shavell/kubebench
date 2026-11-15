import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { tabTitle } from './tabTitle'

/**
 * The unsaved-changes guard of the editor tabs: asks before a close
 * discards the changes of dirty tabs (layoutStore.pendingClose), and lets
 * the browser ask before the page is left while any tab has some.
 */
export function TabCloseGuard() {
  const { t } = useTranslation()
  const pending = useLayoutStore((s) => s.pendingClose)
  const tabs = useLayoutStore((s) => s.tabs)
  const resolve = useLayoutStore((s) => s.resolvePendingClose)
  const anyDirty = tabs.some((tab) => tab.dirty)

  useEffect(() => {
    if (!anyDirty) return
    const warn = (event: BeforeUnloadEvent) => event.preventDefault()
    window.addEventListener('beforeunload', warn)
    return () => window.removeEventListener('beforeunload', warn)
  }, [anyDirty])

  if (!pending) return null
  const titles = pending.tabIds.flatMap((id) => {
    const tab = tabs.find((x) => x.id === id)
    return tab ? [tabTitle(tab, t)] : []
  })
  const count = titles.length
  return (
    <Dialog
      open
      role="alertdialog"
      size="small"
      icon="warning"
      title={t('shell.tabs.unsaved.title', { count })}
      description={t('shell.tabs.unsaved.message', { count, titles: titles.join(', ') })}
      onClose={() => resolve(false)}
      footer={
        <>
          <Button variant="secondary" onClick={() => resolve(false)} data-autofocus="">
            {t('common.cancel')}
          </Button>
          <Button variant="danger" onClick={() => resolve(true)}>
            {t('shell.tabs.unsaved.discard')}
          </Button>
        </>
      }
    />
  )
}
