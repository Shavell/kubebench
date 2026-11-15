import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { moveIndex, scrollTopFor, visibleRange } from './virtualRange.ts'

describe('visibleRange', () => {
  const base = { viewportHeight: 270, bodyOffset: 30, rowHeight: 27, count: 400, overscan: 2 }

  it('renders the rows in view plus overscan', () => {
    // Rows 0–8 are in view below the 30px offset: 9 rows plus 2 of overscan.
    assert.deepEqual(visibleRange({ ...base, scrollTop: 0 }), { start: 0, end: 11 })
    // 2700px down, rows 98.9… to 108.8… are visible.
    assert.deepEqual(visibleRange({ ...base, scrollTop: 2700 }), { start: 96, end: 111 })
  })

  it('stays inside the list', () => {
    assert.deepEqual(visibleRange({ ...base, scrollTop: 99999 }), { start: 400, end: 400 })
    assert.deepEqual(visibleRange({ ...base, scrollTop: 10770 }), { start: 395, end: 400 })
    assert.deepEqual(visibleRange({ ...base, scrollTop: 0, count: 0 }), { start: 0, end: 0 })
  })
})

describe('scrollTopFor', () => {
  const view = { viewportHeight: 270, bodyOffset: 30, rowHeight: 27 }

  it('leaves visible rows alone', () => {
    assert.equal(scrollTopFor(3, { ...view, scrollTop: 0 }, 30), null)
  })

  it('scrolls up so the row clears the sticky header', () => {
    // Row 10 starts at 300; the 30px header covers the top of the viewport.
    assert.equal(scrollTopFor(10, { ...view, scrollTop: 400 }, 30), 270)
  })

  it('scrolls down just enough to show the row', () => {
    assert.equal(scrollTopFor(20, { ...view, scrollTop: 0 }, 30), 30 + 21 * 27 - 270)
  })
})

describe('moveIndex', () => {
  it('starts at an end and stays in range', () => {
    assert.equal(moveIndex(-1, 1, 5), 0)
    assert.equal(moveIndex(-1, -1, 5), 4)
    assert.equal(moveIndex(3, 10, 5), 4)
    assert.equal(moveIndex(1, -10, 5), 0)
    assert.equal(moveIndex(0, 1, 0), -1)
  })

  it('jumps to an end on Home/End, also without an active row', () => {
    assert.equal(moveIndex(-1, -5, 5), 0)
    assert.equal(moveIndex(-1, 5, 5), 4)
    assert.equal(moveIndex(3, -5, 5), 0)
  })
})
