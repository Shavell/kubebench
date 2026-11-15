/**
 * Shapes of the Helm plugin's API (`/api/plugins/helm`, design §7.4);
 * they mirror `backend/internal/plugins/helm`. Timestamps are RFC 3339
 * strings ("" when unknown).
 */

/** Helm's release status, e.g. `deployed`, `failed`, `pending-upgrade`, `superseded`, `uninstalled`. */
export type HelmReleaseStatus = string

/** A release as the list shows it: its latest revision. */
export interface HelmRelease {
  name: string
  namespace: string
  chart: string
  chartVersion: string
  appVersion: string
  revision: number
  status: HelmReleaseStatus
  updated: string
  description: string
}

export interface HelmRevision {
  revision: number
  status: HelmReleaseStatus
  updated: string
  description: string
  chart: string
  chartVersion: string
  appVersion: string
}

/** Chart.yaml metadata. */
export interface HelmChartInfo {
  name: string
  version: string
  appVersion: string
  description: string
  home: string
  icon: string
  sources: string[]
  keywords: string[]
  kubeVersion: string
  deprecated: boolean
}

export interface HelmHook {
  name: string
  kind: string
  events: string[]
  weight: number
  /** Phase of the last run (`Succeeded`, `Failed`, …); "" when it never ran. */
  phase: string
  startedAt: string
  completedAt: string
}

/** GET /releases/{ns}/{name}: the latest revision with chart, notes, hooks and history (newest first). */
export interface HelmReleaseDetail extends HelmRelease {
  firstDeployed: string
  notes: string
  chartInfo: HelmChartInfo
  hooks: HelmHook[]
  history: HelmRevision[]
}

/** A chart repository; credentials are never returned, `authenticated` only says some are stored. */
export interface HelmRepo {
  name: string
  url: string
  authenticated: boolean
  chartCount: number
  updated: string
  /** Why the cached index cannot be read (e.g. never downloaded). */
  error?: string
}

export interface HelmRepoInput {
  name: string
  url: string
  username?: string
  password?: string
}

export interface HelmRepoUpdate {
  name: string
  ok: boolean
  error?: string
}

export interface HelmChartVersion {
  version: string
  appVersion: string
  created: string
  deprecated: boolean
}

/** A chart of an added repository, by its latest version, with every version (newest first). */
export interface HelmChartResult {
  repo: string
  name: string
  version: string
  appVersion: string
  description: string
  icon: string
  deprecated: boolean
  versions: HelmChartVersion[]
}

/** The chart of an install or upgrade: from a repository, an OCI registry, or an upload. */
export type HelmChartRef =
  | { repo: string; name: string; version?: string }
  | { ref: string; version?: string }
  | { uploadId: string }

export interface HelmChartValues {
  chart: HelmChartInfo
  /** The chart's values.yaml as written, comments included. */
  values: string
}

export interface HelmUpload {
  uploadId: string
  chart: HelmChartInfo
  expiresAt: string
}

export interface HelmInstallRequest {
  name: string
  namespace: string
  createNamespace: boolean
  chart: HelmChartRef
  /** YAML overriding the chart defaults. */
  values: string
  dryRun: boolean
}

export interface HelmInstallResult {
  release: HelmRelease
  manifest: string
  notes: string
}

export interface HelmUpgradeRequest {
  /** Omitted: the release's current chart (a values-only upgrade). */
  chart?: HelmChartRef
  /** YAML; merged over the current values with `reuseValues`, else replacing them. */
  values: string
  reuseValues: boolean
  dryRun: boolean
}

export interface HelmUpgradeResult {
  release: HelmRelease
  manifest: string
  notes: string
  /** Manifest of the revision being replaced, for the diff. */
  currentManifest: string
}

export interface HelmValuesQuery {
  /** 0 or absent: the latest revision. */
  revision?: number
  /** Merged with the chart defaults instead of only the user-supplied values. */
  all?: boolean
}
