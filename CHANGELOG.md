# Changelog

All notable changes to KubeBench are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

<!--
Versions 0.1.0 to 0.4.0 were never tagged. Their entries were reconstructed
from `git log --reverse acc8149`: 0.2.0, 0.3.0 and 0.4.0 are the image
versions that the commit messages of b382f96, 6bec8c1 and acc8149 report as
deployed; 0.1.0 collects everything up to the first container image and Helm
chart (chart appVersion 0.1.0, d14cadc and a0129ab). Their links point at
those commits.
-->

## [Unreleased]

### Added

- Development: Go fuzz tests for the code that reads untrusted input
  (apply manifests, Secret and Helm manifest redaction, the service
  browser's paths and redirects, LDAP filters, OIDC claims, the release
  list), run for a short while in CI on `main` and weekly.

### Changed

- Releases carry their build provenance as `kubebench-<version>.intoto.jsonl`
  too (it was only in the attestations API), so it can be verified offline;
  added to 0.8.1 as well.
- The container image's base images (Node, Go, distroless) are pinned by
  the digest of their multi-arch index; Dependabot updates tag and digest
  together.
- Pushing a `v*` tag starts the release workflow (image and chart
  included; the GitHub release stays a draft), and pull requests that touch
  the image build and smoke-test it.

### Security

- A Helm manifest document with a stray leading indent is redacted too:
  the YAML parser read only its first key while Helm, which trims each
  document, saw the whole Secret (found by the CI fuzz job).
- A Helm release's manifest view no longer shows the values of a Secret
  written on the `---` line (`--- {kind: Secret, …}`) or whose kind is
  spelled with YAML escapes or tags; both were passed on unredacted.
- The service browser keeps redirects inside the browsing session: a
  `Location` with `..` segments, a leading space or backslash, or an
  `http:` URL without a host is no longer able to send the frame to
  KubeBench's own paths.
- `backend/go.mod` now requires Go 1.26.11 or later, so a build with an
  older Go (whose standard library has known vulnerabilities) is refused
  instead of producing a vulnerable binary. Release builds keep using the
  `toolchain` (Go 1.27.4).


## [0.8.1] - 2026-11-18

### Added

- **Verifiable releases**: every archive and package comes with an SPDX
  SBOM (`<file>.sbom.json`), `checksums.txt` is signed keylessly with
  cosign (`checksums.txt.sigstore.json`), the container image is signed
  with cosign, and the binaries, packages, chart, manifests, installer,
  offline bundle and image have GitHub build provenance attestations.
  `cosign verify-blob`, `cosign verify` and `gh attestation verify` check
  them; the commands are in `SECURITY.md` and `docs/releasing.md`.

### Security

- The binaries and `.deb`/`.rpm` packages are built with Go 1.27.4, like
  the container image. 0.8.0's binaries and packages were built with Go
  1.26.5, whose standard library has known vulnerabilities (among them in
  `net/http`, `crypto/tls` and `crypto/x509`). The 0.8.0 image already
  used Go 1.27.
- go-jose 4.1.6 (CVE-2026-116587, denial of service through crafted RSA
  keys in a JWK set, and two smaller fixes). govulncheck finds the affected
  code unreachable from KubeBench; the update keeps image scans clean.
- The YAML editor's bundled DOMPurify (through Monaco) is 3.4.18, which
  fixes several sanitizer bypasses.
- Every push and pull request is scanned with CodeQL, govulncheck,
  `npm audit`, Trivy and dependency review; OpenSSF Scorecard rates the
  repository weekly.

### Fixed

- Helm upgrades are no longer marked failed in the background after they
  succeeded: Helm 3.22 watches the upgrade's context from a goroutine that
  could see its cancellation after a successful upgrade had returned.


## [0.8.0] - 2026-11-18

_The GitHub release (its downloads) was withdrawn on 2026-11-18: its builds
have known vulnerabilities and were not signed. The `v0.8.0` tag remains for
the history. Use 0.8.1 or later._

### Added

- **OpenShift monitoring as a metrics source**: on OpenShift the charts,
  the overview and the status bar read from the cluster monitoring's Thanos
  Querier (with KubeBench's own token; it needs the
  `cluster-monitoring-view` ClusterRole), including network and filesystem
  series. Node charts also find OpenShift's node-exporter series, the disk
  chart reads RHCOS' root filesystem, and pod CPU and memory no longer count
  the pod cgroup twice.
