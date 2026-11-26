import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { isNamespaceName, normalizeAllowedNamespaces } from './rules.ts'

describe('allowed namespaces', () => {
  it('accepts namespace names only', () => {
    assert.equal(isNamespaceName('team-a'), true)
    assert.equal(isNamespaceName('Team_A'), false)
    assert.equal(isNamespaceName('-a'), false)
    assert.equal(isNamespaceName('a'.repeat(64)), false)
  })

  it('drops blanks and duplicates like the server', () => {
    assert.deepEqual(normalizeAllowedNamespaces([' team-a', '', 'team-b', 'team-a']), { names: ['team-a', 'team-b'] })
  })

  it('names the first invalid entry', () => {
    assert.deepEqual(normalizeAllowedNamespaces(['team-a', 'Bad', 'x_y']), { names: ['team-a'], invalid: 'Bad' })
  })
})
