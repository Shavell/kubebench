import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { screenZoom, xtermOptions } from './scale.ts'

describe('terminal under the interface scale', () => {
  it('cancels the zoom on the screen so pointer positions map to the right cells', () => {
    assert.equal(screenZoom(1), '')
    for (const scale of [0.9, 1.1, 1.2, 1.3]) {
      assert.ok(Math.abs(Number(screenZoom(scale)) * scale - 1) < 1e-9, `scale ${scale}`)
    }
  })

  it('puts the scale into the font size instead', () => {
    const theme = { background: '#000' }
    assert.deepEqual(xtermOptions({ fontSize: 12.5, fontFamily: 'mono', theme, scale: 1.3 }), {
      fontSize: 16.25,
      fontFamily: 'mono',
      theme,
    })
    assert.equal(xtermOptions({ fontSize: 14, fontFamily: 'mono', theme, scale: 1 }).fontSize, 14)
  })
})