- **OpenShift install with least privilege**: the Helm chart detects
  OpenShift (`openshift.enabled: auto`) and then leaves the UIDs and
  `fsGroup` to the SCC and pins the pod to `restricted-v2`, so both the
  `cluster-admin` and the `read-only` presets install without extra SCC
  grants. `route.enabled` adds a Route (edge, reencrypt with a service CA
  certificate, or passthrough), the `read-only` preset reads the OpenShift
  API groups (Routes, Projects, DeploymentConfigs, Builds, ImageStreams,
  SCCs; no SCC `use`) and gets `cluster-monitoring-view` for OpenShift
  monitoring, and the NetworkPolicy reaches OpenShift's DNS on port 5353
  and the Thanos Querier on port 9091. `hostUsers: false` runs the pod in a user namespace
  (`restricted-v3`). Plain manifests for OpenShift with a Route ship as
  `kubebench-openshift.yaml`; see the OpenShift section in
  docs/deployment.md.
- **OpenShift resources**: on OpenShift clusters the Workloads sidebar has
  an OpenShift group with Routes, DeploymentConfigs, BuildConfigs, Builds,
  ImageStreams, ImageStreamTags and Security Context Constraints, and
  Projects next to Namespaces, with columns of their own (Route host,
  TLS termination and admission, build phase and duration, …), conditions
  on the detail pages and Route → Service edges in the topology. Other
  clusters do not show them. A Route's TLS private key and the webhook
  secrets of BuildConfigs and Builds are hidden like Secret values,
  in Helm release manifests too.
- **Open a Route in the service browser**: "Open in browser" on a Route
  opens the Service behind it with the Route's host, and "Open route URL"
  opens its public address.
- **OpenShift version support**: the provider badge shows the OpenShift
  version (read from the ClusterVersion) and Red Hat's life cycle (full
  support, maintenance support, Extended Update Support), with a warning
  when it ends.
- **OpenShift security checks**: privileged and anyuid SCCs granted to
  users, groups or ServiceAccounts (in the SCC or through RBAC), workloads
  admitted with them and custom privileged SCCs; risky roles list SCC
  "use" grants. An SCC that forces UID 0 (or a UID range starting at 0)
  counts as anyuid, and one that adds capabilities such as `SYS_ADMIN`
  as privileged.

### Changed

- **Security scans skip OpenShift's own namespaces** (`openshift`,
  `openshift-*`, `kube-*` and those OpenShift labels as its own) on
  OpenShift unless system namespaces are included; on a fresh OpenShift
  5.0 cluster the default scan went from 633 to 73 findings. Other
  clusters still skip only `kube-system`, `kube-public` and
  `kube-node-lease`.

### Fixed

- **No made-up metrics**: a cluster without a working metrics API no
  longer reports metrics-server as available and 0% CPU and memory use.
  The overview, the allocation bars and the status bar show "—" / "No
  metrics" with the reason (e.g. the metrics API is not served, or its
  APIService is not available), and the charts show why they are empty.
  Nodes and pods that were not measured have no points instead of zeros.
- The container image's `/data` is writable by an arbitrary UID in group 0
  (OpenShift, `docker run --user <uid>:0`), not only by UID 65532.

## [0.7.0] - 2026-11-18

_The GitHub release (its downloads) was withdrawn on 2026-11-18: its builds
have known vulnerabilities and were not signed. The `v0.7.0` tag remains for
the history. Use 0.8.1 or later._

### Added

- **Interface scale and fonts**: Settings → Appearance zooms the whole
  interface (90–130%), and Settings → Editor and Terminal and logs choose
  the font family and size of the code editors and of the terminal, exec
  and log views. They are kept in the browser for each user.
- **Notes live updates**: other users' changes to shared notes, and your
  own changes in another tab, appear at once instead of when the window
  regains focus. Personal notes are announced to their owner only.
- **Open an Ingress in the service browser**: "Open in browser" on an
  Ingress picks one of its rules (host and path) and shows the Service
  behind it, sending the rule's host, so virtual-hosted applications work.
