# v0.5 feature specs (inputs for workflow W2/W3)

Each spec is the `spec` text handed to one implementation agent through
`args.features` of [`w2-features.md`](w2-features.md). They extend the
contract in [`../../design-v0.5.md`](../../design-v0.5.md) §7. Ports are
there so agents running in parallel never collide. Each agent runs its
backend on `port` and a Vite dev server on `port + 1000`.

**Test cluster rule on the new VM.** Mutation paths may be exercised
against a disposable local cluster (k3d/k3s/kind), using its own
kubeconfig file. The developer's homelab (`k3s-homelab`) stays read-only.
See [`../v0.5-handoff.md`](../v0.5-handoff.md) §6.

| Wave | id | title | port |
|---|---|---|---|
| W2a | `terminal` | Terminal & logs (WebSocket PTY, SSE logs) | 19881 |
| W2a | `resource-editing` | YAML apply / dry-run / diff / delete / labels / secret reveal | 19882 |
| W2a | `workload-ops` | Node, rollout, workload, pod, CronJob, namespace operations | 19883 |
| W2b | `helm` | Helm plugin v2 | 19884 |
| W2b | `argocd` | Argo CD plugin | 19885 |
| W2b | `storage` | Storage plugin (PV/PVC/StorageClass creator) | 19886 |
| W2c | `security` | Security posture plugin | 19887 |
| W2c | `access` | Access (RBAC) plugin | 19888 |
| W2c | `topology` | Topology plugin (frontend only) | 19889 |
| W3 | `live-updates` | Shared informers + SSE invalidation | 19890 |
| W3 | `ux-polish` | Namespace pins, kubectl-like palette, shortcuts, table virtualization | 19891 |
| W3 | `updates` | GitHub release check, update mode, What's new | 19899 |
| W3 | `cloud` | EKS, AKS, GKE: detection, version support, in-process credentials | 19900 |
| v0.6 | `notes` | Personal and shared Markdown notes (backend + frontend plugin) | 19920 |
| v0.6 | `service-browser` | Service browser: API server proxy, isolated frame, proxy origin | 19921 |

---

## F1 `terminal` — Terminal & logs (§7.1)

Backend:
- `GET /api/exec/ws` (access **exec**; audited once at session start as
  action `exec.session`, target `pod/<ns>/<pod>/<container>`). Upgrade with
  `github.com/gorilla/websocket` (promote it to a direct dependency). The
  same-origin check is strict: the Origin host must equal the request Host,
  and a missing Origin is allowed only for non-browser clients. Use the
  §7.1 protocol exactly.
- Start the exec with client-go `remotecommand` (TTY + stdin + stdout).
  Use the WebSocket executor with SPDY fallback
  (`remotecommand.NewFallbackExecutor(NewWebSocketExecutor, NewSPDYExecutor, httpstream.IsUpgradeFailure || httpstream.IsHTTPSProxyError)`).
  Feed resize messages into a `TerminalSizeQueue`.
- Default command: `["/bin/sh","-c","command -v bash >/dev/null 2>&1 && exec bash || exec sh"]`.
  The optional `command` param must be on an allowlist of shells.
- Limits: 64 KiB per message, ping every 30 s, 30 min idle timeout.
  Closing either side closes the other. Report the exit code with
  `{"type":"exit","code":N}`.
- Put the exec plumbing behind an executor-factory interface. Then a test
  can inject a fake that echoes stdin and honours resize. httptest +
  websocket client tests cover:
  - role gating: viewer → 403 before the upgrade; read-only cluster → 403;
  - origin rejection;
  - input/output round trip;
  - resize;
  - the exit message.
