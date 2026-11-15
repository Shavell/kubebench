# Notes: KubeBench vs KubelizeMe

*Türkçe: [kubelizeme-comparison.tr.md](kubelizeme-comparison.tr.md)*

These notes record how KubeBench differs from **KubelizeMe**, a desktop
Kubernetes IDE that we used as a reference while planning v0.5. They
explain which ideas we adopted, which we deliberately did differently, and
what is still open.

> **Method.** We only had KubelizeMe's built frontend bundle (a Tauri app:
> minified Vite chunks, no source code). Its feature inventory comes from
> the ~220 Tauri command names (`list_pods`, `helm_upgrade_dry_run`,
> `run_security_audit`, …) and the UI strings in each chunk. Behavioural
> details are therefore inferred. No code, assets or text from KubelizeMe
> are reused in KubeBench.

> **Status.** The v0.5 column below was reconciled with what shipped in
> 0.5.0 (checked against the code on 2026-11-15). What is still missing is
> listed in §4.

## 1. Positioning

| | KubelizeMe | KubeBench |
|---|---|---|
| Form factor | Desktop app (Tauri), one user per machine | Web app: one Go binary with the UI embedded. Runs in a cluster, on a VM, or locally |
| Credentials | Local kubeconfig files, contexts, service-account tokens, WSL kubeconfigs | Stored server-side. In-cluster ServiceAccount, uploaded kubeconfigs, or `KUBECONFIG` on the host |
| Users | Single user | Multi-user: sessions, global roles, per-cluster roles (viewer/operator), read-only clusters, audit log *(v0.5)* |
| Clusters at once | Several contexts open simultaneously, colour groups | One active cluster per user; switching reloads the UI |
| Local tooling | Shells out to `helm`/`kubectl`, has a toolchain probe and binary path overrides | Native SDKs only (client-go, Helm v3 SDK); nothing needed on the host |
| Distribution | Desktop installers + auto-updater | Container image, Helm chart, plain manifests, `.deb`/`.rpm`, tarballs, `curl \| bash` installer, offline bundle; an update check that shows new releases and their upgrade steps but installs nothing *(v0.5)* |
| Extensibility | Closed app; extends the AI side through MCP servers | Plugin platform: built-in backend+frontend plugins, admin enable/disable, runtime-loaded external UI plugins *(v0.5)* |
| Languages | English | English, Turkish and German, per-plugin translation namespaces; more languages are data only *(v0.5)* |

## 2. Feature comparison

Legend: ✓ available · ◐ partial · — missing · **v0.5** added in this release.

### Resources & editing

| Area | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| Built-in resource types (workloads, config, network, storage, RBAC, webhooks, quotas, HPA/PDB, …) | ✓ | ✓ | ✓ |
| CRDs and custom resources | ✓ | ✓ | ✓ |
| Gateway API / Traefik routes | ✓ (`list_network_routes`) | via CRD browser | via CRD browser |
| YAML view | ✓ | ✓ (read-only) | ✓ |
| Edit + apply, server-side dry-run, diff against the live object | ✓ | — | **v0.5** |
| "Neat" YAML (strip server-managed fields) | ✓ | — | **v0.5** |
| Create from YAML / import files | ✓ (Composer, import) | — | **v0.5** |
| Generic delete with cascade explanation, conflict detection | ✓ | pods only | **v0.5** |
| Edit labels/annotations | ✓ | — | **v0.5** |
| Reveal Secret values (with confirmation) | ✓ | — (always redacted) | **v0.5** (exec permission, audited) |

### Operations

| Area | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| Scale/restart Deployments | ✓ | ✓ | ✓ |
| Restart/scale StatefulSets, DaemonSets | ✓ | — | **v0.5** |
| Rollout history, undo, pause/resume | ✓ | — | **v0.5** |
| Node cordon / drain / taints | ✓ | — | **v0.5** |
| Force delete, bulk delete pods | ✓ | — | **v0.5** |
| Debug (ephemeral) container | ✓ | — | **v0.5** |
| CronJob trigger / suspend | — (not seen) | — | **v0.5** |
| Namespace create/delete | ✓ | — | **v0.5** |
| Port-forward | ✓ (local ports) | — | — (roadmap: a backend HTTP proxy fits a web app better than local ports) |

### Terminal & logs

| Area | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| Interactive shell (PTY, resize, multiple sessions) | ✓ (xterm, search, prompt marks) | ◐ one-shot `sh -c`, 20 s timeout | **v0.5** (WebSocket PTY + xterm.js) |
| Local terminal with the cluster kubeconfig | ✓ | — | — (a web app has no local shell; by design) |
| Streaming logs: follow, tail size, previous container, timestamps | ✓ | ◐ 300-line snapshot | **v0.5** |
| Multi-pod logs by label selector | ✓ | — | **v0.5** |
| Save logs to file | ✓ | — | **v0.5** (download) |

### Helm & GitOps

| Area | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| Releases, history, values, rollback, uninstall | ✓ | ✓ | ✓ |
| Install / upgrade with dry-run | ✓ | — | **v0.5** |
| Repositories add/remove/update, chart search | ✓ (local `helm` CLI) | — | **v0.5** (SDK; repo state kept server-side) |
| Values diff between revisions | ✓ | — | **v0.5** |
| Install from an uploaded chart (offline) | — | — | **v0.5** |
| Argo CD applications, sync, refresh, rollback | — | — | **v0.5** (Argo CD plugin, works through the CRDs) |

