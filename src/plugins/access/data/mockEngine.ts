import type {
  AccessRequest,
  BindingGraph,
  Grant,
  GrantPath,
  GraphNode,
  PolicyRule,
  RbacRef,
  Risk,
  RiskId,
  RiskyRole,
  ScopeRules,
  Severity,
  Subject,
  SubjectDetail,
  SubjectGrants,
  SubjectSummary,
} from './types'

/**
 * The mock's RBAC evaluator: a port of backend/internal/plugins/access
 * (itself following the Kubernetes RBAC authorizer), so demo answers
 * behave like live ones. Pure functions over an RbacCluster.
 */

export interface MockRole {
  ref: RbacRef
  rules: PolicyRule[]
  aggregated?: boolean
}

export interface MockBinding {
  ref: RbacRef
  roleRef: RbacRef
  subjects: Subject[]
}

export interface RbacCluster {
  roles: MockRole[]
  bindings: MockBinding[]
  serviceAccounts: { namespace: string; name: string }[]
  /** OpenShift: the risky SCCs by level (design §7.15); absent elsewhere. */
  sccLevels?: Record<string, 'privileged' | 'anyuid'>
}

const refKey = (r: RbacRef) => `${r.kind}/${r.namespace ?? ''}/${r.name}`
const subjectKey = (s: Subject) => `${s.kind}/${s.namespace ?? ''}/${s.name}`
const scopeOf = (b: MockBinding) => (b.ref.kind === 'RoleBinding' ? (b.ref.namespace ?? '') : '')
const has = (list: string[] | undefined, value: string) => (list ?? []).includes(value)

// --- matching ------------------------------------------------------------------

function resourceMatches(rule: PolicyRule, combined: string, subresource: string): boolean {
  return (rule.resources ?? []).some(
    (res) => res === '*' || res === combined || (subresource !== '' && res.startsWith('*/') && res.slice(2) === subresource),
  )
}

function urlMatches(rule: PolicyRule, path: string): boolean {
  return (rule.nonResourceURLs ?? []).some(
    (url) => url === '*' || url === path || (url.endsWith('*') && path.startsWith(url.slice(0, -1))),
  )
}

export function ruleAllows(rule: PolicyRule, r: AccessRequest): boolean {
  if (!has(rule.verbs, '*') && !has(rule.verbs, r.verb)) return false
  if (r.path) return urlMatches(rule, r.path)
  const sub = r.subresource ?? ''
  const combined = sub ? `${r.resource}/${sub}` : (r.resource ?? '')
  if (!has(rule.apiGroups, '*') && !has(rule.apiGroups, r.group)) return false
  if (!resourceMatches(rule, combined, sub)) return false
  return !rule.resourceNames?.length || rule.resourceNames.includes(r.name ?? '')
}

function appliesIn(b: MockBinding, r: AccessRequest): boolean {
  const scope = scopeOf(b)
  return scope === '' || (!r.path && (r.namespace ?? '') === scope)
}

interface Identity {
  user: string
  groups: string[]
}

export function identityOf(s: Subject): Identity {
  if (s.kind === 'ServiceAccount') {
    return {
      user: `system:serviceaccount:${s.namespace}:${s.name}`,
      groups: ['system:serviceaccounts', `system:serviceaccounts:${s.namespace}`, 'system:authenticated'],
    }
  }
  if (s.kind === 'User') {
    return { user: s.name, groups: [s.name === 'system:anonymous' ? 'system:unauthenticated' : 'system:authenticated'] }
  }
  return { user: '', groups: [s.name] }
}

function matches(id: Identity, bs: Subject, bindingNs: string): { ok: boolean; via?: string } {
  if (bs.kind === 'User') return { ok: id.user !== '' && bs.name === id.user }
  if (bs.kind === 'Group') return id.groups.includes(bs.name) ? { ok: true, via: bs.name } : { ok: false }
  return { ok: id.user !== '' && id.user === `system:serviceaccount:${bs.namespace || bindingNs}:${bs.name}` }
}