- `GET /api/logs/stream` (access **read**). Server-Sent Events.
  - Params: `ns`, `pod`, `container`, `tail` (default 500, max 10000, 0 = all), `previous`, `timestamps`, `sinceSeconds`.
  - Alternatively `selector`: at most 10 pods. Each line is prefixed `[pod/container] `. `container` is optional and defaults to the first container.
  - Headers: `text/event-stream`, `no-cache`, `X-Accel-Buffering: no`.
  - One `data:` line per log line. Strip `\r` and handle long lines.
  - Send a `: ping` heartbeat every 15 s, `event: error` on failure, `event: end` when the stream finishes.
  - Stop when the client disconnects.
  - Tests use the fake clientset (fixed GetLogs body) and cover framing, selector fan-out and the error event.
- Keep the one-shot `GET /api/logs` and `POST /api/exec` for compatibility.

Frontend (terminal plugin; touch the workloads plugin only where it opens logs or a shell):
- Add `@xterm/xterm` and `@xterm/addon-fit`, lazy-loaded so the main chunk stays small, and include the xterm CSS.
- `TerminalView`:
  - connects to `ws(s)://<host>/api/exec/ws?...`;
  - writes binary output with `term.write(Uint8Array)` and sends input and resize messages;
  - shows the connection status with the reason;
  - has Reconnect and a container picker;
  - takes its font size from settings and its colors from tokens (both themes);
  - supports copy and paste.
- Bottom-panel Terminal: several sessions as sub-tabs, a new-session button and close. The `pod-exec` tab kind keeps working.
- Mock mode: a fake shell in xterm (prompt, echo, canned commands through the existing mock `execCommand`), clearly labelled "mock".
- `LogsView`:
  - reads from an EventSource;
  - toolbar: follow/pause, tail size (100/500/1000/5000/all), previous container, timestamps, wrap, filter, level colouring, download `.log`, clear;
  - container picker, a 10 000-line ring buffer, auto-scroll only while at the bottom, reconnect with backoff.
  - Mock mode: a deterministic simulated stream.
- Multi-pod logs: new tab kind `workload-logs` (`{namespace, selector, title}`). It opens from Deployment, StatefulSet, DaemonSet, ReplicaSet and Job detail pages and rows ("Logs (all pods)").
- Permissions: the shell needs `canExec`; logs need read.

Checks:
- Go tests above.
- Live read-only check: `curl -N` the SSE endpoint for a kube-system pod for about 5 s, and again with `selector=k8s-app=kube-dns`.
- A viewer must get 403 on `/api/exec/ws` with no Kubernetes call.
- Real exec sessions only against the disposable test cluster.
- Mock UI flows in the browser.

## F2 `resource-editing` — YAML apply, dry-run, diff, delete (§7.2)

Backend (new files, e.g. `kube/apply.go`, `kube/delete.go`, `kube/neat.go`, `api/apply.go`):
- `POST /api/resource/apply?dryRun=&force=&ns=`.
  - Body: YAML or JSON, multi-document (k8s `yaml.NewYAMLOrJSONDecoder`). Skip empty docs; max body 4 MiB.
  - Resolve the GVR through `Runtime.Mapper()`/`ResolveKind`.
  - Namespaced kinds without a namespace get `ns` or `default`. Cluster-scoped kinds must not carry a namespace: strip it and return a warning.
  - Server-side apply with `fieldManager=kubebench`, `force` and `DryRun=[All]`.
  - Each result gets an action: `created`, `configured` or `unchanged`.
  - Errors are reported per document. An SSA conflict is reported with code `conflict` so the UI can offer Force.
  - Secret docs that contain the redaction marker are rejected (`redacted_secret`).
  - Access is write; dry-run needs read and is not audited.
- `GET /api/yaml` gains `apiVersion` and `neat=true`. Neat is a pure function with table tests. It strips status, `managedFields`, uid, resourceVersion, generation, creationTimestamp, selfLink, and the `last-applied-configuration` and `deployment.kubernetes.io/revision` annotations.
- `DELETE /api/resource?apiVersion=&kind=&ns=&name=&propagation=&gracePeriod=` (write; also accepts the catalog `key`).
- `PATCH /api/resource/metadata` with `{labels, annotations}` as a JSON merge patch (null deletes) (write).
- `GET /api/resource/get?key=secrets&…&reveal=true`: exec access, audited, returns the raw object.
- Tests use the dynamic fake plus `meta.NewDefaultRESTMapper`. Cover:
  - multi-doc apply;
  - dry-run passes `DryRun`;
  - namespace defaulting and cluster-scoped handling;
  - redacted secret rejection and the conflict path;
  - delete propagation, metadata patch and neat.

