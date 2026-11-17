import type { PluginApi } from '@/data/pluginData'
import type { NoteKey, NotesProvider } from './provider'
import type { Note, NoteList, NoteObjectFilter, NoteQuery, ObjectNoteCount, TagCount } from './types'

function objectParams(resource: NoteObjectFilter | undefined) {
  return {
    apiVersion: resource?.apiVersion,
    kind: resource?.kind,
    namespace: resource?.namespace,
    name: resource?.name,
  }
}

function queryParams(query: NoteQuery = {}) {
  return {
    scope: query.scope,
    clusters: query.clusters,
    ...objectParams(query.resource),
    tag: query.tag,
    q: query.q?.trim(),
    body: query.body ? 'true' : undefined,
    limit: query.limit,
  }
}

const notePath = (note: NoteKey) => `/notes/${note.visibility}/${note.id}`

/** NotesProvider over /api/plugins/notes. */
export function createLiveNotes(api: PluginApi): NotesProvider {
  return {
    list: (query) => api.get<NoteList>('/notes', queryParams(query)),
    get: (id) => api.get<Note>(`/notes/${id}`),
    tags: async (query) => (await api.get<{ tags: TagCount[] }>('/tags', queryParams(query))).tags,
    counts: async (resource) =>
      (await api.get<{ items: ObjectNoteCount[] }>('/counts', objectParams(resource))).items,
    create: ({ visibility, global, ...note }) =>
      api.post<Note>(`/notes/${visibility}`, visibility === 'personal' ? { ...note, global } : note),
    update: (note, content, version) => api.put<Note>(notePath(note), { ...content, version }),
    pin: (note, pinned) => api.put<Note>(`${notePath(note)}/pin`, { pinned }),
    remove: async (note) => {
      await api.del(notePath(note))
    },
  }
}
