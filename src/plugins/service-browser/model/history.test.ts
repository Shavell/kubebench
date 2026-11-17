import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { canGoBack, canGoForward, currentPath, goBack, goForward, startHistory, visit } from './history.ts'

describe('navigation history', () => {
  it('records new locations and ignores the current one', () => {
    let h = startHistory('/')
    h = visit(h, '/a')
    h = visit(h, '/a')
    h = visit(h, '/b')
    assert.deepEqual(h, { entries: ['/', '/a', '/b'], index: 2 })
    assert.equal(canGoForward(h), false)
  })

  it('goes back and forward without recording the visits again', () => {
    let h = visit(visit(startHistory('/'), '/a'), '/b')
    h = goBack(h)
    assert.equal(currentPath(h), '/a')
    h = visit(h, '/a') // the frame reports the page it was sent back to
    assert.equal(canGoForward(h), true)
    h = goForward(h)
    assert.equal(currentPath(h), '/b')
  })

  it('drops the forward entries on a new visit', () => {
    let h = goBack(goBack(visit(visit(startHistory('/'), '/a'), '/b')))
    assert.equal(canGoBack(h), false)
    h = visit(h, '/c')
    assert.deepEqual(h.entries, ['/', '/c'])
    assert.equal(goBack(goBack(h)).index, 0)
  })

  it('keeps at most 100 entries', () => {
    let h = startHistory('/0')
    for (let i = 1; i <= 150; i++) h = visit(h, `/${i}`)
    assert.equal(h.entries.length, 100)
    assert.equal(currentPath(h), '/150')
  })
})