Frontend:
- Extend the core `ClusterProvider`: apply, delete, patchMetadata, getYaml(neat), revealSecret. Both mock and live. The mock keeps an in-memory store, so created and edited objects show up in mock lists.
- YAML tab (`yaml`): stays backwards compatible and takes an optional `ref` `{apiVersion, kind, namespace, name}`. The editing flow:
  - Neat, Copy, Download;
  - Edit → Review diff (DiffEditor) → Dry run (results per document) → Apply → refresh + notify;
  - a conflict offers "Force apply";
  - an unsaved-changes guard.
- "New resource": template picker (Deployment, Service, ConfigMap, Secret, Ingress, Job, CronJob, Namespace, PVC, ServiceAccount, Role/RoleBinding). It uses the active namespace, can import from a file or drag & drop (multi-doc), and offers Dry run / Apply.
- Generic Delete on every detail page and row:
  - ConfirmDialog with a propagation choice;
  - explains the cascade (Deployment → ReplicaSets → Pods);
  - type-to-confirm on production clusters.
- Labels/annotations editor dialog (KeyValueEditor).
- Secret "Reveal values" (`canExec`, warning dialog): decoded values with copy buttons, never cached.

Checks: Go tests; read-only `GET /api/yaml?neat=true` against the homelab; real apply only against the disposable cluster; mock UI flows.

## F3 `workload-ops` — node and workload operations (§7.3)

Backend (new files, e.g. `kube/nodes.go`, `rollout.go`, `workloadops.go`, `cronjobs.go`, `namespaces.go`, `debug.go`):
- **Nodes:**
  - cordon/uncordon with a merge patch on `spec.unschedulable`;
  - drain through `k8s.io/kubectl/pkg/drain.Helper`. Capture per-pod results via `OnPodDeletionOrEvictionFinished` and return `{cordoned, evicted, failed:[{pod,error}], blockedByPDB}`;
  - taints PUT replaces the list (validate effect and key).
- **Deployments:**
  - history from owned ReplicaSets (owner UID, revision annotation, change-cause, images, created, replicas), current revision marked;
  - rollback copies that ReplicaSet's pod template (dropping `pod-template-hash`). Refuse when the Deployment is paused or the revision is current;
  - pause/resume.
- **Workloads:**
  - `POST /api/workloads/restart` (Deployment, StatefulSet, DaemonSet);
  - `POST /api/workloads/scale` (Deployment, StatefulSet, ReplicaSet through the scale subresource);
  - the old `/api/deployments/scale|restart` keep working.
- **Pods:**
  - force delete (grace 0);
  - `POST /api/pods/bulk-delete` (≤200, results per item);
  - `POST /api/pods/debug` (exec access) adds an ephemeral container through the `pods/ephemeralcontainers` subresource: name `debugger-xxxxx`, default image `busybox:1.37`, optional target container, stdin and tty on.
- **CronJobs:**
  - trigger creates a Job from `jobTemplate` named `<cj>-manual-xxxxx` (≤63 chars), with an ownerRef and the annotation `cronjob.kubernetes.io/instantiate=manual`;
  - suspend toggle.
- **Namespaces:**
  - create (DNS-1123 label);
  - delete refuses system namespaces and KubeBench's own namespace unless forced.
- Every operation is audited. Tests use the fake clientset for every operation, structured so the drain logic is testable.

