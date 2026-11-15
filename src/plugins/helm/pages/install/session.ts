import type { HelmChartInfo, HelmChartRef, HelmInstallResult, HelmUpload } from '../../data'

export type InstallStep = 'chart' | 'values' | 'review'
export type ChartSource = 'repo' | 'oci' | 'upload'

/** The install wizard's state, kept while its tab is open (see sessions.ts). */
export interface InstallSession {
  step: InstallStep
  source: ChartSource
  /** Chart search of the repository source. */
  query: string
  selected: { repo: string; name: string } | null
  /** Version of the selected repository chart; "" = its latest. */
  version: string
  ociRef: string
  /** "" = the registry's latest version. */
  ociVersion: string
  upload: HelmUpload | null
  /** The chart the values step loaded, and its metadata. */
  chart: HelmChartRef | null
  chartInfo: HelmChartInfo | null
  /** Values being edited, starting from the chart defaults. */
  values: string
  name: string
  namespace: string
  createNamespace: boolean
  /** The last dry run, for the current inputs (any change clears it). */
  result: HelmInstallResult | null
}

export const initialSession = (namespace: string): InstallSession => ({
  step: 'chart',
  source: 'repo',
  query: '',
  selected: null,
  version: '',
  ociRef: '',
  ociVersion: '',
  upload: null,
  chart: null,
  chartInfo: null,
  values: '',
  name: '',
  namespace,
  createNamespace: false,
  result: null,
})

/** The chart the chart step names, or null while it names none. */
export function chartRefOf(s: InstallSession): HelmChartRef | null {
  switch (s.source) {
    case 'repo':
      return s.selected ? { repo: s.selected.repo, name: s.selected.name, version: s.version || undefined } : null
    case 'oci':
      return s.ociRef.trim().startsWith('oci://') ? { ref: s.ociRef.trim(), version: s.ociVersion.trim() || undefined } : null
    case 'upload':
      return s.upload ? { uploadId: s.upload.uploadId } : null
  }
}

/** Release names are DNS names of at most 53 characters (Helm's limit). */
export const RELEASE_NAME = /^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$/
export const NAMESPACE_NAME = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/

export function nameError(name: string): 'required' | 'invalid' | null {
  if (!name) return 'required'
  return name.length <= 53 && RELEASE_NAME.test(name) ? null : 'invalid'
}

export function namespaceError(namespace: string): 'required' | 'invalid' | null {
  if (!namespace) return 'required'
  return namespace.length <= 63 && NAMESPACE_NAME.test(namespace) ? null : 'invalid'
}
