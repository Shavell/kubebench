# HTTP API reference

The KubeBench server (the Go binary in [`backend/`](../backend)) serves the
web UI and a JSON API under `/api`. This page lists every route as it is
registered in
[`backend/internal/api/routes.go`](../backend/internal/api/routes.go) and in
the `Routes` method of each backend plugin under
[`backend/internal/plugins/`](../backend/internal/plugins). The TypeScript
shapes the UI uses for these answers live in
[`src/data/types.ts`](../src/data/types.ts) and, for plugins, in
`src/plugins/<id>/data/types.ts`. The behaviour behind each route is
specified in [`design-v0.5.md`](design-v0.5.md); the section is given where
it matters.

The API is made for the KubeBench UI and external UI plugins. It is not a
stable public API before 1.0: a minor release may change it, and the
CHANGELOG says how.

## Conventions

- **Session.** `POST /api/auth/login` sets the HttpOnly cookie
  `kubebench_session` (SameSite=Lax; `Secure` per
  `KUBEBENCH_SECURE_COOKIES`). Every route except the public ones needs it.
- **Active cluster.** Each user has one active cluster. Routes of level
  `read`, `write` and `exec`, and every plugin route, act on it. Only
  `POST /api/clusters/activate` changes it, and `GET /api/info` picks the
  first usable cluster when none is active.
- **Origin check.** A non-GET request (and a WebSocket upgrade) whose
  `Origin` header does not match the request host is refused with
  `403 forbidden`, and so is a GET of a write or exec route that a
  browser marks `Sec-Fetch-Site: cross-site`.
- **Bodies** are JSON (`Content-Type: application/json`) unless stated.
  They must arrive within one minute. Most query parameters that name an
  object are `ns` and `name`.
- **Booleans** in query strings are `true` or `false`; anything else is
  `400 bad_request`, except on the passthrough (below).
- **Mutations without a result** answer `{"ok": true}`.
- **Dry runs.** Routes marked *dry-run* take `dryRun=true` (exactly that
  value). Such a request needs only `read`, is not audited and performs a
  real server-side dry run (§3.2). Exception: a dry run of `apply` or
  `edit` that touches a Secret needs `write` and is audited.

### Access levels

| Level | Needs | Audited |
|---|---|---|
| `public` | nothing | no |
| `authed` | a session (also one that must still change its password) | no |
| `read` | session, a usable active cluster and any role on it (viewer, operator, admin) | no |
| `write` | `read` + role operator or admin + the cluster is not read-only | yes |
| `exec` | the same gate as `write` | yes |
| `browse` | `read` + role operator or admin; allowed on read-only clusters (the service browser then sends only GET and HEAD, without WebSocket upgrades) | yes |
| `admin` | a global admin | mutations |
| `contribute` | plugin routes only: `read` + role operator or admin; the cluster may be read-only (KubeBench-side data, e.g. shared notes) | mutations |

While a user must change their password, every `read`, `write`, `exec` and
`admin` route answers `403 password_change_required`. The router checks, in
this order, before a handler runs: `password_change_required` →
`admin_required` → `plugin_disabled` (plugin routes) → `no_cluster` →
`cluster_read_only` (admins included) → `forbidden_role` →
`plugin_unavailable` (plugin routes). Refused attempts on audited routes are
audited too.

### Errors

Every error is `{"error": "<message>", "code": "<code>"}`. When KubeBench
explains the error itself (rather than passing on the Kubernetes API's
answer), the body also carries a stable **reason code** with its
parameters:

```json
{"error": "cluster \"prod\" is read-only; an administrator can clear the flag in the Clusters view",
 "code": "cluster_read_only", "reason": "cluster.read_only", "params": {"cluster": "prod"}}
```

`error` is the English text; the UI shows the translation of `reason`
(`src/core/reasons/locales`, `{{param}}` placeholders) and falls back to
`error` for a code it does not know. Reasons are `<group>.<name>` and stable
across releases; parameters named `detail` carry a message from elsewhere
(Kubernetes, Helm, the network), untranslated. Messages of the Kubernetes
API itself (`code: kubernetes`) have no reason, except a `403`:
`kubernetes.forbidden_namespace` (`verb`, `resource`, `namespace`,
`user`), `kubernetes.forbidden_cluster` (`verb`, `resource`, `user`) or
`kubernetes.forbidden` (`detail`, another wording), so the UI can say what
the cluster's credential may not do (design §7.16). The same pair explains
status objects: `reasonCode`/`reasonParams` next to `reason` in plugin
statuses (`GET /api/plugins`), live-update topic statuses (`event: status`
of `/api/watch`) and the security report's `skipped`;
`clusterErrorReason`/`clusterErrorParams` in `/api/info`;
`errorReason`/`errorParams` in the update status; `warningReasons`
(`[{"reason","params"}]`, index by index) next to the `warnings` of the
security report, of apply results and of the storage plugin's creates; a
failed object of an apply has `reason`/`params` next to its `error`; a
drain's `failed[]` has `reason` when KubeBench explains why a pod stayed. Streams end with a **reason message**
`{"error": "<message>", "reason"?: "<code>", "params"?: {…}}`: the data of a
log stream's `event: error` and a watch stream's `event: end`, and
`reason`/`params` next to `message` in a terminal's error frame. A message
can be a parameter itself, translated in place, e.g. `streams.closed`
("the session was closed: {{cause}}") with the refusal that ended the
stream as `cause`. `backend/internal/api/reasons_test.go` fails when a code
the server defines lacks a string in a UI language.

Codes:

