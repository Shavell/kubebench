import { openInSideGroup } from '@/core/plugin/sideGroup'
import type { ResourceRef } from '@/core/plugin/types'
import { NS, translate } from './i18n'
import { browserKind } from './model/ports'
import type { BrowserKind } from './data/types'

/** Tab kind of a browser page, opened in the side editor group (design §7.14). */
export const TAB_KIND = 'service-browser'
export const ICON = 'globe'

/** Kinds the "Open in browser" action applies to. */
export const BROWSABLE_KINDS = ['Service', 'Pod', 'Ingress', 'Route']

/** Props of a browser tab: a Service or Pod (pick a port), an Ingress (pick a route) or an OpenShift Route. */
export interface BrowserTabProps {
  kind: BrowserKind | 'ingresses' | 'routes'
  namespace: string
  name: string
}

/** Opens (or focuses) the browser tab of a Service, Pod, Ingress or Route, to the right of the editor area. */
export function openBrowser(ref: ResourceRef): void {
  const kind = browserKind(ref.kind)
  if (!kind || !ref.namespace) return
  const props: BrowserTabProps = { kind, namespace: ref.namespace, name: ref.name }
  const titleValues = { name: ref.name }
  openInSideGroup({
    id: `${TAB_KIND}:${kind}/${ref.namespace}/${ref.name}`,
    title: translate('tab.title', titleValues),
    titleKey: 'tab.title',
    titleNs: NS,
    titleValues,
    icon: ICON,
    kind: TAB_KIND,
    props: { ...props },
  })
}
