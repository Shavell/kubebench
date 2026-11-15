import type { RbacCluster } from './mockEngine'
import type { PolicyRule, RbacRef, Subject } from './types'

/**
 * The demo cluster's RBAC: Kubernetes' default roles (trimmed), the mock
 * cluster's namespaces and add-ons (cert-manager, monitoring, shop), and a
 * few deliberate problems — an impersonating CI role, a binding to a
 * deleted role and one to a ServiceAccount that does not exist.
 */

const rule = (verbs: string[], apiGroups: string[], resources: string[], resourceNames?: string[]): PolicyRule =>
  resourceNames ? { verbs, apiGroups, resources, resourceNames } : { verbs, apiGroups, resources }
const urls = (verbs: string[], nonResourceURLs: string[]): PolicyRule => ({ verbs, nonResourceURLs })

const READ = ['get', 'list', 'watch']
const WRITE = ['create', 'update', 'patch', 'delete', 'deletecollection']

const clusterRole = (name: string): RbacRef => ({ kind: 'ClusterRole', name })
const role = (namespace: string, name: string): RbacRef => ({ kind: 'Role', namespace, name })
const user = (name: string): Subject => ({ kind: 'User', name })
const group = (name: string): Subject => ({ kind: 'Group', name })
const sa = (namespace: string, name: string): Subject => ({ kind: 'ServiceAccount', namespace, name })

const viewRules = [
  rule(READ, [''], ['pods', 'pods/log', 'services', 'endpoints', 'configmaps', 'persistentvolumeclaims', 'events']),
  rule(READ, ['apps'], ['deployments', 'replicasets', 'statefulsets', 'daemonsets', 'deployments/scale']),
  rule(READ, ['batch'], ['jobs', 'cronjobs']),
  rule(READ, ['networking.k8s.io'], ['ingresses', 'networkpolicies']),
]
const editRules = [
  ...viewRules,
  rule([...READ, ...WRITE], [''], ['pods', 'services', 'configmaps', 'secrets', 'persistentvolumeclaims']),
  rule(['create'], [''], ['pods/exec', 'pods/attach', 'pods/portforward', 'serviceaccounts/token']),
  rule(['impersonate'], [''], ['serviceaccounts']),
  rule([...READ, ...WRITE], ['apps'], ['deployments', 'statefulsets', 'daemonsets', 'replicasets', 'deployments/scale']),
  rule([...READ, ...WRITE], ['batch'], ['jobs', 'cronjobs']),
]

