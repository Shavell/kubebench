import type { ReactNode } from 'react'
import type { KubeObject } from '@/data/types'
import { formatNumber } from '@/i18n/format'
import { Badge } from '@/shared/Badge'
import { toneForStatus, type BadgeTone } from '@/shared/badgeTone'
import { tw } from './i18n'
import type { ResourceColumn, ResourceDescriptor } from './resourceCatalog'

/**
 * OpenShift's resource types in the generic browser (design §7.15):
 * Routes, Projects, SecurityContextConstraints, DeploymentConfigs,
 * BuildConfigs, Builds, ImageStreams and ImageStreamTags. They are
 * `optional`: listed only where the cluster serves their API group
 * (capabilities.optionalResources). The catalog's field helpers come in
 * as `h`, so this module does not import resourceCatalog at run time.
 */

export interface CatalogHelpers {
  get: (o: KubeObject, ...path: string[]) => unknown
  str: (o: KubeObject, ...path: string[]) => string
  num: (o: KubeObject, ...path: string[]) => number
  withCommon: (namespaced: boolean, mid: ResourceColumn[]) => ResourceColumn[]
}

interface RouteIngress {
  host?: string
  routerName?: string
  conditions?: { type?: string; status?: string; reason?: string; message?: string }[]
}

const list = <T,>(value: unknown): T[] => (Array.isArray(value) ? (value as T[]) : [])

/** The public URL of a Route: https when it terminates TLS. */
export function routeUrl(o: KubeObject): string | null {
  const spec = (o as { spec?: { host?: unknown; path?: unknown; tls?: unknown } }).spec
  if (!spec || typeof spec.host !== 'string' || spec.host === '') return null
  const path = typeof spec.path === 'string' ? spec.path : ''
  return `${spec.tls ? 'https' : 'http'}://${spec.host}${path}`
}

/** Whether every router that reported on the Route admitted it: true, false, or null (none reported yet). */
function routeAdmitted(o: KubeObject): { admitted: boolean | null; reason: string } {
  const ingress = list<RouteIngress>((o as { status?: { ingress?: unknown } }).status?.ingress)
  if (ingress.length === 0) return { admitted: null, reason: '' }
  for (const i of ingress) {
    const cond = (i.conditions ?? []).find((c) => c.type === 'Admitted')
    if (cond?.status !== 'True') return { admitted: false, reason: cond?.reason ?? '' }
  }
  return { admitted: true, reason: '' }
}

function admittedBadge(o: KubeObject): ReactNode {
  const { admitted, reason } = routeAdmitted(o)
  if (admitted === null) return <Badge tone="neutral">{tw('openshift.admission.pending')}</Badge>
  if (admitted) return <Badge tone="success">{tw('openshift.admission.admitted')}</Badge>
  return (
    <span title={reason || undefined}>
      <Badge tone="error">{tw('openshift.admission.rejected')}</Badge>
    </span>
  )
}

/** A link to the Route's public URL that does not open the row. */
function hostLink(o: KubeObject): ReactNode {
  const url = routeUrl(o)
  const host = (o as { spec?: { host?: string } }).spec?.host
  if (!url || !host) return '—'
  return (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      title={tw('openshift.openRouteUrl', { url })}
      onClick={(e) => e.stopPropagation()}
    >
      {host}
    </a>
  )
}

/** "2m 5s" from seconds. */
export function formatDuration(seconds: number): string {
  const s = Math.max(0, Math.round(seconds))
  const h = Math.floor(s / 3600)
  const m = Math.floor((s % 3600) / 60)
  const rest = s % 60
  if (h > 0) return tw('openshift.duration.hm', { h, m })
  if (m > 0) return tw('openshift.duration.ms', { m, s: rest })
  return tw('openshift.duration.s', { s: rest })
}

