import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import type { CrdDefinition, CustomResource } from '../../data/types.ts'
import { printerColumns, printerSortValue } from './printerColumns.ts'

const crd: CrdDefinition = {
  name: 'widgets.compat.example.com',
  group: 'compat.example.com',
  kind: 'Widget',
  versions: ['v1'],
  scope: 'Namespaced',
  shortNames: [],
  age: '1d',
  printerColumns: [
    { name: 'Size', type: 'integer', priority: 0 },
    { name: 'Ready', type: 'string', priority: 0 },
    { name: 'FirstPort', type: 'integer', priority: 0 },
    { name: 'Owner', type: 'string', priority: 0 },
    { name: 'Node', type: 'string', priority: 1 },
  ],
}

const row = (extra: Record<string, string>): CustomResource => ({ name: 'w', crdName: crd.name, status: 'Active', age: '1d', extra })

describe('printerColumns', () => {
  it('shows every priority-0 column in the CRD order, more than three', () => {
    const { shown, hasWide } = printerColumns(crd, [row({ Owner: 'alice', Size: '3' })], false)
    assert.deepEqual(shown.map((c) => c.name), ['Size', 'Ready', 'FirstPort', 'Owner'])
    assert.equal(hasWide, true)
  })

  it('adds the higher priorities when wide', () => {
    assert.deepEqual(printerColumns(crd, [], true).shown.map((c) => c.name), ['Size', 'Ready', 'FirstPort', 'Owner', 'Node'])
  })

  it('falls back to the row keys before the CRD is known', () => {
    const { shown, hasWide } = printerColumns(undefined, [row({ A: '1' }), row({ A: '2', B: 'x' })], false)
    assert.deepEqual(shown.map((c) => c.name), ['A', 'B'])
    assert.equal(hasWide, false)
  })
})

describe('printerSortValue', () => {
  it('sorts numbers numerically, missing first', () => {
    const key = printerSortValue({ name: 'Size', type: 'integer', priority: 0 })!
    assert.equal(key('10'), 10)
    assert.equal(key(undefined), Number.NEGATIVE_INFINITY)
  })

  it('does not sort dates and sorts the rest as text', () => {
    assert.equal(printerSortValue({ name: 'Started', type: 'date', priority: 0 }), undefined)
    assert.equal(printerSortValue({ name: 'Owner', type: 'string', priority: 0 })!(undefined), '')
  })
})
