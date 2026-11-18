import assert from 'node:assert/strict'
import { test } from 'node:test'
import { credentialFields, credentialPlaceholder } from './credentials.ts'

test('a Route and the build kinds hold credentials, other kinds none', () => {
  assert.ok(credentialFields('route.openshift.io/v1', 'Route'))
  assert.ok(credentialFields('build.openshift.io/v1', 'BuildConfig'))
  assert.ok(credentialFields('build.openshift.io/v1', 'Build'))
  assert.equal(credentialFields('example.com/v1', 'Route'), undefined)
  assert.equal(credentialFields('v1', 'ConfigMap'), undefined)
})

test('the first credential field holding a placeholder', () => {
  const route = credentialFields('route.openshift.io/v1', 'Route')!
  assert.equal(credentialPlaceholder(route, { spec: { tls: { key: '<1704 bytes, redacted>' } } }), 'spec.tls.key')
  assert.equal(credentialPlaceholder(route, { spec: { tls: { key: '-----BEGIN PRIVATE KEY-----' } } }), null)
  const bc = credentialFields('build.openshift.io/v1', 'BuildConfig')!
  const triggers = [{ type: 'ConfigChange' }, { type: 'Generic', generic: { secret: '<8 bytes, redacted>' } }]
  assert.equal(credentialPlaceholder(bc, { spec: { triggers } }), 'spec.triggers[1].generic.secret')
  assert.equal(credentialPlaceholder(bc, { metadata: { name: 'api' } }), null)
})
