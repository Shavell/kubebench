// Relative import with extension: unit-tested with node --test.
import type { NoteObjectFilter, NoteObjectRef } from '../data/types.ts'

/** Limits of design §7.13, mirrored from the backend for early feedback. */
export const MAX_TITLE = 200
export const MAX_BODY_BYTES = 64 * 1024
export const MAX_TAGS = 16
export const MAX_NOTES_PER_USER = 1000

const TAG = /^[\p{L}\p{N}][\p{L}\p{N}._-]{0,39}$/u

/**
 * Splits a comma or space separated tag list into normalized tags (sorted
 * by code point, as the server stores them) and the entries that are not tags.
 */
export function parseTags(text: string): { tags: string[]; invalid: string[] } {
  const tags: string[] = []
  const invalid: string[] = []
  for (const raw of text.split(/[\s,]+/)) {
    if (!raw) continue
    const tag = raw.toLowerCase()
    if (!TAG.test(tag)) invalid.push(raw)
    else if (!tags.includes(tag)) tags.push(tag)
  }
  return { tags: tags.sort(), invalid }
}

/** A reason a field cannot be saved: an i18n key in the notes namespace and its values. */
export interface Problem {
  key: string
  values: Record<string, string | number>
}

export interface ContentProblems {
  title?: Problem
  body?: Problem
  tags?: Problem
}

export function contentProblems(title: string, body: string, tagsText: string): ContentProblems {
  const problems: ContentProblems = {}
  const trimmed = title.trim()
  if (!trimmed) problems.title = { key: 'editor.errors.titleRequired', values: {} }
  else if ([...trimmed].length > MAX_TITLE) problems.title = { key: 'editor.errors.titleTooLong', values: { max: MAX_TITLE } }
  if (new TextEncoder().encode(body).length > MAX_BODY_BYTES) {
    problems.body = { key: 'editor.errors.bodyTooLarge', values: { max: MAX_BODY_BYTES / 1024 } }
  }
  const { tags, invalid } = parseTags(tagsText)
  if (invalid.length) problems.tags = { key: 'editor.errors.tagInvalid', values: { tag: invalid[0] } }
  else if (tags.length > MAX_TAGS) problems.tags = { key: 'editor.errors.tooManyTags', values: { max: MAX_TAGS } }
  return problems
}

/** API group of an apiVersion: "apps/v1" → "apps", "v1" → "". */
export function groupOf(apiVersion: string): string {
  const slash = apiVersion.indexOf('/')
  return slash < 0 ? '' : apiVersion.slice(0, slash)
}

/** Whether a note's object is the one `filter` names (absent filter fields match anything). */
export function objectMatches(object: NoteObjectRef, filter: NoteObjectFilter): boolean {
  if (filter.apiVersion !== undefined && groupOf(filter.apiVersion) !== groupOf(object.apiVersion)) return false
  if (filter.kind !== undefined && filter.kind !== object.kind) return false
  if (filter.name !== undefined && filter.name !== object.name) return false
  // An object named without a namespace is cluster-scoped.
  const namespace = filter.namespace ?? (filter.name !== undefined ? '' : undefined)
  return namespace === undefined || namespace === object.namespace
}

/** "Deployment shop/web" or "Node worker-1". */
export function objectLabel(object: Pick<NoteObjectRef, 'kind' | 'namespace' | 'name'>): string {
  return `${object.kind} ${object.namespace ? `${object.namespace}/` : ''}${object.name}`
}
