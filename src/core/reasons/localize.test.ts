import assert from 'node:assert/strict'
import { test } from 'node:test'
import { localizeReason, reasonParamText, type ReasonTranslator } from './localize.ts'

const strings: Record<string, string> = {
  'cluster.read_only': '"{{cluster}}" cluster\'ı salt okunur',
  'field.range': '{{field}} {{min}} ile {{max}} arasında olmalı',
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
  assert.deepEqual(reasonParamText({ a: 'x', b: 3, c: true, d: null }), { a: 'x', b: '3', c: 'true', d: 'null' })
  assert.deepEqual(reasonParamText(undefined), {})
})