Frontend (workloads plugin + core provider, mock and live):
- node cordon/uncordon, a drain dialog with options and results, a taints editor;
- rollout history with rollback, pause/resume;
- a scale dialog for StatefulSet and ReplicaSet; restart for StatefulSet and DaemonSet;
- multi-select bulk delete and force delete for pods, and a debug-container dialog that opens the `pod-exec` tab for the new container;
- CronJob trigger and suspend;
- namespace create, and delete with type-to-confirm;
- permission gating (write; debug needs exec), i18n, notify, `refreshData`.

Checks: Go tests with fakes; read-only `GET /api/deployments/history` against the homelab; viewer → 403 on cordon; real drain only on a worker of the disposable cluster, uncordoned afterwards.

## F4 `helm` — Helm plugin v2 (§7.4)

- Move `backend/internal/kube/helm.go` into the backend plugin `internal/plugins/helm`. Routes live under `/api/plugins/helm`; remove the old `/api/helm/*` routes.
- Helm environment:
  - `cli.EnvSettings` with `RepositoryConfig=$DATA/helm/repositories.yaml`, `RepositoryCache=$DATA/helm/cache` and a registry config under `$DATA/helm/`;
  - repo index download with `repo.NewChartRepository`;
  - OCI through `registry.NewClient`;
  - `action.Install`/`action.Upgrade` with `DryRun` (client-only render when the cluster is unreachable is not required);
  - chart upload: multipart `.tgz` ≤ 20 MiB, validated with `loader.LoadArchive` and stored under `$DATA/helm/uploads/<id>.tgz`, cleaned after 24 h.
- `Detect`: can list Secrets with `owner=helm`.
- Endpoints exactly as §7.4. Repos add/remove are admin. Install, upgrade, rollback, uninstall and repo update are write; dry-runs are read.
- UI:
  - release list and detail tabs: Overview, Values (user/all, per revision), Manifest, Notes, History (rollback, values diff between two revisions);
  - Upgrade (values editor, version picker, dry-run manifest diff);
  - install wizard (search → version → values from the chart defaults → dry-run → install, or upload `.tgz`);
  - repositories page;
  - uninstall with keep-history.
- Mock data extends `mockHelm`. Tests use Helm's `action.Configuration` with an in-memory storage driver (`storage.Init(driver.NewMemory())`) and `kubefake.PrintingKubeClient`.

## F5 `argocd` — Argo CD plugin (§7.5)

- Uses the CRDs only: `argoproj.io/v1alpha1` applications, appprojects, applicationsets.
- `Detect`: discovery serves `applications`.
- Sync = merge patch on `.operation`:
  `{"operation":{"initiatedBy":{"username":"kubebench:<user>"},"sync":{"revision":…,"prune":…,"dryRun":…,"resources":[…],"syncOptions":[…]}}}`.
  Return 409 when `status.operationState.phase` is `Running`.
- Refresh = annotation `argocd.argoproj.io/refresh: normal|hard`.
- Rollback = a sync to the history entry's revision and source. Return 409 when `spec.syncPolicy.automated` is set.
- Delete with cascade adds the finalizer `resources-finalizer.argocd.argoproj.io` before deleting.
- Summaries and details follow §7.5 and are mapped defensively, because fields may be missing.
- UI:
  - apps grouped by project, sync/health badges;
  - detail: resource tree that opens resources through tab kinds, conditions, history with rollback, operation state;
  - sync dialog (prune, dry-run, revision, selective resources);
  - refresh/hard refresh, delete (cascade, type-to-confirm);
  - projects and ApplicationSets views;
  - an Unavailable state when Argo CD is missing.
- Mock: deterministic apps (guestbook, helm-guestbook, kustomize-app) with mixed states. Tests use the dynamic fake.
- On the disposable cluster: apply the Argo CD CRDs only, plus sample Applications with a populated status (the Application CRD has no status subresource).

## F6 `storage` — PV/PVC/StorageClass creator (§7.6)