| Code | Typical status | Meaning |
|---|---|---|
| `unauthenticated` | 401 | no session |
| `session_expired` | 401 | the session ended |
| `password_change_required` | 403 | change the password first (`POST /api/auth/password`) |
| `forbidden` | 403 | refused by KubeBench (Origin, wrong current password, system namespace, path kubeconfigs, Secrets in the passthrough, …) |
| `forbidden_role` | 403 | the caller's role on the cluster does not allow it (viewer) |
| `cluster_read_only` | 403 | the cluster is flagged read-only |
| `admin_required` | 403 | the route is for global admins |
| `no_cluster` | 409 / 503 | no usable active cluster (409), or its connection cannot be built (503) |
| `not_found` | 404 | unknown object, kind or API path |
| `bad_request` | 400 / 405 / 413 | invalid input; `405` with an `Allow` header for a known path with the wrong method |
| `conflict` | 409 | state does not allow it (paused rollout, running Argo CD operation, last admin, …) |
| `rate_limited` | 429 | too many attempts or streams; see `Retry-After` |
| `plugin_disabled` | 404 | the backend plugin is disabled |
| `plugin_unavailable` | 503 | the plugin's Detect says it cannot work on this cluster |
| `capability_unavailable` | 503 | the cluster lacks what the route needs (e.g. Prometheus), with the reason |
| `redacted_secret` | 400 | a Secret (or a Route, BuildConfig or Build credential) still holds redaction placeholders |
| `kubernetes` | the API server's status | an error from the cluster; its `401` becomes `502` |
| `internal` | 500 | a KubeBench-side failure |

Unknown `/api` paths answer `404 not_found`.

## Health and version

| Method | Path | Level | Answer |
|---|---|---|---|
| GET | `/healthz` | public | `{"status":"ok"}` |
| GET | `/api/version` | public | `{"version","commit","date"}` |

## Session and account

| Method | Path | Level | Parameters | Answer |
|---|---|---|---|---|
| POST | `/api/auth/login` | public | `{"username","password","source"?}` | `{"id","username","role","mustChangePassword","source","displayName"}`; sets the cookie. 10 failed attempts per client IP and username in 15 min → `429`. `source` names a password provider (a directory, §3.6) of `GET /api/auth/methods`; empty or `local` signs in with a local account, which external users have not. A directory takes `user`, `DOMAIN\user` or a UPN; its attempts are counted per normalized login, and a refusal carries the reason code `auth.signin.<failure class>` |
| GET | `/api/auth/methods` | public | — | `{"local":true,"providers":[{"source":"oidc","name","type":"redirect","loginUrl":"/api/auth/oidc/login"}]}` (§3.6); password providers have `"type":"password"` |
| GET | `/api/auth/oidc/login` | public | — | `302` to the provider (authorization code flow with PKCE, state and nonce); sets the short-lived `kubebench_oidc_state` cookie, which holds the pending sign-in sealed by the server (nothing is kept server-side). `404` without OIDC |
| GET | `/api/auth/oidc/callback` | public | `?code=&state=` (from the provider) | `302` to `<public URL>/` with the session cookie, or to `<public URL>/?signInError=<class>` when refused (`invalid_credentials`, `account_disabled`, `domain_not_allowed`, `no_matching_group`, `username_conflict`, `invalid_username`, `invalid_request`, `provider_error`, `rate_limited`). Callbacks refused after the provider redeemed the code count against the login limit per client IP (a wrong state or a made-up code does not); every outcome is audited (`auth.signin`) |
| POST | `/api/auth/logout` | authed | — | `{"ok":true,"logoutUrl"?}`; `logoutUrl` (OIDC sessions, providers with RP-initiated logout) ends the provider's session and returns to KubeBench |
| GET | `/api/auth/me` | authed | — | `{"id","username","role","mustChangePassword","source","displayName","lastSeenVersion"}` |
| PATCH | `/api/auth/me` | authed | `{"lastSeenVersion":"0.8.2"}` (1–64 characters) | the `me` view |
| POST | `/api/auth/password` | authed | `{"currentPassword","newPassword"}` | `{"ok":true}`; new password 8 characters to 72 bytes and different; a wrong current password is `403 forbidden` and throttled like logins; other sessions of the user end. External users (`source` other than `local`) get `400` |
| GET | `/api/info` | authed | — | server, session and active-cluster facts (below) |

`GET /api/info` (§4.1) waits up to 3 s for a fresh cluster connection's
capability probe and provider detection:

```json
{
  "version": "0.8.2", "commit": "abc1234",
  "user": "alice", "role": "admin", "mustChangePassword": false, "authSource": "local",
  "context": "k3s-dev", "server": "https://10.0.0.1:6443",
  "cluster": {"id": 1, "name": "k3s-dev", "environment": "production", "readOnly": false,
              "platform": {"provider": "k3s", "cloud": "", "version": "v1.35.5+k3s1",
                           "support": {"status": "supported", "endDate": "2027-02-28", "basis": "kubernetes"},
                           "detected": true}},
  "permissions": {"role": "admin", "canRead": true, "canWrite": true, "canExec": true, "isAdmin": true, "readOnly": false},
  "capabilities": {"crds": true, "helm": true, "exec": true, "optionalResources": [],
                   "listNamespaces": true, "namespaces": [], "defaultNamespace": ""},
  "features": {"pathKubeconfig": true, "externalPlugins": true, "ambientCloudIdentity": false},
  "clusterError": ""
}
```

Without a usable cluster, `cluster` is `null` and `clusterError` says why.

