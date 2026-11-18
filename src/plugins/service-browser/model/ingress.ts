import type { IngressRoute } from '../data/types.ts'
import { guessScheme } from './ports.ts'

/**
 * Routes of an Ingress for the browser (design §7.14): every path of every
 * rule, then the default backend, each with the Service port it leads to
 * — the same list, in the same order, as the backend resolves. Resource
 * backends are left out (the browser reaches Services only).
 */

interface BackendFields {
  service?: { name?: unknown; port?: { name?: unknown; number?: unknown } }
}

function asArray<T>(value: unknown): T[] {
  return Array.isArray(value) ? (value as T[]) : []
}

function backendOf(backend: BackendFields | undefined): { service: string; port: string; name?: string; number?: number } | null {
  const svc = backend?.service
  if (!svc || typeof svc.name !== 'string') return null
  const name = typeof svc.port?.name === 'string' && svc.port.name !== '' ? svc.port.name : undefined
  const number = typeof svc.port?.number === 'number' ? svc.port.number : undefined
  if (name === undefined && number === undefined) return null
  return { service: svc.name, port: name ?? String(number), name, number }
}

/** The Service routes of an Ingress object. */
export function ingressRoutes(ingress: { metadata?: unknown; spec?: unknown }): IngressRoute[] {
  const name = String((ingress.metadata as { name?: unknown } | undefined)?.name ?? '')
  const spec = (ingress.spec ?? {}) as { rules?: unknown; defaultBackend?: BackendFields; tls?: unknown }
  const tlsHosts = new Set(asArray<{ hosts?: unknown }>(spec.tls).flatMap((t) => asArray<string>(t.hosts)))
  const routes: IngressRoute[] = []
  const add = (host: string, path: string, raw: BackendFields | undefined, defaultBackend = false) => {
    const backend = backendOf(raw)
    if (!backend) return
    routes.push({
      name,
      host,
      path,
      tls: host !== '' && tlsHosts.has(host),
      defaultBackend,
      service: backend.service,
      port: backend.port,
      scheme: guessScheme(backend.number ?? 0, backend.name),
    })
  }
  for (const rule of asArray<{ host?: unknown; http?: { paths?: unknown } }>(spec.rules)) {
    const host = typeof rule.host === 'string' ? rule.host : ''
    for (const p of asArray<{ path?: unknown; backend?: BackendFields }>(rule.http?.paths)) {
      add(host, typeof p.path === 'string' ? p.path : '', p.backend)
    }
  }
  if (spec.defaultBackend) add('', '', spec.defaultBackend, true)
  return routes
}

/** Identifies a route within its Ingress (host and path, or the default backend). */
export function routeKey(route: { host: string; path: string; defaultBackend: boolean }): string {
  return route.defaultBackend ? 'default' : `rule ${route.host} ${route.path}`
}

/**
 * The path a route's page starts at: its rule's path when that is a plain
 * path (a regular expression of ImplementationSpecific paths is not), "/"
 * otherwise.
 */
export function startPath(path: string): string {
  return /^\/[A-Za-z0-9\-._~/%]*$/.test(path) ? path : '/'
}
