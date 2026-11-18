import { ApiError } from '@/data/apiClient'
import { reasonError } from '@/data/reasonError'
import { delay, delayReject } from '@/data/mock/delay'
import { MOCK_CATALOG, mockMessage } from './mockCatalog'
import { exportText } from './mockExport'
import { scoreFindings } from './mockScore'
import type { SecurityProvider } from './provider'
import type { Acceptance, AcceptedRisk, Finding, ScanOptions, SecurityReport, SecurityResource } from './types'

/**
 * Mock security posture of the demo cluster (its namespaces, workloads
 * and a few RBAC objects). The findings are fixed, so every scan of the
 * same options reports the same; accepted risks live in memory. Nothing
 * reads the clock: scans are stamped from a fixed base time.
 */

const MOCK_NOW = '2026-08-11T09:15:00Z'
const BASE_TIME = Date.parse(MOCK_NOW)
const SYSTEM_NAMESPACES = ['kube-system', 'kube-public', 'kube-node-lease']

type Seed = [checkId: string, resource: SecurityResource, evidence: string[]]

const deploy = (namespace: string, name: string): SecurityResource => ({ apiVersion: 'apps/v1', kind: 'Deployment', namespace, name })
const daemonSet = (namespace: string, name: string): SecurityResource => ({ apiVersion: 'apps/v1', kind: 'DaemonSet', namespace, name })
const statefulSet = (namespace: string, name: string): SecurityResource => ({ apiVersion: 'apps/v1', kind: 'StatefulSet', namespace, name })
const pod = (namespace: string, name: string): SecurityResource => ({ apiVersion: 'v1', kind: 'Pod', namespace, name })
const ns = (name: string): SecurityResource => ({ apiVersion: 'v1', kind: 'Namespace', name })
const rbac = (kind: string, name: string, namespace?: string): SecurityResource => ({
  apiVersion: 'rbac.authorization.k8s.io/v1',
  kind,
  namespace,
  name,
})

const SEEDS: Seed[] = [
  ['host-namespaces', daemonSet('monitoring', 'prometheus-node-exporter'), ['hostNetwork', 'hostPID']],
  ['host-path', daemonSet('monitoring', 'prometheus-node-exporter'), ['proc (/proc)', 'sys (/sys)', 'root (/)']],
  ['host-ports', daemonSet('monitoring', 'prometheus-node-exporter'), ['node-exporter: 9100']],
  ['privileged', deploy('shop', 'shop-payments'), ['payments']],
  ['capabilities', deploy('shop', 'shop-payments'), ['payments: NET_ADMIN']],
  ['hardcoded-secret', deploy('shop', 'shop-payments'), ['payments: DB_PASSWORD']],
  ['secret-env', deploy('shop', 'shop-payments'), ['payments: STRIPE_API_KEY', 'payments: envFrom secret payments-env']],
  ['resource-limits', deploy('shop', 'shop-payments'), ['payments (cpu, memory)']],
  ['image-tag', deploy('shop', 'shop-frontend'), ['shop-frontend: ghcr.io/acme/shop-frontend:latest']],
  ['probes', deploy('shop', 'shop-frontend'), ['shop-frontend (liveness)']],
  ['privilege-escalation', deploy('shop', 'shop-frontend'), ['shop-frontend']],
  ['run-as-non-root', deploy('shop', 'shop-frontend'), ['shop-frontend']],
  ['seccomp', deploy('shop', 'shop-frontend'), ['shop-frontend (not set)']],
  ['drop-capabilities', deploy('shop', 'shop-frontend'), ['shop-frontend']],
  ['read-only-root', deploy('shop', 'shop-frontend'), ['shop-frontend']],
  ['service-account-token', deploy('shop', 'shop-frontend'), ['serviceAccountName: default', 'automountServiceAccountToken: true']],
  ['run-as-non-root', statefulSet('shop', 'shop-postgres'), ['postgres (runAsUser 0)']],
  ['privilege-escalation', statefulSet('shop', 'shop-postgres'), ['postgres']],
  ['read-only-root', statefulSet('shop', 'shop-postgres'), ['postgres']],
  ['seccomp', deploy('monitoring', 'prometheus-server'), ['prometheus (not set)', 'config-reloader (not set)']],
  ['resource-requests', deploy('monitoring', 'metrics-server'), ['metrics-server (memory)']],
  ['read-only-root', deploy('cert-manager', 'cert-manager'), ['cert-manager']],
  ['default-namespace', pod('default', 'netshoot'), ['namespace: default']],
  ['image-tag', pod('default', 'netshoot'), ['netshoot: nicolaka/netshoot']],
  ['cluster-admin-binding', rbac('ClusterRoleBinding', 'ci-deployer'), ['ServiceAccount kubebench/ci']],
  ['rbac-wildcards', rbac('ClusterRole', 'monitoring-reader'), ['get,list,watch *']],
  ['secrets-access', rbac('ClusterRole', 'cert-manager-controller-certificates'), ['get,list,watch secrets']],
  ['pod-create', rbac('Role', 'shop-ops', 'shop'), ['create pods/exec']],
  ['rbac-escalation', rbac('ClusterRole', 'namespace-admin-delegate'), ['bind: bind rbac.authorization.k8s.io/clusterroles']],
  ['default-service-account', { apiVersion: 'v1', kind: 'ServiceAccount', namespace: 'shop', name: 'default' }, ['automountServiceAccountToken: true']],
  ['network-policy', ns('default'), []],
  ['network-policy', ns('monitoring'), []],
  ['network-policy', ns('cert-manager'), []],
  ['psa-labels', ns('default'), ['pod-security.kubernetes.io/enforce: unset']],
  ['psa-labels', ns('monitoring'), ['pod-security.kubernetes.io/enforce: privileged']],
]

