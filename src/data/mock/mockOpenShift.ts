import type { ClusterPlatform, KubeObject } from '../types'

/**
 * OpenShift objects of the mock dataset (design §7.15), keyed like the
 * generic catalog. Only the OpenShift mock cluster serves their types
 * (mockOptionalResources), so the other mock clusters never show them.
 * Deterministic: synthetic ages, fixed timestamps.
 */

/** The optional catalog keys an OpenShift cluster serves (backend kube.OptionalResources). */
export const OPENSHIFT_RESOURCES = [
  'routes',
  'projects',
  'securitycontextconstraints',
  'deploymentconfigs',
  'buildconfigs',
  'builds',
  'imagestreams',
  'imagestreamtags',
]

/** The optional resource types the active mock cluster serves. */
export function mockOptionalResources(platform: ClusterPlatform | null | undefined): string[] {
  return platform?.provider === 'openshift' ? [...OPENSHIFT_RESOURCES] : []
}

function os(apiVersion: string, kind: string, name: string, namespace: string | undefined, age: string, rest: Record<string, unknown> = {}): KubeObject {
  const { metadata, ...fields } = rest
  return {
    apiVersion,
    kind,
    metadata: { name, ...(namespace ? { namespace } : {}), labels: {}, ...(metadata as object | undefined) },
    age,
    ...fields,
  }
}

const admitted = (host: string) => ({
  ingress: [{ host, routerName: 'default', conditions: [{ type: 'Admitted', status: 'True', lastTransitionTime: '2026-08-01T10:00:00Z' }] }],
})

const route = (name: string, rest: Record<string, unknown>) => os('route.openshift.io/v1', 'Route', name, 'shop', '45d', rest)
const project = (name: string, displayName: string, age: string) =>
  os('project.openshift.io/v1', 'Project', name, undefined, age, {
    metadata: { annotations: { 'openshift.io/display-name': displayName, 'openshift.io/requester': 'developer' } },
    status: { phase: 'Active' },
  })
const scc = (name: string, age: string, fields: Record<string, unknown>) =>
  os('security.openshift.io/v1', 'SecurityContextConstraints', name, undefined, age, fields)
const build = (n: number, phase: string, extra: Record<string, unknown>) =>
  os('build.openshift.io/v1', 'Build', `shop-api-${n}`, 'shop', `${10 - n}d`, {
    metadata: { labels: { buildconfig: 'shop-api' }, ownerReferences: [{ apiVersion: 'build.openshift.io/v1', kind: 'BuildConfig', name: 'shop-api', controller: true }] },
    spec: { strategy: { type: 'Docker' }, output: { to: { kind: 'ImageStreamTag', name: 'shop-api:latest' } } },
    status: { phase, ...extra },
  })

