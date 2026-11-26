import assert from 'node:assert/strict'
import { test } from 'node:test'
import { isChangedOnServer, isFieldConflict } from './editConflict.ts'

const base = { apiVersion: 'v1', kind: 'ConfigMap', namespace: 'shop', name: 'cm1' }

test('a stale edit is told apart from a field manager conflict', () => {
  const stale = { ...base, error: 'modified', code: 'conflict', reason: 'edit.changed_on_server' }
  const owned = { ...base, error: 'conflict with "kubectl"', code: 'conflict' }
  assert.equal(isChangedOnServer(stale), true)
  assert.equal(isFieldConflict(stale), false)
  assert.equal(isChangedOnServer(owned), false)
  assert.equal(isFieldConflict(owned), true)
  assert.equal(isChangedOnServer({ ...base, action: 'configured' }), false)
  assert.equal(isFieldConflict({ ...base, error: 'boom', code: 'kubernetes' }), false)
})
