import { formatAge } from './mappers.mjs'

/** Real CRD browsing over the apiextensions + dynamic APIs. */

export async function listCrds(kube) {
  const list = await kube.getJSON('/apis/apiextensions.k8s.io/v1/customresourcedefinitions')
  return list.items.map((crd) => ({
    name: crd.metadata.name,
    group: crd.spec.group,
    kind: crd.spec.names.kind,
    versions: crd.spec.versions.filter((v) => v.served).map((v) => v.name),
    scope: crd.spec.scope,
    shortNames: crd.spec.names.shortNames ?? [],
    age: formatAge(crd.metadata.creationTimestamp),
  }))
}

/** Evaluate a simple additionalPrinterColumns jsonPath like ".spec.replicas". */
function evalJsonPath(obj, jsonPath) {
  const parts = jsonPath.replace(/^\./, '').split('.')
  let value = obj
  for (const part of parts) {
    if (value == null) return undefined
    value = value[part]
  }
  if (value == null || typeof value === 'object') return undefined
  return String(value)
}

function crStatus(obj) {
  const ready = (obj.status?.conditions ?? []).find((c) => c.type === 'Ready' || c.type === 'Available')
  if (ready) return ready.status === 'True' ? 'Ready' : 'NotReady'
  return obj.status?.phase ?? 'Active'
}

export async function listCustomResources(kube, crdName) {
  const crd = await kube.getJSON(
    `/apis/apiextensions.k8s.io/v1/customresourcedefinitions/${crdName}`,
  )
  const version = crd.spec.versions.find((v) => v.served) ?? crd.spec.versions[0]
  const path = `/apis/${crd.spec.group}/${version.name}/${crd.spec.names.plural}`
  const list = await kube.getJSON(path)

  const printerColumns = (version.additionalPrinterColumns ?? [])
    .filter((c) => !['Age', 'Name', 'Namespace'].includes(c.name))
    .slice(0, 3)

  return list.items.map((obj) => {
    const extra = {}
    for (const col of printerColumns) {
      const value = evalJsonPath(obj, col.jsonPath)
      if (value !== undefined) extra[col.name] = value
    }
    return {
      name: obj.metadata.name,
      namespace: obj.metadata.namespace,
      crdName,
      status: crStatus(obj),
      age: formatAge(obj.metadata.creationTimestamp),
      extra,
    }
  })
}

/** Fetch one custom resource as an object, resolving the CRD by kind. */
export async function getCustomResourceByKind(kube, kind, namespace, name) {
  const crds = await kube.getJSON('/apis/apiextensions.k8s.io/v1/customresourcedefinitions')
  const crd = crds.items.find((c) => c.spec.names.kind === kind)
  if (!crd) return null
  const version = crd.spec.versions.find((v) => v.served) ?? crd.spec.versions[0]
  const base = `/apis/${crd.spec.group}/${version.name}`
  const path =
    crd.spec.scope === 'Namespaced' && namespace
      ? `${base}/namespaces/${namespace}/${crd.spec.names.plural}/${name}`
      : `${base}/${crd.spec.names.plural}/${name}`
  return kube.getJSON(path)
}
