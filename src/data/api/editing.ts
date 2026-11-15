import { apiDelete, apiFetch, apiGet, apiPatch, apiPost, type QueryParams } from '../apiClient'
import type { ClusterProvider } from '../providers'
import type { ApplyResult, KubeObject, ObjectRef } from '../types'

/** The query parameters that name an object on the generic endpoints. */
function objectParams(ref: ObjectRef): QueryParams {
  return { apiVersion: ref.apiVersion, kind: ref.kind, ns: ref.namespace, name: ref.name }
}

/** `flag=true`, or nothing: the server reads exactly "true" (dryRun, force, neat). */
const flag = (on: boolean | undefined) => (on ? 'true' : undefined)

/** The resource editing endpoints of design §7.2. */
export const apiEditing: Pick<
  ClusterProvider,
  'getYaml' | 'applyYaml' | 'editYaml' | 'deleteResource' | 'patchMetadata' | 'revealSecret'
> = {
  getYaml: async (ref, options = {}) =>
    (await apiGet<{ yaml: string }>('/yaml', { ...objectParams(ref), neat: flag(options.neat) })).yaml,
  applyYaml: async (manifests, options = {}) => {
    const body = await apiFetch<{ results: ApplyResult[] }>('/resource/apply', {
      method: 'POST',
      body: manifests,
      headers: { 'Content-Type': 'application/yaml' },
      params: { ns: options.namespace, dryRun: flag(options.dryRun), force: flag(options.force) },
    })
    return body.results
  },
  editYaml: async (ref, original, edited, options = {}) =>
    (
      await apiPost<{ results: ApplyResult[] }>(
        '/resource/edit',
        { original, edited },
        { ...objectParams(ref), dryRun: flag(options.dryRun) },
      )
    ).results,
  deleteResource: async (ref, options = {}) => {
    await apiDelete('/resource', {
      ...objectParams(ref),
      propagation: options.propagation,
      gracePeriod: options.gracePeriodSeconds,
    })
  },
  patchMetadata: async (ref, patch) => {
    await apiPatch('/resource/metadata', patch, objectParams(ref))
  },
  revealSecret: (namespace, name) => apiPost<KubeObject>('/secrets/reveal', undefined, { ns: namespace, name }),
}
