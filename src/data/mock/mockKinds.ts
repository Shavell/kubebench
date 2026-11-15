import { crds } from './mockCrds'

/**
 * A kind the mock cluster serves: the mock's RESTMapper, so the resource
 * editing endpoints (design §7.2) know a kind's scope and the generic
 * browser key its objects are listed under.
 */
export interface MockKind {
  apiVersion: string
  kind: string
  namespaced: boolean
  /** Generic browser key (genericMock) of the kinds the catalog lists. */
  key?: string
}

const typed = (apiVersion: string, kind: string, namespaced: boolean): MockKind => ({ apiVersion, kind, namespaced })
const generic = (apiVersion: string, kind: string, key: string, namespaced = true): MockKind => ({
  apiVersion,
  kind,
  namespaced,
  key,
})

const KINDS: MockKind[] = [
  // The kinds with their own typed lists (mockCluster).
  typed('v1', 'Pod', true),
  typed('apps/v1', 'Deployment', true),
  typed('v1', 'Service', true),
  typed('v1', 'Node', false),
  typed('v1', 'Namespace', false),
  // The generic catalog (backend kube.Catalog).
  generic('v1', 'ConfigMap', 'configmaps'),
  generic('v1', 'Secret', 'secrets'),
  generic('v1', 'ResourceQuota', 'resourcequotas'),
  generic('v1', 'LimitRange', 'limitranges'),
  generic('autoscaling/v2', 'HorizontalPodAutoscaler', 'horizontalpodautoscalers'),
  generic('policy/v1', 'PodDisruptionBudget', 'poddisruptionbudgets'),
  generic('scheduling.k8s.io/v1', 'PriorityClass', 'priorityclasses', false),
  generic('node.k8s.io/v1', 'RuntimeClass', 'runtimeclasses', false),
  generic('coordination.k8s.io/v1', 'Lease', 'leases'),
  generic('admissionregistration.k8s.io/v1', 'MutatingWebhookConfiguration', 'mutatingwebhookconfigurations', false),
  generic('admissionregistration.k8s.io/v1', 'ValidatingWebhookConfiguration', 'validatingwebhookconfigurations', false),
  generic('apps/v1', 'StatefulSet', 'statefulsets'),
  generic('apps/v1', 'DaemonSet', 'daemonsets'),
  generic('apps/v1', 'ReplicaSet', 'replicasets'),
  generic('v1', 'ReplicationController', 'replicationcontrollers'),
  generic('batch/v1', 'Job', 'jobs'),
  generic('batch/v1', 'CronJob', 'cronjobs'),
  generic('networking.k8s.io/v1', 'Ingress', 'ingresses'),
  generic('networking.k8s.io/v1', 'IngressClass', 'ingressclasses', false),
  generic('v1', 'Endpoints', 'endpoints'),
  generic('discovery.k8s.io/v1', 'EndpointSlice', 'endpointslices'),
  generic('networking.k8s.io/v1', 'NetworkPolicy', 'networkpolicies'),
  generic('v1', 'PersistentVolumeClaim', 'persistentvolumeclaims'),
  generic('v1', 'PersistentVolume', 'persistentvolumes', false),
  generic('storage.k8s.io/v1', 'StorageClass', 'storageclasses', false),
  generic('v1', 'ServiceAccount', 'serviceaccounts'),
  generic('rbac.authorization.k8s.io/v1', 'Role', 'roles'),
  generic('rbac.authorization.k8s.io/v1', 'RoleBinding', 'rolebindings'),
  generic('rbac.authorization.k8s.io/v1', 'ClusterRole', 'clusterroles', false),
  generic('rbac.authorization.k8s.io/v1', 'ClusterRoleBinding', 'clusterrolebindings', false),
  generic('v1', 'Event', 'events'),
  // Custom resources of the mock CRDs.
  ...crds.map((crd) => typed(`${crd.group}/${crd.versions[0]}`, crd.kind, crd.scope === 'Namespaced')),
]

/** The kind as the mock cluster serves it, or undefined (the API's "no matches for kind"). */
export function findKind(apiVersion: string, kind: string): MockKind | undefined {
  return KINDS.find((k) => k.apiVersion === apiVersion && k.kind === kind)
}

/** The API version the mock serves a kind in, e.g. for the generic mock objects. */
export function apiVersionOf(kind: string): string | undefined {
  return KINDS.find((k) => k.kind === kind)?.apiVersion
}
