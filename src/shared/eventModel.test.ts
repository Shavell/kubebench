import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import type { KubeObject } from '../data/types.ts'
import { ageToMinutes, formatAge, groupByReason, recentWarnings, toEventRow } from './eventModel.ts'

const NOW = Date.parse('2026-11-15T12:00:00Z')

const event = (name: string, fields: Record<string, unknown>): KubeObject => ({
  apiVersion: 'v1',
  kind: 'Event',
  metadata: { name, namespace: 'shop' },
  ...fields,
})

describe('toEventRow', () => {
  it('reads core/v1 events', () => {
    const row = toEventRow(
      event('web.1', {
        type: 'Warning',
        reason: 'BackOff',
        message: 'Back-off restarting failed container',
        count: 7,
        involvedObject: { kind: 'Pod', name: 'web-1' },
        lastTimestamp: '2026-11-15T11:55:00Z',
      }),
      NOW,
    )
    assert.equal(row.key, 'shop/web.1')
    assert.equal(row.object, 'Pod/web-1')
    assert.equal(row.count, 7)
    assert.equal(row.lastSeenMinutes, 5)
  })

  it('falls back to the series, eventTime and creation time', () => {
    const series = toEventRow(event('a', { series: { count: 3, lastObservedTime: '2026-11-15T11:59:00Z' }, eventTime: '2026-11-15T10:00:00Z' }), NOW)
    assert.equal(series.count, 3)
    assert.equal(series.lastSeenMinutes, 1)
    assert.equal(toEventRow(event('b', { eventTime: '2026-11-15T11:00:00Z' }), NOW).lastSeenMinutes, 60)
    const bare = toEventRow({ metadata: { name: 'c' } }, NOW)
    assert.equal(bare.type, 'Normal')
    assert.equal(bare.count, 1)
    assert.equal(bare.lastSeenMinutes, Infinity)
  })

  it('uses the synthetic age of mock events', () => {
    assert.equal(toEventRow(event('d', { age: '2h' }), NOW).lastSeenMinutes, 120)
  })
})

describe('grouping and recent warnings', () => {
  const rows = [
    toEventRow(event('1', { type: 'Normal', reason: 'Pulled', count: 20, age: '1m' }), NOW),
    toEventRow(event('2', { type: 'Warning', reason: 'BackOff', count: 3, age: '5m', involvedObject: { kind: 'Pod', name: 'a' } }), NOW),
    toEventRow(event('3', { type: 'Warning', reason: 'BackOff', count: 2, age: '2m', involvedObject: { kind: 'Pod', name: 'b' } }), NOW),
    toEventRow(event('4', { type: 'Warning', reason: 'FailedMount', count: 9, age: '1d' }), NOW),
  ]

  it('groups by reason, warnings first, then by occurrences', () => {
    const groups = groupByReason(rows)
    assert.deepEqual(groups.map((g) => g.reason), ['FailedMount', 'BackOff', 'Pulled'])
    const backOff = groups[1]
    assert.equal(backOff.occurrences, 5)
    assert.equal(backOff.lastSeenMinutes, 2)
    assert.deepEqual(backOff.events.map((e) => e.name), ['3', '2'])
  })

  it('lists the most recent warnings', () => {
    assert.deepEqual(recentWarnings(rows, 2).map((e) => e.name), ['3', '2'])
  })
})

describe('ages', () => {
  it('parses and formats', () => {
    assert.equal(ageToMinutes('1h30m'), 90)
    assert.equal(ageToMinutes('—'), Infinity)
    assert.equal(formatAge(0.5), '30s')
    assert.equal(formatAge(90), '1h')
    assert.equal(formatAge(3 * 1440), '3d')
    assert.equal(formatAge(Infinity), '—')
  })
})