const bindingSubject = (bs: Subject, bindingNs: string): Subject =>
  bs.kind === 'ServiceAccount' ? { ...bs, namespace: bs.namespace || bindingNs } : { kind: bs.kind, name: bs.name }

const KIND_ORDER: Record<string, number> = { User: 0, Group: 1, ServiceAccount: 2 }

export function compareSubjects(a: Subject, b: Subject): number {
  return (
    KIND_ORDER[a.kind] - KIND_ORDER[b.kind] ||
    (a.namespace ?? '').localeCompare(b.namespace ?? '') ||
    a.name.localeCompare(b.name)
  )
}

// --- evaluation ------------------------------------------------------------------

export class RbacEngine {
  private readonly roles: Map<string, MockRole>
  private readonly cluster: RbacCluster

  constructor(cluster: RbacCluster) {
    this.cluster = cluster
    this.roles = new Map(cluster.roles.map((r) => [refKey(r.ref), r]))
  }

  private roleOf(b: MockBinding) {
    return this.roles.get(refKey(b.roleRef))
  }

  private pathOf(b: MockBinding, via?: string): GrantPath {
    const path: GrantPath = { binding: b.ref, role: b.roleRef, scope: scopeOf(b) }
    if (via) path.via = via
    if (!this.roleOf(b)) path.roleMissing = true
    return path
  }

  private saExists(s: Subject) {
    return this.cluster.serviceAccounts.some((sa) => sa.namespace === s.namespace && sa.name === s.name)
  }

  whoCan(r: AccessRequest): SubjectGrants[] {
    const out = new Map<string, SubjectGrants>()
    for (const b of this.cluster.bindings) {
      if (!appliesIn(b, r)) continue
      const rule = this.roleOf(b)?.rules.find((rule) => ruleAllows(rule, r))
      if (!rule) continue
      for (const bs of b.subjects) {
        const s = bindingSubject(bs, scopeOf(b))
        const entry = out.get(subjectKey(s)) ?? { ...s, grants: [] }
        entry.grants.push({ ...this.pathOf(b), rule })
        out.set(subjectKey(s), entry)
      }
    }
    return [...out.values()].sort(compareSubjects)
  }

  allows(subject: Subject, r: AccessRequest): Grant[] {
    const id = identityOf(subject)
    const out: Grant[] = []
    for (const b of this.cluster.bindings) {
      if (!appliesIn(b, r)) continue
      const rule = this.roleOf(b)?.rules.find((rule) => ruleAllows(rule, r))
      if (!rule) continue
      const hit = b.subjects.map((bs) => matches(id, bs, scopeOf(b))).find((m) => m.ok)
      if (hit) out.push({ ...this.pathOf(b, hit.via), rule })
    }
    return out
  }

  subjects(): SubjectSummary[] {
    const out = new Map<string, SubjectSummary>()
    const get = (s: Subject) => {
      const entry = out.get(subjectKey(s)) ?? { ...s, clusterBindings: 0, namespaces: [], roles: [] }
      out.set(subjectKey(s), entry)
      return entry
    }
    for (const sa of this.cluster.serviceAccounts) get({ kind: 'ServiceAccount', ...sa })
    for (const b of this.cluster.bindings) {
      const scope = scopeOf(b)
      for (const bs of b.subjects) {
        const s = bindingSubject(bs, scope)
        const entry = get(s)
        if (scope === '') entry.clusterBindings += 1
        else if (!entry.namespaces.includes(scope)) entry.namespaces.push(scope)
        if (!entry.roles.some((r) => refKey(r) === refKey(b.roleRef))) entry.roles.push(b.roleRef)
        if (s.kind === 'ServiceAccount' && !this.saExists(s)) entry.missing = true
      }
    }
    return [...out.values()].map((s) => ({ ...s, namespaces: [...s.namespaces].sort() })).sort(compareSubjects)
  }

