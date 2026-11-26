import assert from 'node:assert/strict'
import { afterEach, beforeEach, describe, it, mock } from 'node:test'
import { MAX_GAP_MS, refreshData, refreshTopics, registerQuery, type QueryHandle } from './invalidation.ts'

const PODS = [{ topic: 'pods', namespace: 'shop' }]

/** A query whose reads take `readMs` (0 = they end at once). */
function query(readMs = 0) {
  const q = { runs: [] as number[], handle: null as unknown as QueryHandle }
  q.handle = registerQuery({ topics: ['pods'], namespace: 'shop' }, () => {
    q.runs.push(Date.now())
    q.handle.begin()
    if (readMs === 0) q.handle.end()
    else setTimeout(() => q.handle.end(), readMs)
  })
  return q
}

describe('invalidation pacing', () => {
  beforeEach(() => mock.timers.enable({ apis: ['setTimeout', 'Date'], now: 1_000_000 }))
  afterEach(() => mock.timers.reset())

  it('refreshes at once when changes are rare', () => {
    const q = query()
    refreshTopics(PODS)
    mock.timers.tick(30_000)
    refreshTopics(PODS)
    assert.equal(q.runs.length, 2)
    q.handle.unregister()
  })

  it('backs off while changes are continuous and keeps the last one', () => {
    const q = query()
    for (let t = 0; t < 60_000; t += 500) {
      refreshTopics(PODS)
      mock.timers.tick(500)
    }
    const lastChange = Date.now() - 500
    for (let t = 0; t < MAX_GAP_MS; t += 100) mock.timers.tick(100)
    // 120 changes in a minute: at most one refresh per MAX_GAP_MS after the first few.
    assert.ok(q.runs.length <= 60_000 / MAX_GAP_MS + 5, `${q.runs.length} refreshes`)
    assert.ok(q.runs.length >= 60_000 / MAX_GAP_MS - 2, `${q.runs.length} refreshes`)
    assert.ok(q.runs.at(-1)! >= lastChange, 'the last change was refreshed')
    for (let i = 1; i < q.runs.length; i++) assert.ok(q.runs[i] - q.runs[i - 1] <= MAX_GAP_MS)
    q.handle.unregister()
  })

  it('never overlaps a running read', () => {
    const q = query(3_000)
    refreshTopics(PODS)
    for (let i = 0; i < 5; i++) {
      mock.timers.tick(500)
      refreshTopics(PODS)
    }
    assert.equal(q.runs.length, 1)
    mock.timers.tick(500) // the read ends at 3 s
    mock.timers.tick(MAX_GAP_MS)
    assert.equal(q.runs.length, 2, 'one more refresh for the changes during the read')
    q.handle.unregister()
  })

  it('marks hidden queries stale and refreshes them once when shown', () => {
    const q = query()
    q.handle.setActive(false)
    for (let i = 0; i < 10; i++) {
      refreshTopics(PODS)
      refreshData()
      mock.timers.tick(1_000)
    }
    assert.equal(q.runs.length, 0)
    q.handle.setActive(true)
    assert.equal(q.runs.length, 1)
    q.handle.setActive(false)
    q.handle.setActive(true)
    assert.equal(q.runs.length, 1, 'nothing missed, nothing to refresh')
    q.handle.unregister()
  })

  it('refreshes visible queries at once after a change the UI made', () => {
    const q = query()
    for (let i = 0; i < 6; i++) {
      refreshTopics(PODS)
      mock.timers.tick(100)
    }
    const before = q.runs.length
    refreshData()
    assert.equal(q.runs.length, before + 1)
    q.handle.unregister()
  })

  it('ignores other topics and namespaces', () => {
    const q = query()
    refreshTopics([{ topic: 'pods', namespace: 'blog' }, { topic: 'services', namespace: 'shop' }])
    assert.equal(q.runs.length, 0)
    refreshTopics([{ topic: 'pods', namespace: '' }])
    assert.equal(q.runs.length, 1)
    q.handle.unregister()
  })
})
