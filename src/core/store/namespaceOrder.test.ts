import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { orderNamespaces, toggled } from './namespaceOrder.ts'

describe('orderNamespaces', () => {
  const names = ['shop', 'default', 'kube-system', 'monitoring', 'cert-manager']

  it('puts pinned namespaces first, in pin order, then the rest alphabetically', () => {
    assert.deepEqual(orderNamespaces(names, { pinned: ['shop', 'default'], hidden: [] }), {
      pinned: ['shop', 'default'],
      others: ['cert-manager', 'kube-system', 'monitoring'],
    })
  })

  it('leaves hidden namespaces out unless asked or selected', () => {
    const prefs = { pinned: [], hidden: ['kube-system', 'cert-manager'] }
    assert.deepEqual(orderNamespaces(names, prefs).others, ['default', 'monitoring', 'shop'])
    assert.deepEqual(orderNamespaces(names, prefs, { keep: 'kube-system' }).others, ['default', 'kube-system', 'monitoring', 'shop'])
    assert.equal(orderNamespaces(names, prefs, { includeHidden: true }).others.length, 5)
  })

  it('ignores preferences for namespaces the cluster does not have', () => {
    assert.deepEqual(orderNamespaces(['a', 'b'], { pinned: ['gone', 'b'], hidden: ['old'] }), { pinned: ['b'], others: ['a'] })
  })
})

describe('toggled', () => {
  it('adds or removes', () => {
    assert.deepEqual(toggled(['a'], 'b'), ['a', 'b'])
    assert.deepEqual(toggled(['a', 'b'], 'a'), ['b'])
  })
})
