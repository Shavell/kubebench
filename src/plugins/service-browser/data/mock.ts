import { currentPermissions } from '@/core/permissions'
import { ApiError } from '@/data/apiClient'
import { delay, delayReject } from '@/data/mock/delay'
import { pods, services } from '@/data/mock/mockCluster'
import { translate } from '../i18n'
import { parsePortList, targetAddress } from '../model/ports'
import type { ServiceBrowserProvider } from './provider'
import type { ProxySession } from './types'

const escapeHtml = (text: string) =>
  text.replace(/[&<>"']/g, (c) => `&#${c.charCodeAt(0)};`)

/**
 * The page the mock "serves" for a path: a placeholder naming the target
 * and the path, since demo mode has no cluster to proxy to. Deterministic.
 */
function placeholderPage(session: ProxySession, path: string): string {
  const address = escapeHtml(targetAddress(session.target) + path)
  const title = escapeHtml(translate('mock.title'))
  const body = escapeHtml(translate('mock.body'))
  return `<!doctype html><html><head><meta charset="utf-8"><title>${title}</title>
<style>body{font:14px system-ui,sans-serif;margin:32px}</style>
</head><body><h1>${title}</h1><p><code>${address}</code></p><p>${body}</p></body></html>`
}

/** Mock ServiceBrowserProvider over the mock cluster's Services and Pods. */
export function createMockServiceBrowser(): ServiceBrowserProvider {
  const sessions = new Map<string, ProxySession>()
  let opened = 0

  const notFound = (what: string) => delayReject(new ApiError(404, 'not_found', translate('mock.notFound', { what })))

  return {
    listPorts: (kind, namespace, name) => {
      if (kind === 'services') {
        const svc = services.find((s) => s.namespace === namespace && s.name === name)
        return svc ? delay(parsePortList(svc.ports)) : notFound(`${namespace}/${name}`)
      }
      const pod = pods.find((p) => p.namespace === namespace && p.name === name)
      if (!pod) return notFound(`${namespace}/${name}`)
      return delay(pod.containers.flatMap((c) => (c.ports ? parsePortList(c.ports) : [])))
    },
    open: (target) => {
      opened += 1
      const session: ProxySession = {
        token: `mock-${opened}`,
        base: '',
        sandboxed: true,
        readOnly: currentPermissions().readOnly,
        target,
        location: '',
      }
      sessions.set(session.token, session)
      return delay(session)
    },
    session: (token) => {
      const session = sessions.get(token)
      return session ? delay({ ...session }) : notFound(token)
    },
    close: (token) => {
      sessions.delete(token)
      return delay(undefined)
    },
    frame: (session, path) => {
      const stored = sessions.get(session.token)
      if (stored) stored.location = path
      return { srcDoc: placeholderPage(session, path) }
    },
  }
}
