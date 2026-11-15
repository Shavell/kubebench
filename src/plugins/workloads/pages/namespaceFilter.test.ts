import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { initialLocalNamespace, listNamespace, LOCAL_ALL_NAMESPACES } from './namespaceFilter.ts'

describe('namespace filter', () => {
  it('presets the local filter from the tab props', () => {
    assert.equal(initialLocalNamespace(undefined), '')
    assert.equal(initialLocalNamespace({ namespace: 'shop' }), 'shop')
    assert.equal(initialLocalNamespace({ allNamespaces: true }), LOCAL_ALL_NAMESPACES)
  })

  it('lists the local namespace, all of them, or follows the global filter', () => {
    assert.equal(listNamespace('', 'shop'), 'shop')
    assert.equal(listNamespace('', ''), undefined)
    assert.equal(listNamespace('default', 'shop'), 'default')
    assert.equal(listNamespace(LOCAL_ALL_NAMESPACES, 'shop'), undefined)
  })
})
