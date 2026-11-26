# KubeBench roadmap

This page lists what is being built now, what is likely to come next, and
what has already shipped. It is a statement of direction, not a promise:
items move, merge or disappear as we learn more.

## How the roadmap works

- **Themes, not dates.** Work is grouped by theme. Only the version in
  progress has a fixed scope; everything under [Next](#next-candidates)
  is a candidate, in no particular order.
- **GitHub is the place to discuss it.** Proposals, votes and questions go
  to [issues](https://github.com/Shavell/kubebench/issues) and
  [discussions](https://github.com/Shavell/kubebench/discussions) on
  `Shavell/kubebench`. Before you start on something large, open an issue
  so we can agree on the approach ([CONTRIBUTING.md](CONTRIBUTING.md)).
- **Versions follow [Semantic Versioning](https://semver.org/spec/v2.0.0.html).**
  While KubeBench is `0.x`, a minor version may change behaviour; the
  [CHANGELOG](CHANGELOG.md) says what to do when upgrading
  ([docs/releasing.md](docs/releasing.md) describes how a release is cut).
- **The CHANGELOG records what shipped.** This page records intent. When an
  item ships it moves to the CHANGELOG and leaves this page.
- **Sizes** are rough: **S** is a contained change in one layer, **M** a
  feature across the backend and the UI, **L** a new subsystem or a change
  to the security model.

## v0.6 (released 2026-11-18)

Two new plugins, built on the v0.5 plugin platform: notes (a backend
plugin with its own frontend plugin, like Argo CD or Storage) and the
service browser (a frontend plugin on new core proxy routes).

### Notes plugin

- Write notes attached to a cluster or to a single resource
  (kind, namespace and name), and find them again from that resource.
- **Personal** notes are visible only to their author; **shared** notes
  are visible to every user who can open the cluster, the way accepted
  security risks are shared today.
- Notes live in KubeBench's own database, so the managed cluster stays
  untouched: no CRDs, annotations or ConfigMaps are written.

### Service browser

- Open a Service's or Pod's web UI (a dashboard, an admin console, a
  health page) in a browser tab next to the editor, without a
  port-forward or an Ingress: back, forward, reload, path, port and
  scheme, open in a new browser tab.
- Traffic goes through the Kubernetes API server's proxy, using the
  cluster credentials KubeBench already holds, so the browser never needs
  network access to the cluster or its credentials. Operators and admins
  only; on read-only clusters only GET and HEAD requests are forwarded
  (no WebSocket connections).
- The page is shown sandboxed, so the proxied application cannot reach
  KubeBench's session, cookies or UI.
- An optional, separate proxy origin (a host on another domain) serves the pages
  unsandboxed, isolated from KubeBench by the origin, so apps that need
  cookies or storage (for example sign-ins) work too.

## v0.7 (released 2026-11-18)

### External identities

- **OIDC / SSO**: sign in with the organization's identity provider
  (authorization code flow with PKCE; Keycloak, Entra ID, Google, Okta, …).
- **LDAP and Active Directory**: directory sign-in over LDAPS or StartTLS
  with `uid`, `sAMAccountName`, UPN or `DOMAIN\user`, nested AD groups and
  disabled-account checks.
- Users are created at first sign-in; directory and IdP groups map to the
  admin role and per-cluster viewer/operator assignments, re-computed at
  every sign-in and managed in Settings → Authentication. A local
  break-glass admin always remains.

### Polish

- **Open an Ingress in the service browser**, **live shared notes**,
  **interface scale and font settings**, and a lint-clean frontend.

### Security and polish

- **Hashed session tokens**: the database keeps only a SHA-256 digest of
  each session token, so a copy of it does not hold usable sessions;
  sessions of earlier versions are converted in place on upgrade.
- **Localized server-side reasons and errors**: the server's own errors
  and unavailable reasons carry stable reason codes with parameters, which
  the UI shows in every UI language.

## v0.8 (released 2026-11-18)

### OpenShift

- **Least-privilege install**: the chart detects OpenShift, lets the SCC
  assign user and group IDs, pins `restricted-v2` (or `restricted-v3` with
  user namespaces), ships a Route template and an OpenShift NetworkPolicy;
  plain OpenShift manifests in `deploy/openshift/`.
- **OpenShift-aware UI**: Routes, Projects, SCCs, DeploymentConfigs,
  BuildConfigs, Builds and ImageStreams; Routes open in the service
  browser; the security scan skips OpenShift's own namespaces and checks
  SCC grants; OpenShift version support status.
- **Monitoring**: OpenShift's Thanos Querier as a metrics source.

### Honest metrics

- A cluster without a working metrics API shows why metrics are
  unavailable instead of zeros, on every metrics view.

## Next (candidates)

Collected from the [v0.5 design contract](docs/design-v0.5.md) (its "out of
scope" list), the [v0.5 hand-off](docs/dev/v0.5-handoff.md#95-known-limitations-after-the-w4-review)
and the limitations documented in [docs/security.md](docs/security.md).

### Operations

| Item | Value | Size |
|---|---|---|
| Port-forward for non-HTTP ports | The v0.6 service browser covers HTTP; databases and other TCP ports still need a local `kubectl port-forward` | M |
| Exact service browser address with self-navigating nested frames | The address bar follows the page's first document load; a widget frame that navigates on its own between two page loads is taken for the page at the next load ([design §7.14](docs/design-v0.5.md#714-service-browser-core--service-browser-plugin--v06)). Knowing the page's frame for sure needs a report from inside the page itself | M |
| Cluster compare | A/B diff of two clusters or namespaces on neat YAML, to find drift between environments | L |
| Manifest Composer | Form wizards for common resources beyond storage, with templates | L |
| Several active clusters per user | Side-by-side views instead of switching the one active cluster | L |
| Gateway API views | First-class Gateway and route pages instead of the generic CRD browser | M |
| Admission policy views | ValidatingAdmissionPolicy, Kyverno and Gatekeeper policies next to the webhook lists | M |

### GitOps

| Item | Value | Size |
|---|---|---|
| Multi-cluster apply | Apply the same manifests (from the Composer or a YAML import) to several clusters in one step, to keep environments in line | M |

### Security

| Item | Value | Size |
|---|---|---|
| Background directory sync | Disable or demote external users between sign-ins when the IdP or directory changes (today roles update at the next sign-in) | M |
| Kubernetes impersonation per user | Let the cluster's own RBAC and audit see the real KubeBench user | L |
| NSA/CISA mapping and PDF export | Map the posture checks to the NSA/CISA guide and export a printable report | M |
| Narrower ambient cloud identity | Limit `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY` instead of all or nothing | M |
| Real-cloud validation of EKS, AKS and GKE sign-in | The in-process token providers are tested against fakes only; confirm them on real clusters | M |
| AI assistant (opt-in, bring your own model) | Ask about the cluster through a server-side assistant with read-only tools by default and every action audited; off unless an admin configures a model | L |

### Platform and plugins

| Item | Value | Size |
|---|---|---|
| Record how a kubeconfig was added | Allow removing credentials of clusters added by path, which is refused today | S |
| Context-aware discovery | Bound discovery calls so that closing a runtime of an unreachable cluster ends promptly | M |
| Update checks for private repositories | Accept a token for the GitHub release check, for forks and private mirrors | S |
| KubeVirt plugin | Manage virtual machines next to containers on clusters running KubeVirt (detected like Argo CD): VirtualMachines and their instances, start, stop, restart, pause and live migration, serial and VNC consoles in a tab, and DataVolumes and disk imports with CDI. Operators and admins only for power and console actions, audited like exec | L |

### UX and i18n

| Item | Value | Size |
|---|---|---|
| More UI languages | English, Turkish and German ship today; a language is data only ([docs/i18n.md](docs/i18n.md)) | S each |
| Smaller main bundle | Split the main chunk (over 800 kB minified) further, for a faster first load | M |

### Distribution

| Item | Value | Size |
|---|---|---|
| APT and YUM repositories | Upgrade VM installs with the system package manager | M |

## Done

### v0.5

v0.5 turned KubeBench from a viewer into a multi-user workbench:
per-cluster roles, read-only clusters and an audit log; YAML editing with
diff and dry run, real terminals and streaming logs, node, rollout and
workload operations; Helm install and upgrade, and the Argo CD, Storage,
Security, Access and Topology plugins on a new plugin platform with
runtime-loaded external UI plugins; live updates over Server-Sent Events;
a kubectl-style command palette and keyboard navigation; English, Turkish
and German; EKS, AKS and GKE sign-in without the clouds' CLIs; update
checks with What's new; and packages for every install method (image,
chart, manifests, `.deb`/`.rpm`, installer, offline bundle).

The full list, including upgrade notes and security fixes, is in the
[CHANGELOG](CHANGELOG.md).
