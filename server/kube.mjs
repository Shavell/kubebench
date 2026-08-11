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
  if (!server) throw new Error(`No cluster server found in ${kubeconfigPath}`)
  if (!token) throw new Error(`No bearer token in ${kubeconfigPath} (exec/cert auth not supported)`)

  const dispatcher = new Agent({
    connect: caData ? { ca: Buffer.from(caData, 'base64').toString('utf8') } : { rejectUnauthorized: false },
  })

  async function request(path, { method = 'GET', body, contentType } = {}) {
    const res = await undiciFetch(`${server}${path}`, {
      method,
      dispatcher,
      headers: {
        Authorization: `Bearer ${token}`,
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
    getJSON: async (path) => JSON.parse(await request(path)),
    getRaw: (path) => request(path),
    delete: (path) => request(path, { method: 'DELETE' }),
    mergePatch: (path, body) =>
      request(path, { method: 'PATCH', body, contentType: 'application/merge-patch+json' }),
    strategicPatch: (path, body) =>
      request(path, { method: 'PATCH', body, contentType: 'application/strategic-merge-patch+json' }),
  }
}