- Endpoints as §7.6. Server-side validation covers DNS-1123 names, the quantity regex, access modes (RWO/ROX/RWX/RWOP) and volume modes.
- Resize: only when the StorageClass sets `allowVolumeExpansion` and the new size is larger.
- `GET /snapshots` only when `snapshot.storage.k8s.io` is served.
- UI wizards build the object from the form, with a live read-only YAML preview, Validate (dry-run), Create, and opening the new resource afterwards.
  - PVC: storage class with a default marker or "none", access modes, size + unit, volume mode, bind to an Available PV, clone from a PVC or a snapshot, labels.
  - PV: capacity, access modes, reclaim policy, class, volume mode, and a source of hostPath, NFS, local (+ required node affinity picked from nodes) or CSI (driver from CSIDriver list, volumeHandle, fsType, attributes).
  - StorageClass: provisioner, parameters, reclaim policy, binding mode, expansion, default flag.
- Expand PVC on rows. Mock and live implementations, tests with the fake clientset.

## F7 `security` — Security posture (§7.7)

- A check engine over pods and workload templates, RBAC objects, namespaces and NetworkPolicies, about 25 checks. Each check has `id, title, severity, category, frameworks, remediation`.
  - **Pod Security Standards, baseline:** privileged, hostNetwork/PID/IPC, hostPath, hostPorts, capabilities beyond the baseline allowlist, procMount, unsafe sysctls, SELinux/AppArmor overrides.
  - **Pod Security Standards, restricted:** allowPrivilegeEscalation, runAsNonRoot, seccomp, drop ALL, restricted volume types.
  - **Hygiene:** requests/limits, probes, `:latest` or untagged images.
  - **RBAC (CIS 5.1.x):** cluster-admin bindings, wildcards, secrets access, pod create, default SA usage/automount, bind/escalate/impersonate.
  - **Network (5.3.2):** namespaces without a NetworkPolicy.
  - **Secrets in env (5.4.1).**
  - **Namespaces:** missing PSA labels, workloads in `default`.
- System namespaces are excluded by default.
- Score = weighted pass ratio per category.
- Accepted risks live in the plugin KV (`cluster/<id>/accepted/<key>`), with an optional expiry.
- Export as Markdown or CSV.
- UI: overview with small multiples and severity tiles, findings table with filters, grouping and accept/un-accept, checks catalog, export.
- Mock report. Table tests per check.

## F8 `access` — RBAC (§7.8)

- Evaluate RoleBindings, ClusterRoleBindings, Roles and ClusterRoles, using already-aggregated ClusterRole rules. Wildcard matching covers verbs, resources, apiGroups, resourceNames and nonResourceURLs.
- `who-can` returns the subjects together with their binding path. Subject view: effective rules grouped by namespace.
- `POST /review` sends a SubjectAccessReview (Read; needs `create` on SAR at the cluster level, otherwise it fails with a clear error).
- Graph data: subjects → bindings → roles.
- UI pages:
  - Who can…;
  - Subjects → detail with effective permissions and a quick can-i;
  - risky roles;
  - a 3-column binding graph (SVG).
- Mock data. Table tests for the matcher.

## F9 `topology` — Topology (frontend only, §7.9)

- Builds the graph from core providers and generic lists:
  - Ingress → Service by backend service name;
  - Service → Pods by selector;
  - Pods ← ReplicaSet/StatefulSet/DaemonSet by ownerReferences, ReplicaSet ← Deployment;
  - Pod → PVC → PV;
  - optional ConfigMap/Secret references.
- Layered SVG layout by kind, nodes coloured by status through tokens, pan/zoom, hover highlights edges, click opens the resource.
- Resource action "Show topology" for Deployment, StatefulSet, DaemonSet, Service and Ingress, focused on that object.
- Mock objects get consistent labels so the graph is meaningful.

## F10 `live-updates` (W3, §7.10)