/** Findings of the system namespaces, reported only when a scan includes them. */
const SYSTEM_SEEDS: Seed[] = [
  ['privileged', daemonSet('kube-system', 'kube-proxy'), ['kube-proxy']],
  ['host-namespaces', daemonSet('kube-system', 'kube-proxy'), ['hostNetwork']],
  ['host-path', daemonSet('kube-system', 'kube-proxy'), ['xtables-lock (/run/xtables.lock)', 'lib-modules (/lib/modules)']],
  ['host-namespaces', pod('kube-system', 'etcd-kb-control-plane'), ['hostNetwork']],
  ['host-path', pod('kube-system', 'etcd-kb-control-plane'), ['etcd-data (/var/lib/etcd)', 'etcd-certs (/etc/kubernetes/pki/etcd)']],
  ['cluster-admin-binding', rbac('ClusterRoleBinding', 'cluster-admin'), ['Group system:masters']],
  ['rbac-wildcards', rbac('ClusterRole', 'cluster-admin'), ['* */*', '* *']],
  ['network-policy', ns('kube-system'), []],
  ['psa-labels', ns('kube-system'), ['pod-security.kubernetes.io/enforce: unset']],
]

/** Objects each target covers: the demo cluster's workloads, namespaces, roles, bindings, default ServiceAccounts. */
const SUBJECTS = {
  normal: { workload: 11, namespace: 4, role: 9, binding: 7, serviceaccount: 4 },
  system: { workload: 17, namespace: 7, role: 61, binding: 48, serviceaccount: 7 },
}

const INVENTORY: Record<string, number> = {
  namespaces: 6, pods: 19, deployments: 6, statefulsets: 1, daemonsets: 2, replicasets: 6, jobs: 0, cronjobs: 0,
  serviceaccounts: 14, roles: 3, clusterroles: 68, rolebindings: 4, clusterrolebindings: 52, networkpolicies: 1,
}

function findingOf([checkId, resource, evidence]: Seed): Finding {
  const check = MOCK_CATALOG.find((c) => c.id === checkId)!
  return {
    key: `${checkId}:${resource.kind}:${resource.namespace ?? ''}:${resource.name}`,
    checkId,
    title: check.title,
    severity: check.severity,
    category: check.category,
    frameworks: check.frameworks,
    resource,
    message: mockMessage(checkId, evidence),
    evidence: evidence.length ? evidence : undefined,
    remediation: check.remediation,
  }
}

const SEVERITY_ORDER = ['critical', 'high', 'medium', 'low']

