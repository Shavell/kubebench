import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { contentProblems, groupOf, objectLabel, objectMatches, parseTags } from './rules.ts'

describe('parseTags', () => {
  it('splits, lower-cases, deduplicates and sorts', () => {
    assert.deepEqual(parseTags(' Runbook, shop  runbook,çay-1'), { tags: ['runbook', 'shop', 'çay-1'], invalid: [] })
  })
  it('reports entries that are not tags', () => {
    assert.deepEqual(parseTags('ok -bad x!').invalid, ['-bad', 'x!'])
  })
})

describe('contentProblems', () => {
  it('needs a title and valid tags', () => {
    assert.deepEqual(Object.keys(contentProblems('  ', '', '')), ['title'])
    assert.equal(contentProblems('x'.repeat(201), '', '').title?.key, 'editor.errors.titleTooLong')
    assert.equal(contentProblems('ok', '', Array.from({ length: 17 }, (_, i) => `t${i}`).join(' ')).tags?.key, 'editor.errors.tooManyTags')
    assert.deepEqual(contentProblems('ok', 'body', 'a b'), {})
  })
  it('limits the body in bytes, not characters', () => {
    assert.equal(contentProblems('ok', 'ü'.repeat(32 * 1024 + 1), '').body?.key, 'editor.errors.bodyTooLarge')
    assert.equal(contentProblems('ok', 'u'.repeat(64 * 1024), '').body, undefined)
  })
})

describe('objectMatches', () => {
  const web = { apiVersion: 'apps/v1', kind: 'Deployment', namespace: 'shop', name: 'web' }
  it('matches by group, so every version names the same object', () => {
    assert.equal(objectMatches(web, { apiVersion: 'apps/v1beta2', kind: 'Deployment', namespace: 'shop', name: 'web' }), true)
    assert.equal(objectMatches(web, { apiVersion: 'extensions/v1beta1', kind: 'Deployment', namespace: 'shop', name: 'web' }), false)
    assert.equal(objectMatches(web, { kind: 'Deployment', namespace: 'shop', name: 'web' }), true)
  })
  it('treats a name without namespace as cluster-scoped', () => {
    assert.equal(objectMatches(web, { kind: 'Deployment', name: 'web' }), false)
    assert.equal(objectMatches({ apiVersion: 'v1', kind: 'Node', namespace: '', name: 'n1' }, { kind: 'Node', name: 'n1' }), true)
    assert.equal(objectMatches(web, { namespace: 'shop' }), true)
  })
})

describe('labels', () => {
  it('names objects and groups', () => {
    assert.equal(objectLabel({ kind: 'Deployment', namespace: 'shop', name: 'web' }), 'Deployment shop/web')
    assert.equal(objectLabel({ kind: 'Node', namespace: '', name: 'n1' }), 'Node n1')
    assert.equal(groupOf('v1'), '')
    assert.equal(groupOf('argoproj.io/v1alpha1'), 'argoproj.io')
  })
})
