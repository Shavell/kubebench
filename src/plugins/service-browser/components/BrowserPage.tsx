import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import type { TabDescriptor } from '@/core/plugin/types'
import { refreshData } from '@/data/DataContext'
import { usePluginQuery } from '@/data/pluginData'
import { EmptyState } from '@/shared/EmptyState'
import { Unavailable } from '@/shared/Unavailable'
import { serviceBrowserData } from '../data'
import type { BrowserKind, BrowserTarget, Scheme } from '../data/types'
import { NS } from '../i18n'
import { routeKey } from '../model/ingress'
import type { BrowserTabProps } from '../tabs'
import { BrowserSession } from './BrowserSession'
import { PortPicker, RoutePicker, SchemePicker } from './TargetPickers'
import '../serviceBrowser.css'

/**
 * A browser tab (design §7.14): picks the port of the tab's Service or
 * Pod, or the route of its Ingress, then shows it through a browsing
 * session. Changing the port, route or scheme starts a new session.
 */
export function BrowserPage({ tab }: { tab: TabDescriptor }) {
  const { kind, namespace, name } = tab.props as unknown as BrowserTabProps
  const permission = usePermission('browse')
  if (!permission.allowed) return <Unavailable code={permission.code} reason={permission.reason} />
  if (kind === 'ingresses') return <IngressBrowser namespace={namespace} name={name} />
  if (kind === 'routes') return <RouteBrowser namespace={namespace} name={name} />
  return <ObjectBrowser kind={kind} namespace={namespace} name={name} />
}

/** A Service or Pod: one of its TCP ports. */
function ObjectBrowser({ kind, namespace, name }: { kind: BrowserKind; namespace: string; name: string }) {
  const { t } = useTranslation(NS)
  const ports = usePluginQuery(
    () => serviceBrowserData().listPorts(kind, namespace, name),
    [kind, namespace, name],
  )
  const [choice, setChoice] = useState<{ port: number; scheme: Scheme } | null>(null)

  if (ports.error) return <Unavailable code={ports.errorCode} reasonCode={ports.errorReason} reason={ports.error} onRetry={refreshData} />
  if (!ports.data) return <div className="page-loading">{t('page.loadingPorts')}</div>
  if (ports.data.length === 0) {
    return <EmptyState icon="plug" title={t('page.noPorts')} hint={t('page.noPortsHint')} />
  }

  const selected = ports.data.find((p) => p.port === choice?.port) ?? ports.data[0]
  const scheme = choice?.port === selected.port ? choice.scheme : selected.scheme
  const target: BrowserTarget = { kind, namespace, name, port: String(selected.port), scheme }

  return (
    <BrowserSession
      key={`${target.port}/${target.scheme}`}
      target={target}
      pickers={
        <PortPicker
          ports={ports.data}
          port={selected.port}
          scheme={scheme}
          onChange={(port, nextScheme) => setChoice({ port, scheme: nextScheme })}
        />
      }
    />
  )
}

/** An Ingress: one of its routes, reaching the route's Service port with the rule's host. */
function IngressBrowser({ namespace, name }: { namespace: string; name: string }) {
  const { t } = useTranslation(NS)
  const routes = usePluginQuery(() => serviceBrowserData().listIngressRoutes(namespace, name), [namespace, name])
  const [choice, setChoice] = useState<{ key: string; scheme: Scheme } | null>(null)

  if (routes.error) return <Unavailable code={routes.errorCode} reasonCode={routes.errorReason} reason={routes.error} onRetry={refreshData} />
  if (!routes.data) return <div className="page-loading">{t('ingress.loading')}</div>
  if (routes.data.length === 0) {
    return <EmptyState icon="plug" title={t('ingress.noRoutes')} hint={t('ingress.noRoutesHint')} />
  }

  const selected = routes.data.find((r) => routeKey(r) === choice?.key) ?? routes.data[0]
  const scheme = choice?.key === routeKey(selected) ? choice.scheme : selected.scheme
  const target: BrowserTarget = {
    kind: 'services',
    namespace,
    name: selected.service,
    port: selected.port,
    scheme,
    ingress: { name, host: selected.host, path: selected.path, tls: selected.tls, defaultBackend: selected.defaultBackend },
  }

  return (
    <BrowserSession
      key={`${routeKey(selected)}/${scheme}`}
      target={target}
      pickers={
        <RoutePicker
          routes={routes.data}
          route={selected}
          scheme={scheme}
          onChange={(route, nextScheme) => setChoice({ key: routeKey(route), scheme: nextScheme })}
        />
      }
    />
  )
}

/**
 * An OpenShift Route (design §7.15): its Service, reached with the Route's
 * host; the server resolves the Route's targetPort to the Service port.
 */
function RouteBrowser({ namespace, name }: { namespace: string; name: string }) {
  const { t } = useTranslation(NS)
  const route = usePluginQuery(() => serviceBrowserData().getRoute(namespace, name), [namespace, name])
  const [scheme, setScheme] = useState<Scheme | null>(null)

  if (route.error) return <Unavailable code={route.errorCode} reasonCode={route.errorReason} reason={route.error} onRetry={refreshData} />
  if (!route.data) return <div className="page-loading">{t('route.loading')}</div>

  const r = route.data
  const chosen = scheme ?? r.scheme
  const target: BrowserTarget = {
    kind: 'services',
    namespace,
    name: r.service,
    port: r.targetPort,
    scheme: chosen,
    route: { name: r.name, host: r.host, path: r.path, tls: r.tls },
  }
  return <BrowserSession key={chosen} target={target} pickers={<SchemePicker value={chosen} onChange={setScheme} />} />
}
