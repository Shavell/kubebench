import { useTranslation } from 'react-i18next'
import type { TabDescriptor } from '@/core/plugin/types'
import { useProviders } from '@/data/DataContext'
import { Unavailable } from '@/shared/Unavailable'
import { NS } from './i18n'
import { podTarget } from './panels/podTarget'
import { tabSession } from './shell/sessions'
import { TerminalView } from './shell/TerminalView'
import { useTerminalBlocker } from './shell/useTerminalBlocker'
import { TabPage } from './TabPage'

/**
 * Editor tab of kind "pod-exec" (props `{namespace, pod, container}`): a
 * terminal whose session lasts as long as the tab, also while another tab
 * is shown.
 */
export function PodExecTab({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const providers = useProviders()
  const blocker = useTerminalBlocker()
  const target = podTarget(tab.props)

  if (blocker) return <Unavailable code={blocker.code} reason={blocker.reason} />
  if (!target) return <Unavailable code="bad_request" reason={t('shell.badTarget')} />
  return (
    <TabPage>
      <TerminalView session={tabSession(tab.id, target, providers.cluster)} />
    </TabPage>
  )
}