- Per-runtime watch hub: dynamic shared informers started lazily per GVR on the first subscription. Stop them after 5 min without subscribers.
- `GET /api/watch?topics=` SSE: `event: change` with `{topic, namespace}`, debounced 500 ms, plus a 25 s heartbeat.
- Optionally serve the core typed lists (pods, deployments, services, nodes, namespaces, events) from the informer cache once synced. Fall back to direct calls when list/watch is forbidden.
- Frontend:
  - an `EventSource` manager;
  - scoped invalidation (`refreshData(topic?)`, queries declare topics);
  - a status-bar Live/Polling/Offline indicator;
  - a Settings toggle;
  - auto-refresh stays as the fallback.

## F11 `ux-polish` (W3)

- Namespace pin/hide: favourites in the namespace picker, persisted per cluster.
- kubectl-like palette commands: `get <kind> [-n ns]`, `describe <kind>/<name>`, `logs <pod>`, `exec <pod>`, `ns <name>`, `ctx <cluster>`, plus user-defined aliases in Settings.
- Keyboard shortcuts cheat sheet (`?`) and a list in Settings.
- Table virtualization above about 300 rows, and keyboard navigation (↑/↓/Enter).
- "Copy name" / "Copy YAML" actions.
- Events view: Warning-only filter and group by reason.
- A cluster dashboard card with recent warnings.

## F12 `updates` — Updates and What's new (§7.11)

Requested by the product owner: "the product can track through GitHub
whether it is up to date; Settings has an update mode; there is a What's new
screen".

Backend:
- New package `internal/updates`: a checker for the GitHub Releases API of
  `Shavell/kube-bench` (`KUBEBENCH_UPDATE_URL`, for tests and mirrors).
  It honours `HTTPS_PROXY`/`NO_PROXY`, has a 10 s timeout, sends
  `If-None-Match` and caches results for about 6 h. It never blocks
  startup and makes no request at all when the mode is `off`.
- SemVer comparison with `main.version`: `dev`, git-describe and snapshot
  builds report `current: "unknown"` and never nag.
- Modes `off` | `stable` | `prerelease`, default `KUBEBENCH_UPDATE_MODE`
  (`stable`). The admin's choice is persisted in `app_settings` (store
  migration v3), not in the plugin KV.
- Routes:
  - `GET /api/updates` (authed);
  - `PUT /api/settings/updates` (admin, audited);
  - `POST /api/updates/check` (admin, audited);
  - `lastSeenVersion` in `GET /api/auth/me`, stored by `PATCH /api/auth/me`
    (`users.last_seen_version`, migration v3).
- Config, `packaging/kubebench.env`, the chart (`config.updateMode`) and the
  deploy docs gain both variables. Air-gapped installs use `off`:
  `install.sh --offline` sets it, the offline bundle README documents it.

Frontend:
- Settings → Updates: mode selector for admins, read-only for others,
  "Check now", the last check and the error reason.
- Status bar item "Update available: vX.Y.Z" for admins, opening a dialog
  with the release notes (Markdown subset renderer, no HTML injection) and
  the upgrade steps per install method, linking `docs/deployment.md#upgrades`.
  About shows the latest version to everyone.
- What's new: `CHANGELOG.md` bundled with `?raw`, parsed by its Keep a
  Changelog sections. It is shown once per user after an upgrade, and the
  command "Help: What's New" and an About link show the full changelog.
- Mock mode: a deterministic fake release; the demo user has seen the
  current version.

Tests:
- Go: an httptest fake GitHub covering latest stable, prerelease, `304` with
  ETag, a rate-limit `403`, a network error, malformed JSON and version
  comparison (v prefix, prerelease ordering).
- A store migration test and API permission tests.
- Live: the real GitHub API ("no releases published" stays calm); a local
  fake via `KUBEBENCH_UPDATE_URL` showing an available update in the UI;
  What's new appearing once after the build version changes.

## F13 `cloud` — Cloud providers (§7.12)

Requested by the product owner: "EKS, AKS and GKE support; take tricks from
KubelizeMe". KubelizeMe detects the provider from the API server URL, shows
a provider badge and checks the version against the provider's support
window; KubeBench adapts that and adds in-process cloud sign-in.

