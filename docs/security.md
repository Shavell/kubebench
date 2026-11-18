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
| browse: open a Service's or Pod's web interface (service browser) | ✓ | ✓ | — |
| contribute: create, edit, pin and delete shared notes (also on read-only clusters) | ✓ | ✓ | — |
| users, clusters, plugins, audit, update settings | ✓ | — | — |

**Read-only clusters.** An admin can flag a cluster read-only: every write
and exec request is refused for everyone, admins included, until the flag is
cleared. Use it for production clusters people should only look at. The
service browser stays available there, but sends only GET and HEAD and opens
no WebSocket connections.

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
- Single sign-on through an OpenID Connect provider and sign-in with an
  LDAP directory or Active Directory (v0.7; see below).

## External identities

With `KUBEBENCH_OIDC_*` set, users can sign in through an OpenID Connect
provider, and with `KUBEBENCH_LDAP_*` with their directory account
([configuration](configuration.md#external-identities), design §3.6):

- **Flow.** Authorization code flow with PKCE (S256), `state` bound to the
  browser by an HttpOnly cookie, and `nonce`. The ID token is verified
  against the provider's discovery document and signing keys (issuer,
  audience, expiry, nonce). TLS to the provider is always verified; add a
  private CA with `KUBEBENCH_OIDC_CA_FILE`.
- **Secrets.** The client secret comes from the environment, a file
  (`KUBEBENCH_OIDC_CLIENT_SECRET_FILE`) or a Kubernetes Secret (chart
  `identity.oidc.existingSecret`). No API returns it (Settings shows only
  whether it is set), and it is never logged or audited. The same holds
  for the LDAP bind password (`KUBEBENCH_LDAP_BIND_PASSWORD_FILE`, chart
  `identity.ldap.existingSecret`).
- **Directory sign-in.** The password is checked by binding as the user
  (KubeBench never sees password hashes); empty passwords are refused
  before the directory is contacted, since a bind without a password is an
  unauthenticated bind many directories accept. Login names are escaped in
  search filters, and a login matching no entry or several entries is
  refused. Active Directory accounts with the disabled flag are refused.
  Use `ldaps://` or StartTLS: the bind sends the user's password to the
  directory. Certificates are verified (add a private CA with
  `KUBEBENCH_LDAP_CA_FILE`); `KUBEBENCH_LDAP_INSECURE_SKIP_VERIFY` and plain
  `ldap://` exist for tests and are flagged at startup and in Settings. The
  service account only needs read access to users and groups.
- **Roles from groups.** External users get their global role and cluster
  roles from group mappings, re-computed at every sign-in; manual edits do
  not survive the next sign-in. Map narrowly: whoever controls group
  membership at the provider controls KubeBench admin rights.
- **No takeover.** An external sign-in never adopts an existing account:
  a username that belongs to a local user (or another provider's user) is
  refused. External users have no local password.
- **Break-glass.** The last local admin cannot be deleted or demoted, so
  local sign-in keeps working when the provider is unavailable. Keep that
  password in your secret store and use it only in emergencies (its
  sign-ins are visible in the server log; external sign-ins are audited).
- **Revocation.** There is no background sync: disabling a user at the
  provider stops new sign-ins, while open sessions last until they expire
  (`KUBEBENCH_SESSION_TTL`) unless the user tries to sign in again (a
  directory account found disabled loses its sessions then). Use a short
  TTL with external sign-in, or delete the user in Users to end their
  sessions at once.
- **Audit and throttling.** Every external sign-in is audited
  (`auth.signin`, with the failure class of refused ones), as is every
  group mapping change; failed callbacks are throttled per client IP like
  failed logins, failed directory sign-ins per client IP and login name
  (all forms of a login, such as `CORP\jdoe` and `jdoe`, share one count).

## Service browser

"Open in browser" (design §7.14) shows the web interface of a Service or
Pod through the API server's proxy subresource, **with KubeBench's cluster
credential**. That reaches inside the cluster network, including admin
interfaces of in-cluster applications that trust it, so:

- **Who.** It needs the `browse` access: operator or admin on the cluster,
  never viewer. It is not a change of Kubernetes objects, so a read-only
  cluster allows it, but only GET and HEAD requests are forwarded there
  (a form post to an in-cluster admin UI is a change too), and WebSocket
  upgrades are refused: a web terminal or database console sends its
  commands over a WebSocket, which no method rule sees.
- **Audit.** Opening a page writes one row, `service.proxy` (or
  `pod.proxy`) with target `service/<ns>/<name>:<port>`; the requests the
  page then makes are not recorded one by one.
- **Credential.** Opening returns a grant: 256 random bits in the URL,
  `/proxy/<token>/…`. Proxied requests are authorized by that token alone,
  never by the session cookie; each one checks again that the session that
  opened it is alive and still holds `browse` on that cluster. A grant ends
  after 1 h unused, 12 h in all, when its tab closes, or with the session.
  It reaches only its one Service or Pod port. Responses carry
  `Referrer-Policy: no-referrer`, so the token does not leak in `Referer`.
- **What is forwarded.** The browser's `Authorization`, `Proxy-Authorization`,
  `Impersonate-*` and `X-Remote-*` headers are dropped (they would replace
  or extend KubeBench's credential at the API server), and so are
  `Referer` and the `kubebench_session` cookie. The application's own
  cookies pass, scoped to `/proxy/<token>/`. Answers that would act on
  KubeBench's origin are removed: `Set-Cookie` of `kubebench_session`,
  `Clear-Site-Data`, `Strict-Transport-Security`, `Service-Worker-Allowed`,
  `X-Frame-Options` and `frame-ancestors` of the application. Redirects are
  never followed by the proxy; `Location` headers pointing into the
  application are rewritten below the grant.
- **Isolation.** On KubeBench's own origin every proxied response carries
  `Content-Security-Policy: sandbox allow-scripts allow-forms allow-popups
  allow-downloads; frame-ancestors <KubeBench>` (no `allow-same-origin`,
  no top navigation, no modal dialogs), and the frame has the same
  `sandbox`. The page runs in an opaque origin: it cannot read KubeBench's
  cookies, storage or frames, navigate KubeBench, or send a request that
  KubeBench would accept as the user's (Chromium does not even attach the
  session cookie; CORS keeps the answers unreadable, and the `Origin: null`
  of its posts fails the CSRF check of every `/api` route). A live
  Playwright test with a hostile page checks exactly this.
- **Separate origin.** Applications that need cookies or storage (sign-ins)
  do not work sandboxed. `KUBEBENCH_PROXY_ORIGIN` serves the same grants
  on another origin, unsandboxed; that origin serves nothing but `/proxy/`.
  It must be **another site** than KubeBench: cookies ignore ports, and a
  page can set cookies for its parent domain, so a proxied page on
  KubeBench's host (on another port), on a subdomain or on a sibling
  subdomain could plant a `kubebench_session` cookie and replace the
  user's session. KubeBench refuses to open pages when the proxy origin's
  host equals KubeBench's, is a subdomain of it (or the other way round)
  or shares its registrable domain; use a host on another domain, e.g.
  `kubebench-proxy.example.net` for `kubebench.example.com`. All applications
  opened there share that one origin, so they are isolated from KubeBench,
  not from each other.

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

## Outbound connections (no tracking)

KubeBench does not track its users or customers: there is no telemetry,
no usage analytics, no crash reporting, no license or activation check and
no third-party script, font or CDN in the web UI (Monaco and every other
asset are bundled). The project receives no data about your installation.

The server opens outbound connections only to:

| Destination | When | How to avoid it |
|---|---|---|
| The update source (`KUBEBENCH_UPDATE_URL`, GitHub by default) | Update check, at most every few hours, one GET with `User-Agent: KubeBench/<version>` | `KUBEBENCH_UPDATE_MODE=off` (sends nothing) |
| The Kubernetes API servers of the clusters an admin added | Every cluster operation | — (that is the product) |
| Helm repositories and OCI registries | Only when a user adds a repository, updates it, searches it or installs/upgrades a chart from it | Do not configure remote repositories; upload `.tgz` charts instead |
| Cloud token endpoints (AWS STS, Google, Microsoft Entra ID) | Only for EKS, GKE or AKS clusters that use the in-process token providers | Use static credentials in the kubeconfig |
| In-cluster services | Only through the service browser, at the user's request, via the API server | Do not grant the `browse` access |
| The OIDC provider (`KUBEBENCH_OIDC_ISSUER`) | Discovery, keys, token and userinfo requests during sign-ins, and the admins' provider status check | Leave OIDC unconfigured |

The browser loads everything from KubeBench itself; the only external
links in the UI are documentation and release pages that open when a user
clicks them.

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
- [ ] If you set `KUBEBENCH_PROXY_ORIGIN`, use a host on another domain than
      KubeBench's: not its host on another port, not a subdomain (KubeBench
      refuses those).

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