  subject(subject: Subject): SubjectDetail {
    const id = identityOf(subject)
    const bindings: GrantPath[] = []
    for (const b of this.cluster.bindings) {
      const hit = b.subjects.map((bs) => matches(id, bs, scopeOf(b))).find((m) => m.ok)
      if (hit) bindings.push(this.pathOf(b, hit.via))
    }
    const byScope = new Map<string, ScopeRules>()
    for (const path of bindings) {
      const role = this.roles.get(refKey(path.role))
      if (!role) continue
      // A RoleBinding never grants non-resource URLs.
      const rules = path.scope ? role.rules.filter((rule) => !rule.nonResourceURLs?.length) : role.rules
      if (rules.length === 0) continue
      const group = byScope.get(path.scope) ?? { namespace: path.scope, rules: [] }
      group.rules.push(...rules.map((rule) => ({ ...rule, path })))
      byScope.set(path.scope, group)
    }
    const detail: SubjectDetail = {
      subject,
      groups: id.groups,
      bindings,
      rules: [...byScope.values()].sort((a, b) => a.namespace.localeCompare(b.namespace)),
    }
    if (id.user) detail.user = id.user
    if (subject.kind === 'ServiceAccount' && !this.saExists(subject)) detail.missing = true
    return detail
  }

  risky(): RiskyRole[] {
    const out: RiskyRole[] = []
    for (const role of this.cluster.roles) {
      const risks = risksOf(role.rules, this.cluster.sccLevels)
      if (!risks.length) continue
      const bindings = this.cluster.bindings.filter((b) => refKey(b.roleRef) === refKey(role.ref))
      const subjects = new Set(bindings.flatMap((b) => b.subjects.map((s) => subjectKey(bindingSubject(s, scopeOf(b))))))
      out.push({
        role: role.ref,
        aggregated: role.aggregated,
        severity: risks.reduce<Severity>((max, r) => (RANK[r.severity] > RANK[max] ? r.severity : max), 'medium'),
        risks,
        bindings: bindings.map((b) => b.ref),
        subjects: subjects.size,
      })
    }
    return out.sort((a, b) => RANK[b.severity] - RANK[a.severity] || b.subjects - a.subjects)
  }

  graph(namespace: string, includeSystem: boolean): BindingGraph {
    const isSystem = (name: string) => name.startsWith('system:')
    const graph: BindingGraph = { subjects: [], bindings: [], roles: [], edges: [], truncated: false }
    const seen = new Set<string>()
    for (const b of this.cluster.bindings) {
      const scope = scopeOf(b)
      if (!includeSystem && (isSystem(b.ref.name) || isSystem(b.roleRef.name))) continue
      const namesSa = b.subjects.some((s) => s.kind === 'ServiceAccount' && s.namespace === namespace)
      if (namespace && scope !== namespace && !(scope === '' && namesSa)) continue
      const bNode = node('b', b.ref, false)
      graph.bindings.push(bNode)
      const rNode = node('r', b.roleRef, !this.roleOf(b))
      if (!seen.has(rNode.id)) {
        seen.add(rNode.id)
        graph.roles.push(rNode)
      }
      graph.edges.push({ from: bNode.id, to: rNode.id })
      for (const bs of b.subjects) {
        const s = bindingSubject(bs, scope)
        const sNode = node('s', s, s.kind === 'ServiceAccount' && !this.saExists(s))
        if (!seen.has(sNode.id)) {
          seen.add(sNode.id)
          graph.subjects.push(sNode)
        }
        graph.edges.push({ from: sNode.id, to: bNode.id })
      }
    }
    graph.subjects.sort((a, b) => compareSubjects(a as Subject, b as Subject))
    graph.roles.sort((a, b) => a.id.localeCompare(b.id))
    return graph
  }
}

function node(prefix: 's' | 'b' | 'r', ref: RbacRef | Subject, missing: boolean): GraphNode {
  const n: GraphNode = { id: `${prefix}:${ref.kind}/${ref.namespace ?? ''}/${ref.name}`, kind: ref.kind, name: ref.name }
  if (ref.namespace) n.namespace = ref.namespace
  if (missing) n.missing = true
  return n
}

// --- risks ---------------------------------------------------------------------------

const RANK: Record<Severity, number> = { critical: 3, high: 2, medium: 1 }

