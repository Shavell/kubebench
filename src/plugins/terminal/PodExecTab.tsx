import type { TabDescriptor } from '@/core/plugin/types'
import { ExecView } from './ExecView'

/** Editor-tab wrapper around ExecView. kind: "pod-exec". */
export function PodExecTab({ tab }: { tab: TabDescriptor }) {
  return (
    <ExecView
      namespace={tab.props?.namespace as string}
      pod={tab.props?.pod as string}
      container={tab.props?.container as string}
    />
  )
}
