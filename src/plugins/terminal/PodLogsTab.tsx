import { useTranslation } from 'react-i18next'
import type { TabDescriptor } from '@/core/plugin/types'
import { Unavailable } from '@/shared/Unavailable'
import { NS } from './i18n'
import { LogsView } from './logview/LogsView'
import { podTarget } from './panels/podTarget'
import { TabPage } from './TabPage'

/** Editor tab of kind "pod-logs" (props `{namespace, pod, container}`): the container's live log. */
export function PodLogsTab({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const target = podTarget(tab.props)
  if (!target) return <Unavailable code="bad_request" reason={t('logs.badTarget')} />
  return (
    <TabPage>
      <LogsView key={`${target.namespace}/${target.pod}`} target={{ kind: 'pod', ...target }} />
    </TabPage>
  )
}
