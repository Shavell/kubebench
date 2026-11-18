import { apiDelete, apiGet, apiPost } from '@/data/apiClient'
import { ingressRoutes } from '../model/ingress'
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
    open: (target) =>
      target.ingress
        ? apiPost<ProxySession>(`/proxy/ingresses/${seg(target.namespace)}/${seg(target.ingress.name)}`, undefined, {
            host: target.ingress.host,
            path: target.ingress.path,
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
