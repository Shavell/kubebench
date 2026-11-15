/**
 * Shapes of the Argo CD plugin API (/api/plugins/argocd, design §7.5),
 * mirrored from backend/internal/plugins/argocd/types.go. Fields Argo CD
 * has not filled in yet are empty strings or lists.
 */

export interface ArgoDestination {
  server: string
  name: string
  namespace: string
}

export interface ArgoSource {
  repoURL: string
  path: string
  chart: string
  targetRevision: string
  ref: string
}

/** Synced | OutOfSync | Unknown */
export type ArgoSyncStatus = string
/** Healthy | Progressing | Degraded | Suspended | Missing | Unknown */
export type ArgoHealthStatus = string

export interface ArgoApplicationSummary {
  name: string
  namespace: string
  project: string
  repo: string
  path: string
  chart: string
  targetRevision: string
  sourceCount: number
  destination: ArgoDestination
  sync: ArgoSyncStatus
  health: ArgoHealthStatus
  /** Running | Terminating | Succeeded | Failed | Error, or '' when no operation ran. */
  operationPhase: string
  /** An operation is requested or running; sync and rollback answer 409 meanwhile. */
  operating: boolean
  lastSyncedAt: string
  revision: string
  autoSync: boolean
  deleting: boolean
}

export interface ArgoSyncPolicy {
  automated: boolean
  prune: boolean
  selfHeal: boolean
  syncOptions: string[]
}

export interface ArgoResource {
  group: string
  version: string
  kind: string
  namespace: string
  name: string
  status: string
  health: string
  healthMessage: string
  hook: boolean
  requiresPruning: boolean
  /** Key of KubeBench's generic resource list for the kind, '' when there is none. */
  catalogKey: string
}

export interface ArgoCondition {
  type: string
  status: string
  reason: string
  message: string
  lastTransitionTime: string
}

export interface ArgoHistoryEntry {
  id: number
  revision: string
  revisions: string[]
  deployedAt: string
  deployStartedAt: string
  sources: ArgoSource[]
  initiatedBy: string
  automated: boolean
}

export interface ArgoResultResource {
  group: string
  kind: string
  namespace: string
  name: string
  status: string
  message: string
  hookPhase: string
  syncPhase: string
}

export interface ArgoOperationState {
  phase: string
  message: string
  startedAt: string
  finishedAt: string
  initiatedBy: string
  automated: boolean
  revision: string
  dryRun: boolean
  prune: boolean
  retryCount: number
  resources: ArgoResultResource[]
}

export interface ArgoApplicationDetail extends ArgoApplicationSummary {
  sources: ArgoSource[]
  syncPolicy: ArgoSyncPolicy
  healthMessage: string
  reconciledAt: string
  resources: ArgoResource[]
  conditions: ArgoCondition[]
  /** Newest first. */
  history: ArgoHistoryEntry[]
  operation: ArgoOperationState | null
  images: string[]
  urls: string[]
}

export interface ArgoProject {
  name: string
  namespace: string
  description: string
  sourceRepos: string[]
  destinations: ArgoDestination[]
  /** Allowed cluster-scoped kinds as "group/kind". */
  clusterResources: string[]
  roles: string[]
  syncWindows: number
}

export interface ArgoAppSetApplication {
  name: string
  status: string
  health: string
  message: string
}

export interface ArgoApplicationSet {
  name: string
  namespace: string
  generators: string[]
  templateName: string
  project: string
  applications: ArgoAppSetApplication[]
  conditions: ArgoCondition[]
}

/** One resource of a selective sync. */
export interface ArgoResourceKey {
  group: string
  kind: string
  namespace: string
  name: string
}

/** Body of POST /applications/{ns}/{name}/sync. */
export interface ArgoSyncRequest {
  /** Empty syncs the source's targetRevision. */
  revision?: string
  prune?: boolean
  dryRun?: boolean
  /** Only these resources; omitted syncs all. */
  resources?: ArgoResourceKey[]
  /** Replaces the application's sync options for this sync; omitted keeps them. */
  syncOptions?: string[]
}
