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

## [0.5.0] - Unreleased

### Added

- Per-cluster roles: users get `viewer` or `operator` on each assigned
  cluster (admins keep full access). Clusters can be flagged read-only, which
  blocks every change and shell for everyone, and tagged with an environment
  (development, staging, production) shown in the status bar.
- Audit log of all changes, shells and admin actions, with filters and
  automatic pruning (`KUBEBENCH_AUDIT_RETENTION_DAYS`).
- Plugin platform: backend plugins with per-cluster availability checks and
  their own storage, a frontend plugin SDK, an Extensions view to enable or
  disable plugins, and runtime-loaded external plugins.
- New plugins: Argo CD (applications, sync, refresh, rollback), Storage (PVC,
  PV and StorageClass wizards, PVC expansion), Security (posture checks with
  accepted risks and exports), Access (RBAC "who can", effective permissions)
  and Topology (Ingress to Service to Pod graph).
- English and Turkish user interface, with per-plugin translations.
- Light, dark and system themes.
- Real terminal sessions (WebSocket, xterm.js), streaming logs with follow,
  previous container and multi-pod selection.
- YAML editing with diff, server-side dry run and apply; resource creation
  from templates; generic delete; label and annotation editing; Secret
  values revealed on demand (shell permission, audited). Tabs with unsaved
  changes ask before they close.
- Node cordon, drain and taints; Deployment rollout history, rollback and
  pause; restart and scale for StatefulSets, DaemonSets and ReplicaSets;
  force and multi-select bulk delete of Pods; debug containers; CronJob
  trigger and suspend; namespace create and delete, which refuses system
  namespaces and KubeBench's own (`KUBEBENCH_NAMESPACE`) unless forced.
- Helm: install and upgrade with values editing and dry-run diffs, chart
  repositories, search, and chart upload for offline clusters.
- Live updates pushed from shared informers.
- Distribution: multi-arch container image `ghcr.io/shavell/kube-bench`, Helm
  chart on `oci://ghcr.io/shavell/charts`, plain manifests
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

### Changed

- Live mode never shows mock data: a missing capability is reported as
  unavailable with its reason. Mock data is only used in development and
  demo builds.
- Server configuration is read and validated once from the environment
  (`PORT`, `KUBEBENCH_*`); see `docs/deployment.md`.
- The default `admin` password must be changed at the first sign-in.
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
- Helm API routes moved under `/api/plugins/helm`.

### Removed

- The `/api/helm/*` routes (replaced by `/api/plugins/helm/*`).

### Fixed

- Events of one object no longer include events of other kinds with the same
  name.
- Deleting a user or cluster removes its sessions and assignments.

### Security

- Login rate limiting, an Origin check on state-changing requests, `Secure`
  session cookies behind TLS, and `KUBEBENCH_ALLOW_PATH_KUBECONFIG` (off in
  clusters) for adding clusters by server-side path.
- Pasted kubeconfigs may no longer run exec plugins or auth providers or
  read files on the server unless `KUBEBENCH_ALLOW_KUBECONFIG_EXEC=true`;
  kubeconfigs added by path still may.
- Secret values no longer leak to viewers through kubectl's
  `last-applied-configuration` annotation.
- The `/api/k8s` passthrough and the cluster clients behind the other
  endpoints no longer follow redirects, which would send the stored
  credential elsewhere, and the passthrough no longer serves the cluster's
  content type from KubeBench's origin.
- Request bodies must arrive within a minute; the database is created
  readable by its owner only; audit rows have a size limit and requests
  too long to be recorded completely are refused.
- The chart's NetworkPolicy can restrict who reaches the pod
  (`networkPolicy.ingressFrom`), as needed behind an Ingress.

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