/** A Build's run time: status.duration (nanoseconds) once done, else start to completion. */
function buildDuration(o: KubeObject, h: CatalogHelpers): string {
  const ns = h.get(o, 'status', 'duration')
  if (typeof ns === 'number' && ns > 0) return formatDuration(ns / 1e9)
  const start = h.get(o, 'status', 'startTimestamp')
  const end = h.get(o, 'status', 'completionTimestamp')
  if (typeof start === 'string' && typeof end === 'string') {
    return formatDuration((Date.parse(end) - Date.parse(start)) / 1000)
  }
  return '—'
}

/** Build phases (New, Pending, Running, Complete, Failed, Error, Cancelled) and others by their usual tone. */
const PHASE_TONES: Record<string, BadgeTone> = { Complete: 'success', Running: 'info', New: 'neutral', Cancelled: 'neutral' }

const joined = (values: unknown[]): string => values.filter((v) => typeof v === 'string' && v !== '').join(', ') || '—'

export function openShiftResources(h: CatalogHelpers): ResourceDescriptor[] {
  const phaseBadge = (path: string[]): ResourceColumn => ({
    key: 'phase',
    render: (o) => {
      const phase = h.str(o, ...path)
      return phase === '—' ? phase : <Badge tone={PHASE_TONES[phase] ?? toneForStatus(phase)}>{phase}</Badge>
    },
    sortValue: (o) => h.str(o, ...path),
  })
  return [
    {
      key: 'projects', kind: 'Project', icon: 'folder-library', group: 'cluster', namespaced: false, optional: true,
      topic: 'projects.project.openshift.io',
      columns: h.withCommon(false, [
        { key: 'displayName', render: (o) => h.str(o, 'metadata', 'annotations', 'openshift.io/display-name') },
        phaseBadge(['status', 'phase']),
        { key: 'requester', render: (o) => h.str(o, 'metadata', 'annotations', 'openshift.io/requester') },
      ]),
    },
    {
      key: 'routes', kind: 'Route', icon: 'globe', group: 'openshift', namespaced: true, optional: true,
      topic: 'routes.route.openshift.io',
      columns: h.withCommon(true, [
        { key: 'host', render: hostLink, sortValue: (o) => h.str(o, 'spec', 'host') },
        { key: 'path', render: (o) => h.str(o, 'spec', 'path') },
        {
          key: 'service',
          render: (o) => {
            const port = h.get(o, 'spec', 'port', 'targetPort')
            const name = h.str(o, 'spec', 'to', 'name')
            return port === undefined ? name : `${name}:${String(port)}`
          },
          sortValue: (o) => h.str(o, 'spec', 'to', 'name'),
        },
        { key: 'tls', render: (o) => h.str(o, 'spec', 'tls', 'termination') },
        { key: 'admitted', render: admittedBadge, sortValue: (o) => String(routeAdmitted(o).admitted) },
      ]),
      details: [
        {
          key: 'routers',
          render: (o) => {
            const ingress = list<RouteIngress>(h.get(o, 'status', 'ingress'))
            if (ingress.length === 0) return '—'
            return ingress
              .map((i) => {
                const cond = (i.conditions ?? []).find((c) => c.type === 'Admitted')
                const state = cond?.status === 'True' ? tw('openshift.admission.admitted') : tw('openshift.admission.rejected')
                return `${i.routerName ?? '—'}: ${state}${cond?.reason ? ` (${cond.reason})` : ''}`
              })
              .join(' · ')
          },
        },
        {
          key: 'alternateBackends',
          render: (o) => joined(list<{ name?: string }>(h.get(o, 'spec', 'alternateBackends')).map((b) => b.name)),
        },
      ],
    },
    {
      key: 'deploymentconfigs', kind: 'DeploymentConfig', icon: 'rocket', group: 'openshift', namespaced: true, optional: true,
      topic: 'deploymentconfigs.apps.openshift.io',
      columns: h.withCommon(true, [
        { key: 'ready', render: (o) => `${h.num(o, 'status', 'readyReplicas')}/${h.num(o, 'spec', 'replicas')}` },
        { key: 'triggers', render: (o) => joined(list<{ type?: string }>(h.get(o, 'spec', 'triggers')).map((t) => t.type)) },
        { key: 'latestVersion', render: (o) => h.str(o, 'status', 'latestVersion'), sortValue: (o) => h.num(o, 'status', 'latestVersion') },
      ]),
    },
    {
      key: 'buildconfigs', kind: 'BuildConfig', icon: 'tools', group: 'openshift', namespaced: true, optional: true,
      topic: 'buildconfigs.build.openshift.io',
      columns: h.withCommon(true, [
        { key: 'strategy', render: (o) => h.str(o, 'spec', 'strategy', 'type') },
        {
          key: 'source',
          render: (o) => {
            const uri = h.get(o, 'spec', 'source', 'git', 'uri')
            return typeof uri === 'string' ? uri : h.str(o, 'spec', 'source', 'type')
          },
        },
        { key: 'lastVersion', render: (o) => h.str(o, 'status', 'lastVersion'), sortValue: (o) => h.num(o, 'status', 'lastVersion') },
      ]),
      details: [{ key: 'output', render: (o) => h.str(o, 'spec', 'output', 'to', 'name') }],
    },
    {
      key: 'builds', kind: 'Build', icon: 'gear', group: 'openshift', namespaced: true, optional: true,
      topic: 'builds.build.openshift.io',
      columns: h.withCommon(true, [
        phaseBadge(['status', 'phase']),
        { key: 'strategy', render: (o) => h.str(o, 'spec', 'strategy', 'type') },
        { key: 'duration', render: (o) => buildDuration(o, h) },
      ]),
      details: [
        { key: 'reason', render: (o) => h.str(o, 'status', 'reason') },
        { key: 'message', render: (o) => h.str(o, 'status', 'message') },
        { key: 'output', render: (o) => h.str(o, 'status', 'outputDockerImageReference') },
      ],
    },
    {
      key: 'imagestreams', kind: 'ImageStream', icon: 'package', group: 'openshift', namespaced: true, optional: true,
      topic: 'imagestreams.image.openshift.io',
      columns: h.withCommon(true, [
        {
          key: 'repository',
          render: (o) => {
            const pub = h.get(o, 'status', 'publicDockerImageRepository')
            return typeof pub === 'string' && pub ? pub : h.str(o, 'status', 'dockerImageRepository')
          },
        },
        { key: 'tags', render: (o) => joined(list<{ tag?: string }>(h.get(o, 'status', 'tags')).map((t) => t.tag)) },
      ]),
    },
    {
      // ImageStreamTags cannot be watched: they change with their ImageStream.
      key: 'imagestreamtags', kind: 'ImageStreamTag', icon: 'tag', group: 'openshift', namespaced: true, optional: true,
      topic: 'imagestreams.image.openshift.io',
      columns: h.withCommon(true, [
        { key: 'image', render: (o) => h.str(o, 'image', 'dockerImageReference') },
      ]),
    },
    {
      key: 'securitycontextconstraints', kind: 'SecurityContextConstraints', icon: 'shield', group: 'openshift', namespaced: false, optional: true,
      topic: 'securitycontextconstraints.security.openshift.io',
      columns: h.withCommon(false, [
        {
          key: 'privileged',
          render: (o) => tw(h.get(o, 'allowPrivilegedContainer') ? 'common.yes' : 'common.no'),
        },
        { key: 'runAsUser', render: (o) => h.str(o, 'runAsUser', 'type') },
        { key: 'seLinux', render: (o) => h.str(o, 'seLinuxContext', 'type') },
        {
          key: 'grantedTo',
          render: (o) => formatNumber(list(h.get(o, 'users')).length + list(h.get(o, 'groups')).length),
          sortValue: (o) => list(h.get(o, 'users')).length + list(h.get(o, 'groups')).length,
        },
      ]),
      details: [
        { key: 'users', render: (o) => joined(list(h.get(o, 'users'))) },
        { key: 'groups', render: (o) => joined(list(h.get(o, 'groups'))) },
        { key: 'volumes', render: (o) => joined(list(h.get(o, 'volumes'))) },
      ],
    },
  ]
}
