import { useState } from 'react'
import { refreshData, useProviders } from '@/data/DataContext'
import type { ApplyAction, ApplyOptions, ApplyResult, ObjectRef } from '@/data/types'
import { describeError, errorCodeOf } from '@/shared/apiErrorText'
import { te } from './i18n'

/** The outcome of the last dry run or apply, kept in the tab's session. */
export interface ApplyOutcome {
  dryRun: boolean
  force: boolean
  /** One per object; empty when the request was refused as a whole. */
  results: ApplyResult[]
  /** Why the whole request was refused (invalid YAML, redacted Secret values, permissions, …). */
  refusal?: { message: string; detail?: string; code: string | null }
}

export const outcomeFailed = (o: ApplyOutcome) => Boolean(o.refusal) || o.results.some((r) => r.error)
export const outcomeHasConflict = (o: ApplyOutcome) => o.results.some((r) => r.code === 'conflict')

type Tally = Partial<Record<ApplyAction | 'failed', number>>

export function tally(results: ApplyResult[]): Tally {
  const counts: Tally = {}
  for (const r of results) {
    const key = r.error ? 'failed' : (r.action ?? 'unchanged')
    counts[key] = (counts[key] ?? 0) + 1
  }
  return counts
}

const TALLY_ORDER = ['created', 'configured', 'unchanged', 'failed'] as const

/** "1 created · 2 unchanged" (dry runs: "1 would be created · …"), translated. */
export function summarize(results: ApplyResult[], dryRun: boolean): string {
  const counts = tally(results)
  return TALLY_ORDER.filter((key) => counts[key])
    .map((key) => te(`apply.${dryRun ? 'dryCount' : 'count'}.${key}`, { count: counts[key] }))
    .join(' · ')
}

/**
 * Runs dry runs and applies of manifests (`run`) and of edits of one
 * object (`runEdit`). One that changed anything refreshes every view; a
 * refused request becomes the outcome's refusal rather than an exception.
 */
export function useApplyRunner() {
  const { cluster } = useProviders()
  const [busy, setBusy] = useState<'dryRun' | 'apply' | null>(null)

  const execute = async (dryRun: boolean, force: boolean, send: () => Promise<ApplyResult[]>): Promise<ApplyOutcome> => {
    setBusy(dryRun ? 'dryRun' : 'apply')
    try {
      const results = await send()
      if (!dryRun && results.some((r) => r.action && r.action !== 'unchanged')) refreshData()
      return { dryRun, force, results }
    } catch (err) {
      return { dryRun, force, results: [], refusal: { ...describeError(err), code: errorCodeOf(err) } }
    } finally {
      setBusy(null)
    }
  }

  const run = (manifests: string, options: ApplyOptions) =>
    execute(Boolean(options.dryRun), Boolean(options.force), () => cluster.applyYaml(manifests, options))

  const runEdit = (ref: ObjectRef, original: string, edited: string, dryRun: boolean) =>
    execute(dryRun, false, () => cluster.editYaml(ref, original, edited, { dryRun }))

  return { busy, run, runEdit }
}
