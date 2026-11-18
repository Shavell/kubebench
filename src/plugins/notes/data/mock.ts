import { currentPermissions } from '@/core/permissions'
import { ApiError } from '@/data/apiClient'
import { reasonError } from '@/data/reasonError'
import { delay, delayReject } from '@/data/mock/delay'
import { mockClusterNames } from '@/data/mock/mockInventory'
import { dataMode } from '@/data/mode'
import { contentProblems, MAX_NOTES_PER_USER, objectMatches, parseTags } from '../model/rules'
import type { NoteKey, NotesProvider } from './provider'
import type { NewNote, Note, NoteContent, NoteObjectRef, NoteQuery, ObjectNoteCount, TagCount } from './types'

/**
 * Mock notes (design §7.13): a few fixed notes on the first two mock
 * clusters, kept in memory with the backend's visibility and permission
 * rules. Nothing reads the clock: seeds carry fixed times and new notes are
 * stamped one minute apart from a fixed base time.
 */

const BASE_TIME = Date.parse('2026-11-02T09:00:00Z')
const DEV_CLUSTER = 1
const STAGING_CLUSTER = 2
/** Another user's name, so shared notes show more than one author. */
const COLLEAGUE = 'ops-lead'

const stamp = (minutes: number) => new Date(BASE_TIME + minutes * 60_000).toISOString().replace('.000Z', 'Z')

const deployment = (namespace: string, name: string): NoteObjectRef => ({ apiVersion: 'apps/v1', kind: 'Deployment', namespace, name })

interface MockNote extends Omit<Note, 'clusterName'> {
  /** The owner of a personal note. */
  owner?: string
}

function seeds(me: string): MockNote[] {
  const note = (id: number, fields: Partial<MockNote> & Pick<MockNote, 'visibility' | 'title' | 'body'>, minutes: number): MockNote => ({
    id,
    clusterId: DEV_CLUSTER,
    resource: null,
    tags: [],
    pinned: false,
    version: 1,
    createdBy: me,
    createdAt: stamp(minutes - 600),
    updatedBy: me,
    updatedAt: stamp(minutes),
    ...fields,
  })
  return [
    note(1, {
      visibility: 'shared',
      title: 'shop-payments: known crash loop',
      resource: deployment('shop', 'shop-payments'),
      tags: ['incident', 'shop'],
      pinned: true,
      createdBy: COLLEAGUE,
      updatedBy: COLLEAGUE,
      version: 3,
      body: [
        '## Status',
        '',
        'The `3.1.0-rc.2` image fails to read `STRIPE_API_KEY` at start-up. **Do not scale it up**:',
        'every new replica crash-loops as well.',
        '',
        '- Rollback target: revision 4 (`3.0.7`)',
        '- Owner: team payments, see [the incident doc](https://example.com/incidents/4711)',
      ].join('\n'),
    }, -30),
    note(2, {
      visibility: 'shared',
      title: 'Maintenance window',
      tags: ['runbook'],
      body: 'Node upgrades happen on **Tuesdays 06:00–08:00 UTC**. Cordon one worker at a time and wait for the shop Pods to be ready again.',
    }, -2_880),
    note(3, {
      visibility: 'personal',
      owner: me,
      title: 'Frontend rollout checklist',
      resource: deployment('shop', 'shop-frontend'),
      tags: ['shop'],
      body: '1. Check the canary error rate\n2. Raise `replicas` to 3\n3. Watch the Pending Pod on `kb-worker-2`',
    }, -120),
    note(4, {
      visibility: 'personal',
      owner: me,
      clusterId: null,
      title: 'kubectl snippets',
      tags: ['cli'],
      body: '```\nkubectl get pods -A --field-selector=status.phase!=Running\nkubectl top pods -n shop --sort-by=memory\n```',
    }, -4_320),
    note(5, {
      visibility: 'personal',
      owner: me,
      clusterId: STAGING_CLUSTER,
      title: 'Staging database credentials rotate monthly',
      tags: ['runbook'],
      body: 'Ask the platform team before the 1st of each month.',
    }, -10_080),
  ]
}

const notFound = () => reasonError(404, 'not_found', 'notes.not_found')

function canContribute(): boolean {
  const { role } = currentPermissions()
  return role === 'admin' || role === 'operator'
}

