import type {
  HelmChartRef,
  HelmChartResult,
  HelmChartValues,
  HelmInstallRequest,
  HelmInstallResult,
  HelmRelease,
  HelmReleaseDetail,
  HelmRepo,
  HelmRepoInput,
  HelmRepoUpdate,
  HelmUpgradeRequest,
  HelmUpgradeResult,
  HelmUpload,
  HelmValuesQuery,
} from './types'

/** The Helm plugin's data capability (design §7.4), live or mock. */
export interface HelmData {
  /** Latest revision of every release; `namespace` limits the list to one. */
  listReleases(namespace?: string): Promise<HelmRelease[]>
  getRelease(namespace: string, name: string): Promise<HelmReleaseDetail>
  /** Values of a revision as YAML ("" when there are none). */
  getValues(namespace: string, name: string, query?: HelmValuesQuery): Promise<string>
  /** Rendered manifest of a revision (0: the latest), hooks included. */
  getManifest(namespace: string, name: string, revision?: number): Promise<string>
  /** Creates a new revision from the chart and values of `revision`, like `helm rollback`. */
  rollback(namespace: string, name: string, revision: number): Promise<HelmRelease>
  /** `helm uninstall`; with `keepHistory` the release stays listed as uninstalled. */
  uninstall(namespace: string, name: string, keepHistory: boolean): Promise<void>
  install(request: HelmInstallRequest): Promise<HelmInstallResult>
  upgrade(namespace: string, name: string, request: HelmUpgradeRequest): Promise<HelmUpgradeResult>

  listRepos(): Promise<HelmRepo[]>
  addRepo(input: HelmRepoInput): Promise<HelmRepo>
  removeRepo(name: string): Promise<void>
  /** Downloads every repository's index again, like `helm repo update`. */
  updateRepos(): Promise<HelmRepoUpdate[]>

  /** Charts of the added repositories matching `query` ("" lists them all, at most 100). */
  searchCharts(query: string): Promise<HelmChartResult[]>
  /** Metadata and default values.yaml of a chart version. */
  chartValues(chart: HelmChartRef): Promise<HelmChartValues>
  /** Stores a chart archive (.tgz, at most 20 MiB) for 24 hours to install from. */
  uploadChart(file: File): Promise<HelmUpload>
}
