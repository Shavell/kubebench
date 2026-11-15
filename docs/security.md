# Security model and hardening

This page is for operators who run KubeBench for a team. It describes what
KubeBench protects, what it trusts, and how to deploy it tightly. To report
a vulnerability, see [SECURITY.md](../SECURITY.md).

## What KubeBench holds

KubeBench is a web front end to clusters it can reach with **stored
credentials**: the in-cluster ServiceAccount token, kubeconfigs pasted or
seeded by admins, and optional per-cluster cloud credentials. Its SQLite
database therefore contains everything needed to act on every registered
cluster, plus password hashes (bcrypt) and sessions. Whoever reads the
database, its backups or the data volume has the clusters.

- The database and its `-wal`/`-shm` files are created `0600` (existing
  ones are tightened at start). Keep backups as protected as the clusters.
- The browser never receives cluster credentials. Cloud credentials and
  kubeconfig contents are write-only in the API.

## Who can do what

**Global roles.** `admin` manages users, the cluster inventory, plugins,
update settings and reads the audit log. `user` sees only the clusters
assigned to them.

**Per-cluster roles.** Each assignment is `viewer` (read) or `operator`
(read, write, exec). Admins have full access to every cluster.

| | admin | operator | viewer |
|---|---|---|---|
| read: lists, YAML, events, logs, metrics, scans | ✓ | ✓ | ✓ |
| write: apply, edit, delete, scale, drain, Helm, Argo CD sync, … | ✓ | ✓ | — |
| exec: shells, debug containers, Secret reveal, Helm release values | ✓ | ✓ | — |
| users, clusters, plugins, audit, update settings | ✓ | — | — |

**Read-only clusters.** An admin can flag a cluster read-only: every write
and exec request is refused for everyone, admins included, until the flag is
cleared. Use it for production clusters people should only look at.

**Environment tags** (`development`, `staging`, `production`) colour the
status bar; on `production` clusters destructive dialogs require typing the
resource name.

KubeBench's roles are checked **before** the cluster's RBAC, never instead
of it: a request still needs the stored credential to allow it. The
ServiceAccount or kubeconfig decides the ceiling; KubeBench's roles narrow
it per user.