Namespace access (design §7.16): `listNamespaces` is whether the cluster's
credential may list namespaces; `namespaces` are the namespaces the UI
offers when it may not (the cluster's `allowedNamespaces`, else the
kubeconfig context's namespace); `defaultNamespace` is where the UI starts
(`""` = all namespaces). When exec or Helm are allowed only in some of
those namespaces, `execNamespaces` / `helmNamespaces` list them (absent =
cluster-wide), e.g. for a ServiceAccount bound to `edit` in `team-a`:
`{"exec": true, "execNamespaces": ["team-a"], "helm": true, "helmNamespaces": ["team-a"],
"listNamespaces": false, "namespaces": ["team-a"], "defaultNamespace": "team-a"}`.

On OpenShift (§7.15) `capabilities.optionalResources` lists the OpenShift
catalog keys the cluster serves (`routes`, `projects`,
`securitycontextconstraints`, `deploymentconfigs`, `buildconfigs`, `builds`,
`imagestreams`, `imagestreamtags`), and `platform` carries the OpenShift
version and its life cycle:

```json
"platform": {"provider": "openshift", "cloud": "", "version": "v1.36.5", "productVersion": "5.0.0",
             "support": {"status": "supported", "endDate": "2028-11-30", "basis": "openshift",
                         "phase": "full", "fullSupportEnd": "2027-11-30"},
             "detected": true}
```

`productVersion` is read from the ClusterVersion `version`
(`config.openshift.io/v1`); without it the support status is `unknown`.
`endDate` is the end of maintenance support; `phase` is `full`,
`maintenance`, `eus` (past maintenance, in Extended Update Support, which
an EUS subscription needs: the status stays `end-of-life`) or `ended`;
`eusEnd` is set for minors with Extended Update Support.

## Users (admin)

| Method | Path | Level | Parameters | Answer |
|---|---|---|---|---|
| GET | `/api/users` | admin | — | `[{"id","username","role","mustChangePassword","clusters":["name"],"assignments":[{"clusterId","cluster","role"}],"source":"local"\|"oidc"\|"ldap","displayName","email"}]` (`AppUser`) |
| POST | `/api/users` | admin | `{"username","password","role":"admin"\|"user"}` | `{"id"}`; the user must change the password at the first sign-in. `409` (`users.exists`) when the name is taken, ignoring case |
| PATCH | `/api/users/{id}` | admin | `{"role"?, "password"?}` | `{"ok":true}`; a reset sets must-change and ends the user's sessions; external users have no password to reset (`400`), and their role and assignments are replaced at their next sign-in (§3.6) |
| DELETE | `/api/users/{id}` | admin | — | `{"ok":true}`; not yourself, not the last local admin (`409`): it is the break-glass account |
| PUT | `/api/users/{id}/clusters` | admin | `{"assignments":[{"clusterId","role":"viewer"\|"operator"}]}` or the v0.4 form `{"clusterIds":[…]}` (operator) | `{"ok":true}` |

## Authentication settings (admin, §3.6)

| Method | Path | Level | Parameters | Answer |
|---|---|---|---|---|
| GET | `/api/settings/auth` | admin | — | `{"publicUrl","publicUrlConfigured","requireGroup","providers":[{"source","name","settings":[{"key","value","secretSet"?}],"status":{"ok","error"},"warnings":["plaintext"\|"tlsVerifyDisabled"]}]}` (`AuthSettings`); secrets (client secret, bind password) only report `secretSet`, never their value; `status` checks the OIDC discovery document or a search of the directory's user base DN as the service account |
| GET | `/api/settings/auth/mappings` | admin | — | `[{"id","group","source":""\|"oidc"\|"ldap","role":"admin"\|"operator"\|"viewer","cluster","clusterExists"}]` (`GroupMapping`) |
| POST | `/api/settings/auth/mappings` | admin | `{"group","source"?,"role","cluster"?}` (a cluster for operator and viewer, none for admin) | `{"id"}`; `409` when the group (ignoring case), source and cluster repeat a mapping. Audited (target `mapping/<group>`) |
| PUT | `/api/settings/auth/mappings/{id}` | admin | the same body | `{"ok":true}`. Audited |
| DELETE | `/api/settings/auth/mappings/{id}` | admin | — | `{"ok":true}`. Audited |
| POST | `/api/settings/auth/preview` | admin | `{"source":"oidc"\|"ldap","username","groups"?:[…]}` | "Who would I be" without a user or session: `{"source","username","groups","groupsFrom":"input"\|"directory"\|"lastSignIn"\|"none","admin","clusters":[{"cluster","role","exists"}],"matched":[<mapping id>],"allowed","reason","existingSource","lastSignIn"}` (`SignInPreview`). Without `groups` a directory looks the user up as its service account (it also resolves `DOMAIN\user` or a UPN to the account's username), else the groups of their last sign-in are used |

## Clusters

| Method | Path | Level | Parameters | Answer |
|---|---|---|---|---|
| GET | `/api/clusters` | authed | — | `ClusterEntry[]`: `{"id","name","source","active","context"?,"server"?,"address"?,"error"?,"readOnly","environment","role","platform","allowedNamespaces"}`; admins also get `kubeconfigPath`, `cloudAuth` and `cloudCredentials` (the provider only, never the secrets). Listing never connects to a cluster |
| POST | `/api/clusters/activate` | authed | `?name=` | `{"ok":true}`; `403` without an assignment, `400` for address bookmarks or a cluster that cannot be reached |
| POST | `/api/clusters` | admin | `{"name", "content"? \| "path"? \| "address"?, "context"?, "credentials"?, "allowedNamespaces"?: [names]}` (the v0.4 query form `?name=&path=&address=&context=` still works) | `{"ok":true,"id"}`; `path` needs `features.pathKubeconfig`, else `403`, and a path inside a directory KubeBench writes from requests (the plugins' data directories such as the Helm cache, the external plugins directory) is refused with `403`; pasted kubeconfigs that run exec plugins, auth providers or read server files are refused with `403` unless `KUBEBENCH_ALLOW_KUBECONFIG_EXEC=true` (EKS, GKE and AKS stanzas are handled in process); pasted EKS, GKE and AKS kubeconfigs without credentials of their own (none, or an AWS `profile`) are refused with `403` unless `features.ambientCloudIdentity` (`KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY`) |
| PATCH | `/api/clusters/{id}` | admin | `{"name"?, "readOnly"?, "environment"?: ""\|"development"\|"staging"\|"production", "credentials"?: object\|null, "allowedNamespaces"?: [names]}` | `{"ok":true}`; `allowedNamespaces` (design §7.16) are namespace names (at most 20, `[]` clears them) offered when the credential cannot list namespaces: a UI list, not a security boundary; removing the credentials of an EKS, GKE or AKS kubeconfig, or replacing them with an AWS `profile`, is refused with `403` unless `features.ambientCloudIdentity` |
| DELETE | `/api/clusters` | admin | `?name=` | `{"ok":true}`; sessions' active cluster and assignments go with it |

Cloud `credentials` (§7.12) are one of
`{"provider":"aws","accessKeyId","secretAccessKey","sessionToken"?}`,
`{"provider":"aws","profile"}`, `{"provider":"gcp","serviceAccountJson"}`,
`{"provider":"azure","tenantId","clientId","clientSecret"}`. They are
write-only.

## Plugins, audit, updates

| Method | Path | Level | Parameters | Answer |
|---|---|---|---|---|
| GET | `/api/plugins` | authed | — | `{"plugins":[{"id","name","version","description","category","builtin","external","enabled","status":{"available","reason"?,"reasonCode"?,"reasonParams"?},"entry"}],"settings":{"<id>":{"enabled"}}}`; Detect results are cached 60 s per cluster |
| PUT | `/api/plugins/{id}` | admin | `{"enabled": bool}`; id `^[a-z0-9][a-z0-9-]{0,39}$` | `{"ok":true}` |
| GET | `/plugins/{id}/{file...}` | authed | — | a file of an enabled external plugin from `$KUBEBENCH_PLUGINS_DIR/<id>/` (`.js .mjs .css .json .map .svg .png` only) |
| GET | `/api/audit` | admin | `?limit=` (default 100) `&before=<id>&user=&cluster=&action=` | `{"items":[{"id","ts","user","cluster","action","target","status","detail"}],"nextBefore":<id>\|null}` |
| GET | `/api/updates` | authed | — | `{"mode","defaultMode","current","latest","available","prerelease","publishedAt","url","notes","checkedAt","error","errorReason"?,"errorParams"?}` (`UpdateStatus`, §7.11); checks GitHub when the 6 h cache expired |
| PUT | `/api/settings/updates` | admin | `{"mode":"off"\|"stable"\|"prerelease"}` | the update status |
| POST | `/api/updates/check` | admin | — | the update status; `409` while the mode is `off` |

## Resources (active cluster)

All `read` unless stated.

| Method | Path | Parameters | Answer |
|---|---|---|---|
| GET | `/api/nodes` | — | `K8sNode[]` (with `unschedulable` and `taints` when set) |
| GET | `/api/namespaces` | — | `Namespace[]` |
| GET | `/api/pods` | `?ns=` (empty = all) | `Pod[]` |
| GET | `/api/deployments` | `?ns=` | `Deployment[]` (with `paused` when set) |
| GET | `/api/services` | `?ns=` | `Service[]` |
| GET | `/api/resource` | `?key=<catalog key>&ns=` | generic list of a built-in kind (`configmaps`, `secrets`, `statefulsets`, `ingresses`, `persistentvolumeclaims`, `clusterroles`, …; see `backend/internal/kube/catalog.go`); OpenShift's `routes`, `projects`, `securitycontextconstraints`, `deploymentconfigs`, `buildconfigs`, `builds`, `imagestreams`, `imagestreamtags` (§7.15) list as empty on clusters that do not serve them |
| GET | `/api/resource/get` | `?key=&ns=&name=` | one object of that kind |
| GET | `/api/yaml` | `?apiVersion=&kind=&ns=&name=&neat=true` | `{"yaml","resourceVersion"}`; without `apiVersion` only the kinds v0.4 knew; `neat` strips server-maintained fields (`resourceVersion` is still returned beside the YAML) |
| GET | `/api/events` | `?kind=&ns=&name=` | `K8sEvent[]` about that object |
| GET | `/api/logs` | `?ns=&pod=&container=&tail=` (default 300, max 5000) `&previous=&sinceSeconds=` | `string[]` (lines) |
| GET | `/api/crds` | — | `CrdDefinition[]`; `printerColumns` `[{name, type, description?, priority}]` are the `additionalPrinterColumns` of the listed (first served) version in order, without Name, Namespace and Age |
| GET | `/api/crds/instances` | `?crd=<name>` | `CustomResource[]`; `extra` maps each printer column to its value, evaluated with full JSONPath (filters, indexes) and formatted by type like `kubectl get` (a `date` as an age); a column without a value is missing |
| GET | `/api/k8s/{path...}` | the Kubernetes API's own query | raw JSON (passthrough, below) |

Secret values are redacted on every read route: `data` and `stringData`
values and kubectl's `last-applied-configuration` annotation. Revealing them
is `POST /api/secrets/reveal`. The credentials of OpenShift kinds are
redacted the same way and cannot be revealed: a Route's `spec.tls.key`,
the `secret` of a BuildConfig's `spec.triggers[].github|generic|gitlab|bitbucket`
and of a Build's `spec.triggeredBy[].*WebHook` (§7.15).

### Kubernetes read passthrough

`GET /api/k8s/{path...}` (§4.5) forwards a GET to the cluster's API
(`/api/...`, `/apis/...`, `/version`) with the stored credential and returns
the raw JSON (16 MiB at most). It refuses Secrets (`403`), `watch=true`,
`/watch/` paths and `follow=true` (`400`), and the `proxy`, `exec`, `attach`
and `portforward` subresources (`403`). Booleans are read the way the API
server reads them. Redirects are never followed; answers are
`application/json`, `text/plain` or `application/octet-stream` with
`X-Content-Type-Options: nosniff` and a sandboxing CSP. External plugins
use it through `KubeBench.api.k8sGet`.

## Metrics

All `read`. `source` is `auto` (default: Prometheus when one was found,
else metrics-server), `prometheus` or `metrics-server`; `minutes` defaults
to 60. Series answer `MetricPoint[]`.

| Method | Path | Parameters | Answer |
|---|---|---|---|
| GET | `/api/metrics/sources` | — | `{"metricsServer":{"available","location","unavailable"?},"prometheus":{"available","location","unavailable"?}}` |
| GET | `/api/metrics/overview` | — | `ClusterStats` |
| GET | `/api/metrics/allocation` | — | `ClusterAllocation` |
| GET | `/api/metrics/cluster` | `?metric=cpu\|memory\|network\|fs&minutes=&source=` | series |
| GET | `/api/metrics/node` | `?node=&metric=cpu\|memory\|network\|fs&minutes=&source=` | series |
| GET | `/api/metrics/pod` | `?ns=&pod=&metric=cpu\|memory\|network&minutes=&source=` | series |

A failing Prometheus query is reported as `503 capability_unavailable`;
only `source=auto` falls back to metrics-server, for CPU and memory. Network
and filesystem series need Prometheus.

metrics-server is `available` only while the sampler's latest scrape of
the metrics API (`metrics.k8s.io`) measured nodes; otherwise its series
answer `503 capability_unavailable` with the reason (`metrics.api_missing`:
the API is not served; `metrics.api_unavailable`: its APIService is not
Available, with the condition's message; `metrics.api_forbidden`,
`metrics.api_failed`, `metrics.api_empty`, `metrics.not_sampled`). An
unavailable source carries that reason as `unavailable: {"error",
"reason", "params"}`; Prometheus says `metrics.no_prometheus`, or why
OpenShift monitoring cannot be used (`metrics.thanos_*`).

`ClusterStats.cpuUsagePercent`/`memoryUsagePercent` and
`ClusterAllocation.{cpu,memory}.usage` come from the newest metrics-server
sample, else from Prometheus' current values; when neither has them they
are `null` (never `0`) and `usageUnavailable` explains why. The other
fields (counts, capacity, requests, limits) are always present.

## Editing (§7.2)

| Method | Path | Level | Parameters | Answer |
|---|---|---|---|---|
| POST | `/api/resource/apply` | write, dry-run | `?ns=&force=&dryRun=`; body YAML or JSON, any number of documents, ≤ 4 MiB | `{"results":[{"apiVersion","kind","namespace"?,"name","action"?:"created"\|"configured"\|"unchanged","error"?,"code"?,"reason"?,"params"?,"warnings"?,"warningReasons"?}]}` (server-side apply, field manager `kubebench`) |
| POST | `/api/resource/edit` | write, dry-run | `?apiVersion=&kind=` or `?key=`, `&ns=&name=&dryRun=`; body `{"original","edited","resourceVersion"?}` | as apply, one result (sent as a JSON merge patch with `resourceVersion` — or the original's `metadata.resourceVersion` — as precondition: an object changed since fails with `code: conflict`, `reason: edit.changed_on_server`) |
| DELETE | `/api/resource` | write | `?apiVersion=&kind=` or `?key=`, `&ns=&name=&propagation=Background\|Foreground\|Orphan&gracePeriod=` | `{"ok":true}`; namespaces use `DELETE /api/namespaces` |
| PATCH | `/api/resource/metadata` | write | the same object parameters; body `{"labels":{…},"annotations":{…}}` (`null` removes a key) | `{"ok":true}` |
| POST | `/api/secrets/reveal` | exec | `?ns=&name=` | the Secret with its `data` (base64), `Cache-Control: no-store` |

## Workload and node operations (§7.3)

| Method | Path | Level | Parameters | Answer |
|---|---|---|---|---|
| DELETE | `/api/pods` | write | `?ns=&name=&force=` (force = grace period 0) | `{"ok":true}` |
| POST | `/api/pods/bulk-delete` | write | `{"items":[{"namespace","name"}],"force":bool}` (1–200 items) | `{"results":[{"namespace","name","error"?}]}` |
| POST | `/api/pods/debug` | exec | `?ns=&pod=&image=` (default `busybox:1.37`) `&target=<container>` | `{"namespace","pod","container"}` once the ephemeral container runs (≤ 60 s, else `409`) |
| POST | `/api/deployments/scale` | write | `?ns=&name=&replicas=` | `{"ok":true}` |
| POST | `/api/deployments/restart` | write | `?ns=&name=` | `{"ok":true}` |
| GET | `/api/deployments/history` | read | `?ns=&name=` | `{"currentRevision","paused","revisions":[{"revision","replicaSet","images","changeCause","created","replicas","readyReplicas","current"}]}` |
| POST | `/api/deployments/rollback` | write | `?ns=&name=&revision=` | `{"ok":true}`; `409` while paused or for the current revision |
| POST | `/api/deployments/pause` | write | `?ns=&name=&paused=true\|false` (default true) | `{"ok":true}` |
| POST | `/api/workloads/restart` | write | `?kind=Deployment\|StatefulSet\|DaemonSet&ns=&name=` | `{"ok":true}` |
| POST | `/api/workloads/scale` | write | `?kind=Deployment\|StatefulSet\|ReplicaSet&ns=&name=&replicas=`, or `?apiVersion=&kind=` for any kind with a `scale` subresource (discovered, e.g. an Argo Rollout) | `{"ok":true}`; `400` for a kind without a scale subresource |
| POST | `/api/nodes/cordon` | write | `?name=&unschedulable=true\|false` (default true) | `{"ok":true}` |
| POST | `/api/nodes/drain` | write | `?name=&ignoreDaemonSets=true&deleteEmptyDirData=false&force=false&gracePeriod=-1&timeoutSeconds=120` | `{"node","cordoned","completed","evicted","failed","blockedByPDB","skipped","errors"}` (`DrainResult`) |
| PUT | `/api/nodes/taints` | write | `?name=`; body `{"taints":[{"key","value","effect"}]}` | `{"taints":[…]}` |
| POST | `/api/cronjobs/trigger` | write | `?ns=&name=` | `{"namespace","job"}` |
| POST | `/api/cronjobs/suspend` | write | `?ns=&name=&suspend=true\|false` (default true) | `{"ok":true}` |
| POST | `/api/namespaces` | write | `{"name","labels"?}` | `{"ok":true}` |
| DELETE | `/api/namespaces` | write | `?name=&force=` | `{"ok":true}`; system namespaces and `KUBEBENCH_NAMESPACE` are `403 forbidden` unless `force=true` |

## Shells, logs and live updates

| Method | Path | Level | Parameters | Protocol |
|---|---|---|---|---|
| POST | `/api/exec` | exec | `?ns=&pod=&container=&cmd=` | one-shot command → `string[]` (output lines) |
| GET | `/api/exec/ws` | exec | `?ns=&pod=&container=&command=<shell>` | WebSocket terminal (§7.1); audited once as `exec.session` |
| GET | `/api/logs/stream` | read | `?ns=&pod=&container=&tail=&previous=&timestamps=&sinceSeconds=` or `?ns=&selector=<labels>` (≤ 10 pods) | Server-Sent Events |
| GET | `/api/watch` | read | `?topics=pods,deployments,applications.argoproj.io,kubebench:notes` (1–32) | Server-Sent Events (§7.10) |
| GET | `/api/watch/status` | read | — | `{"subscribers","informers":[{"resource","mode","reason"?,"subscribers","idleSince"?}]}` |

- **Terminal frames.** Client → server text frames
  `{"type":"input","data":"…"}` and `{"type":"resize","cols":N,"rows":N}`;
  server → client binary frames carry terminal output, text frames
  `{"type":"exit","code":N}` and `{"type":"error","message":"…","reason"?,"params"?}`. Without
  `command` the server runs `bash`, falling back to `sh`; `command` may only
  name an interactive shell (`sh`, `bash`, `ash`, `dash`, `zsh` and their
  usual paths). 30 s ping, 30 min idle timeout.
- **Log stream events.** `event: pods` `{"pods":["<pod>/<container>"],"total":N}`
  first, then one `data:` line per log line (prefixed `[pod/container]` with
  a selector), `event: error` with a reason message (a failing log, or why
  the server ended the stream), `event: end`, and `: ping` every 15 s.
- **Watch events.** `event: status` `{"topic","mode":"starting"|"live"|"polling","reason"?}`,
  `event: change` `{"topic","namespace"}` (batched for 500 ms), `event: end`
  with a reason message; a comment heartbeat every 25 s. Topics starting with
  `kubebench:` name KubeBench's own data (`kubebench:notes`): always
  `live`, a change has `namespace` `""` and reaches only the users who may
  see the changed data.
- **Limits.** Per user: 10 terminals, 20 log streams and 16 watch streams
  (`429 rate_limited` beyond that), 32 watched resource types. Streams are
  re-authorized at every heartbeat and end when the session, the assignment
  or (terminals) the exec permission goes away.

## Service browser (§7.14)

| Method | Path | Level | Parameters | Response |
|---|---|---|---|---|
| POST | `/api/proxy/services/{ns}/{name}/{port}` | browse | `?scheme=http\|https` (default `http`); `port` is a Service port number or name | `ProxySession`; audited once as `service.proxy`, target `service/<ns>/<name>:<port>` |
| POST | `/api/proxy/pods/{ns}/{name}/{port}` | browse | as above; a port name must be a container port's | `ProxySession`; audited once as `pod.proxy` |
| POST | `/api/proxy/ingresses/{ns}/{name}` | browse | `?host=&path=` of one rule path, or `?default=true` for the default backend (a rule may have neither host nor path, so empty values do not name the default backend), `?scheme=` | `ProxySession` for the Service port of that route, with `target.host` = the rule's host (none for a wildcard host); audited once as `ingress.proxy`, target `ingress/<ns>/<name>`, detail `{"scheme","host","path","service"}` (plus `"default":true`) |
| POST | `/api/proxy/routes/{ns}/{name}` | browse | `?scheme=` | OpenShift (§7.15): `ProxySession` for the Service port behind the Route (its `targetPort` as a Service port name or a named target port; a number as a numbered target port, the endpoint port of the Service's EndpointSlices behind a named target port, or the Service port; none: the first port), with `target.host` = the Route's host; `400` (`proxy.route_not_service`, `proxy.route_no_port`) for a Route that does not lead to a Service port; audited once as `route.proxy`, target `route/<ns>/<name>`, detail `{"scheme","host","path","service"}` |
| GET | `/api/proxy/sessions/{token}` | authed | only the session that opened it | `ProxySession` with the latest `navigations` |
| DELETE | `/api/proxy/sessions/{token}` | authed | only the session that opened it | `{"ok":true}` |
| GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS | `/proxy/{token}/{path...}` | public (the token is the credential) | anything the proxied page sends | the page, through `/api/v1/namespaces/<ns>/<services\|pods>/<scheme>:<name>:<port>/proxy/<path>` |

`ProxySession` is `{"token","base","sandboxed","readOnly","target":{"kind","namespace","name","port","scheme","host"?},"navigations":[{"seq","location"}]}`
(the latest 16 document loads of the frame and of the frames nested in its
page, oldest first; `location` is the in-cluster path and query, `""` when
longer than 4 KiB);
`base` is `/proxy/<token>` (or `<KUBEBENCH_PROXY_ORIGIN>/proxy/<token>`).
Opening a session fails with `503 capability_unavailable` when
`KUBEBENCH_PROXY_ORIGIN` is on the same site as KubeBench's origin (design
§7.14). Proxied requests are re-authorized every time (session alive,
browse access on the session's cluster, GET/HEAD only on a read-only
cluster) and never audited one by one. Their errors are plain text pages
(they are shown in the browser frame), not JSON. WebSocket upgrades are
forwarded, except on a read-only cluster (`403`).

## Backend plugins

Plugin routes live under `/api/plugins/<id>/`. A disabled plugin answers
`404 plugin_disabled`; one whose Detect fails on the active cluster
`503 plugin_unavailable` with the reason.

### Helm — `/api/plugins/helm` (§7.4)

Available when the credential can list the Helm release Secrets.

| Method | Path | Level | Parameters | Answer |
|---|---|---|---|---|
| GET | `/releases` | read | `?ns=` | `[{"name","namespace","chart","chartVersion","appVersion","revision","status","updated","description"}]` |
| GET | `/releases/{ns}/{name}` | read | — | release + `firstDeployed`, `notes`, `chartInfo`, `hooks`, `history` |
| GET | `/releases/{ns}/{name}/values` | **exec** | `?all=&revision=` | `{"values":"<yaml>"}` (values often hold credentials) |
| GET | `/releases/{ns}/{name}/manifest` | read | `?revision=` | `{"revision","manifest"}` (Secrets and Route/BuildConfig/Build credentials redacted) |
| POST | `/releases/{ns}/{name}/rollback` | write | `{"revision"}` (0 = previous) | `{"release"}` |
| POST | `/releases/{ns}/{name}/upgrade` | write | `{"chart"?, "values", "reuseValues", "dryRun"}` | `{"manifest","notes","currentManifest"}` |
| DELETE | `/releases/{ns}/{name}` | write | `?keepHistory=` | `{"ok":true,"release"}` |
| POST | `/install` | write | `{"name","namespace","createNamespace","chart":{"repo","name","version"}\|{"ref","version"}\|{"uploadId"},"values","dryRun"}` | `{"release","manifest","notes"}` |
| GET | `/repos` | read | — | `[{"name","url","authenticated","chartCount","updated","error"?}]` |
| POST | `/repos` | admin | `{"name","url","username"?,"password"?}` | the repository; `409` for a duplicate name |
| DELETE | `/repos/{name}` | admin | — | `{"ok":true}` |
| POST | `/repos/update` | write | — | `{"repos":[{"name","ok","error"?}]}` |
| GET | `/charts/search` | read | `?q=` | ≤ 100 charts with their versions |
| GET | `/charts/values` | read | `?repo=&chart=&version=` or `?uploadId=` | `{"chart","values"}` |
| POST | `/charts/oci/values` | write | `{"ref":"oci://…","version"}` | `{"chart","values"}` |
| POST | `/charts/upload` | write | multipart field `chart` (`.tgz`, ≤ 20 MiB) | `{"uploadId","chart","expiresAt"}` (24 h) |

Install and upgrade dry runs need `write` and are audited: Helm's
server-side dry run reads objects for template `lookup`s with KubeBench's
own credential.

### Argo CD — `/api/plugins/argocd` (§7.5)

Available when the cluster serves `applications.argoproj.io/v1alpha1`.
Works through the CRDs; no Argo CD API token.

| Method | Path | Level | Parameters | Answer |
|---|---|---|---|---|
| GET | `/applications` | read | `?ns=` | application summaries |
| GET | `/applications/{ns}/{name}` | read | — | detail: sources, resources, conditions, history, operation state |
| POST | `/applications/{ns}/{name}/sync` | write | `{"revision"?,"prune"?,"dryRun"?,"resources"?,"syncOptions"?}` | the detail; `409` while an operation runs |
| POST | `/applications/{ns}/{name}/refresh` | write | `?hard=` | the detail |
| POST | `/applications/{ns}/{name}/rollback` | write | `{"historyId","prune"?}` | the detail; `409` with automated sync |
| DELETE | `/applications/{ns}/{name}` | write | `?cascade=` (default true) | `{"deleted":true,"cascade"}` |
| GET | `/projects` | read | — | AppProjects |
| GET | `/applicationsets` | read | — | ApplicationSets |

### Storage — `/api/plugins/storage` (§7.6)

Always available.

| Method | Path | Level | Parameters | Answer |
|---|---|---|---|---|
| GET | `/overview` | read | — | storage classes (default marker), counts, capacity |
| GET | `/pvcs` | read | `?ns=` | claim summaries (with `expandBlocker`) |
| GET | `/pvs` | read | — | volume summaries |
| GET | `/csidrivers` | read | — | CSI drivers |
| GET | `/snapshots` | read | `?ns=` | VolumeSnapshots; `503 capability_unavailable` without the snapshot CRDs |
| POST | `/pvcs` | write, dry-run | `{"namespace","name","storageClassName"?,"accessModes","size","volumeMode"?,"volumeName"?,"dataSource"?,"labels"?}` | `{"apiVersion","kind","namespace"?,"name","dryRun","warnings"?,"warningReasons"?}` |
| POST | `/pvs` | write, dry-run | `{"name","capacity","accessModes",…,"hostPath"\|"nfs"\|"local"\|"csi"}` | as above |
| POST | `/storageclasses` | write, dry-run | `{"name","provisioner",…}` | as above |
| PATCH | `/pvcs/{ns}/{name}/resize` | write | `{"size"}` | the claim summary; `409` when not Bound or the class does not allow expansion |

### Security — `/api/plugins/security` (§7.7)

Always available. Scans only read the cluster.

| Method | Path | Level | Parameters | Answer |
|---|---|---|---|---|
| POST | `/scan` | read | `{"includeSystem"?: bool}` | the report: `generatedAt`, `score`, `counts`, `findings`, `skipped`, `warnings`, `inventory`, `systemNamespaces`; `409` while a scan with other options runs. On OpenShift the SCC checks run too (category `openshift`, target `scc`; §7.15) and `inventory` counts `securitycontextconstraints` |
| GET | `/report` | read | — | the last report of the cluster, or `null` |
| GET | `/checks` | read | — | the check catalog |
| GET | `/export` | read | `?format=md\|csv` | a file download; `404` before the first scan |
| GET | `/accepted` | read | — | stored acceptances `{key, checkId, resource, by, reason, at, expiresAt?, expired}` |
| POST | `/accept` | write | `{"key","reason","expiresAt"?}` | the acceptance |
| DELETE | `/accept/{key}` | write | — | `{"ok":true}` |

### Access — `/api/plugins/access` (§7.8)

Always available; every route is `read` and not audited.

| Method | Path | Parameters | Answer |
|---|---|---|---|
| GET | `/subjects` | — | users, groups and ServiceAccounts named by bindings |
| GET | `/subjects/{kind}/{name}` | `?namespace=` | `{subject, user, groups, missing?, bindings, rules}` (effective rules with their binding path) |
| GET | `/who-can` | `?verb=&group=&resource=&subresource=&namespace=&name=` or `?verb=&path=` | `{request, subjects:[{kind, name, namespace?, grants}]}` |
| GET | `/risky` | — | roles with risks, most severe first; on OpenShift also `scc-privileged` (critical) and `scc-anyuid` (high) for "use" of privileged or anyuid SCCs (§7.15) |
| GET | `/graph` | `?namespace=&system=true` | `{subjects, bindings, roles, edges, truncated}` (≤ 300 bindings) |
| POST | `/review` | `{"subject":{kind,name,namespace?},"verb","group","resource","subresource","namespace","name"}` or `{subject, verb, path}` | `{allowed, denied?, reason?, evaluationError?, user, groups, grants}` (a SubjectAccessReview; 30 per user per minute) |

### Notes — `/api/plugins/notes` (§7.13)

Always available. Notes are KubeBench data, so a read-only cluster does not
block them. A note is
`{"id","visibility":"personal"|"shared","clusterId"|null,"clusterName","resource":{"apiVersion","kind","namespace","name"}|null,"title","body","tags","pinned","version","createdBy","createdAt","updatedBy","updatedAt"}`
(`createdBy`/`updatedBy` are `""` once that user was deleted). Another
user's personal note, and a shared note of another cluster, answer
`404 not_found`, for admins too.

| Method | Path | Access | Body / parameters | Answer |
|---|---|---|---|---|
| GET | `/notes` | read | `?scope=all\|personal\|shared&clusters=current\|all&kind=&apiVersion=&namespace=&name=&tag=&q=&limit=200&body=false` | `{"items":[note],"truncated"}`, pinned first, then newest; bodies only with `body=true` |
| GET | `/notes/{id}` | read | — | the note with its body |
| GET | `/tags` | read | the filters of `/notes` | `{"tags":[{"tag","count"}]}` |
| GET | `/counts` | read | `?kind=&apiVersion=&namespace=&name=` | `{"items":[{"apiVersion","kind","namespace","name","personal","shared"}]}` |
| POST | `/notes/personal` | read | `{"title","body","tags","pinned","global","resource"?}` | the note |
| PUT | `/notes/personal/{id}` | read | `{"title","body","tags","version"}` | the note; `409 conflict` for an outdated `version` |
| PUT | `/notes/personal/{id}/pin` | read | `{"pinned"}` | the note |
| DELETE | `/notes/personal/{id}` | read | — | `{"deleted":id}` |
| POST | `/notes/shared` | contribute | `{"title","body","tags","pinned","resource"?}` | the note |
| PUT | `/notes/shared/{id}` | contribute | as for personal notes | the note |
| PUT | `/notes/shared/{id}/pin` | contribute | `{"pinned"}` | the note |
| DELETE | `/notes/shared/{id}` | contribute | — | `{"deleted":id}`; `403 forbidden` unless the caller wrote it or is an admin |

Limits: title 200 characters, body 64 KiB, 16 tags, 1000 notes per author
(`409 conflict`). Shared-note changes are audited with target `note/<id>`
and at most the object (`{"resource":"Deployment/shop/web"}`) or
`{"pinned":true}` as detail; personal notes are not audited.

Live updates: every stored change publishes the topic `kubebench:notes`
on `GET /api/watch` — a shared note's to the streams on its cluster, a
personal note's to its owner's streams only (on any cluster).

The Topology plugin has no backend routes: it reads through the core
routes and the passthrough.

## Audit records

The router writes one row per audited request: `action` is
`"<METHOD> <route pattern>"` (for example `POST /api/deployments/scale`) or
a named action (`exec.session`, `service.proxy`, `route.proxy`, `argocd.sync`; external
sign-ins are `auth.signin` rows with `{"source"}` or
`{"source","reason":"<failure class>"}`, written by the sign-in routes
themselves), `target` is
`<kind>/<namespace>/<name>`, `status` the HTTP status and `detail` a short
JSON object or the refusal. Request bodies are never recorded. See §3.4 of
the design for the full rules.