Backend:
- `internal/cloud`: provider detection from the server URL, refined by node
  `spec.providerID`, well-known node labels and `gitVersion` suffixes
  (EKS, AKS, GKE, DOKS, LKE, OVH, OpenShift, RKE2, k3s; a bare cloud
  providerID names the cloud, not the managed service), cached per runtime
  (background, refreshed after 1 h and on rebuild). A bundled, dated
  support table (upstream, EKS, AKS, GKE; sources and "as of" date in a
  comment) → `supported | ending-soon (≤90 days) | end-of-life | unknown`
  with the end date.
- `GET /api/clusters` entries and `/api/info` `cluster` gain `platform`;
  listing never connects to a cluster (URL only until its runtime exists).
- `internal/cloudauth`: recognise `aws eks get-token`,
  `aws-iam-authenticator token`, `gke-gcloud-auth-plugin` and
  `kubelogin get-token` stanzas (anything else stays an exec plugin under
  `KUBEBENCH_ALLOW_KUBECONFIG_EXEC`) and replace them with token providers:
  EKS (AWS SDK v2 default chain or per-cluster keys/profile, optional role
  assumption, presigned STS `GetCallerIdentity` → `k8s-aws-v1.` token),
  GKE (`golang.org/x/oauth2/google` ADC or a per-cluster service-account
  key, scope `cloud-platform`), AKS (`azidentity`: service principal,
  managed identity, workload identity, environment — never the Azure CLI —
  for `<server-id>/.default`). Tokens cached until 1 min before expiry.
- Per-cluster credentials (`credentials` on `POST /api/clusters` and
  `PATCH /api/clusters/{id}`, store migration v4): write-only; GET returns
  only their provider to admins; audit rows carry the provider only.
- Docs: the design (§7.12, §4.3) and `docs/deployment.md` state that an
  admin can make the server use its ambient cloud identity for any cluster
  they add, and show the chart's `serviceAccount.annotations` for IRSA,
  GKE Workload Identity and Azure Workload Identity.

Frontend:
- Text provider badge (no logos) in the Clusters view, the cluster
  switcher and the status bar; an end-of-life / ending-soon notice on the
  cluster overview.
- Add/Edit cluster dialogs: cloud credential fields (AWS key or profile,
  GCP service-account key, Azure service principal), never pre-filled.
- Mock: deterministic platforms for the mock clusters.

Tests:
- Go: detection (URLs, providerIDs, labels, versions), the version table
  (boundaries, table sanity), exec-stanza recognition (aws,
  aws-iam-authenticator, gke-gcloud-auth-plugin, kubelogin variants;
  unknown commands still refused without the flag), the EKS token against a
  fixed signer and clock (decoded URL, independently recomputed SigV4
  signature), fake GKE/AKS token sources through the transport (caching,
  401 reset), API: platform fields, write-only credentials, clean audit.
- No real cloud account is available: live checks on `kubebench-e2e` only
  cover k3s detection, version status, the UI badges and the credential
  fields of the Add/Edit dialogs (stored, never returned, audit clean).

## F14 `notes` — Notes (§7.13, v0.6)

Requested by the product owner: "not alabilelim, notlar eklentisi yapalım"
("let us take notes; build a notes plugin"). Decision: personal **and**
shared notes.

Backend:
- Store migration v5: `notes` + `note_tags` with foreign keys that give
  the cascades of §7.13 (cluster deleted → its notes; user deleted → their
  personal notes, shared ones kept with a deleted author), indexes for
  (cluster, kind, namespace, name), owner and tag lookups. Not the plugin
  KV: notes need queries, counts and cascades.
- Plugin contract (§5): `plugin.AccessContribute` (operator/admin, a
  read-only cluster allowed, non-GET audited) and `Context.Role` /
  `Context.Store`.
- Plugin `internal/plugins/notes`: the routes of §7.13. Personal routes
  are `AccessRead` and act on the caller's own notes only (another user's
  note is `404`, for admins too); shared routes are `AccessContribute`;
  deleting another author's shared note needs a global admin. Audit
  target `note/<id>`, never the text. Limits: body 64 KiB, title 200,
  16 tags, 1000 notes per author. Optimistic concurrency via `version`
  (`409 conflict`).