Every route declares its access level in one table (`routes.go`, plugin
`Routes`), and the router checks session, role, read-only flag and plugin
state before any handler or cluster call ([api.md](api.md#access-levels)).

## Secrets

- Secret values are **redacted on every read path**: lists, single objects,
  YAML, Helm release manifests (also dry-run and upgrade diffs), and
  kubectl's `last-applied-configuration` annotation, which repeats them.
- Revealing values is a separate `exec` route, audited, answered with
  `Cache-Control: no-store`; the UI keeps them in the dialog only.
- Helm release values often contain credentials, so reading them needs
  `exec` and is audited. Dry runs of apply/edit that touch a Secret are
  treated like the change itself (write, audited), because their outcome
  would confirm guessed values.
- The `/api/k8s` passthrough used by external plugins refuses Secrets,
  watches, and the `proxy`, `exec`, `attach` and `portforward`
  subresources.

## Audit log

Every write, exec and admin action is recorded by the router: time, user,
cluster, action, target, HTTP status and a short detail, **including refused
attempts**. Terminals are recorded once when they open (`exec.session`).
Request bodies are never stored (they may contain passwords or Secret
data). Rows are pruned after `KUBEBENCH_AUDIT_RETENTION_DAYS` (default 90;
`0` keeps them). Admins read and filter the log in the Audit view or with
`GET /api/audit`.

## Authentication and sessions

- Local accounts with bcrypt passwords (8 characters to 72 bytes). The
  bootstrap admin, users created by an admin and passwords reset by an
  admin must be changed at the next sign-in; until then only the session
  routes work.
- Session cookie `kubebench_session`: HttpOnly, SameSite=Lax, `Secure` on
  TLS (`KUBEBENCH_SECURE_COOKIES`). Sessions last `KUBEBENCH_SESSION_TTL`
  (default 7 days); a password change ends the user's other sessions.
- Logins are throttled: 10 failures per client IP and username in 15
  minutes → `429`. Wrong current passwords count too.
- State-changing requests and WebSocket upgrades must carry an `Origin`
  matching the host (CSRF defence in depth on top of SameSite). GETs of
  write and exec routes (such as Helm release values) are refused when the
  browser marks them as sent from another site.
- Terminals, log streams and watch streams are re-authorized at every
  heartbeat and end when the session, the assignment or the permission
  ends. Per-user limits: 10 terminals, 20 log streams, 16 watch streams.
- There is no SSO yet; put KubeBench behind your identity-aware proxy if
  you need one, and still use KubeBench accounts for roles.

## Kubeconfig handling

A pasted kubeconfig is untrusted input: running its exec plugins or reading
the files it names would let any admin run commands or read files as the
server's account.

- `KUBEBENCH_ALLOW_KUBECONFIG_EXEC=false` (default): pasted kubeconfigs
  whose selected context runs an exec credential plugin or auth provider,
  or references server-side files (CA, client certificate or key, token
  file), are refused. Turn it on only if every admin may run commands as
  the KubeBench user.
- `KUBEBENCH_ALLOW_PATH_KUBECONFIG` (default on outside a cluster, off
  inside): admins may add clusters by a path on the server. Such
  kubeconfigs are the server's own files and may use exec plugins; they are
  stored flattened. Paths inside the directories KubeBench itself fills
  from requests (each plugin's data directory under
  `KUBEBENCH_DATA_DIR`, such as the Helm repository cache and chart
  uploads, and `KUBEBENCH_PLUGINS_DIR`) are refused, symbolic links
  resolved: a repository index written there is not a file of the
  server's.
- The runtime's clients never follow HTTP redirects, so a hostile API
  server cannot bounce the stored credential elsewhere. Helm's clients are
  pinned to the API server's host.

## Cloud identities (EKS, AKS, GKE)

The EKS, GKE and AKS exec stanzas (`aws eks get-token`,
`aws-iam-authenticator`, `gke-gcloud-auth-plugin`, `kubelogin`) are never
run; KubeBench obtains the token itself and sends it as a bearer token to
the API server the kubeconfig names, **whatever host that is**. A
kubeconfig pointing at a host of the admin's choosing would therefore hand
them the token, which works outside KubeBench (a GCP token with the
`cloud-platform` scope, a replayable EKS token).

- `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY=false` (default): pasted EKS,
  GKE and AKS kubeconfigs must come with **per-cluster credentials** (AWS
  access key, GCP service-account key of type `service_account`, Azure
  service principal), stored with the cluster and never returned. Without
  them, or with an AWS profile (read from the server's files), adding or
  editing the cluster is refused. Kubeconfigs added by path, which are the
  server's own files, are not affected.
- `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY=true`: **an admin can make
  KubeBench sign in with the server's ambient cloud identity** (IRSA or EKS
  Pod Identity role, GKE or Azure workload identity, the VM's instance
  profile or managed identity, `AWS_*`/`AZURE_*` variables, `~/.aws` and
  gcloud files of the server's user) for any cluster they add, and so
  obtain its token. Turn it on only if every admin may hold that identity,
  and give it only the cloud permissions KubeBench should have. GCP external-account
and impersonation files are refused because they can read files, call URLs
or run commands.

## External plugins and the update check

- External UI plugins run in the page with the signed-in user's session and
  can do whatever that user can. They stay disabled until an admin enables
  them; install only plugins you trust and keep `KUBEBENCH_PLUGINS_DIR`
  writable only by the service account.
- The update check sends one unauthenticated GET to GitHub (or
  `KUBEBENCH_UPDATE_URL`) with `User-Agent: KubeBench/<version>` and nothing
  else. Release notes are rendered as text, never as HTML. Set
  `KUBEBENCH_UPDATE_MODE=off` for air-gapped installs; nothing is ever
  downloaded or installed by KubeBench.

## Hardening checklist

**Everywhere**

- [ ] Change the bootstrap password at once (KubeBench forces it) and keep
      the configured `KUBEBENCH_ADMIN_PASSWORD` source protected.
- [ ] Serve over TLS (Ingress, reverse proxy or `KUBEBENCH_TLS_*`); set
      `KUBEBENCH_TRUST_PROXY=true` only behind a proxy you control.
- [ ] Give people `viewer` by default and `operator` where needed; keep the
      admin group small.
- [ ] Flag production clusters read-only or tag them `production`.
- [ ] Use least-privilege credentials per cluster: the cluster's RBAC is
      the ceiling for everyone.
- [ ] Leave `KUBEBENCH_ALLOW_KUBECONFIG_EXEC` off.
- [ ] Leave `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY` off unless every admin
  may hold the server's cloud identity; prefer per-cluster credentials.
- [ ] Back up the data directory and protect the backups like credentials.

**Kubernetes (Helm chart)**

- [ ] Choose `rbac.preset`: `cluster-admin` (full workbench), `read-only`
      (get/list/watch on everything except Secrets: no changes, no exec) or
      `custom` rules.
- [ ] Assign `operator` on the `in-cluster` cluster only to people you
      would make KubeBench admins: with `cluster-admin`, an operator can
      reach KubeBench's own namespace, start a debug container next to it
      and read its database.
- [ ] Enable `networkPolicy.enabled` with `networkPolicy.ingressFrom`
      naming your ingress controller, so only it reaches the pod (required
      when `trustProxy` is on, else any pod could forge `X-Forwarded-For`
      and evade login throttling).
- [ ] Keep the chart's restricted Pod Security settings (non-root, read-only
      root filesystem, no capabilities, `RuntimeDefault` seccomp).
- [ ] Bind a cloud identity through `serviceAccount.annotations` only if
      KubeBench should use it for cloud clusters.

**VMs**

- [ ] Bind to loopback behind a reverse proxy
      (`KUBEBENCH_LISTEN_ADDR=127.0.0.1:9876`), or terminate TLS natively.
- [ ] Keep `/etc/kubebench/kubebench.env` `root:kubebench 0640`; the
      packaged systemd unit runs as the unprivileged `kubebench` user with
      systemd sandboxing.
- [ ] Turn off `KUBEBENCH_ALLOW_PATH_KUBECONFIG` if admins should not read
      kubeconfigs from the server's disk.

See [deployment.md](deployment.md) for the install-specific details and
[configuration.md](configuration.md) for every setting.
