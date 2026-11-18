import { apiDelete, apiGet, apiPost } from '@/data/apiClient'
import { reasonError } from '@/data/reasonError'
import { ingressRoutes } from '../model/ingress'
import { routeOf } from '../model/route'
import { podPorts, servicePorts } from '../model/ports'
import type { ServiceBrowserProvider } from './provider'
import type { ProxySession } from './types'

const seg = encodeURIComponent

/**
 * ServiceBrowserProvider over the backend (design §7.14). The browser's
 * routes are core routes (/api/proxy/…), not a backend plugin's, because
 * the proxy itself is served outside /api; so this calls the API client
 * directly instead of the plugin client.
 */
export function createLiveServiceBrowser(): ServiceBrowserProvider {
  return {
    listPorts: async (kind, namespace, name) => {
      const object = await apiGet<{ spec?: unknown }>(`/k8s/api/v1/namespaces/${seg(namespace)}/${kind}/${seg(name)}`)
      return kind === 'services' ? servicePorts(object) : podPorts(object)
    },
    listIngressRoutes: async (namespace, name) =>
      ingressRoutes(await apiGet<{ spec?: unknown }>(`/k8s/apis/networking.k8s.io/v1/namespaces/${seg(namespace)}/ingresses/${seg(name)}`)),
    getRoute: async (namespace, name) => {
      const route = routeOf(await apiGet<unknown>(`/k8s/apis/route.openshift.io/v1/namespaces/${seg(namespace)}/routes/${seg(name)}`))
      if (!route) throw reasonError(400, 'bad_request', 'proxy.route_not_service', { namespace, name, kind: '?' })
      return route
    },
    open: (target) =>
      target.route
        ? apiPost<ProxySession>(`/proxy/routes/${seg(target.namespace)}/${seg(target.route.name)}`, undefined, { scheme: target.scheme })
        : target.ingress
        ? apiPost<ProxySession>(`/proxy/ingresses/${seg(target.namespace)}/${seg(target.ingress.name)}`, undefined, {
            ...(target.ingress.defaultBackend
              ? { default: true }
              : { host: target.ingress.host, path: target.ingress.path }),
            scheme: target.scheme,
          })
        : apiPost<ProxySession>(
            `/proxy/${target.kind}/${seg(target.namespace)}/${seg(target.name)}/${seg(target.port)}`,
            undefined,
            { scheme: target.scheme },
          ),
    session: (token) => apiGet<ProxySession>(`/proxy/sessions/${seg(token)}`),
    close: async (token) => {
      await apiDelete(`/proxy/sessions/${seg(token)}`)
    },
    frame: (session, path) => ({ src: session.base + path }),
  }
}
