import { refreshData } from '@/data/DataContext'
import { downloadText } from '@/shared/fileActions'
import { notify } from '@/shared/notify'
import { securityData } from './data'
import type { ExportFormat, SecurityReport } from './data/types'
import { translate } from './i18n'
import { useSecurityStore } from './store'

/**
 * Runs a scan with the chosen options (one at a time per browser; the
 * server shares a running scan between users) and refreshes every page.
 */
export async function runScan(): Promise<void> {
  const store = useSecurityStore.getState()
  if (store.scanning) return
  store.setScanning(true)
  try {
    const report = await securityData().scan({ includeSystem: store.includeSystem })
    notify.success(
      report.score.overall === null
        ? translate('scan.doneNoScore', { count: report.counts.total - report.counts.accepted })
        : translate('scan.done', { score: report.score.overall, count: report.counts.total - report.counts.accepted }),
    )
    refreshData()
  } catch (err) {
    notify.error(err)
  } finally {
    useSecurityStore.getState().setScanning(false)
  }
}

const EXPORT_TYPES: Record<ExportFormat, string> = {
  md: 'text/markdown',
  csv: 'text/csv',
}

/** Saves the last report as Markdown or CSV, named like the server names it. */
export async function downloadReport(report: SecurityReport, format: ExportFormat): Promise<void> {
  try {
    const text = await securityData().exportReport(format)
    const stamp = report.generatedAt.replace(/[-:]/g, '').replace('T', '-').slice(0, 15)
    const cluster = report.cluster.replace(/[^A-Za-z0-9._-]+/g, '_')
    downloadText(`security-${cluster}-${stamp}.${format}`, text, EXPORT_TYPES[format])
  } catch (err) {
    notify.error(err)
  }
}