- Single sign-on with OpenID Connect: "Sign in with <provider>" on the
  sign-in screen (authorization code flow with PKCE), users created at
  their first sign-in, and global and cluster roles re-computed from group
  mappings at every sign-in. Local accounts keep working; the last local
  admin is the break-glass account and cannot be removed (Users explains
  a refused change with that reason: promote another local user first). Allowed email
  domains admit only addresses the provider marks verified. Sign-ins in
  progress are kept in the browser, sealed by the server, so a flood of
  started sign-ins cannot break other users' sign-ins, and callbacks that
  never reached the provider do not count against the login limit. An
  external username that differs from an existing account's only in case
  (`Admin` next to the local `admin`) is refused, and new local users
  cannot be such a variant either.
- Sign-in with an LDAP directory or Active Directory: the sign-in screen
  offers the directory next to SSO and local accounts; users sign in as
  `user`, `DOMAIN\user` or `user@domain`, over LDAPS or StartTLS with a
  custom CA. Active Directory nested groups count for group mappings, and
  disabled accounts are refused.
- Settings > Authentication for admins: the sign-in methods, group
  mappings (audited) and a "test sign-in" preview of what a user would get
  (looked up in the directory for directory users).
- New settings `KUBEBENCH_PUBLIC_URL`, `KUBEBENCH_AUTH_REQUIRE_GROUP`,
  `KUBEBENCH_AUTH_GROUP_MAPPINGS`, `KUBEBENCH_OIDC_*` and `KUBEBENCH_LDAP_*`,
  with matching Helm chart values (`config.publicUrl`, `identity.*`, the
  client secret and bind password from an existing Secret).

### Changed

- **Server messages in your language**: errors and explanations from the
  server (a refused change on a read-only cluster or for a viewer, a missing
  Prometheus, Argo CD not installed, validation errors, why live updates
  fall back to polling, why the update check failed, why a terminal, log
  stream or live-update stream was closed) are shown in Turkish and German
  too. The API adds a stable `reason` code and `params` to such
  errors; messages of the Kubernetes API itself stay as they are.

### Security

- The database keeps only a SHA-256 digest of each session token, so a
  copy of it or of a backup holds no usable sessions. Existing sessions are
  converted when the new version first starts; nobody is signed out.

## [0.6.0] - 2026-11-18

_The GitHub release (its downloads) was withdrawn on 2026-11-18: its builds
have known vulnerabilities and were not signed. The `v0.6.0` tag remains for
the history. Use 0.8.1 or later._

First public release. It also ships everything listed under 0.5.0, which
was never published on its own.

### Added

- **Notes**: personal and shared Markdown notes on a cluster, on any of its
  objects (a "Notes" button with a count on every detail page) or, for
  personal notes, on every cluster — with tags, search, pins, an editor with
  preview and conflict detection. Shared notes are written by operators and
  admins, also on read-only clusters, and their changes are audited.
- **Service browser**: "Open in browser" on Services and Pods shows the
  workload's web interface in a browser tab to the right of the editor
  (back, forward, reload, path, port, open in a new tab), through
  KubeBench's proxy and the API server, sandboxed and isolated from
  KubeBench. Operators and admins only; read-only clusters allow reading
  (no form posts, no WebSockets). `KUBEBENCH_PROXY_ORIGIN` serves the pages
  on a separate origin on another domain, where apps with sign-ins work too.

### Changed

- Dependencies: Kubernetes client libraries 0.37, Helm 3.22, React 19.3,
  TypeScript 7, Vite 8.3, monaco-editor 0.57, SQLite driver 1.62; images
  build with Go 1.27 and Node 26.
- The repository is now `github.com/Shavell/kubebench` and the container
  image `ghcr.io/shavell/kubebench`; the installer, the update check, the
  chart and the docs use the new names.

## [0.5.0] - 2026-11-15

Not published separately; first released as part of 0.6.0.

Upgrading from 0.4 or the 0.1 chart: read **Changed** first (chart
resource names, the forced password change, the moved Helm routes).

### Added

- Per-cluster roles: users get `viewer` or `operator` on each assigned
  cluster (admins keep full access). Clusters can be flagged read-only,
  which blocks every change and shell for everyone, and tagged with an
  environment (development, staging, production) shown in the status bar;
  on production clusters destructive actions ask for the name.
- Audit log of all changes, shells and admin actions, refused attempts
  included, with filters and automatic pruning
  (`KUBEBENCH_AUDIT_RETENTION_DAYS`).