function can(rule: PolicyRule, group: string, resource: string, ...verbs: string[]): boolean {
  const [res, sub = ''] = resource.split('/')
  return verbs.some((verb) => ruleAllows(rule, { verb, group, resource: res, subresource: sub }))
}

const canAny = (rule: PolicyRule, group: string, resources: string[], ...verbs: string[]) =>
  resources.some((res) => can(rule, group, res, ...verbs))

const MUTATING = ['create', 'update', 'patch']
const RBAC = 'rbac.authorization.k8s.io'

const CHECKS: [RiskId, Severity, (r: PolicyRule) => boolean][] = [
  ['full-access', 'critical', (r) => has(r.verbs, '*') && has(r.apiGroups, '*') && has(r.resources, '*') && !r.resourceNames?.length],
  ['escalate-bind', 'critical', (r) => canAny(r, RBAC, ['roles', 'clusterroles'], 'escalate', 'bind')],
  ['impersonate', 'critical', (r) => canAny(r, '', ['users', 'groups', 'serviceaccounts'], 'impersonate') || can(r, 'authentication.k8s.io', 'userextras', 'impersonate')],
  ['wildcard-verbs', 'high', (r) => has(r.verbs, '*') && (r.resources ?? []).length > 0],
  ['wildcard-resources', 'high', (r) => has(r.resources, '*')],
  ['secrets-read', 'high', (r) => can(r, '', 'secrets', 'get', 'list', 'watch')],
  ['pod-exec', 'high', (r) => canAny(r, '', ['pods/exec', 'pods/attach'], 'create', 'get')],
  ['token-create', 'high', (r) => can(r, '', 'serviceaccounts/token', 'create')],
  ['rbac-write', 'high', (r) => canAny(r, RBAC, ['roles', 'clusterroles', 'rolebindings', 'clusterrolebindings'], ...MUTATING)],
  ['node-proxy', 'high', (r) => can(r, '', 'nodes/proxy', 'get', 'create')],
  ['webhook-write', 'high', (r) => canAny(r, 'admissionregistration.k8s.io', ['mutatingwebhookconfigurations', 'validatingwebhookconfigurations'], ...MUTATING)],
  ['csr-approve', 'high', (r) => can(r, 'certificates.k8s.io', 'certificatesigningrequests/approval', 'update', 'patch')],
  ['workload-create', 'medium', (r) => can(r, '', 'pods', 'create') || canAny(r, 'apps', ['deployments', 'daemonsets', 'statefulsets', 'replicasets'], 'create') || canAny(r, 'batch', ['jobs', 'cronjobs'], 'create')],
  ['persistentvolume-create', 'medium', (r) => can(r, '', 'persistentvolumes', 'create')],
]

/** The first rule that may "use" an SCC of the level (all SCCs without resourceNames), like the server's. */
function sccRule(rules: PolicyRule[], levels: Record<string, string>, level: string): PolicyRule | undefined {
  return rules.find((r) => {
    const unnamed: PolicyRule = { ...r, resourceNames: undefined }
    if (!can(unnamed, 'security.openshift.io', 'securitycontextconstraints', 'use')) return false
    return Object.entries(levels).some(([name, l]) => l === level && (!r.resourceNames?.length || r.resourceNames.includes(name)))
  })
}

function risksOf(rules: PolicyRule[], sccLevels?: Record<string, string>): Risk[] {
  const out: Risk[] = []
  for (const [id, severity, check] of CHECKS) {
    const rule = rules.find(check)
    if (rule) out.push({ id, severity, rule })
  }
  const filtered = out.some((r) => r.id === 'full-access')
    ? out.filter((r) => r.id !== 'wildcard-verbs' && r.id !== 'wildcard-resources')
    : out
  if (sccLevels) {
    const privileged = sccRule(rules, sccLevels, 'privileged')
    const anyuid = sccRule(rules, sccLevels, 'anyuid')
    if (privileged) filtered.push({ id: 'scc-privileged', severity: 'critical', rule: privileged })
    if (anyuid) filtered.push({ id: 'scc-anyuid', severity: 'high', rule: anyuid })
  }
  return filtered
}
