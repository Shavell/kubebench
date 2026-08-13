import { readFileSync } from 'node:fs'
import { homedir } from 'node:os'
import { join } from 'node:path'
import * as yaml from 'js-yaml'
import { Agent, fetch as undiciFetch } from 'undici'

/**
 * Minimal Kubernetes API client built from a kubeconfig (token auth).
 * No cloud-provider exec plugins — the kubeconfig must carry a bearer
 * token, which is what service-account kubeconfigs do.
 */
export function createKubeClient(path) {
  const kubeconfigPath = path ?? process.env.KUBECONFIG ?? join(homedir(), '.kube', 'config')
  const config = yaml.load(readFileSync(kubeconfigPath, 'utf8'))

  const contextName = config['current-context'] ?? config.contexts?.[0]?.name
  const context = config.contexts?.find((c) => c.name === contextName)?.context ?? {}
  const cluster = config.clusters?.find((c) => c.name === context.cluster) ?? config.clusters?.[0]
  const user = config.users?.find((u) => u.name === context.user) ?? config.users?.[0]

  const server = cluster?.cluster?.server
  const caData = cluster?.cluster?.['certificate-authority-data']
  const token = user?.user?.token
  const certData = user?.user?.['client-certificate-data']
  const keyData = user?.user?.['client-key-data']
  if (!server) throw new Error(`No cluster server found in ${kubeconfigPath}`)
  if (!token && !(certData && keyData)) {
    throw new Error(
      `No bearer token or client certificate in ${kubeconfigPath} (exec plugins not supported)`,
    )
  }

  const b64 = (data) => Buffer.from(data, 'base64').toString('utf8')
  const dispatcher = new Agent({
    connect: {
      ...(caData ? { ca: b64(caData) } : { rejectUnauthorized: false }),
      ...(certData && keyData ? { cert: b64(certData), key: b64(keyData) } : {}),
    },
  })

  async function request(path, { method = 'GET', body, contentType } = {}) {
    const res = await undiciFetch(`${server}${path}`, {
      method,
      dispatcher,
      headers: {
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...(contentType ? { 'Content-Type': contentType } : {}),
      },
      body: body ? JSON.stringify(body) : undefined,
    })
    const text = await res.text()
    if (!res.ok) {
      let message = text
      try {
        message = JSON.parse(text).message ?? text
      } catch {
        /* keep raw */
      }
      const err = new Error(message)
      err.status = res.status
      throw err
    }
    return text
  }

  return {
    contextName,
    server,
    namespace: context.namespace,
    kubeconfigPath,
    getJSON: async (path) => JSON.parse(await request(path)),
    getRaw: (path) => request(path),
    postJSON: async (path, body) =>
      JSON.parse(await request(path, { method: 'POST', body, contentType: 'application/json' })),
    delete: (path) => request(path, { method: 'DELETE' }),
    mergePatch: (path, body) =>
      request(path, { method: 'PATCH', body, contentType: 'application/merge-patch+json' }),
    strategicPatch: (path, body) =>
      request(path, { method: 'PATCH', body, contentType: 'application/strategic-merge-patch+json' }),
  }
}