- Plugin platform: backend plugins with per-cluster availability checks and
  their own storage, a frontend plugin SDK, an Extensions view to enable or
  disable plugins, and runtime-loaded external UI plugins
  (`window.KubeBench`, example in `examples/plugins/hello-cluster`).
- Argo CD plugin: applications by project with sync and health, resource
  tree, sync with prune, dry run, revision and selected resources, refresh
  and hard refresh, rollback, delete with or without cascade, projects and
  ApplicationSets, through the Argo CD CRDs.
- Storage plugin: overview, PVC, PV and StorageClass wizards with
  validation, YAML preview and dry run, binding to an available PV, cloning
  and restoring from snapshots, and PVC expansion.
- Security plugin: posture scan with about 25 checks (Pod Security
  Standards, workload hygiene, RBAC, NetworkPolicies, secrets in
  environment variables, namespaces), a score per category, accepted risks
  shared by every user of the cluster, and Markdown and CSV export.
- Access plugin: RBAC "who can", effective permissions of a subject with
  the binding path, risky roles, a binding graph and SubjectAccessReview
  checks.
- Topology plugin: a namespace graph of Ingresses, Services, Pods, their
  owners, volumes and ConfigMap/Secret references, with a focus on one
  object ("Show topology" on workloads, Services and Ingresses).
- English, Turkish and German user interface with per-plugin translations;
  further languages are added as data.
- Light, dark and system themes.
- Real terminal sessions (WebSocket, xterm.js) with several sessions and
  resizing; streaming logs with follow, tail size, previous container,
  timestamps, filtering, download and multi-Pod selection by label.
- YAML editing with diff review, server-side dry run and apply; resource
  creation from templates and imported files; generic delete with a
  propagation choice; label and annotation editing; Secret values revealed
  on demand (shell permission, audited). Tabs with unsaved changes ask
  before they close.
- Node cordon, drain and taints; Deployment rollout history, rollback and
  pause; restart and scale for StatefulSets, DaemonSets and ReplicaSets;
  force and multi-select bulk delete of Pods; debug containers; CronJob
  trigger and suspend; namespace create and delete, which refuses system
  namespaces and KubeBench's own (`KUBEBENCH_NAMESPACE`) unless forced.
- Helm: install and upgrade with values editing and dry-run diffs, values
  diff between revisions, chart repositories, search, OCI charts, and chart
  upload for offline clusters.
- Live updates pushed from shared informers over Server-Sent Events, with a
  Live / Polling indicator in the status bar and a setting to turn them off.
- Command palette with kubectl-style commands (`get`, `describe`, `logs`,
  `exec`, `ns`, `ctx`, with `-n`/`-A` and Tab completion) and user aliases;
  keyboard shortcuts with a cheat sheet (`?`) and a setting to turn them
  off.
- Events view sorted newest first with a Warning filter and grouping by
  reason; a recent warnings card on the cluster overview; Copy name and Copy
  YAML in row menus; namespace favourites and hidden namespaces in the
  namespace picker.
- Keyboard navigation of tables, and windowed rendering of lists above 300
  rows.
- Cloud providers: KubeBench detects EKS, AKS, GKE, OpenShift, DigitalOcean,
  Linode, OVH, k3s and RKE2 clusters, shows the provider and version, and
  warns when the Kubernetes version leaves the provider's standard support.
  Kubeconfigs from `aws eks`, `gcloud` and `az aks` work without exec
  plugins: KubeBench gets the tokens itself, with per-cluster credentials
  (AWS access key or profile, GCP service-account key, Azure service
  principal) or, when `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY` is on (or the
  kubeconfig was added by path), the server's own cloud identity.
- Distribution: multi-arch container image `ghcr.io/shavell/kubebench`,
  Helm chart on `oci://ghcr.io/shavell/charts`, plain manifests
  (`deploy/kubernetes/kubebench.yaml`), `.deb` and `.rpm` packages with a
  hardened systemd unit, release archives for Linux, macOS and Windows, the
  `install.sh` installer (online and offline), offline bundles, a Makefile,
  GitHub Actions workflows (manual for now) and Dependabot.
