import type { TabDescriptor } from '@/core/plugin/types'
import { LogsView } from './LogsView'

/** Editor-tab wrapper around LogsView. kind: "pod-logs". */
export function PodLogsTab({ tab }: { tab: TabDescriptor }) {
  return (
    <LogsView
      namespace={tab.props?.namespace as string}
      pod={tab.props?.pod as string}
      container={tab.props?.container as string}
    />
  )
}
