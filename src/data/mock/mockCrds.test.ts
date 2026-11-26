import assert from 'node:assert/strict'
import { test } from 'node:test'
import { crds, customResources, scaleMockCustomResource } from './mockCrds.ts'

const checkout = () => customResources.find((r) => r.name === 'shop-checkout')!

test('the mock has a scalable CRD whose objects carry their replicas', () => {
  const rollouts = crds.find((c) => c.name === 'rollouts.argoproj.io')!
  assert.deepEqual(rollouts.scale?.specReplicasPath, '.spec.replicas')
  assert.equal(checkout().replicas, 3)
  // Kinds without a scale subresource list none, and their objects no replicas.
  assert.equal(crds.find((c) => c.kind === 'Certificate')!.scale, undefined)
  assert.equal(customResources.find((r) => r.name === 'shop-tls')!.replicas, undefined)
})

test('scaling a custom resource sets its replicas and printer columns', () => {
  assert.equal(scaleMockCustomResource('argoproj.io/v1alpha1', 'Rollout', 'shop', 'shop-checkout', 5), 'scaled')
  assert.equal(checkout().replicas, 5)
  assert.deepEqual(checkout().extra, { Desired: '5', Current: '5' })
  assert.equal(scaleMockCustomResource('argoproj.io/v1alpha1', 'Rollout', 'shop', 'shop-checkout', 3), 'scaled')
})

test('scaling fails like the backend for unserved kinds, kinds without scale and missing objects', () => {
  assert.equal(scaleMockCustomResource('argoproj.io/v1', 'Rollout', 'shop', 'shop-checkout', 1), 'not_served')
  assert.equal(scaleMockCustomResource('cert-manager.io/v1', 'Certificate', 'shop', 'shop-tls', 1), 'no_scale')
  assert.equal(scaleMockCustomResource('argoproj.io/v1alpha1', 'Rollout', 'default', 'shop-checkout', 1), 'not_found')
  assert.equal(checkout().replicas, 3)
})