- `kubebench --version`, `kubebench healthcheck`, `GET /api/version`.
- Update checks: administrators see new GitHub releases in the status bar
  with their notes and the upgrade steps for each install method; the mode
  (`off`, `stable`, `prerelease`) is set in Settings or with
  `KUBEBENCH_UPDATE_MODE`, and air-gapped installs turn it off. After an
  upgrade every user sees What's new once; Help: What's New shows the whole
  changelog.
- Documentation: deployment, configuration, security, architecture,
  plugin development, HTTP API and translation guides, CONTRIBUTING and
  SECURITY.

### Changed

- Live mode never shows mock data: a missing capability is reported as
  unavailable with its reason, and a failing Prometheus query is reported
  instead of being replaced by other data. Mock data is only used in
  development and demo builds (`?demo`).
- Server configuration is read and validated once from the environment
  (`PORT`, `KUBEBENCH_*`); an invalid value stops the start with every
  problem listed. See `docs/configuration.md`.
- The bootstrap `admin` password, a configured one included, must be
  changed at the first sign-in; so must passwords an admin sets for others.
- Helm chart 0.5.0: image `ghcr.io/shavell/kubebench`; a random admin
  password kept across upgrades instead of `admin`; RBAC presets
  (`cluster-admin`, `read-only`, `custom`); restricted Pod Security settings
  with a read-only root filesystem; startup probe; optional NetworkPolicy;
  `config.*`, `extraEnv` and `extraVolumes` values; a values schema. Resource
  names follow the usual Helm convention, so the release `kubebench` now
  creates `kubebench` instead of `kubebench-kubebench`: when upgrading a 0.1
  release, keep its data with
  `--set persistence.existingClaim=<release>-kubebench-data`.
- The container image runs on `distroless/static-debian13` and has a
  `HEALTHCHECK`.
- Helm API routes moved under `/api/plugins/helm`; reading a release's
  values now needs the shell permission (values often hold credentials).
- The YAML editor applies an edit as a merge patch of the changes, so
  fields removed in the editor are removed from the object.
- The Monaco editor is bundled instead of loaded from a CDN, so offline
  installs work; plugin pages load on first use, which halves the initial
  download.

### Removed

- The `/api/helm/*` routes (replaced by `/api/plugins/helm/*`).
- The legacy Node proxy under `server/` and its npm script: it was never
  shipped, and was an unauthenticated cluster proxy.

### Fixed

- A deleted cluster's runtime can no longer be rebuilt by a request that
  raced the delete, editing a cluster stops its background work on the old
  record at once, and removing an unreachable cluster no longer stalls
  requests for the other clusters.
- An unreadable `KUBEBENCH_TLS_CERT`/`KUBEBENCH_TLS_KEY` is a configuration
  error (exit status 2) found before anything starts.
- The release archives, packages, image, offline bundle and chart carry
  the GPL-3.0 license text.
- Events of one object no longer include events of other kinds with the same
  name.
- Deleting a user or cluster removes its sessions and assignments.
- Helm releases with the same name in two namespaces are kept apart;
  rollback and uninstall act on the selected one.
- A `KUBECONFIG` path list (`a:b`) seeds the inventory the way kubectl
  merges it.
- Views no longer show a previous query's data for new inputs, and
  sidebars and charts say why they are empty.

### Security

- Signing in with the default password `admin` now forces a password
  change, also for accounts upgraded from 0.4.
- Login rate limiting, an Origin check on state-changing requests and
  WebSocket upgrades, `Secure` session cookies behind TLS, and
  `KUBEBENCH_ALLOW_PATH_KUBECONFIG` (off in clusters) for adding clusters by
  server-side path.
- Pasted kubeconfigs may no longer run exec plugins or auth providers or
  read files on the server unless `KUBEBENCH_ALLOW_KUBECONFIG_EXEC=true`;
  kubeconfigs added by path still may, unless the path lies in a directory
  KubeBench fills from requests (the Helm cache, plugin data, external
  plugins). The EKS, GKE and AKS sign-ins are handled in process instead;
  pasted ones need per-cluster credentials unless
  `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY=true` lends them the server's own
  cloud identity (whose token goes to the API server the kubeconfig
  names). GCP credential files that could read files, call URLs or run
  commands are refused.
- Secret values no longer leak to viewers through kubectl's
  `last-applied-configuration` annotation or through Helm release
  manifests, dry runs and upgrade diffs; Secret dry runs need the same
  permission as the change and are audited.
