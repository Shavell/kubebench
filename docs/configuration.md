# Configuration

The KubeBench server is configured through environment variables only. It
reads them once at startup
([`backend/internal/config`](../backend/internal/config/config.go)),
validates them all and, when a value is invalid, prints every problem and
exits with status 2. Values are trimmed; an empty value counts as unset.

## Variables

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `9876` | HTTP port (1–65535) |
| `KUBEBENCH_LISTEN_ADDR` | `:$PORT` | Full bind address (`host:port`), overrides `PORT`; `127.0.0.1:9876` keeps the UI local |
| `KUBEBENCH_DB` | `kubebench.db` | SQLite database file (created `0600`) |
| `KUBEBENCH_DATA_DIR` | directory of `KUBEBENCH_DB` | Writable state: Helm repository cache and chart uploads (`helm/`), plugin data, external plugins |
| `KUBEBENCH_PLUGINS_DIR` | `$KUBEBENCH_DATA_DIR/plugins` | External (frontend) plugins, one folder per plugin id |
| `KUBEBENCH_ADMIN_PASSWORD` | `admin` | Password of the bootstrap `admin` user, read only when the database is new. A value other than the default must be 8 characters to 72 bytes. Either way the admin must change it at the first sign-in |
| `KUBEBENCH_SECURE_COOKIES` | `auto` | `Secure` attribute of the session cookie: `auto` (when the request is TLS, or carries a trusted `X-Forwarded-Proto: https`), `true` or `false` |
| `KUBEBENCH_TRUST_PROXY` | `false` | Honour `X-Forwarded-Proto`, `X-Forwarded-For` and `X-Forwarded-Host`; enable only behind a reverse proxy that sets them |
| `KUBEBENCH_TLS_CERT`, `KUBEBENCH_TLS_KEY` | unset | Serve HTTPS directly; set both or neither. Both files are loaded at startup: an unreadable certificate or key is a configuration error |
| `KUBEBENCH_SESSION_TTL` | `168h` | Session lifetime as a Go duration, at least `1m` |
| `KUBEBENCH_AUDIT_RETENTION_DAYS` | `90` | Audit rows older than this are pruned daily; `0` keeps them forever |
| `KUBEBENCH_ALLOW_PATH_KUBECONFIG` | `true` outside a cluster, `false` inside | Let admins add clusters by a kubeconfig path on the server |
| `KUBEBENCH_ALLOW_KUBECONFIG_EXEC` | `false` | Let **pasted** kubeconfigs run exec credential plugins and auth providers and read files on the server. Every admin can then run commands as the server's user. The EKS, GKE and AKS exec stanzas need no opt-in (they are handled in process); kubeconfigs added by path may always use exec plugins |
| `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY` | `false` | Let **pasted** EKS, GKE and AKS kubeconfigs (and AWS profile credentials) sign in with the server's own cloud identity: workload identity, instance profile or managed identity, `AWS_*`/`AZURE_*` variables, `~/.aws` and gcloud files. The token goes to the API server the kubeconfig names, so every admin can then obtain it. Off: such clusters need per-cluster credentials. Kubeconfigs added by path are not affected |
| `KUBECONFIG` | unset | Seeds the cluster inventory on the first start; a path list (`a:b`) is merged the way kubectl merges it |
| `KUBEBENCH_NAMESPACE` | in a pod: the pod's namespace | The namespace KubeBench runs in; deleting a namespace of that name from KubeBench is refused unless forced |
| `KUBEBENCH_UPDATE_MODE` | `stable` | Default update check: `off` (never contacts GitHub), `stable` or `prerelease`. An admin's choice in Settings > Updates overrides it |
| `KUBEBENCH_UPDATE_URL` | `https://api.github.com/repos/Shavell/kube-bench/releases` | Releases list the update check reads (GitHub's "list releases" JSON); a mirror or a GitHub Enterprise API works too. `http` or `https` |
| `KUBEBENCH_PROXY_ORIGIN` | unset | Separate origin (`https://host[:port]`, no path) for the service browser (design §7.14): proxied pages run there unsandboxed, isolated from KubeBench by the origin. The host must reach this server too (DNS, reverse proxy, certificate); it serves only `/proxy/`. It must be on another domain than KubeBench (not KubeBench's host on another port, not a subdomain: cookies ignore ports and reach subdomains); otherwise the service browser refuses to open pages. Unset: pages run sandboxed on KubeBench's own origin |

Read by libraries rather than by `internal/config`:

| Variable | Used for |
|---|---|
| `HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY` | The update check (Go's proxy environment) |
| `AWS_*`, `GOOGLE_APPLICATION_CREDENTIALS`, `AZURE_*` | The server's own cloud identity for EKS, GKE and AKS clusters without per-cluster credentials ([deployment.md](deployment.md#cloud-clusters-eks-aks-gke)) |

"First start" means a new database without users. Only then is the
bootstrap admin created and the inventory seeded from `KUBECONFIG` and,
inside a pod, from the pod's ServiceAccount (registered as `in-cluster`).
Deleting a seeded cluster therefore sticks across restarts.

## Where each install sets them

### Helm chart

The chart ([values](../charts/kubebench/README.md#values)) sets `PORT=9876`,
`KUBEBENCH_DATA_DIR=/data` and `KUBEBENCH_DB=/data/kubebench.db`, and maps
its values:

| Value | Variable | Chart default |
|---|---|---|
| `config.secureCookies` | `KUBEBENCH_SECURE_COOKIES` | `auto` |
| `config.trustProxy` | `KUBEBENCH_TRUST_PROXY` | `null` = `true` when `ingress.enabled` |
| `config.sessionTTL` | `KUBEBENCH_SESSION_TTL` | `168h` |
| `config.auditRetentionDays` | `KUBEBENCH_AUDIT_RETENTION_DAYS` | `90` |
| `config.allowPathKubeconfig` | `KUBEBENCH_ALLOW_PATH_KUBECONFIG` | `false` |
| `config.allowAmbientCloudIdentity` | `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY` | `false` |
| `config.updateMode` | `KUBEBENCH_UPDATE_MODE` | `stable` |
| `config.proxyOrigin` | `KUBEBENCH_PROXY_ORIGIN` (set only when not empty; add its host to `ingress.hosts`) | `""` |
| `auth.existingSecret` / `auth.adminPassword` / generated | `KUBEBENCH_ADMIN_PASSWORD` | a random password kept across upgrades |

`extraEnv` adds or overrides any variable (for example
`KUBEBENCH_PLUGINS_DIR` or `KUBEBENCH_ALLOW_KUBECONFIG_EXEC`), and
`extraEnvFrom` reads them from ConfigMaps or Secrets. The plain manifests
(`deploy/kubernetes/kubebench.yaml`) are this chart rendered with its
defaults and without a password Secret.

### Packages and `install.sh`

`/etc/kubebench/kubebench.env`
([`packaging/kubebench.env`](../packaging/kubebench.env)) lists every
variable above, commented out with its default. The systemd unit sets
`KUBEBENCH_DATA_DIR=/var/lib/kubebench` and
`KUBEBENCH_DB=/var/lib/kubebench/kubebench.db`. Apply changes with
`sudo systemctl restart kubebench`. `install.sh` writes `PORT`,
`KUBEBENCH_LISTEN_ADDR`, `KUBEBENCH_ADMIN_PASSWORD` and `KUBECONFIG` from its
options, and `--offline` writes `KUBEBENCH_UPDATE_MODE=off` unless the file
sets a mode.

### Docker

Pass `-e` flags. The image sets `PORT`, `KUBEBENCH_DATA_DIR=/data` and
`KUBEBENCH_DB=/data/kubebench.db`.

## Settings kept in the database

A few settings are changed in the UI and stored in SQLite, not in the
environment:

- the update check mode (Settings > Updates, admins; wins over
  `KUBEBENCH_UPDATE_MODE` once set);
- which plugins are enabled (Extensions, admins);
- users, roles, cluster assignments, the cluster inventory with its
  read-only flag, environment and cloud credentials.

Per-user preferences (theme, language, metrics window, auto refresh, live
updates, keyboard shortcuts, terminal font size, namespace favourites,
palette aliases) live in the browser's local storage.
