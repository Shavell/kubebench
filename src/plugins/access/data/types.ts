/**
 * Shapes of /api/plugins/access (design §7.8), matching
 * backend/internal/plugins/access.
 */

export type SubjectKind = 'User' | 'Group' | 'ServiceAccount'

/** Who a binding grants to; `namespace` is set for ServiceAccounts only. */
export interface Subject {
  kind: SubjectKind
  name: string
  namespace?: string
}

export type RoleKind = 'Role' | 'ClusterRole'
export type BindingKind = 'RoleBinding' | 'ClusterRoleBinding'

/** A Role, ClusterRole or binding. */
export interface RbacRef {
  kind: RoleKind | BindingKind
  namespace?: string
  name: string
}

export interface PolicyRule {
  verbs: string[]
  apiGroups?: string[]
  resources?: string[]
  resourceNames?: string[]
  nonResourceURLs?: string[]
}

/** An access question: a resource request, or a non-resource URL (`path`). */
export interface AccessRequest {
  verb: string
  group: string
  resource?: string
  subresource?: string
  /** Empty: cluster-scoped or all namespaces (only ClusterRoleBindings grant those). */
  namespace?: string
  name?: string
  path?: string
}

/** How a role reaches a subject. `scope` is the namespace it grants in, "" cluster-wide. */
export interface GrantPath {
  binding: RbacRef
  role: RbacRef
  scope: string
  /** The group the binding names, when it reaches the subject through a membership. */
  via?: string
  /** The binding refers to a role that does not exist: it grants nothing. */
  roleMissing?: boolean
}

export interface Grant extends GrantPath {
  rule: PolicyRule
}

export interface SubjectGrants extends Subject {
  grants: Grant[]
}

export interface WhoCanResult {
  request: AccessRequest
  subjects: SubjectGrants[]
}

export interface SubjectSummary extends Subject {
  clusterBindings: number
  /** Namespaces where RoleBindings name the subject. */
  namespaces: string[]
  roles: RbacRef[]
  /** A ServiceAccount bindings name that does not exist. */
  missing?: boolean
}

export interface ScopedRule extends PolicyRule {
  path: GrantPath
}

export interface ScopeRules {
  /** "" = cluster-wide. */
  namespace: string
  rules: ScopedRule[]
}

export interface SubjectDetail {
  subject: Subject
  /** The identity the API server sees. */
  user?: string
  groups: string[]
  missing?: boolean
  bindings: GrantPath[]
  rules: ScopeRules[]
}

export type Severity = 'critical' | 'high' | 'medium'

export type RiskId =
  | 'full-access'
  | 'escalate-bind'
  | 'impersonate'
  | 'wildcard-verbs'
  | 'wildcard-resources'
  | 'secrets-read'
  | 'pod-exec'
  | 'token-create'
  | 'rbac-write'
  | 'node-proxy'
  | 'webhook-write'
  | 'csr-approve'
  | 'workload-create'
  | 'persistentvolume-create'
  // OpenShift (design §7.15): "use" of a privileged or an anyuid SecurityContextConstraints.
  | 'scc-privileged'
  | 'scc-anyuid'

export interface Risk {
  id: RiskId
  severity: Severity
  rule: PolicyRule
}

export interface RiskyRole {
  role: RbacRef
  aggregated?: boolean
  severity: Severity
  risks: Risk[]
  bindings: RbacRef[]
  subjects: number
}

export interface GraphNode {
  id: string
  kind: string
  namespace?: string
  name: string
  missing?: boolean
}

export interface GraphEdge {
  from: string
  to: string
}

export interface BindingGraph {
  subjects: GraphNode[]
  bindings: GraphNode[]
  roles: GraphNode[]
  edges: GraphEdge[]
  truncated: boolean
}

export interface ReviewRequest extends AccessRequest {
  subject: Subject
}

/** The API server's SubjectAccessReview answer, next to KubeBench's own evaluation (`grants`). */
export interface ReviewResult {
  allowed: boolean
  denied?: boolean
  reason?: string
  evaluationError?: string
  user?: string
  groups: string[]
  grants: Grant[]
}