export function createMockNotes(): NotesProvider {
  const me = dataMode.user ?? 'admin'
  const notes = seeds(me)
  let nextId = notes.length + 1
  let clock = 0

  const activeCluster = () => dataMode.cluster?.id ?? DEV_CLUSTER
  const view = (n: MockNote): Note => {
    const { owner: _owner, ...rest } = n
    return { ...rest, clusterName: n.clusterId === null ? '' : (mockClusterNames().get(n.clusterId) ?? '') }
  }
  const visible = (n: MockNote) =>
    n.visibility === 'personal' ? n.owner === me : n.clusterId === activeCluster()

  const select = (query: NoteQuery = {}) => {
    const cluster = activeCluster()
    const terms = (query.q ?? '').toLowerCase().split(/\s+/).filter(Boolean)
    return notes
      .filter((n) => {
        if (!visible(n)) return false
        if (query.scope && query.scope !== 'all' && n.visibility !== query.scope) return false
        const onCurrent = n.clusterId === null || n.clusterId === cluster
        if (!onCurrent && (query.clusters !== 'all' || query.resource)) return false
        if (query.resource && !(n.clusterId === cluster && n.resource && objectMatches(n.resource, query.resource))) return false
        if (query.tag && !n.tags.includes(query.tag.toLowerCase())) return false
        const text = `${n.title}\n${n.body}`.toLowerCase()
        return terms.every((term) => text.includes(term))
      })
      .sort((a, b) => Number(b.pinned) - Number(a.pinned) || b.updatedAt.localeCompare(a.updatedAt) || b.id - a.id)
  }

  /** The note a mutation may change, or the error the backend would answer. */
  const target = (key: NoteKey): MockNote | ApiError => {
    if (key.visibility === 'shared' && !canContribute()) {
      return new ApiError(403, 'forbidden_role', 'your role on this cluster cannot change shared notes')
    }
    const note = notes.find((n) => n.id === key.id)
    return note && visible(note) && note.visibility === key.visibility ? note : notFound()
  }

  const validate = (content: NoteContent): ApiError | null => {
    const problems = contentProblems(content.title, content.body, content.tags.join(' '))
    const field = (['title', 'body', 'tags'] as const).find((f) => problems[f])
    return field ? new ApiError(400, 'bad_request', `${field}: invalid`) : null
  }

  return {
    list: (query) => {
      const all = select(query)
      const limit = query?.limit ?? 200
      const items = all.slice(0, limit).map((n) => ({ ...view(n), body: query?.body ? n.body : '' }))
      return delay({ items, truncated: all.length > limit })
    },
    get: (id) => {
      const note = notes.find((n) => n.id === id)
      return note && visible(note) ? delay(view(note)) : delayReject(notFound())
    },
    tags: (query) => {
      const counts = new Map<string, number>()
      for (const n of select({ ...query, clusters: 'current' })) n.tags.forEach((t) => counts.set(t, (counts.get(t) ?? 0) + 1))
      const tags: TagCount[] = [...counts].map(([tag, count]) => ({ tag, count }))
      return delay(tags.sort((a, b) => b.count - a.count || a.tag.localeCompare(b.tag)))
    },
    counts: (resource) => {
      const byObject = new Map<string, ObjectNoteCount>()
      for (const n of select({ resource: { ...resource } })) {
        if (!n.resource) continue
        const key = `${n.resource.kind}/${n.resource.namespace}/${n.resource.name}`
        const entry = byObject.get(key) ?? { ...n.resource, personal: 0, shared: 0 }
        entry[n.visibility] += 1
        byObject.set(key, entry)
      }
      return delay([...byObject.values()])
    },
    create: (input: NewNote) => {
      if (input.visibility === 'shared' && !canContribute()) {
        return delayReject(new ApiError(403, 'forbidden_role', 'your role on this cluster cannot change shared notes'))
      }
      const tags = parseTags(input.tags.join(' ')).tags
      const invalid = validate({ ...input, tags })
      if (invalid) return delayReject(invalid)
      if (notes.filter((n) => n.createdBy === me).length >= MAX_NOTES_PER_USER) {
        return delayReject(reasonError(409, 'conflict', 'notes.limit_reached', { max: MAX_NOTES_PER_USER }))
      }
      const at = stamp(++clock)
      const note: MockNote = {
        id: nextId++,
        visibility: input.visibility,
        owner: input.visibility === 'personal' ? me : undefined,
        clusterId: input.global ? null : activeCluster(),
        resource: input.resource ?? null,
        title: input.title.trim(),
        body: input.body,
        tags,
        pinned: input.pinned,
        version: 1,
        createdBy: me,
        createdAt: at,
        updatedBy: me,
        updatedAt: at,
      }
      notes.push(note)
      return delay(view(note))
    },
    update: (key, content, version) => {
      const note = target(key)
      if (note instanceof ApiError) return delayReject(note)
      const tags = parseTags(content.tags.join(' ')).tags
      const invalid = validate({ ...content, tags })
      if (invalid) return delayReject(invalid)
      if (note.version !== version) {
        return delayReject(reasonError(409, 'conflict', 'notes.changed'))
      }
      Object.assign(note, { title: content.title.trim(), body: content.body, tags, version: note.version + 1, updatedBy: me, updatedAt: stamp(++clock) })
      return delay(view(note))
    },
    pin: (key, pinned) => {
      const note = target(key)
      if (note instanceof ApiError) return delayReject(note)
      note.pinned = pinned
      return delay(view(note))
    },
    remove: (key) => {
      const note = target(key)
      if (note instanceof ApiError) return delayReject(note)
      if (note.visibility === 'shared' && currentPermissions().role !== 'admin' && note.createdBy !== me) {
        return delayReject(reasonError(403, 'forbidden', 'notes.delete_shared'))
      }
      notes.splice(notes.indexOf(note), 1)
      return delay(undefined)
    },
  }
}
