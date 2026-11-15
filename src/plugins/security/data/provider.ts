import type { Acceptance, AcceptedRisk, ExportFormat, ScanOptions, SecurityCheck, SecurityReport } from './types'

/**
 * The security plugin's data capability (design §7.7). The live
 * implementation calls /api/plugins/security; the mock one scans a fixed
 * demo cluster and keeps accepted risks in memory.
 */
export interface SecurityProvider {
  /** Runs the checks on the active cluster and returns the report. */
  scan(options: ScanOptions): Promise<SecurityReport>
  /** The cluster's last report with the current accepted risks; null before the first scan. */
  report(): Promise<SecurityReport | null>
  checks(): Promise<SecurityCheck[]>
  accepted(): Promise<AcceptedRisk[]>
  /** Accepts a finding's risk for everyone on the cluster, optionally until `expiresAt` (RFC 3339). */
  accept(key: string, reason: string, expiresAt?: string): Promise<Acceptance>
  unaccept(key: string): Promise<void>
  /** The last report as Markdown or CSV text. */
  exportReport(format: ExportFormat): Promise<string>
}
