import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { frameLocation } from './frameLocation.ts'

describe('frame location', () => {
  it('takes the first navigation of a load, not its nested frames', () => {
    const navs = [
      { seq: 1, location: '/' },
      { seq: 2, location: '/withframe' },
      { seq: 3, location: '/inner-widget' },
    ]
    assert.deepEqual(frameLocation(navs, 1), { location: '/withframe', seen: 3 })
  })

  it('knows nothing when no navigation reached the proxy', () => {
    assert.deepEqual(frameLocation([{ seq: 4, location: '/a' }], 4), { location: null, seen: 4 })
    assert.deepEqual(frameLocation([], 0), { location: null, seen: 0 })
  })

  it('skips a location too long to keep, but moves past it', () => {
    assert.deepEqual(frameLocation([{ seq: 5, location: '' }, { seq: 6, location: '/w' }], 4), { location: null, seen: 6 })
  })
})
