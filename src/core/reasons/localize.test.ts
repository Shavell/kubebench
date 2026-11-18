import assert from 'node:assert/strict'
import { test } from 'node:test'
import { localizeReason, reasonParamText, type ReasonTranslator } from './localize.ts'

const strings: Record<string, string> = {
  'cluster.read_only': '"{{cluster}}" cluster\'ı salt okunur',
  'field.range': '{{field}} {{min}} ile {{max}} arasında olmalı',
  'streams.closed': 'Oturum kapatıldı: {{cause}}',
}

const translator: ReasonTranslator = {
  has: (code) => code in strings,
  translate: (code, params) => strings[code].replace(/\{\{(\w+)\}\}/g, (m, name: string) => params[name] ?? m),
}

test('a known code is translated with its parameters', () => {
  assert.equal(
    localizeReason(translator, 'cluster.read_only', { cluster: 'prod' }, 'cluster "prod" is read-only'),
    '"prod" cluster\'ı salt okunur',
  )
  assert.equal(
    localizeReason(translator, 'field.range', { field: 'tail', min: 1, max: 5000 }, 'x'),
    'tail 1 ile 5000 arasında olmalı',
  )
})

test('unknown and missing codes fall back to the English message', () => {
  assert.equal(localizeReason(translator, 'argocd.future_reason', {}, 'from a newer server'), 'from a newer server')
  assert.equal(localizeReason(translator, undefined, undefined, 'plain message'), 'plain message')
  assert.equal(localizeReason(translator, '', null, 'plain message'), 'plain message')
})

test('parameter values become text', () => {
  assert.deepEqual({ ...reasonParamText({ a: 'x', b: 3, c: true, d: null }) }, { a: 'x', b: '3', c: 'true', d: 'null' })
  assert.deepEqual({ ...reasonParamText(undefined) }, {})
})

test('a nested reason is translated in place, an unknown one shows its English text', () => {
  const cause = { error: 'cluster "prod" is read-only', reason: 'cluster.read_only', params: { cluster: 'prod' } }
  assert.equal(
    localizeReason(translator, 'streams.closed', { cause }, 'the session was closed: …'),
    'Oturum kapatıldı: "prod" cluster\'ı salt okunur',
  )
  assert.equal(
    localizeReason(translator, 'streams.closed', { cause: { error: 'pods is forbidden' } }, 'x'),
    'Oturum kapatıldı: pods is forbidden',
  )
  assert.deepEqual({ ...reasonParamText({ cause }) }, { cause: 'cluster "prod" is read-only' })
})

test('server-chosen parameter names cannot reach Object.prototype', () => {
  const params = JSON.parse('{"__proto__": {"polluted": "yes"}, "constructor": "x", "ok": "1"}') as Record<string, unknown>
  const out = reasonParamText(params)
  assert.equal(Object.getPrototypeOf(out), null)
  assert.deepEqual(Object.keys(out), ['ok'])
  assert.equal(({} as Record<string, unknown>).polluted, undefined)
})
