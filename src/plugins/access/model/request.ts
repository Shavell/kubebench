import type { AccessRequest } from '../data/types'

/** A request being edited; `mode` picks a resource or a non-resource URL. */
export interface RequestDraft {
  mode: 'resource' | 'url'
  verb: string
  group: string
  resource: string
  subresource: string
  namespace: string
  name: string
  path: string
}

export const EMPTY_DRAFT: RequestDraft = {
  mode: 'resource',
  verb: 'get',
  group: '',
  resource: 'pods',
  subresource: '',
  namespace: '',
  name: '',
  path: '/healthz',
}

/** The request a draft asks, without empty fields. */
export function requestOf(d: RequestDraft): AccessRequest {
  if (d.mode === 'url') return { verb: d.verb.trim(), group: '', path: d.path.trim() }
  const r: AccessRequest = { verb: d.verb.trim(), group: d.group.trim(), resource: d.resource.trim() }
  if (d.subresource.trim()) r.subresource = d.subresource.trim()
  if (d.namespace) r.namespace = d.namespace
  if (d.name.trim()) r.name = d.name.trim()
  return r
}
