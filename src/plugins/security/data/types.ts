/**
 * Shapes of the security plugin's API (design §7.7); they match
 * backend/internal/plugins/security/types.go.
 */

export const SEVERITIES = ['critical', 'high', 'medium', 'low'] as const
export type Severity = (typeof SEVERITIES)[number]

export const CATEGORIES = [
  'pss-baseline',
  'pss-restricted',
  'hygiene',
  'rbac',
  'network',
  'secrets',
  'namespaces',
] as const
export type Category = (typeof CATEGORIES)[number]

/** The kind of object a check evaluates. */
export type CheckTarget = 'workload' | 'namespace' | 'serviceaccount' | 'role' | 'binding'

export interface SecurityCheck {
  id: string
  /** English title; the UI shows `checks.<id>.title` of the plugin namespace. */
  title: string
  severity: Severity
  category: Category
  frameworks: string[]
  target: CheckTarget
  remediation: string
}

export interface SecurityResource {
  apiVersion: string
  kind: string
  namespace?: string
  name: string
}

export interface Acceptance {
  by: string
  reason: string
  /** RFC 3339. */
  at: string
  expiresAt?: string
}

export interface Finding {
  /** "<checkId>:<Kind>:<namespace>:<name>", stable across scans. */
  key: string
  checkId: string
  title: string
  severity: Severity
  category: Category
  frameworks: string[]
  resource: SecurityResource
  /** English text; the UI translates `checks.<id>.message` with `{{evidence}}`. */
  message: string
  evidence?: string[]
  remediation: string
  accepted?: Acceptance
}

export interface CategoryScore {
  category: Category
  /** 0–100; null when none of the category's checks evaluated anything. */
  score: number | null
  checks: number
  evaluated: number
  failed: number
  accepted: number
  bySeverity: Record<Severity, number>
}

export interface SeverityCounts {
  critical: number
  high: number
  medium: number
  low: number
  accepted: number
  total: number
}

export interface ScanOptions {
  includeSystem: boolean
}

export interface SecurityReport {
  generatedAt: string
  cluster: string
  options: ScanOptions
  score: { overall: number | null; byCategory: CategoryScore[] }
  counts: SeverityCounts
  findings: Finding[]
  /** Checks that could not run; reasonCode and reasonParams translate reason (design §3.3). */
  skipped: { checkId: string; reason: string; reasonCode?: string; reasonParams?: Record<string, unknown> }[]
  warnings: string[]
  /** The reason codes of warnings, index by index (absent from older servers). */
  warningReasons?: { reason: string; params?: Record<string, unknown> }[]
  inventory: Record<string, number>
  systemNamespaces: string[]
}

/** An entry of GET /accepted: an accepted risk, whether or not a report lists it. */
export interface AcceptedRisk extends Acceptance {
  key: string
  checkId: string
  resource: SecurityResource
  expired: boolean
}

export type ExportFormat = 'md' | 'csv'
