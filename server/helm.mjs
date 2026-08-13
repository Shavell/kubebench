import { runCli } from './cli.mjs'
import { formatAge } from './mappers.mjs'

/** Real Helm operations via the local `helm` binary. */

function mapStatus(status) {
  // helm statuses: deployed, failed, superseded, uninstalled, pending-*
  if (status?.startsWith('pending')) return 'pending-upgrade'
  return status ?? 'deployed'
}

function chartNameVersion(chart) {
  // "kube-prometheus-stack-58.2.1" → name + version
  const match = /^(.*)-(\d[\w.+-]*)$/.exec(chart ?? '')
  return match ? { name: match[1], version: match[2] } : { name: chart ?? '—', version: '' }
}

export function createHelmCli(kubeconfigPath) {
  const helm = (args, opts) => runCli('helm', args, kubeconfigPath, opts)

  return {
    async listReleases() {
      const out = await helm(['list', '-A', '-o', 'json'])
      const releases = JSON.parse(out || '[]')
      return Promise.all(
        releases.map(async (r) => {
          const { name: chartName, version: chartVersion } = chartNameVersion(r.chart)
          let history = []
          try {
            const hist = JSON.parse(
              await helm(['history', r.name, '-n', r.namespace, '-o', 'json', '--max', '10']),
            )
            history = hist
              .map((h) => ({
                revision: h.revision,
                status: mapStatus(h.status),
                updated: (h.updated ?? '').replace('T', ' ').slice(0, 16),
                description: h.description ?? '',
              }))
              .reverse()
          } catch {
            /* history is best-effort */
          }
          return {
            name: r.name,
            namespace: r.namespace,
            chart: chartName,
            chartVersion,
            appVersion: r.app_version ?? '—',
            revision: Number(r.revision),
            status: mapStatus(r.status),
            updated: (r.updated ?? '').slice(0, 16).replace('T', ' '),
            history,
          }
        }),
      )
    },

    async listRepositories() {
      try {
        const out = await helm(['repo', 'list', '-o', 'json'])
        return JSON.parse(out || '[]').map((r) => ({ name: r.name, url: r.url, chartCount: 0 }))
      } catch {
        return [] // no repos configured
      }
    },

    async getValuesYaml(name, namespace) {
      const out = await helm(['get', 'values', name, '-n', namespace, '-o', 'yaml'])
      return out || '# no user-supplied values\n'
    },

    rollback: (name, namespace, revision) =>
      helm(['rollback', name, String(revision), '-n', namespace, '--wait=false'], {
        timeoutMs: 60_000,
      }),

    uninstall: (name, namespace) =>
      helm(['uninstall', name, '-n', namespace, '--wait=false'], { timeoutMs: 60_000 }),
  }
}

export { formatAge }
