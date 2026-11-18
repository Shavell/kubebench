import { activeMockCluster } from '@/data/mock/mockInventory'
import { mockOptionalResources } from '@/data/mock/mockOpenShift'
import type { RbacCluster } from './mockEngine'

/**
 * The OpenShift mock cluster's SCC grants (design §7.15): OpenShift's
 * `system:openshift:scc:*` ClusterRoles, bound in shop as
 * `oc adm policy add-scc-to-user` binds them, and the risky SCCs of the
 * mock dataset (data/mock/mockOpenShift) by level.
 */

const sccUseRule = (name: string) => ({
  verbs: ['use'],
  apiGroups: ['security.openshift.io'],
  resources: ['securitycontextconstraints'],
  resourceNames: [name],
})

/** The mock RBAC of the active mock cluster: with SCC grants on the OpenShift one. */
export function mockClusterRbac(base: RbacCluster): RbacCluster {
  if (!mockOptionalResources(activeMockCluster().platform).includes('securitycontextconstraints')) return base
  return {
    ...base,
    sccLevels: { privileged: 'privileged', 'shop-payments-net': 'privileged', anyuid: 'anyuid' },
    roles: [
      ...base.roles,
      { ref: { kind: 'ClusterRole', name: 'system:openshift:scc:privileged' }, rules: [sccUseRule('privileged')] },
      { ref: { kind: 'ClusterRole', name: 'system:openshift:scc:anyuid' }, rules: [sccUseRule('anyuid')] },
      { ref: { kind: 'ClusterRole', name: 'system:openshift:scc:restricted-v2' }, rules: [sccUseRule('restricted-v2')] },
    ],
    bindings: [
      ...base.bindings,
      {
        ref: { kind: 'RoleBinding', namespace: 'shop', name: 'system:openshift:scc:anyuid' },
        roleRef: { kind: 'ClusterRole', name: 'system:openshift:scc:anyuid' },
        subjects: [{ kind: 'ServiceAccount', name: 'shop-postgres' }],
      },
      {
        ref: { kind: 'ClusterRoleBinding', name: 'system:openshift:scc:restricted-v2' },
        roleRef: { kind: 'ClusterRole', name: 'system:openshift:scc:restricted-v2' },
        subjects: [{ kind: 'Group', name: 'system:authenticated' }],
      },
    ],
  }
}
