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

- Service browser: "Open in browser" on Services and Pods shows the
  workload's web interface in a browser tab to the right of the editor
  (back, forward, reload, path, port, open in a new tab), through
  KubeBench's proxy and the API server, sandboxed and isolated from
  KubeBench. Operators and admins only; read-only clusters allow reading.
  `KUBEBENCH_PROXY_ORIGIN` serves the pages on a separate origin, where apps
  with sign-ins work too.

## [0.5.0] - Unreleased

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
- Distribution: multi-arch container image `ghcr.io/shavell/kube-bench`,
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
- Helm chart 0.5.0: image `ghcr.io/shavell/kube-bench`; a random admin
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

[Unreleased]: https://github.com/Shavell/kube-bench/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/Shavell/kube-bench/compare/acc8149...v0.5.0
[0.4.0]: https://github.com/Shavell/kube-bench/commit/acc8149
[0.3.0]: https://github.com/Shavell/kube-bench/commit/6bec8c1
[0.2.0]: https://github.com/Shavell/kube-bench/commit/b382f96
[0.1.0]: https://github.com/Shavell/kube-bench/commit/a0129ab