- Live updates: the watch hub serves Kubernetes topics only, so note
  queries refresh after own changes, on auto-refresh and on window focus.

Frontend:
- `src/plugins/notes`: activity bar + sidebar (search, scope, all
  clusters, tags, list), `notes-editor` tab (CodeEditor `markdown`, Edit /
  Preview / Split with the shared safe Markdown renderer moved to
  `src/shared/markdown`, dirty guard, 409 reload/overwrite, shared-note
  visibility warning), `notes-resource` tab for one object, resource
  action "Notes" on every kind with a count badge
  (`ResourceActionContribution.badge`), palette "Notes: New Note".
- en/tr/de; deterministic mock notes.

Tests:
- Go: migration v5 from a v4 database; store queries (visibility filter,
  resource match by group, tags, search, limits, conflicts, cascades);
  permission matrix through the router (personal isolation between users
  and from admins, viewer refused on shared writes, read-only cluster
  allowed, operator cannot delete another's shared note, admin can);
  audit rows without text.
- Live on `kubebench-e2e` (namespace `e2e-notes-*`): notes on a
  Deployment shown on its detail page; shared vs personal visibility for
  admin, operator and viewer in two browser sessions; audit rows.
  Playwright in mock and live mode, screenshots en/tr/de.

## F15 `service-browser` — Service browser (§7.14, v0.6)

Requested by the product owner: "service'leri yanda açılacak bir browser
açılabilir" — open a Service's web interface in a browser next to the
editor. Decision: the Kubernetes API server's proxy subresource plus an
isolated iframe, with an optional separate proxy origin for full
compatibility. The contract is §7.14; in short:

- Backend (core): `POST /api/proxy/services|pods/{ns}/{name}/{port}` (new
  access level `browse`: operator/admin, allowed on read-only clusters with
  GET/HEAD only; audited once per grant as `service.proxy` / `pod.proxy`)
  returns a grant whose random token is part of the URL;
  `/proxy/{token}/{path...}` forwards every method to
  `…/<services|pods>/<scheme>:<name>:<port>/proxy/<path>` with the
  runtime's credential. The token, never the session cookie, authorizes
  proxied requests; each one re-checks session, role and the read-only
  rule. Strip credentials, impersonation, `Referer` and the session cookie
  from requests; strip the session cookie, HSTS, Clear-Site-Data,
  Service-Worker-Allowed and framing headers from responses; rewrite the
  API server's proxy path in HTML and `Location`, cookie paths below the
  grant. Add `CSP: sandbox allow-scripts allow-forms allow-popups
  allow-downloads; frame-ancestors <KubeBench>`, `X-Frame-Options`,
  `Referrer-Policy: no-referrer`, CORP. WebSocket upgrades pass.
- `KUBEBENCH_PROXY_ORIGIN`: the same proxy on a separate origin, which
  serves nothing else, unsandboxed (config, docs/configuration.md,
  packaging/kubebench.env, chart `config.proxyOrigin` + ingress host
  example).
- Frontend: a side editor group in the core shell (`openInSideGroup`); the
  `service-browser` plugin adds "Open in browser" to Services and Pods
  (permission `browse`) and renders the browser tab: back/forward/reload,
  in-cluster address with editable path, port and scheme, open in a new
  tab, sandbox and read-only notices; mock mode shows a deterministic
  placeholder page. en, tr, de.
- Tests: svcproxy unit tests (headers, rewriting, grants), API tests with a
  fake API server (viewer refused, read-only method rule, one audit row per
  grant, access loss, proxy origin); live on `kubebench-e2e`
  (namespaces `e2e-service-browser-*`): nginx renders, links, redirects and
  a form post work, a hostile page proves the isolation, WebSocket echo,
  read-only and viewer checks, proxy-origin mode; screenshots.
