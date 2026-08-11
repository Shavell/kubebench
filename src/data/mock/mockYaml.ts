import type { CustomResource, Deployment, HelmRelease, K8sNode, Pod, Service } from '../types'

/**
 * Hand-rolled Kubernetes-shaped YAML for the read-only editor tabs.
 * Output-only, so no YAML library is needed.
 */

function labelBlock(labels: Record<string, string>, indent: string): string {
  return Object.entries(labels)
    .map(([k, v]) => `${indent}${k}: ${v}`)
    .join('\n')
}

export function podYaml(pod: Pod): string {
  return `apiVersion: v1
kind: Pod
metadata:
  name: ${pod.name}
  namespace: ${pod.namespace}
  labels:
${labelBlock(pod.labels, '    ')}
  uid: ${fakeUid(pod.name)}
spec:
  nodeName: ${pod.node || '~'}
  containers:
${pod.containers
  .map((c) => {
    const command = c.command ? `\n      command: [${c.command.map((x) => `"${x}"`).join(', ')}]` : ''
    const args = c.args ? `\n      args: [${c.args.map((x) => `"${x}"`).join(', ')}]` : ''
    const env = c.env.length
      ? `\n      env:\n${c.env.map((e) => `        - name: ${e.name}\n          value: "${e.value}"`).join('\n')}`
      : ''
    return `    - name: ${c.name}
      image: ${c.image}
      imagePullPolicy: IfNotPresent${command}${args}${env}
      resources:
        requests:
          cpu: ${c.requests.cpu}
          memory: ${c.requests.memory}
        limits:
          cpu: ${c.limits.cpu}
          memory: ${c.limits.memory}`
  })
  .join('\n')}
  restartPolicy: Always
  dnsPolicy: ClusterFirst
status:
  phase: ${pod.status === 'CrashLoopBackOff' ? 'Running' : pod.status}
  podIP: ${pod.podIP || '~'}
  ${pod.status === 'CrashLoopBackOff' ? `containerStatuses:
    - name: ${pod.containers[0].name}
      restartCount: ${pod.restarts}
      state:
        waiting:
          reason: CrashLoopBackOff
          message: back-off 5m0s restarting failed container` : `conditions:
    - type: Ready
      status: "${pod.status === 'Running' ? 'True' : 'False'}"`}
`
}

export function deploymentYaml(d: Deployment): string {
  return `apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${d.name}
  namespace: ${d.namespace}
  labels:
${labelBlock(d.labels, '    ')}
spec:
  replicas: ${d.replicas}
  selector:
    matchLabels:
${labelBlock(d.labels, '      ')}
  template:
    metadata:
      labels:
${labelBlock(d.labels, '        ')}
    spec:
      containers:
        - name: ${d.name}
          image: registry.example.com/${d.name}:latest
status:
  readyReplicas: ${d.readyReplicas}
  updatedReplicas: ${d.upToDate}
  availableReplicas: ${d.available}
`
}

export function serviceYaml(s: Service): string {
  return `apiVersion: v1
kind: Service
metadata:
  name: ${s.name}
  namespace: ${s.namespace}
spec:
  type: ${s.type}
  clusterIP: ${s.clusterIP}
  ports:
    - port: ${s.ports.split('/')[0].split(':')[0]}
      protocol: TCP${s.externalIP ? `\nstatus:\n  loadBalancer:\n    ingress:\n      - ip: ${s.externalIP}` : ''}
`
}

export function nodeYaml(n: K8sNode): string {
  return `apiVersion: v1
kind: Node
metadata:
  name: ${n.name}
  labels:
${Object.entries(n.labels).map(([k, v]) => `    ${k}: "${v}"`).join('\n')}
  annotations:
${Object.entries(n.annotations).map(([k, v]) => `    ${k}: "${v}"`).join('\n')}
spec: {}
status:
  nodeInfo:
    kubeletVersion: ${n.version}
    osImage: ${n.os}
    kernelVersion: ${n.kernelVersion}
    containerRuntimeVersion: ${n.containerRuntime}
  capacity:
    cpu: "${n.cpuCapacity}"
    memory: ${n.memoryCapacityGi}Gi
  addresses:
    - type: InternalIP
      address: ${n.internalIP}
  conditions:
    - type: Ready
      status: "${n.status === 'Ready' ? 'True' : 'False'}"
`
}

export function customResourceYaml(cr: CustomResource, apiVersion: string, kind: string): string {
  const extras = Object.entries(cr.extra)
    .map(([k, v]) => `  ${k.toLowerCase()}: ${v}`)
    .join('\n')
  return `apiVersion: ${apiVersion}
kind: ${kind}
metadata:
  name: ${cr.name}${cr.namespace ? `\n  namespace: ${cr.namespace}` : ''}
spec:
${extras}
status:
  conditions:
    - type: Ready
      status: "${cr.status === 'Ready' || cr.status === 'Active' ? 'True' : 'False'}"
      reason: ${cr.status}
`
}

export function releaseManifestYaml(r: HelmRelease): string {
  return `# Release "${r.name}" revision ${r.revision} (${r.status})
# Chart: ${r.chart}-${r.chartVersion}  App version: ${r.appVersion}
apiVersion: v2
name: ${r.chart}
version: ${r.chartVersion}
appVersion: "${r.appVersion}"
`
}

function fakeUid(name: string): string {
  let h = 0
  for (const ch of name) h = (h * 31 + ch.charCodeAt(0)) >>> 0
  const hex = h.toString(16).padStart(8, '0')
  return `${hex}-${hex.slice(0, 4)}-4${hex.slice(1, 4)}-9${hex.slice(2, 5)}-${hex}${hex.slice(0, 4)}`
}
