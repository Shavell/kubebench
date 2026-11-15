import type { Acceptance, ExportFormat, SecurityReport, SecurityResource } from './types'

/**
 * The server's report exports (internal/plugins/security/export.go) for
 * the mock provider, in the same layout and columns.
 */

const resourceText = (r: SecurityResource) => `${r.kind} ${r.namespace ? `${r.namespace}/` : ''}${r.name}`
const scoreText = (s: number | null) => (s === null ? 'n/a' : `${s}/100`)
const mdCell = (s: string) => s.replace(/\\/g, '\\\\').replace(/\|/g, '\\|').replace(/[\r\n]/g, ' ')

function acceptedText(a?: Acceptance): string {
  if (!a) return 'open'
  const until = a.expiresAt ? ` until ${a.expiresAt.slice(0, 10)}` : ''
  return `accepted by ${a.by}${until}: ${a.reason}`
}

function markdown(rep: SecurityReport): string {
  const lines: string[] = [`# Security posture: ${rep.cluster}`, '']
  lines.push(
    `Generated ${rep.generatedAt}, system namespaces ${
      rep.options.includeSystem ? 'included' : `excluded (${rep.systemNamespaces.join(', ')})`
    }.`,
    '',
    `**Score: ${scoreText(rep.score.overall)}**`,
    '',
    '| Category | Score | Open findings | Accepted |',
    '|---|---|---|---|',
  )
  for (const c of rep.score.byCategory) lines.push(`| ${c.category} | ${scoreText(c.score)} | ${c.failed} | ${c.accepted} |`)
  const { critical, high, medium, low, accepted } = rep.counts
  lines.push('', `Open findings: ${critical} critical, ${high} high, ${medium} medium, ${low} low; ${accepted} accepted.`, '')
  lines.push('## Findings', '')
  if (rep.findings.length === 0) {
    lines.push('No findings.', '')
  } else {
    lines.push('| Severity | Check | Resource | Message | Frameworks | Status |', '|---|---|---|---|---|---|')
    for (const f of rep.findings) {
      const cells = [f.title, resourceText(f.resource), f.message, f.frameworks.join(', '), acceptedText(f.accepted)]
      lines.push(`| ${f.severity} | ${cells.map(mdCell).join(' | ')} |`)
    }
    lines.push('', '## Remediation', '')
    const seen = new Set<string>()
    for (const f of rep.findings) {
      if (seen.has(f.checkId)) continue
      seen.add(f.checkId)
      lines.push(`- **${f.title}** (${f.checkId}): ${f.remediation}`)
    }
    lines.push('')
  }
  if (rep.skipped.length) lines.push('## Skipped checks', '', ...rep.skipped.map((s) => `- ${s.checkId}: ${s.reason}`), '')
  if (rep.warnings.length) lines.push('## Warnings', '', ...rep.warnings.map((w) => `- ${w}`), '')
  return lines.join('\n')
}

/** Defuses cells a spreadsheet would run as formulas, like the server. */
const defuse = (s: string) => (/^[=+\-@\t\r]/.test(s) ? `'${s}` : s)
const csvField = (s: string) => (/[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s)

function csv(rep: SecurityReport): string {
  const rows: string[][] = [
    ['severity', 'check_id', 'title', 'category', 'frameworks', 'kind', 'namespace', 'name',
      'message', 'remediation', 'status', 'accepted_by', 'accepted_reason', 'accepted_until'],
  ]
  for (const f of rep.findings) {
    const a = f.accepted
    rows.push(
      [
        f.severity, f.checkId, f.title, f.category, f.frameworks.join('; '),
        f.resource.kind, f.resource.namespace ?? '', f.resource.name, f.message, f.remediation,
        a ? 'accepted' : 'open', a?.by ?? '', a?.reason ?? '', a?.expiresAt ?? '',
      ].map(defuse),
    )
  }
  return rows.map((row) => row.map(csvField).join(',')).join('\n') + '\n'
}

export function exportText(rep: SecurityReport, format: ExportFormat): string {
  return format === 'md' ? markdown(rep) : csv(rep)
}
