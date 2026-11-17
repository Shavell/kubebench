import type { BrowserKind, BrowserPort, BrowserTarget, Scheme } from '../data/types.ts'

/**
 * Ports a browser can open (design §7.14): the TCP ports of a Service or
 * of a Pod's containers, with the scheme the API server should use to
 * reach them. Pure functions over Kubernetes-shaped objects.
 */

interface PortFields {
  name?: unknown
  port?: unknown
  containerPort?: unknown
  protocol?: unknown
  appProtocol?: unknown
}

const HTTPS_PORTS = new Set([443, 6443, 8443, 9443])

/** https for ports named or declared as TLS, or on the usual TLS port numbers; else http. */
export function guessScheme(port: number, name?: string, appProtocol?: string): Scheme {
  const hint = `${name ?? ''} ${appProtocol ?? ''}`.toLowerCase()
  if (hint.includes('https') || hint.includes('tls')) return 'https'
  return HTTPS_PORTS.has(port) ? 'https' : 'http'
}

function toPort(raw: PortFields, numberField: 'port' | 'containerPort'): BrowserPort | null {
  const port = raw[numberField]
  const protocol = typeof raw.protocol === 'string' ? raw.protocol : 'TCP'
  if (typeof port !== 'number' || protocol !== 'TCP') return null
  const name = typeof raw.name === 'string' && raw.name !== '' ? raw.name : undefined
  const appProtocol = typeof raw.appProtocol === 'string' ? raw.appProtocol : undefined
  return { port, name, scheme: guessScheme(port, name, appProtocol) }
}

function asArray(value: unknown): PortFields[] {
  return Array.isArray(value) ? (value as PortFields[]) : []
}

/** TCP ports of a Service object (`spec.ports`). */
export function servicePorts(service: { spec?: unknown }): BrowserPort[] {
  const spec = (service.spec ?? {}) as { ports?: unknown }
  return asArray(spec.ports).flatMap((p) => toPort(p, 'port') ?? [])
}

/** TCP ports the containers of a Pod object declare, without duplicates. */
export function podPorts(pod: { spec?: unknown }): BrowserPort[] {
  const spec = (pod.spec ?? {}) as { containers?: unknown }
  const seen = new Set<number>()
  return asArray(spec.containers)
    .flatMap((c) => asArray((c as { ports?: unknown }).ports))
    .flatMap((p) => toPort(p, 'containerPort') ?? [])
    .filter((p) => !seen.has(p.port) && seen.add(p.port))
}

/**
 * TCP ports of a kubectl-style port list ("80:31204/TCP,53/UDP"), as the
 * mock dataset stores them.
 */
export function parsePortList(list: string): BrowserPort[] {
  return list
    .split(',')
    .map((entry) => entry.trim())
    .flatMap((entry) => {
      const [ports, protocol = 'TCP'] = entry.split('/')
      const port = Number(ports.split(':')[0])
      if (protocol !== 'TCP' || !Number.isInteger(port) || port < 1) return []
      return [{ port, scheme: guessScheme(port) }]
    })
}

/** "8080 (http)" or "8080". */
export function portLabel(port: BrowserPort): string {
  return port.name ? `${port.port} (${port.name})` : String(port.port)
}

/**
 * The in-cluster address of a target, as the address bar shows it: a
 * Service by its DNS name (`http://web.shop:80`), a Pod by name.
 */
export function targetAddress(target: BrowserTarget): string {
  const host = target.kind === 'services' ? `${target.name}.${target.namespace}` : target.name
  return `${target.scheme}://${host}:${target.port}`
}

/** The API kind (resource) of a Kubernetes kind the browser opens. */
export function browserKind(kind: string): BrowserKind | null {
  if (kind === 'Service') return 'services'
  if (kind === 'Pod') return 'pods'
  return null
}

/** A path typed into the address bar, made absolute ("api/x" → "/api/x"). */
export function normalizePath(input: string): string {
  const trimmed = input.trim()
  return trimmed.startsWith('/') ? trimmed : `/${trimmed}`
}
