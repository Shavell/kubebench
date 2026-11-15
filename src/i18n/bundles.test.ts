import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { localeBundles, subtreeBundles } from './bundles.ts'

describe('localeBundles', () => {
  it('keys each bundle by its file name', () => {
    const en = { a: 'A' }
    const de = { a: 'Ä' }
    assert.deepEqual(localeBundles({ './locales/en.json': en, './locales/de.json': de }), { en, de })
    assert.deepEqual(localeBundles({ '/src/core/locales/pt-BR.json': en }), { 'pt-BR': en })
  })

  it('skips entries that are not JSON objects', () => {
    assert.deepEqual(localeBundles({ './locales/en.json': 'text', './locales/readme.md': {} }), {})
  })
})

describe('subtreeBundles', () => {
  it('picks one top-level section of every language', () => {
    const bundles = { en: { workbench: { x: 'X' }, other: {} }, de: { workbench: { x: 'Y' } }, tr: {} }
    assert.deepEqual(subtreeBundles(bundles, 'workbench'), { en: { x: 'X' }, de: { x: 'Y' } })
  })
})