- The `/api/k8s` passthrough and the cluster clients behind the other
  endpoints no longer follow redirects, which would send the stored
  credential elsewhere, and the passthrough no longer serves the cluster's
  content type from KubeBench's origin. Helm's clients are pinned to the
  API server's host, and OCI chart downloads use HTTPS only, need write
  access and are POST requests (`POST /api/plugins/helm/charts/oci/values`),
  so that the Origin check covers them; GETs of write and exec routes sent
  from another site are refused.
- Terminals, log streams and live-update streams end when the session, the
  cluster assignment or the permission they need goes away, and when the
  server shuts down (terminals are hung up, so their shells do not keep
  running in the container); terminals may only start interactive shells;
  per-user stream limits apply.
- Every change is attributed to the `kubebench` field manager, and every
  Pod of a bulk delete is recorded in the audit log.
- Request bodies must arrive within a minute; the database is created
  readable by its owner only; audit rows have a size limit, requests too
  long to be recorded completely are refused, and long lists (cluster
  assignments, labels, taints) are recorded across several rows.
- The chart's NetworkPolicy can restrict who reaches the pod
  (`networkPolicy.ingressFrom`), as needed behind an Ingress. Rendered
  offline (`helm template`, GitOps) without `networkPolicy.apiServer.cidrs`
  it fails instead of allowing every destination on the API server ports.
- Plugin routes refuse a misspelt `dryRun` (anything but `true` or
  `false`) instead of making the change for real.
- The security report export escapes Markdown and HTML, and CSV cells that
  a spreadsheet would run as formulas.

## [0.4.0] - 2026-08-14

### Added

- Right-click menus on table rows: details, logs, shell, YAML, delete for
  Pods; details, restart, scale and YAML for Deployments; YAML for other
  resources.
- Live Pod logs and shell in the bottom panel, targeted from the row menus.

## [0.3.0] - 2026-08-14

### Added

- The remaining built-in resource types: ResourceQuotas, LimitRanges,
  HorizontalPodAutoscalers, PodDisruptionBudgets, PriorityClasses,
  RuntimeClasses, Leases, mutating and validating webhook configurations,
  IngressClasses, EndpointSlices and ReplicationControllers.

### Changed

- Resource types that a cluster does not serve show an empty list instead of
  an error.

## [0.2.0] - 2026-08-14

### Added

- Generic resource browser driven by a resource catalog: ConfigMaps, Secrets
  (values redacted by the server), Ingresses, StorageClasses, RBAC objects
  and more, with list, detail, events and YAML views, grouped into Cluster,
  Workloads, Config, Network, Storage and Access Control.

## [0.1.0] - 2026-08-13

### Added

- VSCode-style workbench: activity bar, resizable sidebar, tabbed editor,
  bottom panel (problems, logs, terminal), status bar, command palette and
  context menus.
- Plugin modules for workloads, CRDs, Helm, metrics, cluster inventory,
  settings and users, on typed data providers with a deterministic mock
  cluster.
- Rich Pod, Node and Deployment pages: utilization charts, events, logs,
  exec, delete, scale and restart.
- Helm releases with history, values, rollback and uninstall.
- Metrics from metrics-server or an auto-discovered Prometheus: cluster
  overview with allocation (requests, limits, usage) and per-node pages.
- List filters (name, namespace, Pod status).
- Go backend: a single binary with the frontend embedded, client-go and the
  Helm SDK, SQLite storage, users with sessions, per-user cluster assignment
  and active cluster.
- Container image and Helm chart for running KubeBench inside the cluster it
  manages.

[Unreleased]: https://github.com/Shavell/kubebench/compare/v0.8.1...HEAD
[0.8.1]: https://github.com/Shavell/kubebench/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/Shavell/kubebench/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/Shavell/kubebench/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/Shavell/kubebench/compare/acc8149...v0.6.0
[0.5.0]: https://github.com/Shavell/kubebench/compare/acc8149...v0.6.0
[0.4.0]: https://github.com/Shavell/kubebench/commit/acc8149
[0.3.0]: https://github.com/Shavell/kubebench/commit/6bec8c1
[0.2.0]: https://github.com/Shavell/kubebench/commit/b382f96
[0.1.0]: https://github.com/Shavell/kubebench/commit/a0129ab
