import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { deniedCode, deniedReasonKey, type PermissionFacts } from './permissionRules.ts'

const admin: PermissionFacts = { role: 'admin', canRead: true, canWrite: true, canExec: true, isAdmin: true, readOnly: false }
const viewer: PermissionFacts = { ...admin, role: 'viewer', canWrite: false, canExec: false, isAdmin: false }
// On a read-only cluster nobody may write or exec, admins included.
const readOnlyAdmin: PermissionFacts = { ...admin, canWrite: false, canExec: false, readOnly: true }

describe('deniedCode', () => {
  it('is null for a held permission', () => {
    assert.equal(deniedCode('exec', admin), null)
  })

  it('blames the cluster, not the role, on a read-only cluster', () => {
    assert.equal(deniedCode('exec', readOnlyAdmin), 'cluster_read_only')
    assert.equal(deniedCode('write', { ...viewer, readOnly: true }), 'cluster_read_only')
  })

  it('blames the role otherwise', () => {
    assert.equal(deniedCode('exec', viewer), 'forbidden_role')
    assert.equal(deniedCode('admin', viewer), 'admin_required')
  })

  it('agrees with the reason shown next to it', () => {
    assert.equal(deniedReasonKey('exec', readOnlyAdmin), 'permissions.clusterReadOnly')
    assert.equal(deniedReasonKey('exec', viewer), 'permissions.viewerRole')
  })
})

describe('browse', () => {
  const operator: PermissionFacts = { ...admin, role: 'operator', isAdmin: false }

  it('needs the operator or admin role', () => {
    assert.equal(deniedCode('browse', operator), null)
    assert.equal(deniedCode('browse', viewer), 'forbidden_role')
    assert.equal(deniedReasonKey('browse', viewer), 'permissions.browseViewer')
  })

  it('is held on a read-only cluster, unlike exec', () => {
    assert.equal(deniedCode('browse', readOnlyAdmin), null)
    assert.equal(deniedCode('browse', { ...operator, canWrite: false, canExec: false, readOnly: true }), null)
  })
})