export const mockRbac: RbacCluster = {
  roles: [
    { ref: clusterRole('cluster-admin'), rules: [rule(['*'], ['*'], ['*']), urls(['*'], ['*'])] },
    {
      ref: clusterRole('admin'),
      aggregated: true,
      rules: [...editRules, rule([...READ, ...WRITE], ['rbac.authorization.k8s.io'], ['roles', 'rolebindings'])],
    },
    { ref: clusterRole('edit'), aggregated: true, rules: editRules },
    { ref: clusterRole('view'), aggregated: true, rules: viewRules },
    {
      ref: clusterRole('system:discovery'),
      rules: [urls(['get'], ['/api', '/api/*', '/apis', '/apis/*', '/healthz', '/livez', '/readyz', '/version', '/version/'])],
    },
    {
      ref: clusterRole('cert-manager-controller'),
      rules: [
        rule(READ, ['cert-manager.io'], ['certificates', 'certificaterequests', 'issuers', 'clusterissuers']),
        rule(['update', 'patch'], ['cert-manager.io'], ['certificates/status', 'certificaterequests/status']),
        rule([...READ, 'create', 'update', 'delete'], [''], ['secrets']),
        rule(['create', 'patch'], [''], ['events']),
      ],
    },
    {
      ref: clusterRole('prometheus-server'),
      rules: [
        rule(READ, [''], ['nodes', 'nodes/metrics', 'services', 'endpoints', 'pods']),
        rule(['get'], [''], ['nodes/proxy']),
        urls(['get'], ['/metrics', '/metrics/cadvisor']),
      ],
    },
    {
      ref: clusterRole('ci-impersonator'),
      rules: [rule(['impersonate'], [''], ['serviceaccounts', 'users']), rule(['create'], ['apps'], ['deployments'])],
    },
    { ref: clusterRole('kubebench-reader'), rules: [rule(READ, ['*'], ['*']), rule(['create'], ['authorization.k8s.io'], ['subjectaccessreviews'])] },
    {
      ref: role('cert-manager', 'cert-manager-cainjector:leaderelection'),
      rules: [rule(['get', 'create', 'update', 'patch'], ['coordination.k8s.io'], ['leases'])],
    },
    {
      ref: role('shop', 'shop-deployer'),
      rules: [
        rule(['*'], ['apps'], ['*']),
        rule(['get', 'update'], [''], ['configmaps'], ['shop-config']),
        rule(['get'], [''], ['pods/log']),
      ],
    },
  ],
  bindings: [
    { ref: { kind: 'ClusterRoleBinding', name: 'cluster-admin' }, roleRef: clusterRole('cluster-admin'), subjects: [group('system:masters')] },
    {
      ref: { kind: 'ClusterRoleBinding', name: 'platform-admins' },
      roleRef: clusterRole('cluster-admin'),
      subjects: [group('platform-admins'), user('ayse@example.com')],
    },
    {
      ref: { kind: 'ClusterRoleBinding', name: 'system:discovery' },
      roleRef: clusterRole('system:discovery'),
      subjects: [group('system:authenticated')],
    },
    {
      ref: { kind: 'ClusterRoleBinding', name: 'cert-manager-controller' },
      roleRef: clusterRole('cert-manager-controller'),
      subjects: [sa('cert-manager', 'cert-manager')],
    },
    {
      ref: { kind: 'ClusterRoleBinding', name: 'prometheus-server' },
      roleRef: clusterRole('prometheus-server'),
      subjects: [sa('monitoring', 'prometheus-server')],
    },
    {
      ref: { kind: 'ClusterRoleBinding', name: 'ci-impersonation' },
      roleRef: clusterRole('ci-impersonator'),
      subjects: [sa('kube-system', 'ci-runner')],
    },
    {
      ref: { kind: 'ClusterRoleBinding', name: 'kubebench' },
      roleRef: clusterRole('kubebench-reader'),
      subjects: [sa('kubebench', 'kubebench')],
    },
    {
      ref: { kind: 'RoleBinding', namespace: 'cert-manager', name: 'cert-manager-cainjector:leaderelection' },
      roleRef: role('cert-manager', 'cert-manager-cainjector:leaderelection'),
      subjects: [sa('cert-manager', 'cert-manager')],
    },
    {
      ref: { kind: 'RoleBinding', namespace: 'default', name: 'legacy-admin' },
      roleRef: clusterRole('legacy-admin'),
      subjects: [user('old-admin')],
    },
    {
      ref: { kind: 'RoleBinding', namespace: 'monitoring', name: 'grafana-view' },
      roleRef: clusterRole('view'),
      subjects: [user('grafana')],
    },
    {
      ref: { kind: 'RoleBinding', namespace: 'shop', name: 'shop-developers' },
      roleRef: clusterRole('edit'),
      subjects: [group('shop-devs'), user('mehmet@example.com')],
    },
    {
      ref: { kind: 'RoleBinding', namespace: 'shop', name: 'shop-deployer' },
      roleRef: role('shop', 'shop-deployer'),
      subjects: [{ kind: 'ServiceAccount', name: 'deployer' }],
    },
    {
      ref: { kind: 'RoleBinding', namespace: 'shop', name: 'shop-sa-view' },
      roleRef: clusterRole('view'),
      subjects: [group('system:serviceaccounts:shop')],
    },
  ],
  serviceAccounts: [
    { namespace: 'cert-manager', name: 'cert-manager' },
    { namespace: 'default', name: 'default' },
    { namespace: 'kube-system', name: 'default' },
    { namespace: 'kubebench', name: 'kubebench' },
    { namespace: 'monitoring', name: 'prometheus-server' },
    { namespace: 'shop', name: 'default' },
    { namespace: 'shop', name: 'deployer' },
  ],
}
