/** Where the upgrade steps of every install method are documented. */
export const UPGRADE_GUIDE_URL = 'https://github.com/Shavell/kube-bench/blob/main/docs/deployment.md#upgrades'

export interface UpgradeMethod {
  /** Key under `methods.` in the updates namespace (title and hint). */
  id: 'helm' | 'manifests' | 'packages' | 'installer' | 'docker'
  /** Shell commands for `version` (without "v"), as docs/deployment.md#upgrades gives them. */
  commands: (version: string) => string[]
}

/** The install methods of docs/deployment.md, in its order. */
export const UPGRADE_METHODS: UpgradeMethod[] = [
  {
    id: 'helm',
    commands: (v) => [
      `helm upgrade kubebench oci://ghcr.io/shavell/charts/kubebench --version ${v} -n kubebench --reset-then-reuse-values`,
    ],
  },
  {
    id: 'manifests',
    commands: (v) => [`kubectl apply -f https://github.com/Shavell/kube-bench/releases/download/v${v}/kubebench.yaml`],
  },
  {
    id: 'packages',
    commands: (v) => [`sudo apt install ./kubebench_${v}_amd64.deb`, `sudo dnf upgrade ./kubebench-${v}-1.x86_64.rpm`],
  },
  {
    id: 'installer',
    commands: (v) => [
      `curl -fsSL https://raw.githubusercontent.com/Shavell/kube-bench/main/install.sh | sudo bash -s -- --version v${v}`,
    ],
  },
  {
    id: 'docker',
    commands: (v) => [`docker pull ghcr.io/shavell/kube-bench:${v}`],
  },
]
