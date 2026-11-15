import en from '../locales/en.json'
import type { Category, CheckTarget, SecurityCheck, Severity } from './types'

/**
 * The check catalog of the mock provider: the server's ids, severities,
 * categories, frameworks and targets (internal/plugins/security/catalog.go),
 * with the English texts taken from the plugin's own locale file.
 */

type Spec = [id: string, severity: Severity, category: Category, target: CheckTarget, frameworks: string[]]

const SPECS: Spec[] = [
  ['privileged', 'critical', 'pss-baseline', 'workload', ['CIS 5.2.2', 'PSS baseline']],
  ['host-namespaces', 'high', 'pss-baseline', 'workload', ['CIS 5.2.3', 'CIS 5.2.4', 'CIS 5.2.5', 'PSS baseline']],
  ['host-path', 'high', 'pss-baseline', 'workload', ['CIS 5.2.12', 'PSS baseline']],
  ['host-ports', 'medium', 'pss-baseline', 'workload', ['CIS 5.2.13', 'PSS baseline']],
  ['capabilities', 'high', 'pss-baseline', 'workload', ['CIS 5.2.8', 'CIS 5.2.9', 'PSS baseline']],
  ['proc-mount', 'medium', 'pss-baseline', 'workload', ['PSS baseline']],
  ['unsafe-sysctls', 'medium', 'pss-baseline', 'workload', ['PSS baseline']],
  ['selinux-apparmor', 'medium', 'pss-baseline', 'workload', ['PSS baseline']],
  ['privilege-escalation', 'medium', 'pss-restricted', 'workload', ['CIS 5.2.6', 'PSS restricted']],
  ['run-as-non-root', 'medium', 'pss-restricted', 'workload', ['CIS 5.2.7', 'PSS restricted']],
  ['seccomp', 'medium', 'pss-restricted', 'workload', ['CIS 5.7.2', 'PSS restricted']],
  ['drop-capabilities', 'low', 'pss-restricted', 'workload', ['CIS 5.2.10', 'PSS restricted']],
  ['volume-types', 'low', 'pss-restricted', 'workload', ['PSS restricted']],
  ['resource-requests', 'low', 'hygiene', 'workload', ['NSA hardening']],
  ['resource-limits', 'medium', 'hygiene', 'workload', ['NSA hardening']],
  ['probes', 'low', 'hygiene', 'workload', ['Best practice']],
  ['image-tag', 'medium', 'hygiene', 'workload', ['Best practice']],
  ['read-only-root', 'low', 'hygiene', 'workload', ['CIS 5.7.3', 'NSA hardening']],
  ['cluster-admin-binding', 'critical', 'rbac', 'binding', ['CIS 5.1.1']],
  ['rbac-wildcards', 'high', 'rbac', 'role', ['CIS 5.1.3']],
  ['secrets-access', 'high', 'rbac', 'role', ['CIS 5.1.2']],
  ['pod-create', 'medium', 'rbac', 'role', ['CIS 5.1.4']],
  ['rbac-escalation', 'high', 'rbac', 'role', ['CIS 5.1.8']],
  ['default-service-account', 'low', 'rbac', 'serviceaccount', ['CIS 5.1.5']],
  ['service-account-token', 'medium', 'rbac', 'workload', ['CIS 5.1.5', 'CIS 5.1.6']],
  ['network-policy', 'medium', 'network', 'namespace', ['CIS 5.3.2']],
  ['secret-env', 'medium', 'secrets', 'workload', ['CIS 5.4.1']],
  ['hardcoded-secret', 'high', 'secrets', 'workload', ['CIS 5.4.1']],
  ['psa-labels', 'low', 'namespaces', 'namespace', ['CIS 5.2.1', 'PSS baseline']],
  ['default-namespace', 'low', 'namespaces', 'workload', ['CIS 5.7.4']],
]

type CheckTexts = Record<string, { title: string; message: string; remediation: string }>
const TEXTS = en.checks as CheckTexts

export const MOCK_CATALOG: SecurityCheck[] = SPECS.map(([id, severity, category, target, frameworks]) => ({
  id,
  title: TEXTS[id].title,
  severity,
  category,
  target,
  frameworks,
  remediation: TEXTS[id].remediation,
}))

/** A finding's English message, as the server words it. */
export function mockMessage(checkId: string, evidence: string[]): string {
  return TEXTS[checkId].message.replace('{{evidence}}', evidence.join(', '))
}
