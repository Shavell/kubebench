import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { isKubernetesForbidden } from './forbidden.ts'
import { sidebarMessages } from './sidebarMessages.ts'

describe('isKubernetesForbidden', () => {
  it('knows the reasons of a Kubernetes 403', () => {
    for (const code of ['kubernetes.forbidden', 'kubernetes.forbidden_namespace', 'kubernetes.forbidden_cluster']) {
      assert.equal(isKubernetesForbidden(code), true, code)
    }
    for (const code of [null, undefined, '', 'role.cannot_change', 'kubernetes.forbiddenx']) {
      assert.equal(isKubernetesForbidden(code), false, String(code))
    }
  })
})

describe('sidebarMessages', () => {
  it('shows each message once and sums up RBAC refusals as one notice', () => {
    const denied = (resource: string) => ({
      error: `no permission to list ${resource} at the cluster scope`,
      errorReason: 'kubernetes.forbidden_cluster',
    })
    assert.deepEqual(
      sidebarMessages([denied('pods'), denied('nodes'), 'boom', { error: 'boom', errorReason: null }, null, { error: null }]),
      { messages: ['boom'], forbidden: true },
    )
  })

  it('reports nothing without errors', () => {
    assert.deepEqual(sidebarMessages([null, undefined, { error: null }]), { messages: [], forbidden: false })
  })
})
