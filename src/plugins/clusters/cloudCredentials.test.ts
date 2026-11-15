import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import type { TFunction } from 'i18next'
import { credentialProblems, emptyDraft, modesFor, toCredentials } from './cloudCredentials.ts'

const t = ((key: string) => key) as unknown as TFunction

describe('credentialProblems', () => {
  it('requires the fields of the chosen mode only', () => {
    assert.deepEqual(credentialProblems(t, emptyDraft('server')), {})
    assert.deepEqual(credentialProblems(t, emptyDraft('keep')), {})
    assert.deepEqual(Object.keys(credentialProblems(t, emptyDraft('aws-keys'))), ['accessKeyId', 'secretAccessKey'])
    assert.deepEqual(Object.keys(credentialProblems(t, emptyDraft('azure'))), ['tenantId', 'clientId', 'clientSecret'])
  })

  it('checks profile names, GUIDs and service-account keys', () => {
    assert.ok(credentialProblems(t, { ...emptyDraft('aws-profile'), profile: '../x' }).profile)
    assert.deepEqual(credentialProblems(t, { ...emptyDraft('aws-profile'), profile: 'prod-admin' }), {})
    const azure = { ...emptyDraft('azure'), tenantId: 'contoso', clientId: '66666666-7777-8888-9999-AAAAAAAAAAAA', clientSecret: 's' }
    assert.deepEqual(Object.keys(credentialProblems(t, azure)), ['tenantId'])
    const key = JSON.stringify({ type: 'service_account', client_email: 'a@b', private_key: 'k' })
    assert.deepEqual(credentialProblems(t, { ...emptyDraft('gcp'), serviceAccountJson: key }), {})
    const other = JSON.stringify({ type: 'external_account' })
    assert.ok(credentialProblems(t, { ...emptyDraft('gcp'), serviceAccountJson: other }).serviceAccountJson)
    assert.ok(credentialProblems(t, { ...emptyDraft('gcp'), serviceAccountJson: '{' }).serviceAccountJson)
  })
})

describe('toCredentials', () => {
  it('maps modes to the request value', () => {
    assert.equal(toCredentials(emptyDraft('keep')), undefined)
    assert.equal(toCredentials(emptyDraft('server')), null)
    assert.deepEqual(toCredentials({ ...emptyDraft('aws-keys'), accessKeyId: ' AKIA ', secretAccessKey: 's' }), {
      provider: 'aws',
      accessKeyId: 'AKIA',
      secretAccessKey: 's',
    })
    assert.deepEqual(
      toCredentials({ ...emptyDraft('aws-keys'), accessKeyId: 'A', secretAccessKey: 's', sessionToken: 'tok' }),
      { provider: 'aws', accessKeyId: 'A', secretAccessKey: 's', sessionToken: 'tok' },
    )
    assert.deepEqual(toCredentials({ ...emptyDraft('aws-profile'), profile: 'p' }), { provider: 'aws', profile: 'p' })
  })
})

describe('modesFor', () => {
  it('offers the modes of the kubeconfig cloud', () => {
    assert.deepEqual(modesFor('aws', ['keep', 'server']), ['keep', 'server', 'aws-keys', 'aws-profile'])
    assert.deepEqual(modesFor('gcp', ['server']), ['server', 'gcp'])
    assert.deepEqual(modesFor(undefined, ['server']), ['server', 'aws-keys', 'aws-profile', 'gcp', 'azure'])
  })
})
