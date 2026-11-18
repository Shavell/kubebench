import { CATEGORIES, SEVERITIES, type Acceptance, type CategoryScore, type Finding, type SecurityCheck, type SeverityCounts } from './types'

/**
 * The server's scoring (internal/plugins/security/score.go) for the mock
 * provider: the severity-weighted share of (check, object) pairs that
 * pass, where accepted findings pass. Rounded down, null when nothing ran.
 */

const WEIGHT = { critical: 10, high: 6, medium: 3, low: 1 } as const

const percent = (passed: number, total: number) => (total === 0 ? null : Math.floor((passed * 100) / total))

/** Applies `accepted` (valid at `now`) to findings and computes counts and scores. */
export function scoreFindings(
  catalog: SecurityCheck[],
  evaluated: Record<string, number>,
  raw: Finding[],
  accepted: Map<string, Acceptance>,
  now: string,
): { findings: Finding[]; counts: SeverityCounts; overall: number | null; byCategory: CategoryScore[] } {
  const counts: SeverityCounts = { critical: 0, high: 0, medium: 0, low: 0, accepted: 0, total: 0 }
  const failed = new Map<string, number>()
  const byCategory = new Map<string, CategoryScore>(
    CATEGORIES.map((category) => [
      category,
      {
        category,
        score: null,
        checks: 0,
        evaluated: 0,
        failed: 0,
        accepted: 0,
        bySeverity: Object.fromEntries(SEVERITIES.map((s) => [s, 0])) as CategoryScore['bySeverity'],
      },
    ]),
  )
  const findings = raw.map((f) => {
    const a = accepted.get(f.key)
    const valid = a && (!a.expiresAt || now < a.expiresAt)
    const finding: Finding = valid ? { ...f, accepted: a } : { ...f, accepted: undefined }
    const cat = byCategory.get(f.category)!
    counts.total++
    if (finding.accepted) {
      counts.accepted++
      cat.accepted++
    } else {
      failed.set(f.checkId, (failed.get(f.checkId) ?? 0) + 1)
      cat.failed++
      cat.bySeverity[f.severity]++
      counts[f.severity]++
    }
    return finding
  })

  let total = 0
  let passed = 0
  const catTotals = new Map<string, { total: number; passed: number }>()
  for (const check of catalog) {
    const n = evaluated[check.id]
    if (n === undefined) continue
    const cat = byCategory.get(check.category)!
    cat.checks++
    cat.evaluated += n
    const w = WEIGHT[check.severity]
    const ok = n - (failed.get(check.id) ?? 0)
    total += w * n
    passed += w * ok
    const t = catTotals.get(check.category) ?? { total: 0, passed: 0 }
    catTotals.set(check.category, { total: t.total + w * n, passed: t.passed + w * ok })
  }
  for (const [category, t] of catTotals) byCategory.get(category)!.score = percent(t.passed, t.total)
  // Like the server, OpenShift's category only where its checks ran.
  const categories = [...byCategory.values()].filter((c) => c.category !== 'openshift' || c.checks > 0)
  return { findings, counts, overall: percent(passed, total), byCategory: categories }
}
