import type { AccessRequest, PolicyRule, RbacRef, Subject } from '../data/types'

/** Kubernetes' bootstrap RBAC objects and identities are named system:…. */
export const isSystemName = (name: string) => name.startsWith('system:')

/** A subject's short name: `namespace/name` for ServiceAccounts, the name otherwise. */
export function subjectName(s: Subject): string {
  return s.kind === 'ServiceAccount' ? `${s.namespace}/${s.name}` : s.name
}

export const subjectKey = (s: Subject) => `${s.kind}/${s.namespace ?? ''}/${s.name}`

/** `shop/web` for a namespaced object, `view` for a cluster-scoped one. */
export function refText(r: RbacRef): string {
  return r.namespace ? `${r.namespace}/${r.name}` : r.name
}

/** The resource part of a request in kubectl's notation: `deployments.apps/scale`, or the URL. */
export function requestTarget(r: AccessRequest): string {
  if (r.path) return r.path
  const resource = r.group ? `${r.resource}.${r.group}` : (r.resource ?? '')
  const withSub = r.subresource ? `${resource}/${r.subresource}` : resource
  return r.name ? `${withSub} "${r.name}"` : withSub
}

/** Resources of a rule with their subresources and resourceNames, e.g. `configmaps [app-config]`. */
export function ruleTargets(rule: PolicyRule): string {
  if (rule.nonResourceURLs?.length) return rule.nonResourceURLs.join(', ')
  const resources = (rule.resources ?? []).join(', ')
  return rule.resourceNames?.length ? `${resources} [${rule.resourceNames.join(', ')}]` : resources
}

/** `get,list secrets` — a rule's verbs and targets on one line. */
export const ruleSummary = (rule: PolicyRule) => `${rule.verbs.join(',')} ${ruleTargets(rule)}`

/** API groups of a rule; the core group ("") reads as `core`. */
export function ruleGroups(rule: PolicyRule, core: string): string {
  if (rule.nonResourceURLs?.length && !rule.resources?.length) return '—'
  return (rule.apiGroups ?? []).map((g) => (g === '' ? core : g)).join(', ')
}

/** Generic resource browser keys (tab kind `generic-detail`) of the kinds this plugin links to. */
export const RESOURCE_KEYS: Record<string, { key: string; icon: string }> = {
  Role: { key: 'roles', icon: 'shield' },
  ClusterRole: { key: 'clusterroles', icon: 'shield' },
  RoleBinding: { key: 'rolebindings', icon: 'link' },
  ClusterRoleBinding: { key: 'clusterrolebindings', icon: 'link' },
  ServiceAccount: { key: 'serviceaccounts', icon: 'account' },
}
