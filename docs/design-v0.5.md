# Design: KubeBench v0.5 — plugins, i18n, roles, live ops, packaging

Status: **accepted** (implementation contract for the v0.5.0 release).
Every change in v0.5 implements against this document. When code and this
document disagree, fix one of them in the same change.

Goals, in priority order:

1. **Trust** — never show fabricated data in live mode; per-cluster roles,
   read-only clusters and an audit log for a multi-user web tool.
2. **Plugin platform** — backend + frontend plugin contracts, enable/disable,
   capability detection, runtime-loaded external (frontend) plugins.
3. **Multi-language UI** — i18next, English + Turkish + German, per-plugin namespaces.
4. **Daily operations parity** (inspired by the KubelizeMe comparison, see
   `docs/notes/kubelizeme-comparison.md`): real PTY shell, streaming logs,
   YAML apply/dry-run/diff, node/workload operations, Helm install/upgrade.
5. **New plugins**: Argo CD, Storage (PV/PVC creator), Security posture,
   Access (RBAC), Topology.
6. **Distribution**: Kubernetes (Helm chart + plain manifests), VM (.deb,
   tarball, `curl | bash` installer), online and offline (air-gapped)
   bundles, GitHub Releases, manual-only GitHub Actions, Dependabot.

Out of scope for v0.5 (roadmap): AI assistant, manifest "Composer",
cluster-to-cluster compare, port-forward, light/dark per-plugin theming.

---

## 0. Ground rules (all changes)

- Read `CLAUDE.md`. Layering rules still hold: `core/shell` never imports
  `plugins/*`; plugins never import each other or `core/shell`; cross-plugin
  flows go through tab kinds (`useLayoutStore.getState().openTab(...)`).
- **Every user-visible string goes through i18n** (`useTranslation(ns)`),
  with an entry in every language (`en.json`, `tr.json`, `de.json`).
  `npm run i18n:check` must pass.
- **Mock + live**: every data capability has a deterministic mock
  implementation (seeded RNG, no `Date.now()`/`Math.random()` in mock data
  generation) and a live implementation. **Live mode never falls back to
  mock data**; it shows an "unavailable" state with the reason instead.
- Colors only via `--vscode-*` tokens (`src/core/theme/theme.css`), which
  must be defined for both dark and light themes. Charts are single-series
  small multiples without legends.
- TS: `verbatimModuleSyntax`, `noUnusedLocals`, `erasableSyntaxOnly` (no
  enums, no parameter properties, no namespaces).
- Verify: `npx tsc -b && npm run build && npm run lint && npm run i18n:check`;
  backend `cd backend && go vet ./... && go test ./... && go build ./...`.
- **The developer's k3s homelab cluster (`~/.kube/config`, context
  `k3s-homelab`) may be used for read-only checks only.** Never send a
  mutating request to it (no create/update/patch/delete/evict/exec-that-
  mutates, not even `dryRun`). Test mutations with client-go fake clients
  (`k8s.io/client-go/kubernetes/fake`, `k8s.io/client-go/dynamic/fake`)
  or the mock frontend.
- Never push, never touch the `origin` (Gitea) remote, never build or push
  Docker images, never deploy to a cluster.
- Commits: Conventional Commits (`feat(argocd): …`, `fix(api): …`), body
  explains why, trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

---

## 1. Versioning & build info

- Go: `package main` declares `var (version = "dev"; commit = "none"; date = "unknown")`,
  set with `-ldflags "-X main.version=… -X main.commit=… -X main.date=…"`.
- `kubebench --version` prints `kubebench <version> (<commit>, <date>)`.
- `kubebench healthcheck` performs `GET /healthz` on the listen address
  (`KUBEBENCH_LISTEN_ADDR`, else `:$PORT`; `127.0.0.1` when it names no
  host or an unspecified one; honours `KUBEBENCH_TLS_*` → https, verifying
  the server against the configured certificate as the only trusted root
  and one of its own names, so self-signed certificates work) and exits 0/1 (2 for an invalid configuration) —
  used by Docker `HEALTHCHECK` (distroless has no shell/curl) and by
  `install.sh` to wait for the service.
- `GET /api/version` (public) → `{"version","commit","date"}`.
- `GET /healthz` (public) → `{"status":"ok"}`.
- Frontend: `__APP_VERSION__` is defined by Vite from `package.json`
  (`define` in vite.config.ts); live mode prefers `/api/info.version`.
- Release version for this work: **0.5.0** (package.json, Chart.yaml
  `version`/`appVersion`).

## 2. Configuration (environment)

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `9876` | HTTP port |
| `KUBEBENCH_LISTEN_ADDR` | `:$PORT` | Full bind address (overrides PORT), e.g. `127.0.0.1:9876` |
| `KUBEBENCH_DB` | `kubebench.db` | SQLite file |
| `KUBEBENCH_DATA_DIR` | dir of `KUBEBENCH_DB` | Writable state: helm repo cache, uploads, plugins |
| `KUBEBENCH_PLUGINS_DIR` | `$KUBEBENCH_DATA_DIR/plugins` | External (frontend) plugins |
| `KUBEBENCH_ADMIN_PASSWORD` | `admin` | Bootstrap admin password (first run only). The admin must change it at first login, the default and a configured one alike |
| `KUBEBENCH_SECURE_COOKIES` | `auto` | `auto` (Secure when request is TLS or trusted `X-Forwarded-Proto: https`), `true`, `false` |
| `KUBEBENCH_TRUST_PROXY` | `false` | Honour `X-Forwarded-Proto/For` (behind ingress/reverse proxy) |
| `KUBEBENCH_TLS_CERT` / `KUBEBENCH_TLS_KEY` | — | Serve HTTPS natively (VM installs without a proxy) |
| `KUBEBENCH_SESSION_TTL` | `168h` | Session lifetime (Go duration) |
| `KUBEBENCH_AUDIT_RETENTION_DAYS` | `90` | Audit rows older than this are pruned daily (0 = keep forever) |
| `KUBEBENCH_ALLOW_PATH_KUBECONFIG` | `true` outside a cluster, `false` in-cluster | Allow adding clusters by server-side file path |
| `KUBEBENCH_ALLOW_KUBECONFIG_EXEC` | `false` | Let pasted kubeconfigs run exec credential plugins and auth providers and read server-side files (certificate, key and token files). This gives every admin command execution as the server's account |
| `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY` | `false` | Let pasted EKS, GKE and AKS kubeconfigs sign in with the server's own cloud identity (§7.12). This gives every admin that identity's token |
| `KUBECONFIG` | — | Dev/VM convenience: seeds the inventory on first run (a path list is merged the way kubectl merges it) |
| `KUBEBENCH_NAMESPACE` | in a pod: the pod's namespace | The namespace KubeBench runs in; deleting a namespace of that name is refused unless forced (§7.3) |
| `KUBEBENCH_UPDATE_MODE` | `stable` | Default update check mode (§7.11): `off` (never contacts GitHub; use it for air-gapped installs), `stable` (new releases) or `prerelease` (also prereleases). An admin's choice in Settings overrides it |
| `KUBEBENCH_UPDATE_URL` | `https://api.github.com/repos/Shavell/kubebench/releases` | Releases endpoint the update check reads: GitHub's "list releases" JSON (a mirror or a GitHub Enterprise API works). `http` or `https` |
| `KUBEBENCH_PUBLIC_URL` | derived from the request | v0.7: the URL users reach KubeBench at (`scheme://host[:port]`); the OIDC redirect URL is built from it (§3.6) |
| `KUBEBENCH_AUTH_REQUIRE_GROUP` | `false` | v0.7: refuse external users whose groups grant nothing (§3.6) |
| `KUBEBENCH_AUTH_GROUP_MAPPINGS` | — | v0.7: JSON list of group mappings seeded once (§3.6) |
| `KUBEBENCH_OIDC_ISSUER`, `_CLIENT_ID`, `_CLIENT_SECRET` (or `_CLIENT_SECRET_FILE`), `_NAME`, `_SCOPES`, `_USERNAME_CLAIM`, `_GROUPS_CLAIM`, `_ALLOWED_DOMAINS`, `_CA_FILE` | — | v0.7: OpenID Connect sign-in (§3.6, `docs/configuration.md`) |
| `KUBEBENCH_LDAP_URL`, `_TYPE`, `_NAME`, `_START_TLS`, `_CA_FILE`, `_INSECURE_SKIP_VERIFY`, `_BIND_DN`, `_BIND_PASSWORD` (or `_BIND_PASSWORD_FILE`), `_USER_BASE_DN`, `_USER_FILTER`, `_USERNAME_ATTRIBUTE`, `_ID_ATTRIBUTE`, `_DISPLAY_NAME_ATTRIBUTE`, `_EMAIL_ATTRIBUTE`, `_GROUPS_FROM`, `_GROUP_BASE_DN`, `_GROUP_FILTER`, `_GROUP_NAME_ATTRIBUTE`, `_TIMEOUT` | — | v0.7: LDAP / Active Directory sign-in (§3.6, `docs/configuration.md`) |
| `KUBEBENCH_PROXY_ORIGIN` | — | v0.6: separate origin (`http(s)://host[:port]`, no path) serving the service browser's proxy unsandboxed (§7.14); unset = sandboxed on KubeBench's own origin |

The backend parses these once into `internal/config.Config`; invalid values
are all reported at once and the binary exits with status 2. Ports, in `PORT`
and `KUBEBENCH_LISTEN_ADDR`, are numbers from 1 to 65535.
`KUBEBENCH_ADMIN_PASSWORD` is only read for a new database and is checked
then: a value other than the default that breaks the password policy
(§3.5) stops the first start with status 2 before the admin is created.

"First run" means a new database (no users yet). Only then are the
bootstrap admin created and the inventory seeded from `KUBECONFIG` and, in
a pod, the in-cluster service account, so deleting a seeded cluster sticks
across restarts. Seeded kubeconfigs are stored flattened (referenced
certificate files embedded).

## 3. Roles, permissions, audit

### 3.1 Model

- Global user role (`users.role`): `admin` | `user` (unchanged).
- Per-cluster assignment role (`user_clusters.role`): `viewer` | `operator`.
  Existing assignments migrate to `operator` (preserves v0.4 behaviour).
- Cluster safety flags (`clusters` table): `read_only` (bool) and
  `environment` (`""` | `development` | `staging` | `production`).

Effective permission of a user on a cluster:

| | admin | operator | viewer |
|---|---|---|---|
| read (lists, YAML, events, logs, metrics) | ✓ | ✓ | ✓ |
| write (mutations) | ✓ | ✓ | ✗ |
| exec (shell, debug container, reveal secret) | ✓ | ✓ | ✗ |
| browse (service browser, §7.14; v0.6) | ✓ | ✓ | ✗ |
| contribute (shared notes, §7.13; v0.6; allowed on read-only clusters) | ✓ | ✓ | ✗ |
| manage users/clusters/plugins/audit | ✓ | ✗ | ✗ |

A **read-only cluster** rejects write and exec for everyone (admins
included) with `403 {"code":"cluster_read_only"}` (browse stays, limited to
GET and HEAD without WebSocket upgrades, §7.14); an admin clears the flag
in the Clusters view. Kubernetes-side authorization is still whatever the
stored credential allows.

### 3.2 Access levels in the API

Every route declares one of:

- `public` — no session (`/healthz`, `/api/version`, `POST /api/auth/login`).
- `authed` — any session (`/api/auth/*`, `/api/info`, `/api/clusters` GET,
  activate, `GET /api/plugins`, external plugin files `/plugins/{id}/…`).
- `read` — session + active cluster runtime + any role on it.
- `write` — `read` + role operator/admin + cluster not read-only. **Audited.**
- `exec` — same gate as write. **Audited.**
- `browse` (v0.6, §7.14) — `read` + role operator/admin; **not** refused on
  read-only clusters (the service browser then sends only GET/HEAD and
  opens no WebSockets).
  **Audited.**
- `admin` — global admin. Mutations audited.
- `contribute` (plugin routes only, v0.6) — `read` + role operator/admin;
  the cluster may be read-only. For KubeBench-side data about a cluster
  that other users of it see (shared notes, §7.13), not for cluster
  mutations. Mutations (non-GET) audited.

Every route is registered once (`backend/internal/api/routes.go`) with its
level. The router runs the gates in this order before the handler, so a
refused request never reaches the cluster: `password_change_required` →
`admin_required` → `plugin_disabled` (plugin routes) → active cluster
(`no_cluster`) → `cluster_read_only` (checked first, admins included) →
`forbidden_role` → `plugin_unavailable` (plugin routes).

Dry-run requests (exactly `dryRun=true`) need `read` and are not audited —
on routes that opt in (core flag `allowDryRun`, plugin flag
`plugin.DryRunnable`) and whose handlers then really perform a dry run
(`plugin.IsDryRun(r)`). The relaxation is per route because a handler that
ignored `dryRun` would otherwise let a viewer mutate.
One exception: a dry run that touches a core Secret or a kind holding
credentials (§7.15: Route, BuildConfig, Build; apply, edit) needs
`write` and is audited like the change itself (detail `"dryRun":true`),
because its result (`unchanged` or not) would let a viewer, who sees
Secret values only redacted, confirm guessed values. Helm install and
upgrade are not dry-runnable at all (§7.4): Helm's server-side dry run
runs template `lookup`s with KubeBench's own credential.

### 3.3 Error format

All errors are JSON: `{"error": "<human message>", "code": "<machine code>"}`,
plus `"reason": "<reason code>", "params": {…}` (v0.7) when KubeBench
explains the error itself: a stable `<group>.<name>` code (e.g.
`cluster.read_only`, `role.cannot_change`, `metrics.no_prometheus`,
`argocd.not_installed`) whose English template (`{{param}}` placeholders)
is `error`. Codes are defined once with `reason.Define` in the Go package
that emits them (`backend/internal/reason`); the UI translates them from
`src/core/reasons/locales` (namespace `reasons`, keys are the codes) and
shows `error` for unknown codes; a test requires a string with the same
placeholders for every defined code in every UI language, English equal
to the server's. Kubernetes API errors (`kubernetes`) carry no reason, and
a `detail` parameter holds text from elsewhere, untranslated. Status
objects explain themselves the same way: plugin `Status`
(`reasonCode`/`reasonParams`, set by `plugin.Unavailable`), watch topic
statuses, `/api/info` (`clusterErrorReason`/`clusterErrorParams`), the
update status (`errorReason`/`errorParams`) and the security report
(`skipped[].reasonCode`, `warningReasons`), apply results and storage
creates (`warningReasons`, apply errors with `reason`/`params`) and drain
results (`failed[].reason`). Streams end with a reason
message `{"error", "reason"?, "params"?}` (`reason.Message`: log stream
`event: error`, watch `event: end`, terminal error frames with
`reason`/`params` next to `message`, §7.1, §7.10); a message can be a
parameter of another reason (`{{cause}}` of `streams.closed`), which the UI
translates in place.
Codes used across the API: `unauthenticated`, `session_expired`,
`forbidden`, `forbidden_role`, `cluster_read_only`, `admin_required`,
`no_cluster`, `not_found`, `bad_request`, `conflict`, `rate_limited`,
`plugin_disabled`, `plugin_unavailable`, `capability_unavailable`,
`redacted_secret`, `password_change_required`, `kubernetes` (Kubernetes API error; HTTP status mirrors
the API status), `internal` (a KubeBench-side 500, e.g. the database).
The frontend `ApiError` exposes `status` and `code`.

- Unknown `/api` paths answer `404 not_found`; a known path with the wrong
  method answers `405 bad_request` with an `Allow` header.
- `no_cluster` is `409` when no usable cluster is active (none chosen, it
  was removed, or the assignment was revoked) and `503` when the active
  cluster's runtime cannot be built.
- A Kubernetes `401` (the stored credential was rejected) is reported as
  `502 kubernetes`, so the browser does not mistake it for the end of its
  own KubeBench session.