### Storage

| Area | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| PV / PVC / StorageClass lists | ✓ | ✓ | ✓ |
| PVC / PV / StorageClass wizards with validation + dry-run | ◐ (inside Composer) | — | **v0.5** (Storage plugin) |
| Expand PVC | — | — | **v0.5** |

### Security & access

| Area | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| Security posture: CIS (section 5), NSA/CISA, Pod Security Standards | ✓ | — | **v0.5** (Security plugin; CIS section 5 + PSS mapping) |
| Accepted risks | ✓ in an in-cluster CRD ("findings ledger") | — | **v0.5**, kept in KubeBench's database and shared by every user of the cluster, no CRD needed |
| Report export (CSV/Markdown/PDF) | ✓ | — | **v0.5** (CSV/Markdown) |
| RBAC "who can …", subject permissions | ✓ | — | **v0.5** (Access plugin; also SubjectAccessReview checks) |
| Admission policies / webhooks view | ✓ | ◐ (webhooks list) | ◐ |
| Per-context safety / read-only mode | ✓ (local setting) | — | **v0.5**: read-only flag per cluster plus viewer/operator roles, enforced server-side |
| Audit trail of actions | — (single user) | — | **v0.5** |

### Observability

| Area | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| CPU/memory from metrics-server | ✓ | ✓ | ✓ |
| Prometheus discovery, history, network and filesystem series | — ("metrics-server not available" only) | ✓ | ✓ |
| Requests/limits vs allocatable, overcommit marker | — | ✓ | ✓ |
| Topology graph (Ingress → Service → workload → Pod) | ✓ (dagre) | — | **v0.5** (Topology plugin) |
| Live updates via watches | ✓ (watch cache) | — (manual or interval refresh) | **v0.5** (shared informers + SSE) |
| Cluster dashboard with recent warnings | ✓ | ◐ (metrics overview) | **v0.5** (recent warnings card, Events view with Warning filter and grouping by reason) |

### Workspace & UX

| Area | KubelizeMe | KubeBench ≤0.4 | KubeBench v0.5 |
|---|---|---|---|
| Command palette | ✓ (+ `get`/`describe`, aliases) | ✓ | **v0.5** kubectl-style `get`/`describe`/`logs`/`exec`/`ns`/`ctx` with completion and aliases |
| Keyboard shortcuts and table keyboard navigation | ✓ | ◐ (palette only) | **v0.5** (configurable, `?` cheat sheet) |
| Namespace pin/hide | ✓ | — | **v0.5** |
| Light/dark themes, fonts, interface scale | ✓ | dark only | ◐ **v0.5** light/dark/system and terminal font size; no interface scale |
| Multi-language UI | — | — | **v0.5** (en, tr, de) |
| Cloud provider detection and Kubernetes version support (EKS, AKS, GKE, …) | ✓ | — | **v0.5** (provider badges, end-of-life notice) |
| EKS/GKE/AKS sign-in without the clouds' CLIs | — (not seen) | — | **v0.5** (in process, no CLIs; per-cluster credentials or the server's identity) |
| In-app updates | ✓ (auto-updater) | — | **v0.5** update check + What's new (installs nothing, by design) |
| AI assistant (multi-provider, tool calls, MCP, playbooks) | ✓ | — | — (roadmap) |
| Manifest Composer (form wizards, templates, multi-cluster apply) | ✓ | — | ◐ (Storage wizards + YAML templates) |
| Cluster-to-cluster compare | ✓ | — | — (roadmap) |

## 3. Deliberate differences

- **Server-side custody of credentials.** In a web tool the browser never
  sees cluster credentials. Every Kubernetes call goes through the backend,
  which applies KubeBench roles before the cluster's own RBAC. This is why
  read-only mode and roles are enforced on the server, not stored as a UI
  preference.
- **Shared state instead of CRDs.** KubelizeMe installs a `SecurityFinding`
  CRD so several desktops can share accepted risks. KubeBench already has a
  multi-user database, so shared state (accepted risks, Helm repositories,
  plugin settings) lives there and the managed cluster stays untouched.
- **No host binaries.** Helm and drain run through the Go SDKs. The
  container stays distroless and air-gapped installs only need the image.
- **Plugins instead of a monolith.** Argo CD, Storage, Security, Access,
  Topology and Helm are plugins. An admin can disable them, they report
  why they are unavailable (for example "Argo CD CRDs not found"), and
  third parties can ship UI plugins without forking.
- **Honest data.** Live mode never falls back to sample data. A missing
  capability is shown as unavailable together with the reason.

## 4. Still missing after v0.5 (roadmap)

1. AI assistant: server-side, read-only tools by default, every action audited.
2. Manifest Composer: multi-resource form wizards beyond storage, with
   multi-cluster apply.
3. Cluster compare (A/B snapshot diff with neat).
4. Port-forward equivalent through a backend HTTP proxy.
5. Several active clusters per user (side-by-side views).
6. Admission policy views (ValidatingAdmissionPolicy, Kyverno, Gatekeeper)
   beyond the webhook lists.
7. NSA/CISA mapping and PDF export for security reports.
8. Interface scale and font settings.
9. Optional OIDC/SSO login and Kubernetes impersonation per user.
10. An APT/YUM repository for VM upgrades.
