import type { Category, Finding, Severity } from '../data/types'
import { SEVERITIES } from '../data/types'
import { frameworkFamily } from './severity'

/** Filters of the findings page; `'all'` (or null) does not filter. */
export interface FindingsFilter {
  search: string
  severity: Severity | 'all'
  category: Category | 'all'
  /** A framework family ("CIS", "PSS baseline", …). */
  framework: string
  namespace: string
  status: 'open' | 'accepted' | 'all'
  checkId: string | null
  /** One object, e.g. from a resource's "Security findings" action. */
  resource: { kind: string; namespace?: string; name: string } | null
}

export const GROUPINGS = ['none', 'check', 'severity', 'category', 'namespace', 'resource'] as const
export type Grouping = (typeof GROUPINGS)[number]

export const DEFAULT_FILTER: FindingsFilter = {
  search: '',
  severity: 'all',
  category: 'all',
  framework: 'all',
  namespace: 'all',
  status: 'open',
  checkId: null,
  resource: null,
}

/** Whether any filter but the search and the status narrows the list. */
export function narrowed(f: FindingsFilter): boolean {
  return (
    f.severity !== 'all' ||
    f.category !== 'all' ||
    f.framework !== 'all' ||
    f.namespace !== 'all' ||
    f.checkId !== null ||
    f.resource !== null
  )
}

/**
 * Applies the filter. The search matches the check, the object, the
 * English message and the evidence, plus `text(f)` (the translated texts
 * the page shows).
 */
export function filterFindings(findings: Finding[], f: FindingsFilter, text: (f: Finding) => string): Finding[] {
  const needle = f.search.trim().toLowerCase()
  return findings.filter((x) => {
    if (f.status === 'open' && x.accepted) return false
    if (f.status === 'accepted' && !x.accepted) return false
    if (f.severity !== 'all' && x.severity !== f.severity) return false
    if (f.category !== 'all' && x.category !== f.category) return false
    if (f.framework !== 'all' && !x.frameworks.some((fw) => frameworkFamily(fw) === f.framework)) return false
    if (f.namespace !== 'all' && (x.resource.namespace ?? '') !== f.namespace) return false
    if (f.checkId && x.checkId !== f.checkId) return false
    if (f.resource && !sameObject(x.resource, f.resource)) return false
    if (!needle) return true
    const haystack = [
      x.checkId,
      x.resource.kind,
      x.resource.namespace ?? '',
      x.resource.name,
      x.message,
      ...(x.evidence ?? []),
      text(x),
    ]
    return haystack.some((s) => s.toLowerCase().includes(needle))
  })
}

/**
 * A resource filter of a Namespace matches the namespace itself and every
 * object in it; any other kind matches that object.
 */
function sameObject(r: Finding['resource'], want: NonNullable<FindingsFilter['resource']>): boolean {
  if (want.kind === 'Namespace') return (r.kind === 'Namespace' && r.name === want.name) || r.namespace === want.name
  return r.kind === want.kind && r.name === want.name && (r.namespace ?? '') === (want.namespace ?? '')
}

export interface FindingGroup {
  key: string
  findings: Finding[]
}

/** Groups findings (already in severity order) by `by`; groups keep first-seen order, severities their rank. */
export function groupFindings(findings: Finding[], by: Grouping): FindingGroup[] {
  if (by === 'none') return [{ key: '', findings }]
  const keyOf = (f: Finding): string => {
    switch (by) {
      case 'check':
        return f.checkId
      case 'severity':
        return f.severity
      case 'category':
        return f.category
      case 'namespace':
        return f.resource.namespace ?? ''
      case 'resource':
        return `${f.resource.kind}/${f.resource.namespace ?? ''}/${f.resource.name}`
    }
  }
  const groups = new Map<string, Finding[]>()
  for (const f of findings) {
    const key = keyOf(f)
    const list = groups.get(key)
    if (list) list.push(f)
    else groups.set(key, [f])
  }
  const out = [...groups].map(([key, list]) => ({ key, findings: list }))
  if (by === 'severity') out.sort((a, b) => SEVERITIES.indexOf(a.key as Severity) - SEVERITIES.indexOf(b.key as Severity))
  return out
}

/** The distinct framework families and namespaces of a report's findings, sorted. */
export function filterOptions(findings: Finding[]): { frameworks: string[]; namespaces: string[] } {
  const frameworks = new Set<string>()
  const namespaces = new Set<string>()
  for (const f of findings) {
    f.frameworks.forEach((fw) => frameworks.add(frameworkFamily(fw)))
    if (f.resource.namespace) namespaces.add(f.resource.namespace)
  }
  return { frameworks: [...frameworks].sort(), namespaces: [...namespaces].sort() }
}