function compare(a: Finding, b: Finding): number {
  const d = SEVERITY_ORDER.indexOf(a.severity) - SEVERITY_ORDER.indexOf(b.severity)
  if (d !== 0) return d
  const ka = [a.checkId, a.resource.namespace ?? '', a.resource.kind, a.resource.name].join('\0')
  const kb = [b.checkId, b.resource.namespace ?? '', b.resource.kind, b.resource.name].join('\0')
  return ka < kb ? -1 : ka > kb ? 1 : 0
}

interface MockScan {
  generatedAt: string
  options: ScanOptions
  findings: Finding[]
  evaluated: Record<string, number>
}

function runScan(options: ScanOptions, generatedAt: string): MockScan {
  const seeds = options.includeSystem ? [...SEEDS, ...SYSTEM_SEEDS] : SEEDS
  const subjects = options.includeSystem ? SUBJECTS.system : SUBJECTS.normal
  const evaluated: Record<string, number> = {}
  // Every check ran on all objects of its target (the demo cluster has no Jobs, which probes skip).
  for (const check of MOCK_CATALOG) evaluated[check.id] = subjects[check.target]
  return { generatedAt, options, findings: seeds.map(findingOf).sort(compare), evaluated }
}

export function createMockSecurity(): SecurityProvider {
  let scans = 0
  // The demo cluster has been scanned once already, so its pages have data.
  let last: MockScan | null = runScan({ includeSystem: false }, MOCK_NOW)
  const accepted = new Map<string, Acceptance & { checkId: string; resource: SecurityResource }>([
    [
      'host-namespaces:DaemonSet:monitoring:prometheus-node-exporter',
      {
        checkId: 'host-namespaces',
        resource: daemonSet('monitoring', 'prometheus-node-exporter'),
        by: 'admin',
        reason: 'node-exporter reads host network and process metrics by design.',
        at: '2026-07-30T14:02:00Z',
        expiresAt: '2027-01-31T00:00:00Z',
      },
    ],
  ])

  const report = (scan: MockScan): SecurityReport => {
    const scored = scoreFindings(MOCK_CATALOG, scan.evaluated, scan.findings, accepted, MOCK_NOW)
    return {
      generatedAt: scan.generatedAt,
      cluster: 'kind-kubebench',
      options: scan.options,
      score: { overall: scored.overall, byCategory: scored.byCategory },
      counts: scored.counts,
      findings: scored.findings,
      skipped: [],
      warnings: [],
      inventory: INVENTORY,
      systemNamespaces: SYSTEM_NAMESPACES,
    }
  }

  return {
    scan: (options) => {
      scans += 1
      last = runScan(options, new Date(BASE_TIME + scans * 60_000).toISOString().replace('.000Z', 'Z'))
      return delay(report(last))
    },
    report: () => delay(last ? report(last) : null),
    checks: () => delay(MOCK_CATALOG),
    accepted: () =>
      delay(
        [...accepted.entries()]
          .map(([key, a]): AcceptedRisk => ({ key, ...a, expired: Boolean(a.expiresAt && a.expiresAt <= MOCK_NOW) }))
          .sort((a, b) => (a.at < b.at ? 1 : a.at > b.at ? -1 : 0)),
      ),
    accept: (key, reason, expiresAt) => {
      const [checkId, kind, namespace, ...name] = key.split(':')
      const finding = last?.findings.find((f) => f.key === key)
      if (!finding || !reason.trim()) {
        return delayReject(new ApiError(400, 'bad_request', finding ? 'reason: required' : `key: no finding ${key}`))
      }
      const acceptance: Acceptance = { by: 'admin', reason: reason.trim(), at: MOCK_NOW, expiresAt }
      accepted.set(key, {
        ...acceptance,
        checkId,
        resource: { apiVersion: finding.resource.apiVersion, kind, namespace: namespace || undefined, name: name.join(':') },
      })
      return delay(acceptance)
    },
    unaccept: (key) => {
      if (!accepted.delete(key)) return delayReject(reasonError(404, 'not_found', 'security.no_accepted_risk', { key }))
      return delay(undefined)
    },
    exportReport: (format) => {
      if (!last) return delayReject(reasonError(404, 'not_found', 'security.no_report'))
      return delay(exportText(report(last), format))
    },
  }
}