export const openShiftMock: Record<string, KubeObject[]> = {
  routes: [
    route('shop-frontend', {
      spec: {
        host: 'shop-frontend-shop.apps.ocp-lab.example.com',
        to: { kind: 'Service', name: 'shop-frontend', weight: 100 },
        port: { targetPort: 80 },
        tls: { termination: 'edge', insecureEdgeTerminationPolicy: 'Redirect' },
      },
      status: admitted('shop-frontend-shop.apps.ocp-lab.example.com'),
    }),
    route('shop-cart', {
      spec: {
        host: 'shop-frontend-shop.apps.ocp-lab.example.com',
        path: '/api/cart',
        to: { kind: 'Service', name: 'shop-cart', weight: 100 },
        port: { targetPort: 8080 },
        tls: { termination: 'edge' },
      },
      status: admitted('shop-frontend-shop.apps.ocp-lab.example.com'),
    }),
    // Its Service is gone, and the router has not admitted it.
    route('legacy-admin', {
      spec: { host: 'admin-shop.apps.ocp-lab.example.com', to: { kind: 'Service', name: 'legacy-admin', weight: 100 } },
      status: {
        ingress: [{ host: 'admin-shop.apps.ocp-lab.example.com', routerName: 'default', conditions: [{ type: 'Admitted', status: 'False', reason: 'HostAlreadyClaimed', message: 'route legacy-admin is newer than another route of this host' }] }],
      },
    }),
  ],
  projects: [
    project('shop', 'Shop', '45d'),
    project('monitoring', 'Monitoring', '61d'),
    project('cert-manager', 'cert-manager', '77d'),
  ],
  securitycontextconstraints: [
    scc('restricted-v2', '92d', {
      metadata: { annotations: { 'include.release.openshift.io/self-managed-high-availability': 'true' } },
      allowPrivilegedContainer: false, allowHostNetwork: false, runAsUser: { type: 'MustRunAsRange' }, seLinuxContext: { type: 'MustRunAs' },
      groups: [],
    }),
    scc('anyuid', '92d', {
      metadata: { annotations: { 'include.release.openshift.io/self-managed-high-availability': 'true' } },
      priority: 10, allowPrivilegedContainer: false, runAsUser: { type: 'RunAsAny' }, seLinuxContext: { type: 'MustRunAs' },
      groups: ['system:cluster-admins'],
    }),
    scc('privileged', '92d', {
      metadata: { annotations: { 'include.release.openshift.io/self-managed-high-availability': 'true' } },
      allowPrivilegedContainer: true, allowHostNetwork: true, allowHostDirVolumePlugin: true, runAsUser: { type: 'RunAsAny' },
      seLinuxContext: { type: 'RunAsAny' }, users: ['system:admin', 'system:serviceaccount:shop:payments'], groups: ['system:cluster-admins'],
    }),
    scc('shop-payments-net', '12d', {
      allowPrivilegedContainer: true, allowHostNetwork: false, runAsUser: { type: 'RunAsAny' }, seLinuxContext: { type: 'RunAsAny' },
      users: ['system:serviceaccount:shop:payments'],
    }),
  ],
  deploymentconfigs: [
    os('apps.openshift.io/v1', 'DeploymentConfig', 'shop-legacy', 'shop', '120d', {
      spec: {
        replicas: 2,
        selector: { app: 'shop-legacy' },
        triggers: [{ type: 'ConfigChange' }, { type: 'ImageChange', imageChangeParams: { automatic: true, from: { kind: 'ImageStreamTag', name: 'shop-api:latest' } } }],
      },
      status: {
        latestVersion: 7, replicas: 2, readyReplicas: 1, availableReplicas: 1,
        conditions: [
          { type: 'Available', status: 'True', reason: 'MinimumReplicasAvailable', message: 'Deployment config has minimum availability.' },
          { type: 'Progressing', status: 'True', reason: 'NewReplicationControllerAvailable', message: 'replication controller "shop-legacy-7" successfully rolled out' },
        ],
      },
    }),
  ],
  buildconfigs: [
    os('build.openshift.io/v1', 'BuildConfig', 'shop-api', 'shop', '30d', {
      spec: {
        source: { type: 'Git', git: { uri: 'https://github.com/example/shop-api.git', ref: 'main' } },
        strategy: { type: 'Docker' },
        output: { to: { kind: 'ImageStreamTag', name: 'shop-api:latest' } },
        triggers: [{ type: 'ConfigChange' }],
      },
      status: { lastVersion: 3 },
    }),
  ],
  builds: [
    build(1, 'Failed', { reason: 'DockerBuildFailed', message: 'Docker build strategy has failed.', startTimestamp: '2026-08-01T09:00:00Z', completionTimestamp: '2026-08-01T09:01:12Z', duration: 72000000000 }),
    build(2, 'Complete', { startTimestamp: '2026-08-04T12:00:00Z', completionTimestamp: '2026-08-04T12:02:05Z', duration: 125000000000, outputDockerImageReference: 'image-registry.openshift-image-registry.svc:5000/shop/shop-api:latest' }),
    build(3, 'Running', { startTimestamp: '2026-08-11T09:10:00Z' }),
  ],
  imagestreams: [
    os('image.openshift.io/v1', 'ImageStream', 'shop-api', 'shop', '30d', {
      status: {
        dockerImageRepository: 'image-registry.openshift-image-registry.svc:5000/shop/shop-api',
        publicDockerImageRepository: 'default-route-openshift-image-registry.apps.ocp-lab.example.com/shop/shop-api',
        tags: [{ tag: 'latest', items: [{ image: 'sha256:5f1c3a8e9b2d', created: '2026-08-04T12:02:05Z' }] }],
      },
    }),
  ],
  imagestreamtags: [
    os('image.openshift.io/v1', 'ImageStreamTag', 'shop-api:latest', 'shop', '7d', {
      tag: { name: 'latest' },
      image: {
        metadata: { name: 'sha256:5f1c3a8e9b2d' },
        dockerImageReference: 'image-registry.openshift-image-registry.svc:5000/shop/shop-api@sha256:5f1c3a8e9b2d',
      },
    }),
  ],
}
