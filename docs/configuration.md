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
| `KUBEBENCH_UPDATE_URL` | `https://api.github.com/repos/Shavell/kubebench/releases` | Releases list the update check reads (GitHub's "list releases" JSON); a mirror or a GitHub Enterprise API works too. `http` or `https` |
| `KUBEBENCH_PROXY_ORIGIN` | unset | Separate origin (`https://host[:port]`, no path) for the service browser (design §7.14): proxied pages run there unsandboxed, isolated from KubeBench by the origin. The host must reach this server too (DNS, reverse proxy, certificate); it serves only `/proxy/`. It must be on another domain than KubeBench (not KubeBench's host on another port, not a subdomain: cookies ignore ports and reach subdomains); otherwise the service browser refuses to open pages. Unset: pages run sandboxed on KubeBench's own origin |
| `KUBEBENCH_PUBLIC_URL` | unset | The URL users reach KubeBench at (`https://host[:port]`, no path). The OIDC redirect URL is `<KUBEBENCH_PUBLIC_URL>/api/auth/oidc/callback`. Unset: derived from each request (scheme from TLS, host from `Host`; behind a trusted proxy from `X-Forwarded-Proto`/`X-Forwarded-Host`). Set it whenever OIDC is on |
| `KUBEBENCH_AUTH_REQUIRE_GROUP` | `false` | Refuse external users whose groups grant nothing ([external identities](#external-identities)); otherwise they sign in and see no cluster |
| `KUBEBENCH_AUTH_GROUP_MAPPINGS` | unset | Group mappings seeded on the first start that has them, a JSON list: `[{"group":"kb-admins","role":"admin"},{"group":"ops","role":"operator","cluster":"prod","source":"oidc"}]` |
| `KUBEBENCH_OIDC_ISSUER` | unset | Issuer URL of the OpenID Connect provider; setting it turns on "Sign in with …" |
| `KUBEBENCH_OIDC_CLIENT_ID` | unset | Client ID (required with the issuer) |
| `KUBEBENCH_OIDC_CLIENT_SECRET`, `KUBEBENCH_OIDC_CLIENT_SECRET_FILE` | unset | Client secret, or a file holding it (a mounted Kubernetes Secret, a systemd credential); not both. Unset: a public client (PKCE only). Never returned by the API, logged or audited |
| `KUBEBENCH_OIDC_NAME` | `SSO` | Provider name on the sign-in button ("Sign in with …") |
| `KUBEBENCH_OIDC_SCOPES` | `openid profile email` | Requested scopes (comma or space separated; `openid` is always added) |
| `KUBEBENCH_OIDC_USERNAME_CLAIM` | `preferred_username` | Claim holding the KubeBench username (1–64 letters, digits, `.`, `_`, `@`, `-`) |
| `KUBEBENCH_OIDC_GROUPS_CLAIM` | `groups` | Claim holding the groups; a dotted path (`realm_access.roles`) reads nested claims. Read from userinfo when the ID token lacks it |
| `KUBEBENCH_OIDC_ALLOWED_DOMAINS` | unset | Admit only verified email addresses of these domains (comma separated): the token must carry `email_verified` as `true` (or the string `"true"`); a missing claim is refused, so providers that never send it (Entra ID) cannot use this setting |
| `KUBEBENCH_OIDC_CA_FILE` | unset | PEM CA bundle trusted for the provider's TLS certificate, in addition to the system roots (TLS verification is always on) |
| `KUBEBENCH_LDAP_URL` | unset | `ldaps://host[:port]` or `ldap://host[:port]` of the directory; setting it turns on directory sign-in ([LDAP and Active Directory](#ldap-and-active-directory)) |
| `KUBEBENCH_LDAP_TYPE` | `openldap` | `openldap` or `ad`; picks the defaults of the filters and attributes below |
| `KUBEBENCH_LDAP_NAME` | `Directory` (`Active Directory` for `ad`) | Name on the sign-in screen |
| `KUBEBENCH_LDAP_START_TLS` | `false` | Upgrade an `ldap://` connection with StartTLS before anything is sent |
| `KUBEBENCH_LDAP_CA_FILE` | unset | PEM CA bundle trusted for the directory's certificate, in addition to the system roots |
| `KUBEBENCH_LDAP_INSECURE_SKIP_VERIFY` | `false` | Do not verify the directory's certificate. **Tests only**: anyone on the network path can read the passwords; logged as a warning and shown in Settings > Authentication |
| `KUBEBENCH_LDAP_BIND_DN` | unset | Service account that searches users and groups; unset searches anonymously |
| `KUBEBENCH_LDAP_BIND_PASSWORD`, `KUBEBENCH_LDAP_BIND_PASSWORD_FILE` | unset | Its password, or a file holding it; not both. Required with the bind DN. Never returned by the API, logged or audited |
| `KUBEBENCH_LDAP_USER_BASE_DN` | unset | Where users are searched (required) |
| `KUBEBENCH_LDAP_USER_FILTER` | `(uid=%s)`; `ad`: `(\|(sAMAccountName=%s)(userPrincipalName=%s))` | Finds the user; `%s` is the escaped login name |
| `KUBEBENCH_LDAP_USERNAME_ATTRIBUTE` | `uid`; `ad`: `sAMAccountName` | The KubeBench username (lower case) |
| `KUBEBENCH_LDAP_ID_ATTRIBUTE` | `entryUUID`; `ad`: `objectGUID` | Stable ID of the entry; a renamed user keeps their KubeBench account |
| `KUBEBENCH_LDAP_DISPLAY_NAME_ATTRIBUTE`, `KUBEBENCH_LDAP_EMAIL_ATTRIBUTE` | `cn` (`ad`: `displayName`), `mail` | Profile shown in Users |
| `KUBEBENCH_LDAP_GROUPS_FROM` | `search` | `search` (a group search) or `memberOf` (the user entry's direct `memberOf` values) |
| `KUBEBENCH_LDAP_GROUP_BASE_DN` | the user base DN | Where groups are searched |
| `KUBEBENCH_LDAP_GROUP_FILTER` | `(\|(member=%d)(uniqueMember=%d)(memberUid=%u))`; `ad`: `(member:1.2.840.113556.1.4.1941:=%d)` | Finds the user's groups; `%d` is the user's DN, `%u` its username attribute. The AD default follows nested groups |
| `KUBEBENCH_LDAP_GROUP_NAME_ATTRIBUTE` | `cn` | The group name mappings match; `dn` uses the group's full DN |
| `KUBEBENCH_LDAP_TIMEOUT` | `10s` | Connect and request timeout (at least `1s`) |
| `KUBEBENCH_OPENSHIFT_MONITORING_ADDRESS` | unset | `host:port` KubeBench connects to for OpenShift monitoring's Thanos Querier instead of resolving its host; the Route's host (or the Service name in a pod) still names the TLS server and the `Host` header. For a router the Route's host does not resolve to from the server, e.g. CRC behind a port forward ([OpenShift monitoring](deployment.md#openshift-monitoring)) |
| `KUBEBENCH_OPENSHIFT_MONITORING_INSECURE` | `false` | Do not verify the Thanos Querier's TLS certificate. By default it is verified against the system roots plus the service CA (in a pod) or the cluster's default ingress CA (`openshift-config-managed/default-ingress-cert`, outside the cluster) |

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
| `config.publicUrl` | `KUBEBENCH_PUBLIC_URL` (set only when not empty) | `""` |
| `identity.requireGroup` | `KUBEBENCH_AUTH_REQUIRE_GROUP` (set only when `true`) | `false` |
| `identity.groupMappings` | `KUBEBENCH_AUTH_GROUP_MAPPINGS` (as JSON) | `[]` |
| `identity.oidc.*` | `KUBEBENCH_OIDC_*`; the client secret from `identity.oidc.existingSecret` or a chart Secret, the CA bundle from a ConfigMap | disabled |

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
updates, keyboard shortcuts, interface scale, editor and terminal fonts,
namespace favourites, palette aliases) live in the browser's local storage;
the interface scale, the fonts and the aliases are kept per signed-in user.

## External identities

Besides local accounts, users can sign in through an OpenID Connect
provider, an LDAP directory or Active Directory (design §3.6, v0.7); OIDC
and a directory can be configured together, and the sign-in screen then
offers both. They share the group mappings, the just-in-time users and
the break-glass rule below.

### OpenID Connect

The flow is the authorization code flow with PKCE, state and nonce; the ID
token is verified against the provider's discovery document and keys.

1. Register a confidential client at the provider with the redirect URL
   `<KUBEBENCH_PUBLIC_URL>/api/auth/oidc/callback` and, for sign-out at the
   provider too, the post-logout redirect URL `<KUBEBENCH_PUBLIC_URL>/`.
   Make the provider put the user's groups into a claim (Keycloak: a
   "Group Membership" mapper named `groups`, "Full group path" off).
2. Set `KUBEBENCH_PUBLIC_URL`, `KUBEBENCH_OIDC_ISSUER`,
   `KUBEBENCH_OIDC_CLIENT_ID` and the client secret (preferably
   `KUBEBENCH_OIDC_CLIENT_SECRET_FILE`).
3. Map groups to roles in Settings > Authentication (or seed them with
   `KUBEBENCH_AUTH_GROUP_MAPPINGS`): a group grants the global admin role,
   or the viewer or operator role on a cluster named in the inventory.

### LDAP and Active Directory

A service account searches the user, then KubeBench binds as the user to
check the password (it never reads or compares password hashes) and reads
the user's groups as the service account.

1. Create a service account that may read users and groups (in Active
   Directory any domain user can; in OpenLDAP grant it `read` in the ACLs).
2. Set `KUBEBENCH_LDAP_URL` (prefer `ldaps://`, or `ldap://` with
   `KUBEBENCH_LDAP_START_TLS=true`), `KUBEBENCH_LDAP_TYPE`,
   `KUBEBENCH_LDAP_BIND_DN`, the password (preferably
   `KUBEBENCH_LDAP_BIND_PASSWORD_FILE`), `KUBEBENCH_LDAP_USER_BASE_DN` and,
   for a private CA, `KUBEBENCH_LDAP_CA_FILE`.
3. Map the directory's group names (their `cn`) to roles, as for OIDC.
   "Test sign-in" with the directory looks a user up and shows the groups
   it found.

Active Directory (`KUBEBENCH_LDAP_TYPE=ad`):

```sh
KUBEBENCH_LDAP_URL=ldaps://dc1.corp.example.com
KUBEBENCH_LDAP_TYPE=ad
KUBEBENCH_LDAP_CA_FILE=/etc/kubebench/ldap-ca.pem
KUBEBENCH_LDAP_BIND_DN=CN=svc-kubebench,OU=Service Accounts,DC=corp,DC=example,DC=com
KUBEBENCH_LDAP_BIND_PASSWORD_FILE=/run/secrets/kubebench-ldap
KUBEBENCH_LDAP_USER_BASE_DN=DC=corp,DC=example,DC=com
KUBEBENCH_LDAP_GROUP_BASE_DN=OU=Groups,DC=corp,DC=example,DC=com
```

- Users sign in as `jdoe`, `CORP\jdoe` or `jdoe@corp.example.com`; all
  three reach the KubeBench user `jdoe` (the `sAMAccountName`, lower case).
- Group membership is nested: a member of a group inside `kb-ops` counts
  as a member of `kb-ops`.
- Accounts with the "account is disabled" flag are refused, and their
  KubeBench sessions end at that attempt.

OpenLDAP (the default type) finds users by `uid` and groups by `member`,
`uniqueMember` or `memberUid`; with the memberOf overlay,
`KUBEBENCH_LDAP_GROUPS_FROM=memberOf` reads the user's direct groups
instead. The default filters can be replaced, e.g.
`KUBEBENCH_LDAP_USER_FILTER=(&(objectClass=inetOrgPerson)(mail=%s))`.

Directory sign-in refuses empty passwords before contacting the directory
(an LDAP bind with no password is an unauthenticated bind that proves
nothing). Login names are case-insensitive; failed attempts count against
the login rate limit per client IP and login name, whichever form of it
the user typed (`CORP\jdoe` and `jdoe` share one count). Connections time
out after `KUBEBENCH_LDAP_TIMEOUT`; a few service-account connections are
reused between sign-ins. A plain `ldap://` URL without StartTLS and
`KUBEBENCH_LDAP_INSECURE_SKIP_VERIFY` are logged as warnings at startup and
shown as warnings in Settings > Authentication.

### External users

How external users behave:

- They are created at their first sign-in, have no local password (the
  local sign-in and password changes refuse them) and keep their KubeBench
  user across username changes at the provider (matched by subject).
- Their global role and cluster roles are **re-computed from their groups
  at every sign-in**; mapped groups win over manual edits in Users. Groups
  match exactly, ignoring case; several mappings for one cluster grant the
  strongest role.
- Without a granting group they sign in and see no cluster, or are refused
  with `KUBEBENCH_AUTH_REQUIRE_GROUP=true`.
- A username that belongs to another account (a local user, or a user of
  another provider), in any letter case, is refused; an external sign-in
  never takes over an account or sits next to it as `Admin` beside the
  local `admin`.
- Sessions use the session cookie and `KUBEBENCH_SESSION_TTL`. There is no
  background sync: a user disabled or removed from groups at the provider
  keeps the current session until it expires or they sign in again (a
  refused sign-in of a known account ends its sessions, e.g. a directory
  account found disabled). Use a short TTL with external sign-in. Sign-out
  ends the provider's session too when it supports RP-initiated logout.

**Break-glass:** local accounts always work. The last local admin cannot be
deleted or demoted (external admins do not count), so a local admin can
always sign in when the provider is down or misconfigured. Keep its
password in your secret store.

Every external sign-in is audited (`auth.signin`, with the failure class of
a refused one), as is every mapping change. Callbacks refused after the
provider redeemed their code count against the login rate limit per client
IP (a callback with a wrong state or a made-up code does not, so nobody can
lock SSO for others behind the same address), failed directory sign-ins
per client IP and login name. Admins can check what a user would get
with "Test sign-in" in Settings > Authentication.