- Missing capabilities of the legacy endpoints (Prometheus-only metrics)
  answer `503 capability_unavailable`. So does a metric series
  whose Prometheus query fails, with the reason: only `source=auto` falls
  back to metrics-server, and only for CPU and memory, the metrics it has.
  metrics-server counts as available only while the sampler's latest
  scrape of `metrics.k8s.io` measured nodes; otherwise its series answer
  `503 capability_unavailable` with the reason, and the overview's and
  allocation's usage values are `null` with `usageUnavailable` (live mode
  never reports an unmeasured use as 0). On OpenShift, Prometheus is the
  monitoring stack's Thanos Querier, queried with the cluster credential's
  token (v0.8; docs/deployment.md#openshift-monitoring).
  The metrics endpoints wait (≤3 s) for a fresh runtime's capability probe
  and Prometheus discovery, like `/api/info` (§4.1). Helm is a plugin
  (§7.4): without access to the release Secrets it answers
  `503 plugin_unavailable`, and its Detect runs after the probe too.

### 3.4 Audit log

Table `audit_log(id INTEGER PK, ts TIMESTAMP, username TEXT, cluster TEXT,
action TEXT, target TEXT, status INTEGER, detail TEXT)`.
Written automatically by the router for `write`/`exec`/admin mutations:
`action` = `"<METHOD> <route pattern>"` (e.g. `POST /api/deployments/scale`)
or an explicit action name for plugins/websockets (`exec.session`;
`service.proxy`, `pod.proxy` and `ingress.proxy`, §7.14),
`target` = `<kind>/<ns>/<name>` style string from query/path values,
`status` = HTTP status, `detail` = short JSON (e.g. `{"replicas":3}`) or
error text. Pruned per `KUBEBENCH_AUDIT_RETENTION_DAYS`.

- Refused attempts are audited too: status 4xx, detail `"<code>: <message>"`.
- The default target kind is the first path segment after `/api/` (or
  `/api/plugins/<id>/`), made singular (`/api/deployments/scale` →
  `deployment`); `auditKinds` in `routes.go` overrides it per route. On the
  core routes under `/api/resource` and `/api/workloads/`, which act on the
  kind their `kind` parameter names, that parameter sets the target kind;
  every other route ignores it. Handlers may refine target and detail once
  they have read the body.
- Rows are bounded: action, target and detail keep at most 200, 512 and
  1000 characters (a handler whose record is a list that can be longer,
  such as a bulk delete, a user's cluster assignments
  (`{"assignments":["<clusterId>:<role>",…]}`), namespace labels
  (`{"labels":["k=v",…]}`), node taints or the keys of a metadata patch
  (`{"keys":["label:<key>","annotation:<key>",…]}`), writes it across
  several rows, each with `"part":"<i>/<n>"`). An audited request whose URL parameters do not fit (a
  target over 512 or a default detail over 1000 characters) is refused
  with `400 bad_request` before its handler runs, so a row always shows a
  command or any other parameter completely.
- Request bodies are never recorded (they may carry passwords or Secret
  data); dry runs are not audited, except those touching Secrets and
  Helm dry runs (§3.2).
- WebSocket rows are written when the connection is hijacked (status 101);
  extra rows written through a plugin's `Context.Audit` have status 0.

`GET /api/audit?limit=100&before=<id>&user=&cluster=&action=` (admin) →
`{"items":[{"id","ts","user","cluster","action","target","status","detail"}],"nextBefore":<id|null>}`.

### 3.5 Hardening

- SQLite opened with `_pragma=foreign_keys(1)`; deleting a user/cluster
  removes sessions/assignments; a deleted active cluster resets
  `active_cluster_id`.
- Login rate limit: 10 failed attempts per (client IP, username) per 15 min
  → `429 rate_limited` with `Retry-After`. Behind a trusted proxy the client
  IP is the last `X-Forwarded-For` entry. Wrong current passwords in
  `POST /api/auth/password` count against the same limit. The limiter keeps
  a digest of the username, at most 10 000 entries.
- Requests must be read within 1 minute (`ReadTimeout`; headers within
  10 s). The deadline ends once the request has been read, so streams and
  long operations are not limited.
- The SQLite file and its `-wal`/`-shm` files are created `0600` (existing
  ones are tightened on start): they hold cluster credentials and sessions.
- Non-GET requests with an `Origin` header that does not match the request
  host → `403 forbidden` (CSRF defence in depth; WebSocket upgrades too).
  `X-Forwarded-Host` is only compared when `KUBEBENCH_TRUST_PROXY` is set.
  A GET or HEAD of a write or exec route marked `Sec-Fetch-Site:
  cross-site` (a link or navigation from another site, which the Lax
  cookie accompanies) → `403 forbidden` too.
- Session cookie `kubebench_session`: HttpOnly, SameSite=Lax, Secure per
  `KUBEBENCH_SECURE_COOKIES`. Expired sessions pruned hourly.
- Session tokens (256 random bits, hex) are stored only as their SHA-256
  digest (`sessions.token_hash`, schema v6, v0.7) and looked up by it;
  the migration hashes the plain tokens of earlier versions in place, so
  live sessions survive the upgrade.
- A successful sign-in with the well-known default password `admin` sets
  `must_change_password` too, so accounts carried over from v0.4 (which had
  no forced change) cannot keep it.
- `users.must_change_password`: set for the bootstrap admin (the default
  password is well known; a configured one stays in the environment, an
  env file or the chart's Secret, readable by whoever may read that
  Secret, through KubeBench too), for users an admin creates, and when an
  admin resets a password — every password someone else knows. While set, every
  `read`/`write`/`exec`/`admin` route returns `403 {"code":"password_change_required"}`;
  the `authed` routes (`/api/auth/*`, `/api/info`, …) keep working.
- Passwords are 8 characters to 72 bytes (the bcrypt limit).
- Cannot delete yourself or demote/delete the last admin (`409 conflict`);
  since v0.7 the last **local** admin (§3.6).

### 3.6 External identities (v0.7)

Users can sign in through identity providers besides local accounts:
OpenID Connect and LDAP / Active Directory, each optional, both at once
possible. One shared layer (`backend/internal/identity`) serves every
provider.

**Providers.** A provider authenticates a user and reports an `Identity`
`{source, subject, username, displayName, email, groups}` (`source` =
`oidc` | `ldap`; `subject` is stable at the provider: OIDC `sub`, the
directory entry's ID). Redirect providers (OIDC, `identity/oidcauth`) run
their own browser flow; password providers implement
`identity.PasswordProvider.Authenticate(ctx, username, password)` and are
reached through `POST /api/auth/login {"source"}`; an optional
`identity.Lookuper.Lookup(ctx, username)` (a directory's service account)
serves the admin preview, and an optional
`identity.UsernameNormalizer` gives the name a login is throttled and
audited by. Refusals are `*identity.Failure` with a class:
`invalid_credentials`, `account_disabled`, `domain_not_allowed`,
`no_matching_group`, `username_conflict`, `invalid_username`,
`invalid_request`, `provider_error`, `rate_limited`. A failure that names
the account (`Subject`) ends that account's sessions.

**Users.** `users.source` is `local` | `oidc` | `ldap`; external users have
`external_id` (the subject, unique per source), `display_name`, `email`,
the groups of their last sign-in and no password: local sign-in, password
changes and resets refuse them. They are created just in time at their
first sign-in and matched by `(source, subject)` afterwards (a renamed
user keeps their account). A username already used by another account (a
local user, another source or subject), compared ignoring ASCII case,
refuses the sign-in (`username_conflict`): external sign-ins never take
over an account nor appear beside it as a case variant (`Admin` next to
the local `admin`). Creating a local user whose name differs from an
existing one only in case is a `409` too. The preview looks names up the
same way.

**Group mappings** (`auth_group_mappings`, schema v7): `{group, source, role,
cluster}` — `role` `admin` (global, no cluster) or `viewer`/`operator` on a
cluster named in the inventory; `source` `""` applies to every provider.
Groups match exactly, ignoring case. At **every** sign-in the user's global
role and all cluster assignments are re-computed from the mappings and
replace whatever an admin set manually (the UI says so); several mappings
for a cluster grant the strongest role; names without a cluster grant
nothing. A user whose groups grant nothing signs in and sees no cluster,
or is refused (`no_matching_group`) with `KUBEBENCH_AUTH_REQUIRE_GROUP`.
Admins manage mappings in Settings > Authentication (audited admin routes
under `/api/settings/auth`, listed in `docs/api.md`); `KUBEBENCH_AUTH_GROUP_MAPPINGS` seeds them once,
on the first start that has them (audited as `auth.mappings.seed`). The
preview ("who would I be", `POST /api/settings/auth/preview`) resolves the
mappings for a username without creating a user or a session.

**Break-glass.** Local accounts always work. The last-admin guard counts
only local admins, so the last local admin can be neither deleted nor
demoted (`409`), whoever asks; external admins lose the role whenever
their groups stop granting it.

**OIDC.** Authorization code flow with PKCE (S256), `state` and `nonce`.
The pending sign-in (state, nonce, verifier, redirect URL, a 10-minute
expiry) is sealed with AES-256-GCM under a key drawn at startup into the
HttpOnly, SameSite=Lax `kubebench_oidc_state` cookie, so a callback only
completes in the browser that started it and the server keeps no state per
started sign-in (a flood of starts cannot evict anyone's sign-in); the
callback clears the cookie and the provider redeems a code once. Discovery and
JWKS through `go-oidc` (retried until the provider is reachable); the ID
token's signature, issuer, audience, expiry and nonce are verified; the
groups claim is read from userinfo when the ID token lacks it; dotted claim
paths read nested claims. Allowed email domains admit only verified
addresses: `email_verified` must be `true` (or the string `"true"`); a
missing or other value is `domain_not_allowed`. The redirect URL is `KUBEBENCH_PUBLIC_URL` (or the request's
origin, forwarded headers only with `KUBEBENCH_TRUST_PROXY`) +
`/api/auth/oidc/callback`. Refusals redirect to `/?signInError=<class>`.
Sign-out answers the provider's end-session URL (`logoutUrl`, with the
session's ID token as hint) when the provider supports RP-initiated
logout.

**LDAP / Active Directory** (`identity/ldapauth`, `go-ldap`). One
directory (`KUBEBENCH_LDAP_URL`, `ldaps://` or `ldap://` with
`KUBEBENCH_LDAP_START_TLS`); `KUBEBENCH_LDAP_TYPE` `openldap` | `ad` picks
the defaults below, each overridable. Sign-in:
1. The login is normalized: trimmed, lower case, a `DOMAIN\` prefix dropped
   (a UPN `user@domain` stays; the AD filter matches it). An empty
   password is refused before anything is sent: a simple bind with a DN
   and no password is an *unauthenticated* bind many directories accept.
2. The service account (`_BIND_DN`; anonymous without one) searches
   `_USER_BASE_DN` with `_USER_FILTER`, `%s` = the escaped login
   (defaults `(uid=%s)`; AD `(|(sAMAccountName=%s)(userPrincipalName=%s))`).
   No entry or more than one is `invalid_credentials`.
3. A set ACCOUNTDISABLE bit (2) of `userAccountControl` refuses the account,
   and its sessions end; it is reported as `account_disabled` only when the
   password is right, `invalid_credentials` otherwise. A bind refused with
   AD's `data 533`/`data 701` (disabled/expired) is `account_disabled` too.
4. The password is checked by a bind as the user's DN on a connection of
   its own (never by comparing hashes).
5. Groups, as the service account: from the entry's `memberOf`
   (`_GROUPS_FROM=memberOf`, the first RDN value) or a search under
   `_GROUP_BASE_DN` (default: the user base) with `_GROUP_FILTER`, `%d` =
   the user's DN and `%u` = its username attribute (defaults
   `(|(member=%d)(uniqueMember=%d)(memberUid=%u))`; AD
   `(member:1.2.840.113556.1.4.1941:=%d)`, LDAP_MATCHING_RULE_IN_CHAIN, so
   nested groups count). A group's name is its `_GROUP_NAME_ATTRIBUTE`
   (`cn`; `dn` reports full DNs).
6. The identity: username = the username attribute (`uid`; AD
   `sAMAccountName`) in lower case, subject = `_ID_ATTRIBUTE` (`entryUUID`;
   AD `objectGUID` in its string form), display name and email.
Connections time out after `_TIMEOUT` (10s, also the server-side search
time limit); up to 4 idle service-account connections are reused (retired
after 5 minutes idle, replaced once when the directory dropped them) and at
most 8 directory operations run at once. Refused sign-ins answer the §3.3
error body with the reason code `auth.signin.<class>`, which the sign-in
screen translates like the failure classes of OIDC sign-ins.

**Sessions.** External sign-ins get the ordinary session cookie and
`KUBEBENCH_SESSION_TTL`. There is no background sync in v0.7: a disabled
account loses its sessions when a sign-in attempt reports it disabled.

**Security.** Secrets (the OIDC client secret, the LDAP bind password)
come from the environment, `*_FILE` files or Kubernetes Secrets only; they
are never returned by an API (settings report `secretSet`), logged or
audited. TLS to the provider is verified (system roots plus
`KUBEBENCH_OIDC_CA_FILE` / `KUBEBENCH_LDAP_CA_FILE`);
`KUBEBENCH_LDAP_INSECURE_SKIP_VERIFY` (tests only) and an unencrypted
`ldap://` URL are logged as warnings at startup and shown as warnings in
Settings > Authentication. Callbacks refused after the provider redeemed
their code (not those with a wrong state or a refused code, which anyone
can send) and failed password-provider sign-ins count against the login
limit (§3.5; callbacks per client IP, directory
sign-ins per client IP and normalized login).
Every external sign-in writes an audit row (`auth.signin`, target
`user/<name>`, detail `{"source"}` or `{"source","reason":"<class>"}`), as
does every mapping change (target `mapping/<group>`).

## 4. API changes (core)

### 4.1 `/api/info`

```json
{
  "version": "0.5.0", "commit": "abc1234",
  "user": "alice", "role": "admin",
  "mustChangePassword": false,
  "context": "k3s-homelab", "server": "https://10.0.0.1:6443",
  "cluster": {"id": 1, "name": "k3s-homelab", "environment": "production", "readOnly": false,
              "platform": {"provider": "k3s", "cloud": "", "version": "v1.35.5+k3s1",
                           "support": {"status": "supported", "endDate": "2027-02-28", "basis": "kubernetes"}, "detected": true}},
  "permissions": {"role": "admin", "canRead": true, "canWrite": true, "canExec": true, "isAdmin": true, "readOnly": false},
  "capabilities": {"crds": true, "helm": true, "exec": true},
  "features": {"pathKubeconfig": true, "externalPlugins": true, "ambientCloudIdentity": false},
  "clusterError": ""
}
```

`permissions.role` is `admin` | `operator` | `viewer` (effective role on the
active cluster). `/api/info` waits (≤3 s) for the runtime's capability probe
to finish so capabilities are never reported `false` merely because the
probe is still running.

When the caller has no usable active cluster, `/api/info` activates the
first cluster they may use (the app reads it at boot, so nothing in flight
is surprised). It is the only route that switches clusters: every other
route answers `409 no_cluster` instead. Without any usable cluster,
`cluster` is `null` and `clusterError` says why.

`POST /api/auth/login` and `GET /api/auth/me` return
`{"id","username","role","mustChangePassword","source","displayName"}`
(`source` since v0.7, §3.6); `GET /api/auth/me` adds `"lastSeenVersion"`
(§7.11). `/api/info` reports the account's source as `"authSource"` (v0.7).

### 4.2 Users & password

- `GET /api/users` (admin) → `[{"id","username","role","mustChangePassword","clusters":["name",…],"assignments":[{"clusterId","cluster","role"}]}]`
  (`clusters` kept for compatibility).
- `POST /api/users` (admin) `{"username","password","role"}` → `{"id"}`; the
  new user has `mustChangePassword` set.
- `PATCH /api/users/{id}` (admin) `{"role"?: "admin"|"user", "password"?: "…"}`
  (a password reset sets `mustChangePassword` and ends the user's sessions,
  except the caller's own when admins reset their own password).
- `DELETE /api/users/{id}` (admin).
- `PUT /api/users/{id}/clusters` (admin) accepts `{"assignments":[{"clusterId":1,"role":"viewer"}]}`
  or legacy `{"clusterIds":[1,2]}` (→ operator).
- `POST /api/auth/password` (authed) `{"currentPassword","newPassword"}`
  (8 characters to 72 bytes and different from the current one, else `400`;
  a wrong current password is `403 forbidden`, throttled like logins;
  clears `mustChangePassword`; other sessions of the user are revoked).

### 4.3 Clusters

- `GET /api/clusters` entries gain `"readOnly"`, `"environment"`, `"role"`
  (the caller's effective role on it) and `"platform"` (§7.12; admins also
  `"cloudAuth"`, `"cloudCredentials"`).
- `PATCH /api/clusters/{id}` (admin) `{"name"?, "readOnly"?, "environment"?, "credentials"?}` (§7.12).
- `POST /api/clusters` (admin) `{"name", "content"? | "path"? | "address"?, "context"?, "credentials"?}`
  → `{"ok": true, "id"}`. With `path` it is rejected with `403 forbidden`
  unless `features.pathKubeconfig`. Unless `KUBEBENCH_ALLOW_KUBECONFIG_EXEC`
  is set, pasted kubeconfigs whose selected context reads server-side files
  (certificate authority, client certificate/key, token file) or runs exec
  plugins or auth providers are rejected the same way: pasted content is
  untrusted, and would otherwise run commands or read files as the server's
  account. A kubeconfig added by path is one of the server's own files and
  may use them, unless it lies in a directory KubeBench writes with
  content from requests (a plugin's data directory under
  `KUBEBENCH_DATA_DIR`, e.g. the Helm repository cache, or
  `KUBEBENCH_PLUGINS_DIR`; symbolic links resolved): such a path is
  refused with `403 forbidden`. The EKS, GKE and AKS exec plugins (`aws eks get-token`,
  `aws-iam-authenticator`, `gke-gcloud-auth-plugin`, `kubelogin get-token`)
  are exempt: they are never run but replaced in process (§7.12), with
  per-cluster `"credentials"` or the server's ambient cloud identity; a
  pasted kubeconfig may use the latter only when
  `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY` is set (otherwise `403
  forbidden`).
  Kubeconfigs added by path are stored flattened (referenced
  certificate files embedded).

### 4.4 Resources (fixes)

- `/api/events?kind=&ns=&name=` filters on `involvedObject.kind` as well as
  name (fixes cross-kind mixing, e.g. Deployment `web` vs Service `web`).
- `/api/logs` accepts `tail` (default 300, max 5000), `previous=true`,
  `sinceSeconds`.
- Secret values are redacted on every read route (`/api/resource`,
  `/api/resource/get`, `/api/yaml`): `data`, `stringData` and kubectl's
  `last-applied-configuration` annotation, which repeats them. So are the
  credentials of the OpenShift kinds that hold them inline (§7.15).

### 4.5 Kubernetes read passthrough (for external plugins)

`GET /api/k8s/{path...}` (read) proxies **GET only** to the cluster API
(`/api/...`, `/apis/...`, `/version`), returning the raw JSON. Requests whose
path targets `secrets` are rejected (`403 forbidden`; use `/api/resource`).
`watch=true` is rejected (use the live-updates SSE). Response size capped
(16 MiB).

- Also rejected: `/watch/` paths and `follow=true` (`400`), and the
  `proxy`, `exec`, `attach` and `portforward` subresources (`403`). Boolean
  flags are read the way the API server reads them (anything but `0` or
  `false` is true).
- Upstream errors become `{"error": <Status.message>, "code": "kubernetes"}`
  with the upstream status (a `401` becomes `502`, see §3.3).
- Redirects are never followed, here or by the runtime's typed, dynamic
  and metrics clients (the stored credential would go along), and an
  upstream `3xx` becomes `502 kubernetes`.
- Responses are served as `application/json` (JSON upstream) or
  `text/plain` (logs), anything else as `application/octet-stream`, with
  `X-Content-Type-Options: nosniff` and `Content-Security-Policy: sandbox;
  default-src 'none'`: the upstream type is never echoed.

### 4.6 Plugins

- `GET /api/plugins` (authed) →
  ```json
  {"plugins": [{"id":"argocd","name":"Argo CD","version":"0.5.0","description":"…",
                "category":"gitops","builtin":true,"external":false,"enabled":true,
                "status":{"available":true,"reason":""},"entry":""}],
   "settings": {"topology": {"enabled": false}}}
  ```
  `plugins` lists backend plugins (with `status` for the caller's active
  cluster; Detect results cached 60 s per cluster) and external plugins
  (`external: true`, `entry: "/plugins/<id>/<entry>"`). `settings` holds the
  admin's enable/disable choices for **any** plugin id, including
  frontend-only plugins the backend does not know.
- `PUT /api/plugins/{id}` (admin) `{"enabled": bool}` — id must match
  `^[a-z0-9][a-z0-9-]{0,39}$`.
- Backend plugin routes live under `/api/plugins/{id}/…`; disabled →
  `404 plugin_disabled`; `Detect` unavailable → `503 plugin_unavailable`.
- External plugin files: `GET /plugins/{id}/{file...}` (authed) served from
  `$KUBEBENCH_PLUGINS_DIR/<id>/` through `os.Root` (no traversal, no
  symlink escapes, no directory listings), only for valid and enabled
  external plugins (disabled → `404 plugin_disabled`) and only `.js`,
  `.mjs`, `.css`, `.json`, `.map`, `.svg` and `.png` files. `plugin.json`
  may carry an optional `category` (default `tools`). An external plugin
  whose id belongs to a built-in backend plugin is skipped; one whose id
  belongs to a built-in frontend plugin is never loaded, its catalog entry
  does not affect the built-in plugin, and Extensions shows the clash.

## 5. Backend plugin contract (`backend/internal/plugin`)

```go
package plugin

type Access int
const (
    AccessRead Access = iota + 1
    AccessWrite
    AccessExec
    AccessAdmin
    // AccessContribute (v0.6): AccessRead + operator/admin role, the
    // cluster may be read-only; non-GET requests audited (§3.2, §7.13).
    AccessContribute
)

// DryRunnable is combined with AccessWrite or AccessExec for a route whose
// handler performs a dry run whenever IsDryRun(r): such requests are
// checked as AccessRead and not audited (§3.2).
const DryRunnable Access = 1 << 8

func IsDryRun(r *http.Request) bool // exactly dryRun=true

type Manifest struct {
    ID, Name, Version, Description, Category string
}

type Status struct {
    Available bool   `json:"available"`
    Reason    string `json:"reason,omitempty"`
}

// Context is what a plugin handler gets for one request.
type Context struct {
    Runtime *kube.Runtime   // the caller's active cluster
    User    *store.User
    Role    string          // the caller's role on it: admin | operator | viewer (v0.6)
    Store   *store.Store    // for plugins with their own tables (store migrations), e.g. notes (v0.6)
    KV      KV              // plugin-scoped persistent key/value (SQLite plugin_kv)
    Audit   func(action, target, detail string) // extra audit rows if needed
    // SetAudit refines the router's row of this (audited) request once the
    // handler has read the body, e.g. target "release/web/shop"; never secrets.
    SetAudit func(target, detail string)
}

type KV interface {
    Get(key string) (string, bool, error)
    Set(key, value string) error
    Delete(key string) error
    List(prefix string) (map[string]string, error)
}

type HandlerFunc func(w http.ResponseWriter, r *http.Request, c *Context)

type Router interface {
    // Handle registers e.g. ("GET", "/applications/{ns}/{name}", AccessRead, h)
    // mounted at /api/plugins/<id>/applications/{ns}/{name}.
    Handle(method, path string, access Access, h HandlerFunc)
}

type Plugin interface {
    Manifest() Manifest
    Routes(r Router)
    Detect(ctx context.Context, rt *kube.Runtime) Status
}

func Register(p Plugin)            // called from init() of each plugin package
func All() []Plugin
func Get(id string) (Plugin, bool)

// Optional: plugins that keep files or run background work implement
// Starter. main.go calls StartAll once before serving; DataDir is
// $KUBEBENCH_DATA_DIR/<id> (created 0700), ctx ends at shutdown.
type Host struct{ DataDir string }
type Starter interface{ Start(ctx context.Context, host Host) error }
func StartAll(ctx context.Context, dataDir string, plugins []Plugin) error

// Helpers shared with core handlers:
func WriteJSON(w http.ResponseWriter, v any)
func WriteError(w http.ResponseWriter, status int, code string, err error)
func WriteK8sError(w http.ResponseWriter, err error) // maps apierrors → status, code "kubernetes"
```

- Built-in plugins live in `backend/internal/plugins/<id>/` and are linked
  via blank imports in `backend/internal/plugins/all/all.go`
  (`main.go` imports `_ "kubebench/internal/plugins/all"`).
- KV keys are scoped per plugin (`plugin_kv(plugin_id, key, value, updated_at)`);
  per-cluster data should prefix keys with `cluster/<clusterID>/`.
- `kube.Runtime` exposes (foundation): `CS`, `Dyn`, `Metrics`, `Config`,
  `Discovery()`, `Mapper()` (cached deferred discovery RESTMapper),
  `ResolveKind(apiVersion, kind) (gvr, namespaced, error)`,
  `ResolveResource(group, resource)`, `Cluster` (store.Cluster incl. ID),
  `Caps()` and `WaitProbed(ctx)` (the capability probe), `RawGet` (the
  §4.5 passthrough). `ResolveKind`/`ResolveResource` reset the mapper and
  retry once on a no-match error (e.g. a CRD installed since the last
  discovery), at most once per 10 s per runtime, so plugins should prefer
  them over `Mapper()`.
  `kube.NewRuntimeForTest(cluster, cs, dyn)` builds one around fake clients
  without background work.
- `WriteK8sError` reports a Kubernetes `401` as `502` (§3.3); a no-match
  from the RESTMapper is `404 not_found`.
- Plugin route paths use `net/http` ServeMux wildcards. `Context.Runtime`
  is the caller's active cluster; requests never switch clusters.

## 6. Frontend contracts

### 6.1 i18n (`src/i18n`)

Already in place: `i18next` + `react-i18next`, languages `en`, `tr`, `de`;
`core` namespace in `src/i18n/locales/<lang>.json`; plugin namespaces via
`PluginManifest.locales` (files `src/plugins/<id>/locales/<lang>.json`, loaded
with `localeBundles(import.meta.glob('./locales/*.json', …))`);
`useManifestText()`/`translateManifestText()` for manifest strings;
`setLanguagePreference()`; `formatNumber/formatDateTime/formatBytes` in
`src/i18n/format.ts`; `npm run i18n:check`.

The language set is data: every `src/i18n/locales/<code>.json` is a
supported language, named in Settings by its own `language.name`; detection
matches the browser's tags exactly, then by base language (`de-AT` → `de`),
and formatting uses the browser's regional variant of the UI language.
Adding a language means adding `<code>.json` next to every `en.json` (and,
optionally, Monaco's message pack in `src/shared/editor/monacoLocale.ts`);
`npm run i18n:check` requires every locales directory to ship it.

Conventions: nested keys grouped by screen (`releases.columns.chart`);
`{{var}}` interpolation; plurals via i18next `_one`/`_other` suffixes;
Kubernetes nouns (Pod, Deployment, namespace, …) stay untranslated in `tr`
and `de` (German: formal "Sie", VS Code terminology).

### 6.2 API client (`src/data/apiClient.ts`)

`apiFetch<T>(path, init?)` (prefix `/api`, JSON in/out, throws
`ApiError {status, code, message}`), `qs(params)` (URL-encodes every value;
skips undefined/empty), helpers `apiGet/apiPost/apiPut/apiPatch/apiDelete`.
All providers use it; no hand-built query strings.

### 6.3 Session, permissions, mode

- `src/data/mode.ts` `DataMode` gains `version`, `permissions`, `cluster`,
  `features`, `mustChangePassword`.
- `src/core/permissions.ts`: `usePermissions()` → `{role, canRead, canWrite,
  canExec, isAdmin, readOnly}` (mock mode: everything true) and
  `<Can perm="write|exec|admin" fallback?>`. Mutation UI is hidden or
  disabled (with a tooltip explaining why) when not permitted.
- **Mock mode policy**: mock only when `import.meta.env.DEV`, or the URL has
  `?demo`, or the build sets `VITE_DEMO_MODE=true`. A production build whose
  backend is unreachable shows a boot error screen with Retry — never mock.
  `?demo` and `VITE_DEMO_MODE=true` force demo data without contacting the
  backend; the dev server falls back to it only when the backend is
  unreachable or answers 5xx (a `401` always shows the sign-in screen).
  Sign-out is disabled in demo mode, which has no session.
- Status bar shows the cluster environment (production = red background,
  staging = amber, development = green) and a "Read-only" item when
  `readOnly` or role viewer; the user item opens a menu (Change password,
  Sign out). The About command shows version/commit.

### 6.4 Plugin SDK v2 (`src/core/plugin`)

`PluginManifest` additions:

```ts
version?: string            // shown in Extensions view
description?: string        // i18n key
category?: 'core' | 'workloads' | 'gitops' | 'storage' | 'security' | 'observability' | 'tools'
core?: boolean              // cannot be disabled (workloads, clusters, settings, terminal, editor, extensions, users, audit)
backendPlugin?: string      // backend plugin id this plugin needs; unavailable → greyed with reason
locales?: Record<string, object>
resourceActions?: ResourceActionContribution[]
settingsSections?: SettingsSectionContribution[]
```

```ts
interface ResourceRef { kind: string; apiVersion?: string; namespace?: string; name: string }
interface ResourceActionContribution {
  id: string
  title: string              // i18n key in the plugin namespace
  icon?: string
  kinds: string[]            // e.g. ['Deployment','Service'] or ['*']
  permission?: 'read' | 'write' | 'exec'
  run: (ref: ResourceRef) => void
  badge?: ComponentType<{ resource: ResourceRef }>  // v0.6: in the detail-page button, e.g. a note count
}
interface SettingsSectionContribution { id: string; title: string; component: ComponentType }
```

- The registry filters disabled/unavailable plugins out of the activity
  bar, sidebar, commands and resource actions, using a zustand
  `pluginStatusStore` loaded from `/api/plugins` at boot (mock mode: all
  enabled & available). Built-in frontend plugins default to enabled;
  external plugins default to **disabled** until an admin enables them.
- `registry.resourceActionsFor(kind)` feeds row context menus and detail
  page toolbars in the workloads/CRD views, through
  `resourceActionItems(ref)` (`core/plugin/resourceActions.ts`) and
  `<ResourceActionButtons resource>`; an action whose permission is not
  held is shown disabled with the reason.
- **Extensions** plugin (core): activity-bar entry listing every plugin
  (name, version, category, builtin/external, enabled, availability +
  reason); admins toggle enable/disable.
- Additions to the shell contract:
  - `TabDescriptor.titleKey` / `titleNs` / `titleValues`: the tab strip
    shows the title translated from that key, so open tabs follow language
    changes; `title` is the fallback.
  - Activity bar entries without `order` sort at 50: after the built-in
    views (0–6), before Extensions (90).
  - Bottom panel tabs may list request `kinds`; `showInPanel(kind, props)`
    (`core/plugin/panelRequests.ts`) shows a request in the tab that handles
    it, e.g. `pod-logs` / `pod-exec` with `{namespace, pod, container}`
    (the terminal plugin). Like `openTab`, it keeps plugins from importing
    each other.
  - v0.6: the side editor group (§7.14): `openInSideGroup(tab)`
    (`core/plugin/sideGroup.ts`) opens a tab to the right of the editor
    area; tabs there stay mounted while inactive.
  - External plugins register only under the id of an enabled external
    plugin whose script is loading, and never as `core`.
  - `CommandContribution.keybinding` (e.g. `"Ctrl+,"`, `"Alt+N"`, `"?"`;
    Ctrl is Cmd on macOS) binds a global shortcut, shown in the palette and
    in the shortcuts list (`?`, Settings); never inside terminals, plain
    keys never while typing, and none when Settings turns shortcuts off.
  - `PluginManifest.resourceKinds` (`ResourceKindContribution`: kind,
    kubectl names, `openList(ns?)` — `null` lists all namespaces for `-A` —, `openDetail`, `listObjects`) feeds the
    palette's kubectl-style commands `get`/`describe`; `logs`, `exec`, `ns`
    and `ctx` are core and respect the read/exec permissions. Aliases are
    kept per user in the browser. Built-in tab pages are loaded lazily with
    `lazyComponent()` (`core/plugin/lazyComponent.tsx`).

### 6.5 Plugin-owned data (`src/data/pluginData.ts`)

```ts
export function pluginApi(pluginId: string): {
  get<T>(path: string, params?: Record<string, unknown>): Promise<T>
  post<T>(path: string, body?: unknown, params?: Record<string, unknown>): Promise<T>
  put<T>(…), patch<T>(…), del<T>(path, params?)
}   // → /api/plugins/<pluginId><path>

export function definePluginData<T>(pluginId: string, impl: {
  mock: () => T
  live: (api: ReturnType<typeof pluginApi>) => T
}): () => T          // memoized; picks by dataMode.mode, never falls back

export function usePluginQuery<T>(fn: () => Promise<T>, deps: DependencyList,
  scope?: { topics?: readonly string[]; namespace?: string }): QueryState<T>   // scope: §7.10
export function usePluginStatus(pluginId: string): { enabled: boolean; available: boolean; reason?: string; loading: boolean }
```

Plugin capabilities live in `src/plugins/<id>/data/` (`types.ts`,
`provider.ts` interface, `mock.ts`, `live.ts`, `index.ts` exporting the
`definePluginData` getter). Core capabilities stay in `src/data/providers.ts`.

### 6.6 Shared UI building blocks (`src/shared`)

`Dialog` (modal; Esc closes; focus first field), `ConfirmDialog`
(danger variant; **type-to-confirm** the resource name when the cluster
environment is `production` or the action is irreversible at scale),
`Form` primitives (`Field`, `TextInput`, `NumberInput`, `Select`,
`Checkbox`, `Textarea`, `KeyValueEditor`), `notify` toasts
(`notify.success/error/info`, `<Toaster/>` mounted by the shell),
`Tabs` (in-page tab strip), `Unavailable` (EmptyState variant with reason),
`CodeEditor` / `DiffEditor` (Monaco **bundled locally** — no CDN; lazy
loaded; theme-aware; languages yaml/json/plaintext/shell, markdown since
v0.6). The safe Markdown renderer (`src/shared/markdown`: no HTML,
`http(s)` links only) serves release notes and notes (§7.11, §7.13).

Destructive and rollback actions of the built-in views (Pod delete, Helm
uninstall and rollback) ask through `ConfirmDialog`, so production
clusters always require the name to be typed.

### 6.7 Themes

`settings.theme`: `dark` | `light` | `system`; `<html data-theme>`;
`theme.css` defines every `--vscode-*` token for both themes (Dark+ /
Light+ palettes). Monaco follows (`vs-dark` / `vs`).

**Interface scale and fonts** (v0.7): `core/store/displayStore.ts`, kept
in local storage per signed-in user (`<mode>:<user>`, like the palette
aliases): `uiScale` 90–130% (CSS `zoom` on `<body>`, so Monaco, which
divides by the zoom of its ancestors below `<html>`, hit-tests correctly;
code that mixes pointer or `getBoundingClientRect()` coordinates with CSS
pixels converts through `shared/zoom.ts`), editor and terminal font
family (a CSS family list, validated, put ahead of `--font-mono`) and
size (the terminal size defaults to the browser-wide `terminalFontSize`
of v0.6). Monaco re-measures its fonts on a change; xterm.js gets the
family and size through `setAppearance`. xterm.js maps pointer positions
to cells without regard to zoom, so the element it opens into cancels the
scale (`zoom: 1/scale`) and its font size is multiplied by the scale
instead (`terminal/shell/scale.ts`); log views use
`--terminal-font-family`/`--terminal-font-size`.

### 6.8 External (runtime) plugins

- Layout on disk: `$KUBEBENCH_PLUGINS_DIR/<id>/plugin.json`
  (`{"id","name","version","description","entry":"index.js"}`) + the IIFE
  bundle.
- Boot: after `/api/info`, the app fetches `/api/plugins`, injects
  `<script src="/plugins/<id>/index.js">` for each enabled external plugin
  (5 s timeout each, failures reported in Extensions), then renders.
- The bundle calls `window.KubeBench.registerPlugin(manifest)`.
  `window.KubeBench` (`src/core/plugin/globalSdk.ts`) exposes
  `{ apiVersion: 1, version, React, jsxRuntime, registerPlugin, i18n: {t, registerTranslations, useTranslation}, api: {apiFetch, pluginApi, k8sGet}, data: {usePluginQuery, refreshData}, layout: {openTab}, permissions: {usePermissions}, ui: {Table, Badge, EmptyState, Toolbar, Dialog, ConfirmDialog, CodeEditor, notify} }`.
  `i18n.useTranslation(ns)` is a hook returning `t` bound to the namespace
  that re-renders on language changes. Types for plugin authors:
  `src/core/plugin/sdk/kubebench.d.ts`, type-checked against the
  implementation.
- Example: `examples/plugins/hello-cluster` (Vite library build, IIFE,
  `react` → `KubeBench.React` globals). Excluded from the app build.

## 7. Feature specifications

Tab kind contract that stays stable: `pod-exec` and `pod-logs` with props
`{namespace, pod, container}`; `yaml` with `{text, readOnly}` or `{ref}`
and `new-resource` with `{namespace?, kind?}` (see 7.2);
`workloads-detail`, `generic-detail`, `crd-instances`.

### 7.1 Terminal & logs (terminal plugin)

- `GET /api/exec/ws?ns=&pod=&container=&command=` (exec, audited as
  `exec.session` at start) — WebSocket. Client→server text frames
  `{"type":"input","data":"…"}` / `{"type":"resize","cols":N,"rows":N}`;
  server→client **binary** frames = terminal output, text frames
  `{"type":"exit","code":N}` / `{"type":"error","message":"…"}` (plus
  `reason`/`params`, §3.3).
  TTY + stdin; default command tries `bash` then `sh`. Uses client-go
  `remotecommand` (WebSocket executor with SPDY fallback) and a
  `TerminalSizeQueue`. Origin checked; 30 s ping; idle timeout 30 min.
- `GET /api/logs/stream?ns=&pod=&container=&tail=&previous=&timestamps=&sinceSeconds=`
  (read) — Server-Sent Events: first `event: pods` with
  `{"pods":["<pod>/<container>"],"total":N}` (followed containers, matching
  pods), then `data:` one log line per event, `event: error` (a reason
  message, §3.3), `event: end`, `: ping` every 15 s. Also `?selector=<labelSelector>` (≤10 pods, lines
  prefixed `[pod/container]`). Failures before the stream starts are JSON
  errors (§3.3).
- Both are authorized when they start and checked again at every ping
  (terminal) or heartbeat (logs): once the session ends (logout, password
  change, user deleted), a password change is due, the cluster is removed,
  the caller's assignment is gone or, for a terminal, the role or the
  cluster's read-only flag no longer allows exec, the terminal ends with
  `{"type":"error","message":"the session was closed: …"}` (reason
  `streams.closed` with the refusal as `cause`) and the log stream with
  `event: error` carrying the same reason message. Switching the active
  cluster does not end them. A user may have at most 10 terminals and 20
  log streams open; more are refused with `429 rate_limited` before the
  upgrade or stream starts. A log stream whose client stops reading ends
  once a write has waited 10 s.
- When the server shuts down (SIGINT/SIGTERM) it ends them before the
  HTTP server's graceful shutdown (10 s): every terminal is hung up like
  an idle one (Ctrl+C, Ctrl+D, then cut after 5 s) and ends with
  `{"type":"error","message":"the server is shutting down"}` (reason
  `api.shutting_down`), log streams end with `event: error` and watch
  streams (§7.10) with `event: end`, both carrying that reason message. The server waits for the terminals within the
  grace period; new terminals are refused with `503`.
- UI: xterm.js terminal (`@xterm/xterm` + fit addon), multiple sessions as
  sub-tabs in the bottom Terminal panel, container picker, reconnect;
  logs viewer with follow/pause, tail size, previous container, timestamps,
  wrap, filter, level colouring, download, 10 000-line ring buffer.
  Mock mode: simulated terminal + deterministic simulated stream.

### 7.2 Resource editing (core + editor/workloads plugins)

- `POST /api/resource/apply?dryRun=&force=&ns=` (write; exactly
  `dryRun=true` → read and not audited, a real server-side dry run, unless
  a document is a Secret: then write and audited, §3.2). Body:
  YAML or JSON, any number of documents (`---`-separated YAML or
  concatenated JSON; `kind: List` is flattened, empty documents skipped),
  at most 4 MiB (`413`). Server-side apply with `fieldManager=kubebench`,
  mapping through the RESTMapper (`ResolveKind`). Namespaced objects that
  name no namespace go to `ns`, else `default`; the namespace of a
  cluster-scoped object is ignored with a warning. `dryRun` and `force`
  accept only `true`/`false`.
  - The whole request is refused (`400`) before anything is applied when a
    document is not valid YAML/JSON or lacks apiVersion, kind or
    metadata.name (`bad_request`, the message names the document), or is a
    Secret whose data/stringData values or last-applied annotation are
    redaction placeholders, or a Route, BuildConfig or Build whose
    credentials (§7.15) or last-applied annotation are (`redacted_secret`).
  - Otherwise every object is attempted in order and the answer is `200`:
    `{"results":[{"apiVersion","kind","namespace"?,"name","action"?:"created|configured|unchanged","error"?,"code"?,"reason"?,"params"?,"warnings"?,"warningReasons"?}]}`.
    A failed object carries `error` and `code` instead of `action`:
    `conflict` for a server-side apply conflict with another field manager
    (applying again with `force=true` takes the fields over), `not_found`
    for a kind the cluster does not serve, `kubernetes` otherwise.
    `unchanged`: the object's content without managedFields,
    resourceVersion and status is the same after the apply.
  - Audit target: the first object, `<kind>/<ns>/<name>` with
    ` (+N more)`; detail: the outcome counts, e.g.
    `{"configured":1,"failed":1}`, and `"force":true`.
- `POST /api/resource/edit?apiVersion=&kind=` or `?key=` `&ns=&name=&dryRun=`
  (write; exactly `dryRun=true` → read and not audited, a real dry run,
  except for Secrets, §3.2). Body `{"original","edited"}`: the object's YAML
  or JSON as editing started from and as edited (one document each, at most
  8 MiB together). The YAML editor's Apply: unlike server-side apply, which
  keeps fields other managers own when a manifest leaves them out, the edit
  is sent like `kubectl edit` as the JSON merge patch from original to
  edited (`fieldManager=kubebench`), so a removed field is removed and
  fields the edit leaves alone keep their live values. Both documents must
  be the named object (apiVersion, kind, name; the namespace may be
  omitted), else `400 bad_request`. Redaction placeholders left unchanged
  are not part of the patch, so a Secret's labels can be edited without
  revealing it; a value changed to a placeholder is `400 redacted_secret`,
  likewise for the credentials of §7.15's kinds.
  Answers like apply with one result (`configured` or `unchanged`; nothing
  is sent without changes); audit target `<kind>/<ns>/<name>`, detail the
  outcome count.
- `GET /api/yaml?apiVersion=&kind=&ns=&name=&neat=true` (read). With
  `apiVersion`, any kind the cluster serves (RESTMapper; a namespaced kind
  needs `ns`); without it, the kinds v0.4 knew by name (`kind`, `name`
  required). managedFields are always stripped; `neat=true` also strips
  status, uid, resourceVersion, generation, creationTimestamp (and the null
  creationTimestamp of templates), selfLink and the
  `kubectl.kubernetes.io/last-applied-configuration` and
  `deployment.kubernetes.io/revision` annotations, dropping annotations
  left empty. Secret values are redacted here as on every read but the
  reveal, and so is the last-applied annotation of Secrets, which repeats
  them.
- `DELETE /api/resource?apiVersion=&kind=` or `?key=` (a generic catalog
  key) `&ns=&name=&propagation=Background|Foreground|Orphan&gracePeriod=`
  (write). Namespaces are refused (`400`): they are deleted with
  `DELETE /api/namespaces` (§7.3) and its protections. Audit target
  `<kind>/<ns>/<name>`.
- `PATCH /api/resource/metadata?apiVersion=&kind=` or `?key=` `&ns=&name=`
  (write), body `{"labels":{…},"annotations":{…}}`: a JSON merge patch, a
  null value removes the key. The audit detail lists the changed keys,
  never values. The labels or annotations of a Secret, Route, BuildConfig
  or Build cannot be set to a redaction placeholder (`redacted_secret`).
- `POST /api/secrets/reveal?ns=&name=` (exec) → the Secret with its values
  (`data`, base64; managedFields stripped), `Cache-Control: no-store`. It
  has its own route because the access level belongs to the route: a
  query flag on `GET /api/resource/get` could not raise it. It changes
  nothing but is audited like every exec request (action
  `POST /api/secrets/reveal`, target `secret/<ns>/<name>`), refused attempts
  included.
- UI, editor plugin (core): the `yaml` tab kind with `{ref: {apiVersion,
  kind, namespace, name}}` shows the object full or neat, copies and
  downloads it, and edits it: Edit → Review changes (DiffEditor, live neat
  ↔ edited) → Dry run → Apply through `POST /api/resource/edit`, with the
  result. The `new-resource` tab applies with server-side apply, where a
  conflict offers "Force apply" (ConfirmDialog). Without `ref` it keeps the v0.4
  `{text, readOnly}` behaviour. The `new-resource` tab kind
  (`{namespace?, kind?}`): templates for the namespace, import of files
  (picker or drag & drop, any number of documents), dry run, apply.
  Plugins open both through `core/plugin/editorTabs` (`openYamlTab`,
  `openNewResourceTab`). Unapplied edits survive tab switches and mark the
  tab dirty (`TabDescriptor.dirty`, `layoutStore.setTabDirty`); closing a
  dirty tab asks first, and so does leaving the page.
- UI, workloads plugin: rows and detail pages get YAML, a generic Delete
  (propagation choice with the cascade explained, ConfirmDialog;
  cluster-scoped objects always ask for the name), a labels/annotations
  editor (KeyValueEditor) and, for Secrets, "Reveal values" (exec, a
  warning first; the values live in the dialog only).

### 7.3 Workload & node operations (core + workloads plugin)

All write unless noted; audited.

- Nodes: `POST /api/nodes/cordon?name=&unschedulable=true|false`;
  `POST /api/nodes/drain?name=&ignoreDaemonSets=true&deleteEmptyDirData=false&force=false&gracePeriod=-1&timeoutSeconds=120`
  (k8s.io/kubectl/pkg/drain; eviction API; result lists evicted/failed pods
  and PDB blocks); `PUT /api/nodes/taints?name=` body `{"taints":[{key,value,effect}]}`.
- Deployments: `GET /api/deployments/history?ns=&name=` (read; revisions from
  owned ReplicaSets: revision, images, change-cause, created, replicas);
  `POST /api/deployments/rollback?ns=&name=&revision=`;
  `POST /api/deployments/pause?ns=&name=&paused=true|false`.
- `POST /api/workloads/restart?kind=Deployment|StatefulSet|DaemonSet&ns=&name=`;
  `POST /api/workloads/scale?kind=Deployment|StatefulSet|ReplicaSet&ns=&name=&replicas=`.
- Pods: `DELETE /api/pods?…&force=true` (grace 0); `POST /api/pods/bulk-delete`
  `{"items":[{"namespace","name"}],"force":bool}`; `POST /api/pods/debug?ns=&pod=&image=&target=` (exec)
  → ephemeral container name (attach via 7.1 with `container=`).
- CronJobs: `POST /api/cronjobs/trigger?ns=&name=` (Job from template,
  annotation `cronjob.kubernetes.io/instantiate=manual`);
  `POST /api/cronjobs/suspend?ns=&name=&suspend=true|false`.
- Namespaces: `POST /api/namespaces` `{"name","labels"?}`; `DELETE /api/namespaces?name=`.
- UI: node actions (cordon/uncordon, drain dialog with results, taints
  editor), rollout history + rollback, pause/resume, restart/scale for
  StatefulSet/DaemonSet/ReplicaSet, multi-select bulk delete on pod lists,
  force delete, debug container (opens a terminal), CronJob trigger/suspend,
  namespace create/delete (type-to-confirm).

Details settled by the implementation:

- Node list entries gain `unschedulable` and `taints` (`[{key,value,effect}]`),
  Deployment entries `paused`; all omitted when false or empty.
- Boolean switches default to the action the route names: `unschedulable`,
  `paused` and `suspend` are `true` when absent. Mutations without a result
  answer `{"ok":true}`.
- Drain: defaults as in the path above; `timeoutSeconds` 1–600, `gracePeriod`
  −1 (the pod's own) to 3600. It cordons first and answers `200` when the
  evictions ended: `{"node","cordoned","completed","evicted":["ns/pod"],
  "failed":[{"pod","error","reason"?}],"blockedByPDB":["ns/pod"],"skipped":[{"pod",
  "reason":"daemonSet"|"mirror"}],"errors":[…]}`. Pods a drain must not lose
  (no controller without `force`, emptyDir without `deleteEmptyDirData`,
  DaemonSet pods without `ignoreDaemonSets`) stop it before any eviction and
  are reported in `errors`, like kubectl. An eviction a PodDisruptionBudget
  refuses (429) is retried every 5 s until the timeout.
- Taints: key a qualified name, value a label value, effect `NoSchedule` |
  `PreferNoSchedule` | `NoExecute`, each key once per effect, else `400`.
  Answers `{"taints":[…]}`; taints kept unchanged keep their `timeAdded`.
- History: `{"currentRevision","paused","revisions":[{"revision","replicaSet",
  "images","changeCause","created","replicas","readyReplicas","current"}]}`,
  newest first. The current revision is the ReplicaSet whose pod template the
  Deployment has (the revision annotations lag behind during a rollout).
- Rollback (kubectl rollout undo): `409 conflict` while paused or for the
  current revision, `404 not_found` for a revision the Deployment does not
  have.
- Restart and scale check `kind` (`400` otherwise); scale goes through the
  `scale` subresource. `/api/deployments/scale|restart` keep working.
- `DELETE /api/pods?force=true` deletes with grace period 0. Bulk delete
  takes 1–200 items (mock mode too; the list's Delete button is disabled
  beyond 200 selected Pods, with the reason as tooltip) and answers `{"results":[{"namespace","name","error"?}]}`
  in order; its audit detail lists the pods (`{"failed","force","pods"}`).
  A list too long for one row is recorded across several rows of the same
  request, each with `"part":"<i>/<n>"`, so no pod is left out.
- Debug: `image` defaults to `busybox:1.37`, `target` must name a container
  of the pod; answers `{"namespace","pod","container"}`, the container being
  `debugger-xxxxx` with stdin and TTY, once that container runs, so the
  terminal opened next can attach. It waits up to 60 s and answers
  `409 conflict` with the kubelet's reason when the container cannot start
  (`ImagePullBackOff`, `InvalidImageName`, …), ended already, or still
  waits; the audit row then records that refusal.
- Trigger answers `{"namespace","job"}`; the Job is the CronJob's controller-
  owned child, like `kubectl create job --from=cronjob/<name>`.
- Namespaces: the name must be a DNS-1123 label and labels valid. Deleting a
  system namespace (`default`, `kube-system`, `kube-public`,
  `kube-node-lease`) or `KUBEBENCH_NAMESPACE` answers `403 forbidden` unless
  `force=true`. The UI does not offer to delete system namespaces; for
  KubeBench's own it offers "Delete anyway" after the refusal.
- Refusals by KubeBench itself use `bad_request`, `not_found`, `conflict` and
  `forbidden`; errors from the cluster keep the code `kubernetes`.
- Frontend: `DataProviders.workloadOps` (`WorkloadOpsProvider`); the shared
  `Table` gains an optional checkbox `selection`.

### 7.4 Helm plugin v2 (backend plugin `helm`)

Backend `internal/plugins/helm`; routes under `/api/plugins/helm`
(old `/api/helm/*` removed). Repos/caches under `$KUBEBENCH_DATA_DIR/helm/`
(`repositories.yaml` 0600 with any credentials, `cache/`,
`registry/config.json`, `uploads/<id>.tgz`), shared by every cluster; the
server account's own Helm configuration and Helm plugins are never used.
Detect: can list Secrets labelled `owner=helm` (run after the capability
probe, cached like every Detect).

`GET /releases?ns=`, `GET /releases/{ns}/{name}` (info, chart metadata,
notes, hooks), `GET /releases/{ns}/{name}/values?all=&revision=` (exec:
values often carry credentials, so reading them is like revealing a
Secret and is audited), `GET /releases/{ns}/{name}/manifest?revision=`,
`POST /releases/{ns}/{name}/rollback {revision}`,
`DELETE /releases/{ns}/{name}?keepHistory=`,
`POST /install {name,namespace,createNamespace,chart:{repo,name,version}|{ref,version}|{uploadId},values,dryRun}` → `{release,manifest,notes}`,
`POST /releases/{ns}/{name}/upgrade {chart…,values,reuseValues,dryRun}` → `{manifest,notes,currentManifest}`,
`GET /repos`, `POST /repos {name,url,username?,password?}` (admin),
`DELETE /repos/{name}` (admin), `POST /repos/update` (write),
`GET /charts/search?q=`, `GET /charts/values?repo=&chart=&version=`,
`POST /charts/oci/values {ref,version}` (write),
`POST /charts/upload` (multipart `.tgz`, write; for offline installs).
Install and upgrade are `write` with or without `dryRun`; every other
read is `read`.

Shapes (they match `src/plugins/helm/data/types.ts`):

- A release is `{name,namespace,chart,chartVersion,appVersion,revision,status,updated,description}`
  (Helm's status verbatim, RFC 3339 times); the detail adds
  `firstDeployed`, `notes`, `chartInfo` (Chart.yaml), `hooks`
  (`{name,kind,events,weight,phase,startedAt,completedAt}`) and `history`
  (newest first). The list includes releases uninstalled with
  `keepHistory` (status `uninstalled`), which can be rolled back.
- `values` → `{"values": "<yaml>"}` (`""` when empty; `revision` 0 or
  absent = latest; `all=true` merges the chart defaults); `manifest` →
  `{"revision","manifest"}` with hooks appended. Rollback answers
  `{"release"}` (the new revision; `revision` 0 = the previous one),
  uninstall `{"ok":true,"release"}`.
- `values` in install/upgrade bodies is YAML text. An upgrade without
  `chart` keeps the release's current chart (values-only upgrade); with
  `reuseValues` the values are merged over the current ones, without they
  replace them (Helm's `--reset-values`, so an empty text resets to the
  chart defaults). Reuse is Helm's `--reset-then-reuse-values`: the chart's
  own defaults stay the defaults, so a later reset returns to them.
  Upgrades and rollbacks keep 10 revisions; nothing waits for readiness
  (no `--wait`); hooks and deletions time out after 5 min.
- Dry runs (`dryRun=true` in the query or the body flag) use Helm's
  server-side dry run: rendered with the cluster's API versions and
  lookups, nothing created, the namespace not created either. Because a
  template `lookup` reads any object with KubeBench's own credential, dry
  runs need `write` and are audited (detail `"dryRun":true`). Upgrade dry
  runs return the current revision's manifest as `currentManifest` for
  the diff.
- Every manifest returned (release manifests of any revision, install and
  upgrade answers, `currentManifest`) shows core Secrets (and the
  credentials of §7.15's Routes, BuildConfigs and Builds) redacted like
  every other read path (§3.1: `data`/`stringData` values become
  `<N bytes, redacted>`, the last-applied annotation `<redacted>`; a
  redacted Secret is re-serialized with sorted keys, other documents are
  verbatim). A document that may be a Secret but cannot be parsed is
  replaced by a comment.
- Chart references: `{repo,name}` must name an added repository and a
  chart in its cached index; `{ref}` must be `oci://` (no digest);
  `{uploadId}` an unexpired upload. Local paths and plain URLs are never
  accepted. `GET /charts/values` takes `repo`+`chart` or `uploadId`
  (plus `version`) as query parameters, `POST /charts/oci/values` takes
  `{ref, version}` as its body (a POST, so that the origin check keeps
  cross-site pages from triggering the fetch); both → `{chart,values}` (values.yaml as
  written). Added repositories are the admin's allow-list of chart
  sources; an OCI reference names any host the server then connects to,
  so every OCI fetch needs `write` (an OCI `ref` on `/charts/values` is a
  `400`).
- The OCI registry client is HTTPS only: Helm's default transport would
  fall back to plain HTTP when a host answers HTTPS with HTTP, and then
  stay on HTTP for every later pull of the shared client.
- `GET /repos` → `[{name,url,authenticated,chartCount,updated,error?}]`:
  credentials are never returned and never audited; a URL with userinfo is
  refused (`400`). Adding downloads the index first (bad URL or
  credentials → `400`, a duplicate name → `409`). `POST /repos/update` →
  `{"repos":[{name,ok,error?}]}`.
- `GET /charts/search` → at most 100 `{repo,name,version,appVersion,description,icon,deprecated,versions:[{version,appVersion,created,deprecated}]}`
  matching name, description or keywords; exact names first.
- Upload: multipart field `chart`, ≤ 20 MiB, validated with
  `loader.LoadArchive` → `{uploadId,chart,expiresAt}`; uploads expire
  (and are removed) after 24 h.
- Audit targets: `release/<ns>/<name>` (install refines its row with the
  body's release and `{"chart","createNamespace"}`), `repo/<name>`,
  `chart/<name>` for uploads.
- Helm builds its own Kubernetes clients from the cluster's rest.Config;
  unlike the runtime's clients (§4.5) they follow redirects. KubeBench
  pins them to the API server's host, so a redirect elsewhere fails
  before the stored credential is sent; redirects within the API server
  are followed. Repository and registry downloads use Helm's getters,
  which follow redirects as well (Go drops the Authorization header on a
  redirect to another host).

UI: releases list/detail (overview, values user/all per revision, manifest,
notes, history with rollback + values diff between two revisions),
upgrade (values editor, version picker, dry-run manifest diff), install
wizard (search → version → values → dry-run → install; or upload `.tgz`),
repositories (add/remove/update), uninstall (keep-history option). Tab
kinds: `helm-releases`, `helm-release-detail` and `helm-upgrade` (props
`{namespace,name}`; tab ids `helm-release:<ns>/<name>`,
`helm-upgrade:<ns>/<name>`), `helm-install`, `helm-repos`.

### 7.5 Argo CD plugin (backend plugin `argocd`, new)

Works through the Argo CD CRDs (no Argo CD API token). Detect: discovery
serves `argoproj.io/v1alpha1` `applications`.

`GET /applications?ns=` → summaries (name, namespace, project, repo, path or
chart, targetRevision, destination, sync, health, operationPhase,
lastSyncedAt, revision, autoSync); `GET /applications/{ns}/{name}` →
detail (sources, status.resources with sync/health, conditions, history,
operationState, summary images/URLs); `POST /applications/{ns}/{name}/sync
{revision?,prune?,dryRun?,resources?,syncOptions?}` (sets `.operation`,
`initiatedBy.username = "kubebench:<user>"`; 409 if an operation is
running); `POST …/refresh?hard=` (annotation
`argocd.argoproj.io/refresh`); `POST …/rollback {historyId}` (409 when
automated sync is on); `DELETE …?cascade=` (adds
`resources-finalizer.argocd.argoproj.io` when cascading);
`GET /projects`; `GET /applicationsets`.

- Sync and rollback build `.operation` like the Argo CD API server: the
  controller does not read `spec.syncPolicy.syncOptions` for a requested
  operation, so they are copied in unless the request names its own
  (`syncOptions: []` syncs without any); an empty `revision` syncs the
  source's `targetRevision` (multi-source apps: each source's, and a
  `revision` is refused with `400`). Rollback (`{historyId, prune?}`)
  copies the history entry's revision and source(s) and uses the apply
  strategy (no hooks). `409 conflict` also while `.operation` is still
  pending; the patch carries the checked `resourceVersion`.
- `DELETE …?cascade=` defaults to `true` (as in Argo CD); `false` removes
  the resources finalizer, so the managed resources stay.
- Every mutation is `write` (not DryRunnable: an Argo CD dry-run sync
  still patches the Application). Sync and rollback add an
  `argocd.sync`/`argocd.rollback` audit row with the requested options.

UI: applications (grouped by project, sync/health badges), detail
(resource tree → open resources in KubeBench via tab kinds, conditions,
history + rollback, operation state), sync dialog (prune, dry-run,
revision, selective resources), refresh/hard refresh, delete (cascade,
type-to-confirm), projects, ApplicationSets; unavailable state when Argo
CD is not installed.

### 7.6 Storage plugin (backend plugin `storage`, new) — PV/PVC creator

Detect: always available. `GET /overview` (storage classes incl. default
marker, counts, capacity), `GET /csidrivers`, `GET /snapshots?ns=` (when
snapshot CRDs exist), `POST /pvcs?dryRun=`, `POST /pvs?dryRun=`,
`POST /storageclasses?dryRun=`, `PATCH /pvcs/{ns}/{name}/resize {size}`.
Bodies are validated server-side (DNS-1123 names, quantities, access modes).

- Also `GET /pvcs?ns=` and `GET /pvs` (read): claim and volume summaries
  for the plugin's lists and wizards (a claim's `expandBlocker`:
  `not_bound` | `no_class` | `class_missing` | `not_allowed` | `""`).
- Create bodies are form-shaped (the server builds the object; unknown
  fields are refused): PVC `{namespace, name, storageClassName?, accessModes,
  size, volumeMode?, volumeName?, dataSource?: {kind, name}, labels?}`
  (omitted class = cluster default, `""` = none); PV `{name, capacity,
  accessModes, reclaimPolicy?, storageClassName?, volumeMode?, mountOptions?,
  labels?, nodes?, hostPath|nfs|local|csi}`; StorageClass `{name,
  provisioner, parameters?, reclaimPolicy?, volumeBindingMode?,
  allowVolumeExpansion?, isDefault?, mountOptions?, labels?}`. They answer
  `{apiVersion, kind, namespace?, name, dryRun, warnings?,
  warningReasons?}`; each create route is `write` + DryRunnable. References are checked before the create
  (`400`): the PV to bind must be Available and fit (class, size, modes,
  volume mode); a clone or restore must be at least as large as its source.
  A PV's `nodes` must exist; its required node affinity matches their
  `kubernetes.io/hostname` labels (the node name when unlabelled).
- Resize (write) answers the updated claim summary; a size that does not
  grow is `400`, a claim that is not Bound or whose class does not allow
  expansion `409 conflict`. `GET /snapshots` without the CRDs is
  `503 capability_unavailable`.
- Audit: one router row per mutation. Creates refine it through
  `SetAudit` with the object the body names (`pvc/<ns>/<name>`, `pv/<name>`,
  `storageclass/<name>`) and a short JSON detail (size, class, source);
  resize adds `{"from","to"}` to the row whose target the URL already names.

UI: overview, PVC/PV/StorageClass lists, wizards — PVC (storage class,
access modes, size+unit, volume mode, bind to an Available PV, clone from
PVC/snapshot, labels), PV (capacity, access modes, reclaim policy, storage
class, volume mode, source: hostPath | NFS | local + node affinity | CSI),
StorageClass; live YAML preview, Validate (dry-run), Create → open the new
resource; Expand PVC.

### 7.7 Security plugin (backend plugin `security`, new)

`POST /scan` (read) runs checks and returns a report
`{generatedAt, score:{overall, byCategory}, counts, findings:[{key, checkId,
title, severity, category, frameworks:["CIS 5.2.2","PSS baseline"], resource:{kind,namespace,name},
message, remediation, accepted?:{by,reason,at,expiresAt}}], skipped:[{checkId, reason}]}`;
`GET /report` (last report, cached per cluster); `GET /checks` (catalog);
`POST /accept {key, reason, expiresAt?}` / `DELETE /accept/{key}` (write;
stored in plugin KV, shared by all users of the cluster);
`GET /export?format=md|csv`.

≈25 checks: Pod Security Standards baseline/restricted (privileged, host
namespaces, hostPath, hostPorts, capabilities, privilege escalation,
runAsNonRoot, seccomp, …), workload hygiene (requests/limits, probes,
`:latest`), RBAC (cluster-admin bindings, wildcards, secrets access,
escalate/bind/impersonate — also when the rule names its targets —, default
SA tokens), network (namespaces without
NetworkPolicy), secrets in env vars, PSA labels, workloads in `default`.
System namespaces excluded by default (toggle).

UI: overview (score, per-category small multiples, severity tiles),
findings table (filters, grouping, accept/un-accept, open resource),
checks catalog, export.

Details settled by the implementation (shapes match
`src/plugins/security/data/types.ts`):

- `POST /scan` takes an optional body `{"includeSystem": bool}` (unknown
  fields `400`). System objects are the system namespaces (`kube-*`,
  `openshift`, `openshift-*` and, since v0.8, those labelled
  `openshift.io/run-level` or `openshift.io/cluster-monitoring=true`,
  §7.15) and the cluster's own RBAC: the
  `kubernetes.io/bootstrapping=rbac-defaults` label, `system:*` names,
  bindings whose subjects are all `system:` users/groups or ServiceAccounts
  of a system namespace (also as `system:serviceaccount:<ns>:<name>` users
  and `system:serviceaccounts:<ns>` groups), and ClusterRoles only such
  bindings use. A binding without the bootstrap label that names
  `system:anonymous`, `system:unauthenticated`, `system:authenticated` or
  `system:serviceaccounts` is never a system object: it grants to
  everyone.
- Cost: every kind is listed once, cluster-wide, in pages of 500 and at
  most 5 000 objects per kind; a report holds at most 20 000 findings.
  Cut lists and kinds that cannot be listed become `warnings`; checks
  whose objects could not be listed at all are `skipped` with the reason.
  Nothing listable at all answers the Kubernetes error. One scan runs per
  cluster at a time (≤ 2 min): a request with the same options waits for
  it and gets its result, other options answer `409 conflict`. Scans are
  read and not audited.
- Workload checks read pod templates: Deployments, StatefulSets,
  DaemonSets, CronJobs, and ReplicaSets, Jobs and Pods not controlled by
  one of those, so a problem is reported once, on the object to fix.
- The report adds `cluster`, `options`, `warnings`, `inventory` (objects
  listed per kind) and `systemNamespaces`; `resource` carries
  `apiVersion`; a finding has `evidence` (container names, fields, rules;
  never values) and `message` is the English text with the evidence.
  `byCategory` is a list `[{category, score, checks, evaluated, failed,
  accepted, bySeverity}]` in the order `pss-baseline`, `pss-restricted`,
  `hygiene`, `rbac`, `network`, `secrets`, `namespaces`.
- Score: the severity-weighted share (critical 10, high 6, medium 3,
  low 1) of (check, object) pairs that pass, where accepted findings
  pass; rounded down; `null` when nothing was evaluated. `counts` has the
  open findings per severity plus `accepted` and `total`. Reports are
  computed from the cached scan on every read, so accepting a risk
  changes them without a new scan. Before the cluster's first scan (the
  cache is in memory) `GET /report` answers `null` and `GET /export`
  `404 not_found`.
- Finding key `<checkId>:<Kind>:<namespace>:<name>`. `POST /accept`
  checks the key (known check and kind), a reason of 1–500 characters and
  an `expiresAt` in the future, and answers the acceptance; an expired
  acceptance no longer applies. `GET /accepted` (read) lists every stored
  acceptance with `{key, checkId, resource, by, reason, at, expiresAt?,
  expired}`, so stale ones can be removed. Audit target `finding/<key>`;
  accept adds `{"checkId","expiresAt"}`.
- Export: `Content-Disposition: attachment;
  filename="security-<cluster>-<yyyymmdd-hhmmss>.md|csv"`; CSV cells a
  spreadsheet would run as formulas are prefixed with `'`; Markdown
  escapes HTML (`& < >` as entities) and Markdown punctuation in every
  value taken from the cluster or a user, so a reason cannot add markup,
  links or images.

### 7.8 Access plugin (backend plugin `access`, new) — RBAC

`GET /subjects`, `GET /subjects/{kind}/{name}?namespace=` (effective
rules with the binding path), `GET /who-can?verb=&group=&resource=&subresource=&namespace=&name=`,
`POST /review` (SubjectAccessReview → `{allowed, reason}`), `GET /graph?namespace=`.
UI: Who can…, subjects + effective permissions, risky roles, binding graph.

- Detect: always available (RBAC is core); every route is `read` and
  lists Roles, ClusterRoles, both kinds of bindings and ServiceAccounts
  cluster-wide per request (a credential without that access gets the
  API server's `403 kubernetes`).
- Evaluation follows the Kubernetes RBAC authorizer: `*` in verbs, API
  groups and resources (a plain resource never covers its subresources;
  `*/scale` covers every resource's `scale`), resourceNames (no wildcard;
  a rule with them never matches a request without a name), non-resource
  URLs (`*`, exact, trailing `*` prefix; granted only by
  ClusterRoleBindings), RoleBindings grant in their namespace only (so a
  request without a namespace sees ClusterRoleBindings only), a
  ServiceAccount subject without a namespace defaults to the binding's.
  Aggregated ClusterRoles are used with the rules the controller filled in.
- `who-can` takes `path=` instead of the resource fields for a
  non-resource URL and answers `{request, subjects: [{kind, name,
  namespace?, grants: [{binding, role, scope, rule}]}]}`; subjects are
  listed as the bindings name them (groups stay groups). A subject's
  detail answers `{subject, user, groups, missing?, bindings: [path],
  rules: [{namespace, rules: [rule + path]}]}` (`namespace` `""` =
  cluster-wide; a namespace never lists non-resource URL rules, which a
  RoleBinding does not grant), where the identity includes the implicit groups
  (`system:serviceaccounts`, `system:serviceaccounts:<ns>`,
  `system:authenticated`) and a path's `via` names the group it reaches
  the subject through; `roleMissing` marks a binding to a missing role.
- `GET /risky` (read): roles with at least one risk (`full-access`,
  `escalate-bind`, `impersonate`, `wildcard-verbs`, `wildcard-resources`,
  `secrets-read`, `pod-exec`, `token-create`, `rbac-write`, `node-proxy`,
  `webhook-write`, `csr-approve`, `workload-create`,
  `persistentvolume-create`; severities `critical` | `high` | `medium`),
  with their bindings and the number of subjects, most severe first. A rule
  limited to resourceNames is a narrow grant and raises no risk, except for
  `escalate-bind` and `impersonate`: naming the target (`cluster-admin`,
  `system:masters`) is the dangerous case there.
- `GET /graph?namespace=&system=true`: `{subjects, bindings, roles, edges,
  truncated}`; with a namespace its RoleBindings plus the
  ClusterRoleBindings naming one of its ServiceAccounts; `system:…`
  bindings only with `system=true`; at most 300 bindings.
- `POST /review` body `{subject: {kind, name, namespace?}, verb, group,
  resource, subresource, namespace, name}` or `{subject, verb, path}`;
  answers `{allowed, denied?, reason?, evaluationError?, user, groups,
  grants}` (`grants`: KubeBench's own evaluation). It is `read` although
  it creates a SubjectAccessReview (which changes nothing), sends the
  subject's user and groups in the review spec and never impersonates;
  without `create` on `subjectaccessreviews` it fails with `403 kubernetes`
  naming that permission. Rate limit: 30 reviews per user per minute,
  then `429 rate_limited` with `Retry-After`.

### 7.9 Topology plugin (frontend-only, new)

Builds a graph from core providers + generic lists: Ingress → Service →
Pods ← ReplicaSet/StatefulSet/DaemonSet ← Deployment; Pods → PVC → PV;
optional ConfigMap/Secret references. Layered SVG layout, pan/zoom, click
opens the resource. Resource action "Show topology" on Deployment,
StatefulSet, DaemonSet, Service, Ingress.

Implementation (`src/plugins/topology`, no backend routes):

- Data, per namespace, each list loaded on its own so the graph fills in
  as they arrive and an unreadable list (e.g. Secrets) leaves the rest:
  Pods, Deployments and Services through the `/api/k8s` passthrough
  (owner references, selectors and volumes intact); ReplicaSets,
  StatefulSets, DaemonSets, Ingresses, PVCs, PVs (cluster-wide, only those
  bound to the namespace's claims are shown), ConfigMaps and Secrets
  through the generic lists. Every list fails all → Unavailable.
- Edges: Ingress → Service (rule paths and default backend), Service →
  Pod (equality selector), controller ownerReference → ReplicaSet/Pod,
  Pod → PVC (also generic ephemeral volumes), PVC → PV (`volumeName`),
  Pod → ConfigMap/Secret (volumes incl. projected, `envFrom`, `env`
  `valueFrom`; not the `kube-api-access-*` token volume) and Ingress →
  TLS Secret. A reference whose list loaded without it is drawn as a
  missing node; optional references only when present.
- Layout: five columns (Deployment | ReplicaSet/StatefulSet/DaemonSet |
  Pod | Service/PVC/ConfigMap/Secret | Ingress/PV), Pods stacked in owner
  groups, every other node level with its placed neighbours; deterministic,
  so refreshes keep positions. Inactive ReplicaSets and config references
  are display options.
- Focus: the object, everything it leads to, and everything leading to it
  or its Pods (shared ConfigMaps do not pull in other workloads). Tabs:
  kind `topology`, props `{namespace, focus: {kind, name} | null}`, one per
  namespace and per focused object; objects open in `workloads-detail`
  (Pod, Deployment, Service), `generic-detail` or `yaml`.

### 7.10 Live updates (core)

`GET /api/watch?topics=pods,deployments,…` (read) — SSE stream of
`event: change` `data: {"topic":"pods","namespace":"default"}` (debounced),
heartbeat comments every 25 s, backed by per-cluster shared informers that
start lazily and stop when unused. Frontend: scoped invalidation
(`refreshData(topic?)`), status bar "Live" indicator, Settings toggle.

- **Topics** are resources as kubectl names them: `pods`, `deployments`,
  or `<resource>.<group>` (`applications.argoproj.io`); 1–32 per stream,
  lower case, duplicates dropped (else `400 bad_request`). A topic is
  resolved through discovery to the preferred version.
- **Internal topics** (v0.7) name KubeBench's own data:
  `kubebench:<name>` (`kubebench:notes`, §7.13), the name like a plugin
  id. They are `live` at once and never polled; their changes are
  published by the code that stores the data, on a server-wide board
  (`watch.Board`; plugins call `plugin.Context.Publish(topic, audience)`
  after the change is stored), as `{topic, namespace: ""}`, to an
  audience: the streams on one cluster (data all its users see) or one
  user's streams on any cluster (personal data). A stream's cluster and
  user are those it was opened for, and its access is re-checked at
  every heartbeat like any stream.
- **Events**: `event: status` `{topic, mode, reason?}` with `mode`
  `starting` (informer listing), `live` or `polling` (the cluster does not
  serve the resource, the credential may not list/watch it, the cluster's
  or the caller's informer limit is reached, or the topic could not be
  resolved yet — discovery failed; it is resolved again every 30 s;
  `reason` says which) — the first
  ones at once, then on every change; `event: change` `{topic, namespace}`
  collected for 500 ms (`namespace` `""` = cluster-scoped, or anywhere
  after more than 256 pending changes); `event: end` with the reason (a
  reason message, §3.3) when
  the server ends the stream: the cluster's runtime was rebuilt or removed,
  the caller's session/assignment ended (checked every heartbeat, like
  log streams) or the server shuts down (§7.1). Clients may reconnect after `end`.
- **Hub** (`internal/watch`, one per Runtime, closed with it): one dynamic
  informer per resource, keeping only object metadata, started on the
  first subscription and stopped 5 min after the last subscriber left.
  Limits: 16 watch streams per user (`429 rate_limited`), 512 subscribers
  and 64 informers per cluster, 32 resource types per user across their
  streams (beyond it the topic is polled). `GET /api/watch/status` (read) lists the
  informers `{resource, mode, reason?, subscribers, idleSince?}` and the
  subscriber count, for diagnostics. Lists are still served by direct API
  calls, not from the informer caches.
- **Frontend**: queries declare `scope: {topics, namespace?}`
  (`useQuery`/`useProviderQuery`/`usePluginQuery`, SDK
  `data.usePluginQuery`). `refreshData()` re-runs every query (mutations);
  `refreshTopics(changes)` only the queries of a changed topic in that
  namespace (the TS signature keeps `refreshData()` argument-free because
  it is used as an event handler). The stream subscribes to the union of
  the mounted queries' topics (`live.watch` provider; the mock reports all
  topics live), opens a replacement before closing the old stream when
  topics are added, refreshes a topic once its informer synced and every
  view after a reconnect. Status bar: Live / Polling / Offline (red;
  reconnects with backoff) / Connecting / Live updates off; its menu lists
  polled topics with the reason and toggles the setting. Settings → Data →
  Live updates (default on). Auto-refresh stays the fallback: while live,
  it refreshes queries without topics and polled topics (polled topics
  every 30 s when auto-refresh is off); otherwise everything.

### 7.11 Updates and What's new (core)

KubeBench tells administrators when a newer release is published on
GitHub, and tells every user what changed after an upgrade. Nothing here
installs anything: upgrading stays the operator's job (§8,
`docs/deployment.md#upgrades`).

**Update check** (`backend/internal/updates`):

- Reads `KUBEBENCH_UPDATE_URL` (§2; GitHub's "list releases" JSON of
  `Shavell/kubebench`, `per_page=20` added unless the URL sets it) with
  `Accept: application/vnd.github+json`, `X-GitHub-Api-Version: 2022-11-28`
  and `User-Agent: KubeBench/<version>`. The proxy comes from
  `HTTPS_PROXY`/`HTTP_PROXY`/`NO_PROXY`; 10 s timeout; the response is
  capped at 4 MiB.
- Results are cached for 6 h and revalidated with `If-None-Match` (a `304`
  keeps the cached list and only moves `checkedAt`). A failed check is
  retried after 15 min at the earliest; a rate limit (`403`/`429` with
  `X-RateLimit-Remaining: 0` or `Retry-After`) waits for
  `Retry-After`/`X-RateLimit-Reset` (at most 6 h). A failure keeps the
  last good result and reports `error` next to it.
- Checks are lazy: the first `GET /api/updates` after the cache expired
  fetches (concurrent callers share one request, which outlives a caller
  that goes away). Startup never makes or waits for a request. Mode `off`
  never contacts the URL at all.
- Releases: drafts and tags that are not SemVer (`v` prefix allowed) are
  ignored; a release is a prerelease when GitHub flags it or its version
  has a prerelease part. `latest` is the highest version the mode admits:
  `stable` → non-prereleases only, `prerelease` → all. An empty list and a
  `404` (the repository has no public releases yet) both mean "no releases
  published": `latest` and `error` are empty.
- The build version (`main.version`) is compared by SemVer precedence. It
  is reported as `current: "unknown"` (never "update available") unless it
  is a release `X.Y.Z` or a release candidate
  `X.Y.Z-<alpha|beta|rc|pre|preview>[N][.<id>…]` whose identifiers carry no
  `-`: `dev`, git-describe (`0.5.0-3-gabc1234`), untagged
  (`0.5.0-abc1234`), `-dirty` and GoReleaser snapshot
  (`0.5.1-snapshot-abc1234`) builds do not nag.
- Mode: `off` | `stable` | `prerelease`. The default is
  `KUBEBENCH_UPDATE_MODE` (default `stable`); an admin's choice is stored in
  `app_settings` (key `updates.mode`, store migration v3) and wins over the
  default from then on.

**API**:

- `GET /api/updates` (authed) →
  `{"mode","defaultMode","current","latest","available","prerelease","publishedAt","url","notes","checkedAt","error"}`.
  `latest`, `url` (the release page; the repository's releases page when
  there is none) and `notes` (the release body: Markdown source, never
  HTML) describe the newest admitted release, empty when there is none;
  `publishedAt` and `checkedAt` are RFC 3339 or `null`; `available` means
  `latest` is newer than a known `current`.
- `PUT /api/settings/updates` (admin, audited; target `setting/updates`,
  detail `{"mode":…}`) `{"mode":"off"|"stable"|"prerelease"}` → the status
  as `GET` returns it.
- `POST /api/updates/check` (admin, audited) checks now (still conditional
  on the ETag) → the status; `409 conflict` while the mode is `off`.
- `GET /api/auth/me` gains `"lastSeenVersion"` (`""` until the user first
  closed What's new); `PATCH /api/auth/me` (authed, not audited)
  `{"lastSeenVersion":"0.5.0"}` (1–64 characters) stores it
  (`users.last_seen_version`, migration v3) → the `me` view.

**Frontend** (`src/core/updates`: a core plugin `updates` registered at boot
like the workbench plugin, strings in its own `updates` namespace):

- Provider `updates` (`status`, `setMode`, `checkNow`, `lastSeenVersion`,
  `markSeen`). Mock: a deterministic fake release one patch above the app
  version, published 2026-11-01, with notes; the demo user has already
  seen the current version, so demo boots show no dialog.
- Settings → **Updates**: the mode (a select for admins; read-only with the
  reason for others), current and latest version, "Check now" (admins;
  disabled while off), the last check time and the error reason, or "No
  releases published yet".
- Status bar (admins only): "Update available: vX.Y.Z", opening the
  **Update available** dialog: version, date, prerelease badge, the release
  notes through a tiny Markdown subset renderer (headings, lists,
  paragraphs, `code`, **bold**, links with `http(s)` targets only; React
  text nodes, never `innerHTML`), the release page link, and how to
  upgrade per install method (Helm, manifests, .deb/.rpm, install.sh,
  Docker) with a link to `docs/deployment.md#upgrades`.
- About (everyone): the latest release next to the version, and a "What's
  new" link.
- **What's new**: `CHANGELOG.md` is bundled at build time (Vite `?raw`) and
  parsed by its Keep a Changelog sections (`## [x.y.z] - date`, `### Added`
  …; `[Unreleased]` is skipped). Once per user and version: when the app
  version is a release version (as above) newer than the user's
  `lastSeenVersion`, or that is empty, the "What's new in vX" dialog shows
  the sections newer than `lastSeenVersion` up to the app version (only
  the app version's when it is empty); closing it stores the version. With
  no matching section the version is stored silently. "Help: What's New"
  (command) and the About link open the `changelog` tab with the whole
  changelog. "Help: Check for Updates" (admins) checks now and opens the
  Update available dialog or reports the result.
- Air-gapped installs use `off`: `install.sh --offline` writes
  `KUBEBENCH_UPDATE_MODE=off`, the offline bundle README says so, and the
  chart has `config.updateMode` (default `stable`).

### 7.12 Cloud providers (core)

KubeBench recognises where a cluster runs, warns when its Kubernetes
version leaves the provider's support window, and signs in to EKS, GKE and
AKS in process instead of running the clouds' exec credential plugins.

**Detection** (`backend/internal/cloud`, read):

- Signals: the API server URL, the server's `gitVersion` (`/version`) and
  up to 20 nodes (`spec.providerID`, labels). A signal the credential may
  not read (nodes) is skipped.
- `provider`, strongest signal first: OpenShift (node label
  `node.openshift.io/os_id`; OpenShift keeps its own lifecycle on a cloud,
  e.g. ROSA/ARO) → a managed service's API host (`*.eks.amazonaws.com[.cn]`
  → `eks`, `*.azmk8s.io` → `aks`, `*.gke.goog` / `container.googleapis.com`
  → `gke`, `*.k8s.ondigitalocean.com` → `doks`, `*.linodelke.net` → `lke`,
  `*.ovh.net` / `*.ovh.cloud` → `ovh`), `gitVersion` suffix (`-eks-`,
  `-gke.`) or node label (`eks.amazonaws.com/nodegroup`,
  `eks.amazonaws.com/compute-type`, `alpha.eksctl.io/cluster-name`,
  `kubernetes.azure.com/cluster`, `kubernetes.azure.com/agentpool`,
  `cloud.google.com/gke-nodepool`, `cloud.google.com/gke-os-distribution`,
  `doks.digitalocean.com/node-pool`, `lke.linode.com/pool-id`) →
  `digitalocean://` / `linode://` providerIDs → k3s / RKE2 (`+k3s` /
  `+rke2` in `gitVersion`, `k3s://` / `rke2://` providerIDs, instance type
  `k3s`) → `kubernetes`.
- `cloud`: the infrastructure from a providerID (`aws://` → `aws`,
  `azure://` → `azure`, `gce://` → `gcp`, `digitalocean`, `linode`) or the
  managed service (`ovh`). A providerID alone never makes a cluster EKS,
  AKS or GKE: self-managed clusters (kops, kubeadm) use them too and do not
  follow the managed services' support windows; the UI shows the cloud
  instead.
- Cached per runtime: detected in the background when the runtime is
  built (next to the capability probe), refreshed in the background when
  read more than 1 h later (upgrades), and anew whenever the runtime is
  rebuilt.

**Version support**: a bundled, dated table (`cloud/versions.go`, with its
source URLs and "as of" date in a comment) of the date standard support of
each minor (1.29–1.37 as of 2026-11-15) ends: upstream (end of patch
support), EKS (end of standard support), AKS (end of life, no LTS), GKE
(end of standard support). Extended/LTS support is not assumed (it costs
extra or must be opted into). `eks`/`aks`/`gke` use their table, OpenShift
its own by OpenShift version (§7.15), everything else (k3s, RKE2, DOKS,
LKE, OVH, kubernetes) upstream's.
`status`: `end-of-life` after the end date (or for a minor older than
the table), `ending-soon` within 90 days of it, `supported` before, else
`unknown` (no table, a minor newer than the table, unparseable version).
`basis` names the table used.

**API**:

- `GET /api/clusters` entries gain `"platform": {"provider","cloud",
  "version","support":{"status","endDate","basis"},"detected"}`. Listing
  never connects to a cluster: a cluster whose runtime is not built (or
  not yet detected) is described from its URL alone (`detected: false`,
  `version: ""`, status `unknown`). Admins also get `"cloudAuth"`
  (`aws`|`gcp`|`azure` when the kubeconfig user's exec stanza is handled
  in process) and `"cloudCredentials"` (the provider of stored per-cluster
  credentials; absent = the server's own identity). Credentials are never
  returned.
- `/api/info` `cluster` gains the same `"platform"`; `/api/info` waits
  (≤3 s, the same deadline as the capability probe) for the first
  detection.
- `POST /api/clusters` accepts `"credentials"` (below); `PATCH
  /api/clusters/{id}` accepts `"credentials"`: an object replaces them,
  `null` removes them, absent keeps them. Both answer `400 bad_request`
  when they are incomplete, for another cloud than the kubeconfig's
  recognised stanza, or for a cluster that is not a kubeconfig. Audit
  details carry the provider only: `{"source":"kubeconfig","credentials":"aws"}`,
  `{"credentials":"aws"|null|"invalid"}`.

**In-process credentials** (`backend/internal/cloudauth`). Recognised exec
stanzas (by the command's base name; anything else stays an exec plugin
gated by `KUBEBENCH_ALLOW_KUBECONFIG_EXEC`, §4.3):

| Stanza | Read from it |
|---|---|
| `aws [global options] eks get-token --cluster-name N` (or `--cluster-id`) | `--region`, `--profile`, `--role-arn`; env `AWS_REGION`, `AWS_DEFAULT_REGION`, `AWS_PROFILE` |
| `aws-iam-authenticator token -i N` | `-r/--role`, `--region`; the same env |
| `gke-gcloud-auth-plugin` | nothing |
| `kubelogin get-token` (`--login` other than `ropc`) | `--server-id` (default `6dae42f8-4368-4678-94ff-3960e28e3630`), `--login`, `--client-id`, `--tenant-id`, `--environment` |

No other environment variable or file path of a stanza is used. The
stanza is removed from the rest config (client-go never runs it) and a
transport wrapper adds `Authorization: Bearer` from a token source cached
until 1 min before expiry (a `401` drops the cached token). Identities
are resolved on the first request, never while the runtime is built.

- **EKS**: AWS SDK v2 credentials — per-cluster access key (+ session
  token) or profile, else the stanza's profile, else the default chain
  (environment, shared files, web identity/IRSA, EKS Pod Identity/ECS,
  EC2 instance metadata); a stanza role is assumed on top through STS.
  Token: SigV4-presigned `GET https://sts.<region>.amazonaws.com/?Action=GetCallerIdentity&Version=2011-06-15`
  with `X-Amz-Expires=60` and the signed header `x-k8s-aws-id: <cluster>`
  (global `sts.amazonaws.com`/`us-east-1` without a region; `.com.cn` for
  `cn-` regions), as `k8s-aws-v1.` + base64url (unpadded) of the URL —
  aws-iam-authenticator's algorithm; valid 15 min.
- **GKE**: `golang.org/x/oauth2/google`, scope `cloud-platform` — a
  per-cluster service-account key (type `service_account` only: external
  account and impersonation files can read files, call URLs or run
  commands), else Application Default Credentials
  (`GOOGLE_APPLICATION_CREDENTIALS`, gcloud's ADC file, the metadata
  server: GKE Workload Identity or the VM's account).
- **AKS**: `azidentity`, scope `<server-id>/.default`, authority from
  `--environment` (public, China, US Government) — per-cluster service
  principal (tenant id, client id, client secret), else `--login spn` →
  `EnvironmentCredential` (`AZURE_*`), `msi` → managed identity (the
  stanza's `--client-id` selects a user-assigned one), `workloadidentity`
  → Azure Workload Identity; `devicecode`, `interactive`, `azurecli`,
  `azd` (and no `--login`) → the ambient chain environment → workload
  identity → managed identity. The Azure CLI is never run (it is not in
  the image).
- **Per-cluster credentials**: `{"provider":"aws","accessKeyId","secretAccessKey","sessionToken"?}`
  or `{"provider":"aws","profile"}`, `{"provider":"gcp","serviceAccountJson"}`,
  `{"provider":"azure","tenantId","clientId","clientSecret"}`. Stored with
  the cluster record (`clusters.credentials`, store migration v4) like
  the kubeconfig (the database is `0600`), write-only, changing them
  rebuilds the runtime.
- **Security**: only admins add and edit clusters (unchanged). The token
  of the identity goes to the API server the kubeconfig names, whatever
  host that is, so the server's own identity is lent to pasted
  kubeconfigs only with `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY`: without
  it, `POST /api/clusters` with pasted content and `PATCH` (removing the
  credentials or setting an AWS `profile`, which reads the server's
  files) answer `403 forbidden` when the recognised stanza would sign in
  without credentials of its own. Kubeconfigs added by path (the server's
  files) and seeded from `KUBECONFIG` are not affected; `/api/info`
  reports the setting as `features.ambientCloudIdentity`. With the
  setting, **an admin
  can make the server sign in with its ambient cloud identity** (the
  pod's IRSA role, GKE or Azure workload identity, the VM's instance
  profile/managed identity, the server's environment and `~/.aws`/gcloud
  files, including the AWS profile named in a stanza) for any cluster
  they add, by pasting a kubeconfig with a matching stanza. Grant that
  identity only the cloud permissions KubeBench should have, or give
  clusters per-cluster credentials. Documented in `docs/deployment.md`
  (Cloud clusters) with the chart's `serviceAccount.annotations` /
  `podLabels` for IRSA (`eks.amazonaws.com/role-arn`), GKE Workload
  Identity (`iam.gke.io/gcp-service-account`) and Azure Workload Identity
  (`azure.workload.identity/client-id` + pod label
  `azure.workload.identity/use: "true"`).

**Frontend**:

- Types `ClusterPlatform`, `VersionSupport`, `CloudCredentials`;
  `ClusterEntry.platform/cloudAuth/cloudCredentials`,
  `ActiveCluster.platform`; `addCluster({…, credentials})`,
  `ClusterUpdate.credentials` (object or `null`).
- Shared `src/shared/cloud`: `ProviderBadge` (a text badge — "EKS",
  "AKS", "GKE", "DOKS", "LKE", "OVH", "OpenShift", "RKE2", "k3s", or the
  cloud — "AWS", "Azure", "GCP", … — for a generic cluster on one; none
  for plain Kubernetes; no brand logos) with the version and support in
  its tooltip, and `VersionSupportNotice`. Strings in the platform
  namespace (`cloud.*`).
- Badges: Clusters view (column "Platform" with the version), the cluster
  switcher entries and the status-bar cluster item.
- Cluster overview: a warning for `end-of-life` and a notice for
  `ending-soon` (end date, provider table), nothing otherwise.
- Add cluster dialog (kubeconfig sources): "Cloud credentials" — server
  identity (default) / AWS access key / AWS profile / GCP service-account
  key (paste or load a file) / Azure service principal; Edit dialog: the
  sign-in in use (`cloudAuth`), the stored kind, replace or remove (secret
  fields are never pre-filled).
- Mock: deterministic platforms per mock cluster (k3s supported, AKS
  ending soon, EKS end of life), no clock in the data.

### 7.13 Notes (backend plugin `notes`, new in v0.6)

Users keep Markdown notes in KubeBench: runbook snippets, "why is this
Deployment pinned to 1 replica", hand-over notes. Notes are **KubeBench
data, not cluster mutations**: they never touch the cluster and never
contain anything read from it (they are user text).

**Model**:

- Visibility `personal` (only its owner sees it, anywhere) or `shared`
  (every user with any role on its cluster sees it).
- Placement: on a cluster (the caller's active cluster when created), on
  one object of it (`apiVersion`/`kind`/`namespace`/`name`; matched by
  group, kind, namespace and name, so `apps/v1` and `apps/v1beta2` name
  the same Deployment; a ref without `apiVersion` matches any group), or
  **global** (no cluster) — personal notes only.
- Fields: `title` (1–200 characters), `body` (Markdown, at most 64 KiB),
  `tags` (at most 16, lower-cased, 1–40 letters, digits, `.`, `_`, `-`,
  starting with a letter or digit), `pinned`, `version` (starts at 1,
  +1 per edit), `createdBy`/`createdAt`, `updatedBy`/`updatedAt` (RFC 3339).
  A note keeps its visibility, cluster and object for life.
- Limits: a user authors at most 1000 notes (personal and shared;
  `409 conflict` beyond it).
- Storage: store migration v5 (§3.5 foreign keys) — table `notes`
  (`owner_id` → users `ON DELETE CASCADE`, set for personal notes only;
  `cluster_id` → clusters `ON DELETE CASCADE`; `created_by`/`updated_by` →
  users `ON DELETE SET NULL`) and `note_tags(note_id, tag)`, indexed for
  (cluster, kind, namespace, name) lookups, owners and tags. Deleting a
  cluster deletes its shared notes and the personal notes placed on it;
  deleting a user deletes their personal notes and keeps their shared
  ones, whose author then reads as a deleted user (`""` in the API).
  Search is a substring match (`LIKE`, case-insensitive for ASCII
  letters) of every term of `q` (at most 8) in title or body — enough for
  thousands of notes, no FTS index.

**Permissions** (new plugin access level `plugin.AccessContribute`, §5):

| | admin | operator | viewer |
|---|---|---|---|
| personal notes (own only) | ✓ | ✓ | ✓ |
| read shared notes of the active cluster | ✓ | ✓ | ✓ |
| create, edit, pin shared notes | ✓ | ✓ | ✗ |
| delete a shared note | any | own | ✗ |

- A **read-only cluster does not block notes**: the flag protects the
  cluster from changes, and notes change only KubeBench's database. A
  viewer may not write shared notes, because every user of the cluster
  reads them (they would otherwise let a read-only role publish text to
  operators and admins).
- Another user's personal note is never readable or changeable by id —
  not even by an admin — and is reported as `404 not_found` (its
  existence is not revealed). A shared note of another cluster is
  `404` too: requests act on the active cluster only.
- Every route needs a session and an active cluster (like every plugin
  route; global notes are listed whatever cluster is active).
- Shared-note mutations are audited by the router (non-GET requests of
  `AccessContribute` routes): target `note/<id>`, detail only the object
  (`{"resource":"Deployment/shop/web"}`) or `{"pinned":true}` — never
  title, body or tags. Personal notes are not audited.

**API** (`/api/plugins/notes`; Detect is always available):

- Note JSON: `{"id","visibility","clusterId"|null,"clusterName","resource":{"apiVersion","kind","namespace","name"}|null,"title","body","tags","pinned","version","createdBy","createdAt","updatedBy","updatedAt"}`.
- `GET /notes?scope=all|personal|shared&clusters=current|all&kind=&apiVersion=&namespace=&name=&tag=&q=&limit=200&body=false`
  (read) → `{"items":[note…],"truncated":bool}`: pinned first, then most
  recently updated. `clusters=current` (default) lists the active
  cluster's shared and personal notes plus global notes; `all` adds the
  caller's personal notes on other clusters. Object filters
  (`kind`, `namespace`, `name`, `apiVersion` by its group; a `name`
  without `namespace` names a cluster-scoped object) imply the active
  cluster. `body=true` includes bodies (else `""`), `limit` ≤ 500.
- `GET /notes/{id}` (read) → note, with body.
- `GET /tags?scope=…` (read) → `{"tags":[{"tag","count"}]}` over the
  notes `GET /notes?clusters=current` would list.
- `GET /counts?kind=&apiVersion=&namespace=&name=` (read) →
  `{"items":[{"apiVersion","kind","namespace","name","personal","shared"}]}`:
  notes per object on the active cluster visible to the caller (badges).
- `POST /notes/personal` (read) `{"title","body","tags","pinned","global":bool,"resource"?}`
  → note. `global` places it on no cluster (no `resource` then).
- `PUT /notes/personal/{id}` (read) `{"title","body","tags","version"}` →
  note; `409 conflict` when `version` is not the stored one (someone —
  another tab — saved in between).
- `PUT /notes/personal/{id}/pin` (read) `{"pinned"}` → note (neither
  `version` nor `updatedAt` change); `DELETE /notes/personal/{id}` (read).
- `POST /notes/shared`, `PUT /notes/shared/{id}`, `PUT /notes/shared/{id}/pin`,
  `DELETE /notes/shared/{id}` (contribute, audited) — the same bodies,
  without `global`; the note is placed on the active cluster.
- A note id under the other visibility's path is `404`.

**Live updates** (v0.7): every stored change (create, edit, pin,
delete) publishes the internal topic `kubebench:notes` (§7.10): a shared
note's to the streams on its cluster, a personal note's to its owner's
streams only, whatever cluster they watch — another user never learns
that a personal note changed. Note queries declare the topic
(`scope: {topics: ['kubebench:notes']}`), so other users' changes appear
within the debounce (500 ms); an editor without unsaved edits follows
the newer version, one with edits keeps them (saving then meets `409`).

**Frontend** (`src/plugins/notes`, backend plugin `notes`):

- Activity bar entry "Notes" (`note` icon); sidebar: search, scope
  (all/personal/shared), "all clusters" toggle, tag filter, the list
  (pinned first) with visibility icon, object and tags; "New note".
- Editor tab `notes-editor` (`{id}` or a draft `{visibility, resource?, global?}`):
  title, tags, visibility (new notes only; shared needs operator/admin,
  disabled with the reason), "global" for personal notes without an
  object, Markdown editing in the shared `CodeEditor` (`markdown`
  language) with Edit / Preview / Split views, the preview rendered by the
  shared safe renderer (`src/shared/markdown`, moved from
  `core/updates`: no HTML, `http(s)` links only). Unsaved changes mark
  the tab dirty (closing asks). A save that gets `409` offers "Reload"
  (discard mine) or "Overwrite". Shared notes show "Visible to every
  user of cluster X". Delete asks through `ConfirmDialog`.
- Resource action "Notes" on every kind (`kinds: ['*']`) with a count
  badge on detail pages (new optional `ResourceActionContribution.badge`
  component); it opens tab `notes-resource` `{ref}`: the object's notes
  rendered, pin/edit/delete, "Add personal note" and "Add shared note".
- Palette: "Notes: New Note" (`notes.new`), "Notes: New Shared Note".
- i18n en/tr/de in the `notes` namespace. Mock: deterministic notes on
  the mock cluster (fixed timestamps; new ones stamped from a fixed base
  time), the demo user acting as admin.

### 7.14 Service browser (core + `service-browser` plugin) — v0.6

Opens the web interface of a Service or Pod in a browser tab next to the
editor, like VS Code's Simple Browser, through the API server's proxy
subresource with KubeBench's cluster credential. The proxied pages must
never run with KubeBench's privileges.

**Access** (new level `browse`, §3.2): `read` + role operator or admin,
audited. Browsing is not a change of Kubernetes objects, but it reaches
inside the cluster network with KubeBench's credential, including admin
interfaces of in-cluster applications that can be posted to, so viewers
never get it. A read-only cluster does **not** refuse it; the proxy then
forwards only GET and HEAD (and answers CORS preflights itself), so a
form post to an in-cluster admin UI is refused like any other change. A
protocol upgrade (`Upgrade` header or `Connection: upgrade`, i.e. a
WebSocket) is refused there too with `403`: it is a GET, but the spliced
two-way connection carries messages no method rule sees, and it would
outlive the per-request access checks.
Frontend permission `browse` (`permissionRules.ts`): held when the role is
admin or operator, whatever the read-only flag.

**API** (core, `internal/api/serviceproxy.go`, `internal/svcproxy`):

- `POST /api/proxy/services/{ns}/{name}/{port}?scheme=http|https` and
  `POST /api/proxy/pods/{ns}/{name}/{port}?scheme=…` (browse) open a
  **grant** and return `ProxySession`
  `{"token","base","sandboxed","readOnly","target":{"kind","namespace","name","port","scheme"},"location"}`.
  The object must exist; a Service port is matched by number or name, a
  Pod port name by its containers' port names (numbers pass: containers
  may listen on undeclared ports); ExternalName Services are refused
  (`400`). Audited **once per grant** as `service.proxy` / `pod.proxy`,
  target `service/<ns>/<name>:<port>`, detail `{"scheme","readOnly"?}`;
  refusals (viewer: `403 forbidden_role`) are audited like every refusal.
- `POST /api/proxy/ingresses/{ns}/{name}?host=&path=&scheme=` (browse,
  v0.7) opens a grant for the **Service port behind one Ingress route**:
  the rule with exactly that host and path (both empty: the default
  backend; resource backends `400`, an unknown route `400`), checked like
  a Service grant. The grant's target gets `host`, the rule's host (none
  for a wildcard host), which the proxy sends as the `Host` header; the
  API server's service proxy passes it on, so virtual-hosted applications
  answer as behind their Ingress. Absolute redirects and HTML links
  naming that host (`http(s)://host/…`, `//host/…`) are mapped below the
  grant like the API server's. Path rewriting by the ingress controller
  (annotations, middlewares) does not apply: the Service is reached
  directly. Audited once as `ingress.proxy`, target `ingress/<ns>/<name>`,
  detail `{"scheme","host","path","service","readOnly"?}`.
- `GET` / `DELETE /api/proxy/sessions/{token}` (authed; only the session
  that opened the grant, else `404 not_found`): the grant with its latest
  16 `navigations` `[{seq, location}]`, oldest first: every frame document
  request (`Sec-Fetch-Mode: navigate`, `Sec-Fetch-Dest: iframe`; not a
  top-level page opened in a browser tab) numbered in arrival order, with
  its in-cluster path and query (`""` when longer than 4 KiB); end it
  (the tab closed).
- `GET|HEAD|POST|PUT|PATCH|DELETE|OPTIONS /proxy/{token}/{path...}`
  (public: **the token is the credential**; `/proxy/{token}` redirects to
  `/proxy/{token}/`) forwards to
  `<server>/api/v1/namespaces/<ns>/<services|pods>/<scheme>:<name>:<port>/proxy/<path>?<query>`.
  The path is outside `/api` so that relative links work; it may contain
  no dot segments or encoded slashes/backslashes (`400`). Errors of the
  proxy itself are short `text/plain` pages for the frame, not JSON.

**Grants** (`svcproxy.Grants`, in memory): 256 random bits (base64url);
bound to the KubeBench session token, user and cluster that opened them;
end after 1 h unused (`IdleTimeout`), 12 h in all (`MaxLifetime`), on
`DELETE`, or with the session; at most 32 per session and 4096 in all
(least recently used end first). Every proxied request re-checks: the
session exists (`401` after sign-out), no forced password change, the
cluster exists, the user still has `browse` on it (`403` after a
demotion or unassignment), and the read-only method rule. Switching the
active cluster does not end a grant: it stays on its own cluster.

**Why a token, not the session cookie.** A cookie-authorized proxy would
let any page it serves send requests with the user's cookie (Chromium
does not attach a `SameSite=Lax` cookie to requests of a sandboxed,
opaque-origin frame, but other browsers or a future change might), and
form posts from sandboxed pages carry `Origin: null`, which the CSRF check
(§3.5) must refuse for the API. With the token, `/proxy/` never reads the
session cookie, is exempt from the Origin check (a forged cross-site
request has no token), and the session cookie is stripped before
forwarding.

**Forwarding** (`net/http/httputil.ReverseProxy`, transport
`kube.Runtime.ProxyTransport`: the runtime's credential, HTTP/1.1 so
WebSocket upgrades work, no redirect following):

- Request headers dropped: `Authorization`, `Proxy-Authorization`,
  `Impersonate-*`, `X-Remote-*` (they would replace or extend KubeBench's
  credential at the API server), `Forwarded`, `X-Forwarded-*`, `Referer`
  (it carries the token), hop-by-hop headers, the `kubebench_session`
  cookie (other cookies pass), `Accept-Encoding` (the transport asks for
  gzip and decompresses, so HTML can be rewritten).
- Limits: request bodies ≤ 32 MiB (`413`), response headers within 1 min
  (`504`); bodies, server-sent events and WebSocket connections stream
  without a limit and end when the server stops.
- The API server rewrites absolute links in HTML to its proxy path; the
  proxy maps that path (with or without the server URL's own path prefix)
  back to `/proxy/<token>` in `text/html` bodies (streaming; also as an
  absolute URL `http(s)://` or `//` naming the API server's host, with or
  without its port, which backends build from the `Host` they got, so the
  token never goes to the API server's host) and in
  `Location`; a `Location` that is an absolute path, or an absolute URL
  naming the API server's host (backends build it from the `Host` they
  got), is moved below the grant too, its `.`/`..` segments resolved first
  so that it cannot climb out of `/proxy/<token>`; other hosts pass
  unchanged (the frame navigates there itself). The `Location` is read as a
  browser reads it (surrounding spaces and controls dropped, `\` as `/`);
  one that does not parse, or an `http(s):` URL without a host, is
  removed.
- `Set-Cookie`: `kubebench_session` is dropped; other cookies lose
  `Domain` and get their `Path` below `/proxy/<token>`.
- Response headers removed: `Strict-Transport-Security`,
  `Clear-Site-Data`, `Service-Worker-Allowed`, `X-Frame-Options`,
  `Alt-Svc`, and the `frame-ancestors` directive of the application's
  CSP (other directives stay).
- Added: `Content-Security-Policy: sandbox allow-scripts allow-forms
  allow-popups allow-downloads; frame-ancestors <origin that opened the
  grant>` (no `allow-same-origin`, no top navigation, no modals: a page
  cannot prompt for a password in KubeBench's name) and
  `X-Frame-Options: SAMEORIGIN`; `Referrer-Policy: no-referrer`;
  `Cross-Origin-Resource-Policy: cross-origin` (the sandboxed page's own
  subresources are cross-origin loads from its opaque origin; a stricter
  value is not dependable across browsers, and other sites cannot name the
  URL anyway). Requests of the sandboxed page with `Origin: null` get
  `Access-Control-Allow-Origin: null` (no credentials) and preflights are
  answered by the proxy, so the page's own fetches work.

**Separate proxy origin** (`KUBEBENCH_PROXY_ORIGIN`, §2): the same grants
served on another origin, which serves nothing but `/proxy/` (everything
else `404`, recognised by `Host`, or `X-Forwarded-Host` behind a trusted
proxy). There responses have no CSP sandbox (only `frame-ancestors`), the
frame gets `allow-same-origin allow-modals allow-popups-to-escape-sandbox`
on top of the sandbox flags, and `base` is absolute; applications with
cookies and storage (sign-ins) work. KubeBench's cookies are host-only, so
they are not sent there, but cookies ignore ports and a page can set
cookies for its parent domain: a page on the same site could plant a
`kubebench_session` cookie (`path=/api`) that KubeBench would read. So the
proxy origin must be **another site** than KubeBench's origin as the
browser sees it (`appOrigin`): opening a grant is refused with `503
capability_unavailable` when the two host names are equal (whatever the
ports), one is a subdomain of the other, or they share a registrable
domain (public suffix list; IP addresses only by equality). Applications
opened there share one origin, so they are isolated from KubeBench, not
from each other (per-application subdomains are a possible follow-up).

**Frontend**:

- Core: a **side editor group** to the right of the editor area
  (`core/store/sideGroupStore.ts`, `core/shell/SideEditorGroup.tsx`;
  plugins call `openInSideGroup(tab)` from `core/plugin/sideGroup.ts`),
  shown while it has tabs, resizable from its left edge, tabs rendered by
  the registered tab renderers and kept mounted while inactive and while
  the bottom panel is maximized (the editors are hidden then, not
  unmounted), so a browser keeps its page, history and grant.
- Plugin `service-browser` (tools, no activity bar entry): resource action
  "Open in browser" on `Service` and `Pod` rows and detail toolbars
  (permission `browse`) opens tab kind `service-browser` with props
  `{kind: 'services'|'pods', namespace, name}` in the side group. The tab
  lists the TCP ports (Service `spec.ports`, Pod container ports; https
  guessed from names, `appProtocol` and 443/6443/8443/9443), opens a grant
  for the chosen port and scheme (new grant on change), ends it on close,
  and shows: back, forward, reload, the in-cluster address
  (`http://<svc>.<ns>:<port>`) with an editable path, port and scheme
  pickers, "open in a new browser tab" (the same proxy URL, sandboxed there
  too; `noopener`), a sandbox (or proxy origin) notice and a read-only
  notice. The frame is cross-origin, so the tab keeps its own history from
  the navigations the backend recorded and navigates by remounting the
  frame. Nested frames of the page navigate too: at each `load` event of
  the frame the tab takes the **first** navigation after the newest one it
  saw at the previous load (`model/frameLocation.ts`), because a page's own
  document is requested before its nested frames, and the frame's `load`
  waits for them. Limitation: a nested frame that navigates on its own
  between two loads of the frame (a link inside a widget) is taken for the
  page at the next load.
- Ingress (v0.7): "Open in browser" on `Ingress` rows and detail pages
  opens tab props `{kind: 'ingresses', namespace, name}`; the tab lists the
  Ingress's routes (every rule path, then the default backend, Service
  backends only: `model/ingress.ts`, the server's order) in a route picker
  (`host/path → service:port`) instead of the port picker; the default
  backend is flagged (`defaultBackend`, opened with `?default=true`), since
  a rule may have neither host nor path, guesses the
  scheme from the Service port, starts at the rule's path (`/` for a
  regular expression) and shows the rule's host as the address
  (`https://` when the Ingress lists it under `tls`).
- Mock: ports from the mock dataset; the frame shows a deterministic
  placeholder page (`srcdoc`) naming the address and path; "open in a new
  tab" is disabled.

**Tests**: `internal/svcproxy` (validation, path cleaning, grant expiry and
limits, header filtering, Location/Set-Cookie/HTML rewriting across read
boundaries), `internal/api` (viewer refused and audited, one audit row per
grant, forwarding through a fake API server with credentials, session cookie
and impersonation stripped, read-only method rule for old and new grants,
access lost: sign-out, demotion, unassignment, cluster removal; proxy
origin), and a live Playwright check on k3d with a hostile in-cluster page
(document.cookie, storage, parent/top access, credentialed `fetch`/XHR of
`/api/auth/me`, a namespace create, a popup, top navigation, a planted
session cookie and `Clear-Site-Data`) — none of which reaches KubeBench —
plus form posts and WebSocket echo through the proxy.

### 7.15 OpenShift (v0.8)

KubeBench knows OpenShift's own APIs, version and security model. Every
part is read-only (no new OpenShift mutations) and appears only where the
cluster serves the API group, so other clusters see no change.

**Resource types** (generic catalog, `kube.Catalog` + workloads
`openshiftCatalog.tsx`): `routes` (route.openshift.io/v1), `projects`
(project.openshift.io/v1, cluster-scoped), `securitycontextconstraints`
(security.openshift.io/v1, cluster-scoped), `deploymentconfigs`
(apps.openshift.io/v1, deprecated upstream but common), `buildconfigs` and
`builds` (build.openshift.io/v1), `imagestreams` and `imagestreamtags`
(image.openshift.io/v1).

- They are *optional* (`kube.OptionalResources`): the capability probe
  reads the server's group list once (`ServerGroups`, one request) and
  reports the served keys as `capabilities.optionalResources` of
  `/api/info`. The UI lists only those (`ResourceDescriptor.optional`,
  `isServed`): the workloads sidebar groups them under "OpenShift"
  (Projects under Cluster, next to Namespaces), the palette's `get`/`describe`
  kinds (short names `dc`, `bc`, `is`, `istag`, `scc`) likewise; a group
  without entries is hidden. `/api/resource` lists an unserved type as
  empty, like any type the cluster does not serve.
- Columns: Route host (a link to the public URL), path, Service and
  targetPort, TLS termination, admission (every router's `Admitted`
  condition: Admitted / Not admitted with the reason / Pending); Project
  display name, phase, requester; SCC privileged, runAsUser and SELinux
  strategies, number of users and groups; DeploymentConfig ready/desired,
  trigger types, latest version; BuildConfig strategy, source (Git URI),
  last build; Build phase, strategy, duration (`status.duration`, else
  start to completion); ImageStream repository (public, else internal)
  and tags; ImageStreamTag image reference.
- Detail pages are the generic detail page with the kind's extra rows
  (`details`: a Route's routers and alternate backends, a BuildConfig's
  output, a Build's reason, message and output image, an SCC's users,
  groups and volumes) and, for every generic kind, a Conditions section
  from `status.conditions`.
- Live-update topics are group-qualified (`ResourceDescriptor.topic`, e.g.
  `builds.build.openshift.io`: `config.openshift.io` has `builds` and
  `projects` too); ImageStreamTags, which cannot be watched, follow their
  ImageStreams.
- **Credentials** (`kube/credentials.go`): a Route's TLS private key
  (`spec.tls.key`), a BuildConfig's inline webhook secrets
  (`spec.triggers[].github|generic|gitlab|bitbucket.secret`) and a Build's
  (`spec.triggeredBy[].githubWebHook|genericWebHook|gitlabWebHook|bitbucketWebHook.secret`)
  are redacted like Secret values on every read path (`/api/resource`,
  `/api/resource/get`, `/api/yaml`, Helm manifests), with the
  last-applied annotation; there is no reveal route for them. Apply, edit
  and metadata writes refuse their placeholders (`400 redacted_secret`,
  `manifest.credential_placeholder`, `edit.credential_placeholder`, the
  `*_annotation_placeholder` codes, `editing.annotation_placeholder`); a
  merge patch replaces lists whole, so an edit that changes a BuildConfig's
  trigger list while a secret in it is redacted is refused. Dry runs that
  touch these kinds need write access and are audited, like Secret dry
  runs (§3.2).
- Topology (§7.9): Route → Service edges (`spec.to` and
  `alternateBackends` of kind Service), Routes in the Ingress column with
  the Ingress status rules; "Show topology" on Routes.

**Service browser** (§7.14): `POST /api/proxy/routes/{ns}/{name}?scheme=`
(browse, audited once as `route.proxy`, target `route/<ns>/<name>`,
detail `{"scheme","host","path","service","readOnly"?}`) opens a grant for
the Service port behind a Route, read through the dynamic client: the
targetPort as the router resolves it (a name is a Service port name, then
a named target port; a number is an endpoint port: the Service port's
numeric target port, then the Service port whose EndpointSlice ports
(label `kubernetes.io/service-name`) have that number, e.g. a container
port behind a named target port, then the Service port itself; none: the
first port), the Route's host as the grant's `host`. A
Route to something other than a Service (`proxy.route_not_service`) or
without a matching port (`proxy.route_no_port`) is `400`. The router
itself (TLS termination, rewrites) is not involved. "Open in browser" on
Route rows and detail pages opens tab props `{kind: 'routes', namespace,
name}`: the address bar shows `https://<host>` (https when the Route
terminates TLS), the frame starts at the Route's path, the scheme picker
defaults to https for passthrough and re-encrypt Routes, and an "Open
route URL" button opens the public `http(s)://<host><path>` through the
router in a new browser tab (`noopener`).

**Version support** (§7.12): detection reads the ClusterVersion `version`
(config.openshift.io/v1; the newest `Completed` entry of
`status.history`, else `status.desired.version`) into
`Detection.ProductVersion`; a readable ClusterVersion also marks the
cluster as OpenShift. `cloud.SupportOf` judges OpenShift by that version
against `openShiftLifecycle` (`internal/cloud/openshift.go`: 4.12–5.0,
full support, maintenance support and EUS term 1 ends, with sources and
an as-of date), everything else as before. `status` follows the end of
maintenance support (`endDate`; EUS is not assumed, like the clouds'
extended support), `phase` is `full`, `maintenance`, `eus` or `ended`,
with `fullSupportEnd` and `eusEnd`; without a readable ClusterVersion the
status is `unknown`. `platform.productVersion` carries the OpenShift
version; the provider badge shows it ("OpenShift 5.0.0 (Kubernetes
v1.36.5)" and the phases in its tooltip), the overview notice names
OpenShift and, in the EUS phase, warns instead of erroring.

**Security plugin** (§7.7):

- System namespaces (excluded by default): on a cluster that serves
  `security.openshift.io`, `kube-*` and `openshift-*` (the prefixes
  OpenShift reserves), `openshift`, and namespaces labelled
  `openshift.io/run-level` (any value) or
  `openshift.io/cluster-monitoring=true`; on other clusters only
  `kube-system`, `kube-public` and `kube-node-lease`, as before (a
  `kube-flannel` or `kube-prometheus-stack` is scanned). ServiceAccounts
  there are system subjects. The report's `systemNamespaces` lists the cluster's
  (sorted), the UI names four and counts the rest.
- SCCs are listed through the dynamic client (`internal/openshift`); not
  served: the OpenShift checks neither run nor count as skipped, and the
  category `openshift` is left out of `byCategory`; listing refused: the
  SCC-target checks are skipped with the reason and the others judge SCCs
  by OpenShift's default risky names. An SCC is *privileged* when it
  allows privileged containers, host directories, network, ports, PID or
  IPC, any capability or one that breaks out of the container
  (`allowedCapabilities` or `defaultAddCapabilities` with `ALL`, `BPF`,
  `DAC_READ_SEARCH`, `NET_ADMIN`, `SYS_ADMIN`, `SYS_BOOT`, `SYS_MODULE`,
  `SYS_PTRACE` or `SYS_RAWIO`, `CAP_` prefix or not), or hostPath
  volumes, *anyuid* when containers may run as root (`runAsUser`
  `RunAsAny`, `MustRunAs` with `uid: 0`, or `MustRunAsRange` with
  `uidRangeMin: 0`; an unset UID or range is the namespace's), else
  restricted; *default* when OpenShift ships it
  (`include.release.openshift.io/*` annotations, ClusterVersion owner, or
  `app.kubernetes.io/part-of: openshift-*`).
- Checks (category `openshift`, frameworks "CIS OpenShift 5.2.x",
  "OpenShift SCC guide"): `scc-privileged-subjects` (high, target `scc`):
  a privileged or anyuid SCC lists users or groups in its own fields that
  are not the cluster's (system users, groups and ServiceAccounts of system
  namespaces are, unless they stand for everyone); `scc-privileged-binding`
  (high, bindings): a role bound to non-system subjects may `use` a
  privileged or anyuid SCC (by name, or every SCC without resourceNames) —
  OpenShift's `system:openshift:scc:*` names do not make such a binding a
  system one, `oc adm policy add-scc-to-user` creates them;
  `scc-privileged-workload` (high, workloads): pods were admitted with a
  privileged or anyuid SCC (`openshift.io/scc` annotation, reported on the
  pod's Deployment, CronJob, StatefulSet, DaemonSet or the pod; build pods
  are left out, the build controller runs every build privileged);
  `scc-custom-privileged` (medium, `scc`): an SCC OpenShift does not ship
  allows privileged containers or host directories. With `includeSystem`
  every subject counts. Finding keys use the kind
  `SecurityContextConstraints`; `inventory` counts
  `securitycontextconstraints`.
- Measured on OpenShift 5.0 (CRC, default options): 633 findings, 505 in
  `openshift-*` namespaces, before; 73, none there, after (4 of them SCC
  findings: the hostpath-provisioner add-on's custom privileged SCC, its
  grant and DaemonSet, and `kubeadmin`'s cluster-admin grant of every SCC).

**Access plugin** (§7.8): `GET /risky` adds `scc-privileged` (critical)
and `scc-anyuid` (high) for rules that may `use` such SCCs, judged by the
cluster's SCCs (classification above; OpenShift's default names when they
cannot be listed; none off OpenShift).

**Mock**: a fourth mock cluster `ocp-lab` (OpenShift 5.0.0, full support)
serves the OpenShift types (`data/mock/mockOpenShift.ts`: Routes, one not
admitted and one to a missing Service, Projects, SCCs incl. a custom
privileged one, a DeploymentConfig, a BuildConfig with three Builds, an
ImageStream and its tag), its security report has the SCC findings and
its RBAC the `system:openshift:scc:*` grants; the other mock clusters
serve none of it.

## 8. Distribution

- **Kubernetes**: Helm chart `charts/kubebench` (OCI `ghcr.io/shavell/charts/kubebench`
  and attached to releases) and rendered manifests
  `deploy/kubernetes/kubebench.yaml` (`kubectl apply -f`).
- **VM**: `.deb` (and `.rpm`) via nfpm/GoReleaser — binary `/usr/bin/kubebench`,
  systemd unit `kubebench.service`, config `/etc/kubebench/kubebench.env`,
  data `/var/lib/kubebench`, system user `kubebench`.
- **Installer**: `install.sh` — `curl -fsSL https://raw.githubusercontent.com/Shavell/kubebench/main/install.sh | sudo bash`
  (online: downloads the release for the detected arch, verifies
  SHA-256) or `sudo bash install.sh --offline ./kubebench-offline-<ver>.tar.gz`.
- **Offline bundle** `kubebench-offline-<version>.tar.gz`: binaries,
  .deb/.rpm, container image tarballs (amd64/arm64), Helm chart `.tgz`,
  manifests, `install.sh`, checksums, README.
- **Container image** `ghcr.io/shavell/kubebench:<version>` (multi-arch).
- **GitHub Actions**: `ci.yml`, `codeql.yml`, `trivy.yml`, `scorecard.yml`
  and `dependency-review.yml` run on pushes to `main`, pull requests and
  weekly schedules; `release.yml` runs on pushed `v*` tags (or by hand),
  `docker-image.yml` builds without pushing for pull requests that touch
  the image, and `helm-chart.yml` stays manual. Releases carry SBOMs, a keyless cosign
  signature of `checksums.txt` and build provenance attestations
  (`docs/releasing.md`).
- **Dependabot**: npm, gomod (`/backend`), github-actions, docker; weekly,
  grouped.
