import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { fontStack, isValidFontFamily } from './fontFamily.ts'

describe('isValidFontFamily', () => {
  it('accepts empty, plain, quoted and listed names', () => {
    for (const value of ['', 'Menlo', 'JetBrains Mono', '"Fira Code", monospace', "'Cascadia Code PL', Consolas"]) {
      assert.equal(isValidFontFamily(value), true, value)
    }
  })

  it('refuses declarations, unbalanced quotes, empty names and long lists', () => {
    for (const value of ['Menlo; color: red', 'a{}', '"Fira Code', 'Fira"Code', 'Menlo,,Consolas', 'Menlo,', 'x'.repeat(201)]) {
      assert.equal(isValidFontFamily(value), false, value)
    }
  })
})

describe('fontStack', () => {
  it('puts a valid choice ahead of the fallback', () => {
    assert.equal(fontStack(' Fira Code ', 'monospace'), 'Fira Code, monospace')
  })

  it('uses the fallback alone without a valid choice', () => {
    assert.equal(fontStack('', 'monospace'), 'monospace')
    assert.equal(fontStack('Menlo; x', 'monospace'), 'monospace')
  })
})
