import type { OpenShiftRoute } from '../data/types.ts'
import { guessScheme } from './ports.ts'

/**
 * An OpenShift Route for the browser (design §7.15): its host, path, TLS
 * termination and the Service it leads to. The backend resolves the
 * Route's targetPort to the Service port when it opens the grant.
 */

interface RouteFields {
  metadata?: { name?: unknown }
  spec?: {
    host?: unknown
    path?: unknown
    to?: { kind?: unknown; name?: unknown }
    port?: { targetPort?: unknown }
    tls?: { termination?: unknown } | null
  }
}

const text = (v: unknown) => (typeof v === 'string' ? v : '')

/** The browser's view of a Route object; null for a Route that does not lead to a Service. */
export function routeOf(object: unknown): OpenShiftRoute | null {
  const route = (object ?? {}) as RouteFields
  const spec = route.spec ?? {}
  if (text(spec.to?.kind) !== 'Service' || !text(spec.to?.name)) return null
  const target = spec.port?.targetPort
  const targetPort = typeof target === 'number' ? String(target) : text(target)
  const termination = text(spec.tls?.termination).toLowerCase()
  // Passthrough and re-encrypt Routes reach a backend that speaks TLS itself.
  const backendTls = termination === 'passthrough' || termination === 'reencrypt'
  return {
    name: text(route.metadata?.name),
    host: text(spec.host),
    path: text(spec.path),
    tls: termination !== '',
    termination,
    service: text(spec.to?.name),
    targetPort,
    scheme: backendTls ? 'https' : guessScheme(Number(targetPort) || 0, targetPort),
  }
}

/** The public URL of a Route, as its router serves it; null without a host. */
export function routePublicUrl(route: { host: string; path: string; tls: boolean }): string | null {
  return route.host ? `${route.tls ? 'https' : 'http'}://${route.host}${route.path}` : null
}
