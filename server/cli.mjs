import { execFile } from 'node:child_process'

/**
 * Helpers for shelling out to helm/kubectl with the active cluster's
 * kubeconfig. Used for operations that are impractical to reimplement
 * over the raw API (helm release management, interactive-ish exec).
 */
export function runCli(bin, args, kubeconfigPath, { timeoutMs = 30_000 } = {}) {
  return new Promise((resolve, reject) => {
    execFile(
      bin,
      args,
      {
        env: { ...process.env, KUBECONFIG: kubeconfigPath },
        timeout: timeoutMs,
        maxBuffer: 8 * 1024 * 1024,
      },
      (error, stdout, stderr) => {
        if (error) {
          const message = (stderr || error.message || '').trim().split('\n').slice(0, 4).join('\n')
          reject(Object.assign(new Error(message || `${bin} failed`), { status: 500 }))
        } else {
          resolve(stdout)
        }
      },
    )
  })
}

export async function cliAvailable(bin) {
  try {
    await runCli(bin, ['version', '--help'], undefined, { timeoutMs: 5000 })
    return true
  } catch {
    return false
  }
}
