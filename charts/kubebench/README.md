# KubeBench Helm chart

[KubeBench](https://github.com/Shavell/kubebench) is a VSCode-style
Kubernetes workbench: workloads, logs, shells, YAML, Helm releases and
metrics for many clusters in one web UI, with users, per-cluster roles and an
audit log. This chart runs it inside a cluster, which it registers as the
`in-cluster` cluster through its ServiceAccount.

The pod is a single replica with the `Recreate` strategy (SQLite on a
ReadWriteOnce volume allows one writer), runs as non-root with a read-only
root filesystem, and meets the Pod Security `restricted` profile; on
OpenShift it runs under the `restricted-v2` SCC ([OpenShift](#openshift)).

## Install

Requires Kubernetes 1.25+ and Helm 3.8+ (OCI support) or Helm 4.

```bash
helm install kubebench oci://ghcr.io/shavell/charts/kubebench --version 0.5.0 \
  --namespace kubebench --create-namespace
```

From a release asset or a checkout instead:

```bash
helm install kubebench ./kubebench-0.5.0.tgz -n kubebench --create-namespace
helm install kubebench charts/kubebench -n kubebench --create-namespace
```

Then follow the printed notes:

```bash
kubectl -n kubebench get secret kubebench-auth -o jsonpath='{.data.admin-password}' | base64 -d; echo
kubectl -n kubebench port-forward svc/kubebench 8080:80     # http://localhost:8080, user "admin"
```

## Admin password

The bootstrap `admin` user is created on the first start only, and
KubeBench asks for a new password at the first sign-in: the bootstrap one
stays in the Secret, which others may be able to read. Afterwards the
password lives in the database.

| Values | Result |
|---|---|
| (defaults) | A random 24-character password is generated into the Secret `<fullname>-auth` on install and kept on upgrades (`lookup`) and after `helm uninstall` (`helm.sh/resource-policy: keep`). |
| `auth.adminPassword=...` | The chart stores that value in the Secret. It also ends up in the Helm release history; prefer `existingSecret` in production. |
| `auth.existingSecret=name` | Your Secret, key `auth.existingSecretKey` (default `admin-password`). |
| `auth.createSecret=false` | No Secret: the server starts with `admin`/`admin` and forces a password change at the first sign-in. Change it before anyone else can reach the UI. |

`helm template` cannot look up the existing Secret, so a GitOps render
without `auth.existingSecret` gets a new random value on every render; use
`auth.existingSecret` with Argo CD or Flux.

## Single sign-on (OIDC) and LDAP

Users can sign in through an OpenID Connect provider (Keycloak, Entra ID,
Okta, Dex, …) or an LDAP directory / Active Directory besides local
accounts. External users are created at their first sign-in; their global
role and cluster roles come from group mappings and are re-computed at every
sign-in. The last local admin always stays as the break-glass account. For
OIDC, register a confidential client with the redirect URL
`<config.publicUrl>/api/auth/oidc/callback`, then:

```yaml
config:
  publicUrl: https://kubebench.example.com
identity:
  groupMappings:
    - {group: kubebench-admins, role: admin}
    - {group: platform-sre, role: operator, cluster: in-cluster}
  oidc:
    enabled: true
    issuer: https://sso.example.com/realms/platform
    clientId: kubebench
    name: Company SSO
    existingSecret: kubebench-oidc   # key client-secret
```

Sign-in with LDAP or Active Directory (alone or next to OIDC) takes a
service account that may read users and groups:

```yaml
identity:
  ldap:
    enabled: true
    url: ldaps://dc1.corp.example.com
    type: ad                          # or openldap
    bindDN: CN=svc-kubebench,OU=Service Accounts,DC=corp,DC=example,DC=com
    existingSecret: kubebench-ldap    # key bind-password
    userBaseDN: DC=corp,DC=example,DC=com
    caBundle: |                       # a private CA, mounted from a ConfigMap
      -----BEGIN CERTIFICATE-----
      ...
```

The mappings are seeded on the first start that has them; admins manage
them in Settings > Authentication afterwards. With `networkPolicy.enabled`,
allow egress to the provider or directory (`networkPolicy.extraEgressCIDRs`). See
[configuration](../../docs/configuration.md#external-identities) for
every setting.

## RBAC presets

KubeBench acts on this cluster with its ServiceAccount, so the binding
decides what the workbench can do here (KubeBench roles and read-only
clusters are enforced on top of it).

| `rbac.preset` | ClusterRole | Use it for |
|---|---|---|
| `cluster-admin` (default) | the built-in `cluster-admin` | the full workbench: changes, shells, Helm, CRDs |
| `read-only` | chart-defined: `get`/`list`/`watch` on every built-in API group except Secrets, plus `pods/log`, `services/proxy` `get` (Prometheus), `metrics.k8s.io`, and the `argoproj.io`, `snapshot.storage.k8s.io`, `gateway.networking.k8s.io` CRD groups; on OpenShift also the `*.openshift.io` groups and `cluster-monitoring-view` of [OpenShift](#openshift) | browsing and troubleshooting; no exec, no changes, no Secrets, no Helm releases (they are Secrets) |
| `custom` | chart-defined from `rbac.rules` | anything in between |

Make more CRD groups readable with the read-only preset through
`rbac.readOnly.extraApiGroups`, e.g. `--set rbac.readOnly.extraApiGroups='{cert-manager.io,traefik.io}'`.
`rbac.create=false` leaves RBAC to you.

The `cluster-admin` binding is named `<fullname>`; the other presets create a
ClusterRole and a ClusterRoleBinding named `<fullname>-read-only` or
`<fullname>-custom`. A binding cannot change its role, so switching presets
with `helm upgrade --reset-then-reuse-values --set rbac.preset=...` replaces
the binding, and restarts the pod so that KubeBench sees its new permissions.

## Ingress and TLS

```yaml
ingress:
  enabled: true
  className: nginx            # or traefik on k3s; empty = default IngressClass
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt
    # Shells (WebSocket) and log streams (SSE) are long-lived connections:
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-buffering: "off"
  hosts:
    - host: kubebench.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: kubebench-tls
      hosts: [kubebench.example.com]
```

With an Ingress, `config.trustProxy` defaults to `true` so that KubeBench
honours `X-Forwarded-Proto/For` (secure cookies behind TLS, client IPs in
the audit log and login rate limiting). Keep the Service `ClusterIP` then;
anyone who can reach the pod directly could forge those headers, so allow
only the ingress controller with `networkPolicy.ingressFrom` (see
[NetworkPolicy](#networkpolicy)). On k3s without DNS,
`kubebench.<node-ip>.sslip.io` makes a handy host name.

## NetworkPolicy

`networkPolicy.enabled=true` allows ingress to the HTTP port from the
peers in `networkPolicy.ingressFrom` (from anywhere when it is empty), and
egress only to DNS, the Kubernetes API server and the destinations you
add. Behind an Ingress, list the ingress controller's pods there, for
Traefik on k3s:

```yaml
networkPolicy:
  enabled: true
  ingressFrom:
    - namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: kube-system
      podSelector:
        matchLabels:
          app.kubernetes.io/name: traefik
```

Most CNIs apply egress rules after Service translation, so the policy
must allow the API server *endpoints* (often `:6443`), not only the
`kubernetes` Service on `:443`. The chart looks both up at install time.
When it renders offline (`helm template`, GitOps tools such as Argo CD or
Flux, a client-side `--dry-run`) nothing can be looked up, and the render
fails unless `networkPolicy.apiServer.cidrs` is set, rather than allow
every destination on `networkPolicy.apiServer.ports`:

```sh
kubectl get service kubernetes -n default -o jsonpath='{.spec.clusterIPs}'
kubectl get endpointslices -n default -l kubernetes.io/service-name=kubernetes \
  -o jsonpath='{.items[*].endpoints[*].addresses}'
```
Remote clusters, Helm repositories and other destinations go into
`networkPolicy.extraEgressCIDRs` or `networkPolicy.extraEgress`; so does
`api.github.com:443` for the update check, unless `config.updateMode` is
`off`.

## OpenShift

On OpenShift the chart switches to OpenShift mode by itself
(`openshift.enabled: auto`, true when the cluster serves
`route.openshift.io/v1` and `security.openshift.io/v1`) and installs under
the `restricted-v2` SCC with either RBAC preset:

```bash
oc new-project kubebench
helm install kubebench oci://ghcr.io/shavell/charts/kubebench --version 0.5.0 \
  -n kubebench --set route.enabled=true
oc -n kubebench get route kubebench -o jsonpath='{.spec.host}'
```

In OpenShift mode the chart

- leaves `runAsUser`, `runAsGroup` and `fsGroup` out of `podSecurityContext`
  and `securityContext`, so the SCC assigns them from the project's ranges
  (the image runs with any UID in group 0); the other restrictions stay;
- pins the pod with `openshift.io/required-scc` to `restricted-v2`
  (`restricted-v3` with `hostUsers: false`, or `openshift.requiredSCC`): a
  ServiceAccount allowed to use more SCCs (`cluster-admin`) would otherwise
  get any SCC its pod fits (`nonroot-v2` for a fixed UID); with the pin a pod
  that `restricted-v2` does not admit is refused;
- renders the Route when `route.enabled` (it fails without OpenShift mode);
- lets the `read-only` preset read the `apps`, `build`, `config`,
  `project`, `route` and `security` `.openshift.io` groups, and image
  streams except their `secrets` and `layers` subresources; no preset
  grants `use` on an SCC. Routes can embed a TLS key (`spec.tls.key`) and
  are readable like the other objects;
- binds the `cluster-monitoring-view` ClusterRole to the ServiceAccount
  when the preset is not `cluster-admin`, so KubeBench can read metrics
  from the cluster monitoring's Thanos Querier
  (`openshift.monitoring.clusterMonitoringView`; `false` leaves metrics to
  metrics-server);
- points the NetworkPolicy's DNS rule at `openshift-dns` on ports 53 and
  5353 (`openshift.dns`, replacing `networkPolicy.dns`) and allows egress
  to the `thanos-querier` pods in `openshift-monitoring` on port 9091
  (`openshift.monitoring`).

Offline renders (`helm template`, Argo CD, Flux) cannot detect OpenShift:
set `openshift.enabled=true`. `openshift.enabled=false` renders the plain
Kubernetes objects with the fixed UID 65532, which only an SCC such as
`nonroot-v2` admits, and only for a ServiceAccount allowed to use it.

| `route.tls.termination` | Traffic | Pod |
|---|---|---|
| `edge` (default) | the router terminates TLS (`route.tls.certificate`/`key`, else its default certificate) and sends HTTP | plain HTTP |
| `reencrypt` | the router terminates TLS and opens TLS to the pod, trusting `route.tls.destinationCACertificate`, else the service CA | HTTPS with `tls.existingSecret`, else a service CA certificate (`service.beta.openshift.io/serving-cert-secret-name: <fullname>-tls`) |
| `passthrough` | TLS goes through to the pod; `route.path` is not allowed | HTTPS with `tls.existingSecret` (required), a certificate for the Route host |

`config.trustProxy` defaults to `true` behind an edge or reencrypt Route
(the router sets `X-Forwarded-*`), `false` with passthrough. Let only the
router reach the pod then:

```yaml
networkPolicy:
  enabled: true
  ingressFrom:
    - namespaceSelector:
        matchLabels:
          policy-group.network.openshift.io/ingress: ""
```

`hostUsers: false` runs the pod in a user namespace under `restricted-v3`
(OpenShift 5). Set it on a new installation only: the database written by
the project UID is not writable from the user namespace. See
[docs/deployment.md](../../docs/deployment.md#openshift) for the plain
OpenShift manifests and monitoring.

## Persistence and backups

Users, sessions, the cluster inventory (including stored kubeconfigs) and
the audit log live in SQLite at `/data/kubebench.db` on the PVC; treat the
volume as sensitive. The PVC is kept on `helm uninstall` (`persistence.keep`).
The image is distroless (no shell, no `tar`), so backups go through a helper
pod while the Deployment is scaled to zero; see
[docs/deployment.md](../../docs/deployment.md#backup-and-restore).

## Upgrading

```bash
helm upgrade kubebench oci://ghcr.io/shavell/charts/kubebench --version <new> -n kubebench \
  --reset-then-reuse-values
```

replaces the pod (`Recreate`); the database is migrated on start. With
`config.updateMode` other than `off`, administrators see new releases in
the status bar, and every user gets the release's What's new once.
`--reset-then-reuse-values` (Helm 3.14+) applies the values you set before on
top of the new chart's defaults. Plain `--reuse-values` keeps the old chart's
defaults as well, so it misses settings that a newer chart adds or changes.

**From chart 0.1.0**: resource names now follow the usual Helm convention
(release `kubebench` creates `kubebench`, not `kubebench-kubebench`). Keep the
existing database by pointing the new release at the old claim. Pass your
own settings again (`-f my-values.yaml`) instead of reusing the old values:
the 0.1.0 keys `replicaCount` and `rbac.clusterRole` are gone, and the
chart's schema rejects them.

```bash
helm upgrade kubebench charts/kubebench -n kubebench \
  --set persistence.existingClaim=kubebench-kubebench-data \
  --set auth.existingSecret=kubebench-kubebench-auth
```

## Uninstall

```bash
helm uninstall kubebench -n kubebench
# the PVC and the password Secret are kept; delete them to remove all data:
kubectl -n kubebench delete pvc/kubebench-data secret/kubebench-auth
```

## Values

| Key | Default | Description |
|---|---|---|
| `nameOverride` | `""` | Replaces the chart name in resource names. |
| `fullnameOverride` | `""` | Replaces the full resource name prefix. |
| `image.repository` | `ghcr.io/shavell/kubebench` | Image repository. |
| `image.tag` | `""` | Image tag; defaults to the chart `appVersion`. |
| `image.digest` | `""` | `sha256:...` digest; takes precedence over the tag. |
| `image.pullPolicy` | `IfNotPresent` | Image pull policy. |
| `imagePullSecrets` | `[]` | Pull secrets for private registries. |
| `auth.adminPassword` | `""` | Bootstrap admin password stored in the chart Secret (see [Admin password](#admin-password)). |
| `auth.existingSecret` | `""` | Existing Secret with the bootstrap password. |
| `auth.existingSecretKey` | `admin-password` | Key in `auth.existingSecret`. |
| `auth.createSecret` | `true` | `false` without `existingSecret`: no Secret, `admin`/`admin` with a forced change. |
| `config.secureCookies` | `auto` | `KUBEBENCH_SECURE_COOKIES`: `auto`, `true` or `false`. |
| `config.trustProxy` | `null` | `KUBEBENCH_TRUST_PROXY`; `null` = `true` when `ingress.enabled` or behind an edge or reencrypt Route. |
| `config.sessionTTL` | `168h` | `KUBEBENCH_SESSION_TTL` (Go duration, at least `1m`). |
| `config.auditRetentionDays` | `90` | `KUBEBENCH_AUDIT_RETENTION_DAYS` (`0` = keep forever). |
| `config.allowPathKubeconfig` | `false` | `KUBEBENCH_ALLOW_PATH_KUBECONFIG`: add clusters by a path inside the pod. |
| `config.allowAmbientCloudIdentity` | `false` | `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY`: pasted EKS, GKE and AKS kubeconfigs may sign in with the pod's own cloud identity (workload identity). Every admin can then obtain its token ([security](../../docs/security.md#cloud-identities-eks-aks-gke)). |
| `config.updateMode` | `stable` | `KUBEBENCH_UPDATE_MODE`: check GitHub for new releases (`stable`, `prerelease`) or never (`off`, for air-gapped clusters). Admins can change it in Settings. |
| `config.proxyOrigin` | `""` | `KUBEBENCH_PROXY_ORIGIN`: a separate origin (e.g. `https://kubebench-proxy.example.net`) where the service browser shows in-cluster web pages unsandboxed; add its host to `ingress.hosts`. Use a host on another domain than KubeBench's (not its host on another port, not a subdomain: KubeBench refuses those). Empty: pages run sandboxed on KubeBench's origin. |
| `config.publicUrl` | `""` | `KUBEBENCH_PUBLIC_URL`: the URL users reach KubeBench at; the OIDC redirect URL is built from it. Empty: derived from each request. |
| `identity.requireGroup` | `false` | `KUBEBENCH_AUTH_REQUIRE_GROUP`: refuse external users whose groups grant nothing (otherwise they sign in and see no cluster). |
| `identity.groupMappings` | `[]` | `KUBEBENCH_AUTH_GROUP_MAPPINGS`, seeded on first start: `{group, role: admin\|operator\|viewer, cluster, source}`. |
| `identity.oidc.enabled` | `false` | Sign-in with an OIDC provider ([Single sign-on](#single-sign-on-oidc)). |
| `identity.oidc.issuer` / `clientId` | `""` | `KUBEBENCH_OIDC_ISSUER`, `KUBEBENCH_OIDC_CLIENT_ID` (required when enabled). |
| `identity.oidc.name` | `SSO` | Shown as "Sign in with <name>". |
| `identity.oidc.scopes` | `[openid, profile, email]` | Requested scopes. |
| `identity.oidc.usernameClaim` / `groupsClaim` | `preferred_username` / `groups` | Claims for the username and groups (a dotted path such as `realm_access.roles` works). |
| `identity.oidc.allowedDomains` | `[]` | Admit only verified emails of these domains (`email_verified` must be true; a missing claim is refused). |
| `identity.oidc.existingSecret` / `existingSecretKey` | `""` / `client-secret` | Secret with the client secret. |
| `identity.oidc.clientSecret` | `""` | Client secret stored in a chart Secret (also in the Helm release history; prefer `existingSecret`). Both empty: public client. |
| `identity.oidc.caBundle` | `""` | PEM CA bundle for the provider's TLS certificate, mounted from a ConfigMap. |
| `extraEnv` | `[]` | Extra container env entries; they override the chart's variables. |
| `extraEnvFrom` | `[]` | Extra `envFrom` sources. |
| `serviceAccount.create` | `true` | Create the ServiceAccount. |
| `serviceAccount.name` | `""` | Name; defaults to the fullname (or `default` when not created). |
| `serviceAccount.annotations` | `{}` | Annotations, e.g. for workload identity: the cloud identity KubeBench signs in to EKS, GKE and AKS clusters with when `config.allowAmbientCloudIdentity` is on ([Cloud clusters](../../docs/deployment.md#cloud-clusters-eks-aks-gke)). |
| `serviceAccount.automount` | `true` | Mount the token; KubeBench needs it to manage this cluster. |
| `rbac.create` | `true` | Create the ClusterRoleBinding (and ClusterRole for `read-only`/`custom`). |
| `rbac.preset` | `cluster-admin` | `cluster-admin`, `read-only` or `custom` (see [RBAC presets](#rbac-presets)). |
| `rbac.readOnly.extraApiGroups` | `[]` | More API groups readable by the `read-only` preset. |
| `rbac.rules` | `[]` | PolicyRules for the `custom` preset. |
| `persistence.enabled` | `true` | Store `/data` on a PVC (otherwise an `emptyDir`). |
| `persistence.existingClaim` | `""` | Use this PVC instead of creating one. |
| `persistence.storageClass` | `""` | StorageClass; `""` = cluster default, `"-"` = none. |
| `persistence.accessMode` | `ReadWriteOnce` | PVC access mode. |
| `persistence.size` | `1Gi` | PVC size. |
| `persistence.annotations` | `{}` | PVC annotations. |
| `persistence.keep` | `true` | Keep the PVC on `helm uninstall`. |
| `service.type` | `ClusterIP` | `ClusterIP`, `NodePort` or `LoadBalancer`. |
| `service.port` | `80` | Service port (the container listens on 9876). |
| `service.nodePort` | `null` | Fixed node port for `NodePort`/`LoadBalancer`. |
| `service.annotations` | `{}` | Service annotations. |
| `ingress.enabled` | `false` | Create an Ingress. |
| `ingress.className` | `""` | IngressClass; empty = cluster default. |
| `ingress.annotations` | `{}` | Ingress annotations (timeouts for WebSocket/SSE, cert-manager, ...). |
| `ingress.hosts` | `kubebench.example.com`, `/` | Hosts and paths. |
| `ingress.tls` | `[]` | TLS entries (`secretName`, `hosts`). |
| `networkPolicy.enabled` | `false` | Create a NetworkPolicy (see [NetworkPolicy](#networkpolicy)). |
| `networkPolicy.ingressFrom` | `[]` | NetworkPolicyPeers allowed to reach the HTTP port; empty = anyone. |
| `networkPolicy.apiServer.cidrs` | `[]` | API server CIDRs; empty = looked up at install time (an offline render then fails). |
| `networkPolicy.apiServer.ports` | `[443, 6443]` | API server ports (looked-up endpoint ports are added). |
| `networkPolicy.dns.namespaceSelector` | `kube-system` | Namespace of the cluster DNS. |
| `networkPolicy.dns.podSelector` | `k8s-app: kube-dns` | Pods of the cluster DNS. |
| `networkPolicy.dns.ports` | `[53]` | DNS ports (UDP and TCP). |
| `networkPolicy.extraEgressCIDRs` | `[]` | Extra egress destinations (any port). |
| `networkPolicy.extraEgress` | `[]` | Extra NetworkPolicyEgressRule objects. |
| `podAnnotations` | `{}` | Pod annotations. |
| `podLabels` | `{}` | Extra pod labels. |
| `podSecurityContext` | non-root 65532, `fsGroup` 65532, `RuntimeDefault` seccomp | Pod security context (OpenShift mode drops `runAsUser`, `runAsGroup`, `fsGroup`). |
| `securityContext` | no privilege escalation, read-only root, drop `ALL`, `RuntimeDefault` seccomp | Container security context (OpenShift mode drops `runAsUser`, `runAsGroup`). |
| `hostUsers` | `null` | `false`: run the pod in a user namespace (`restricted-v3` on OpenShift); `null`: not set. |
| `resources` | requests `50m`/`128Mi`, limit `512Mi` memory | Container resources. |
| `startupProbe` | enabled, every 2s, 60 failures | Startup probe timing on `/healthz` (`enabled`, `periodSeconds`, `timeoutSeconds`, `failureThreshold`, ...). |
| `readinessProbe` | enabled, every 10s | Readiness probe timing on `/healthz`. |
| `livenessProbe` | enabled, every 20s | Liveness probe timing on `/healthz`. |
| `extraVolumes` | `[]` | Extra pod volumes. |
| `extraVolumeMounts` | `[]` | Extra container mounts (e.g. external plugins with `KUBEBENCH_PLUGINS_DIR`). |
| `priorityClassName` | `""` | Pod priority class. |
| `terminationGracePeriodSeconds` | `30` | Grace period for a clean shutdown. |
| `nodeSelector` | `{}` | Node selector. |
| `tolerations` | `[]` | Tolerations. |
| `affinity` | `{}` | Affinity rules. |
| `tls.existingSecret` | `""` | `kubernetes.io/tls` Secret the server serves HTTPS with (`KUBEBENCH_TLS_CERT`/`KEY`, probes over HTTPS); needed behind a passthrough Route. |
| `openshift.enabled` | `auto` | OpenShift mode: `auto` (the Route and SCC APIs exist at install time), `true` or `false` ([OpenShift](#openshift)). |
| `openshift.requiredSCC` | `""` | SCC the pod is pinned to; empty = `restricted-v2`, or `restricted-v3` with `hostUsers: false`. |
| `openshift.dns` | `openshift-dns`, `dns-default` pods, ports `[53, 5353]` | DNS rule of the NetworkPolicy in OpenShift mode (`namespaceSelector`, `podSelector`, `ports`). |
| `openshift.monitoring.clusterMonitoringView` | `true` | Bind `cluster-monitoring-view` (needed to read OpenShift monitoring) when `rbac.preset` is not `cluster-admin`. |
| `openshift.monitoring` | `openshift-monitoring`, `thanos-query` pods, ports `[9091]` | Thanos Querier egress rule of the NetworkPolicy in OpenShift mode (`namespaceSelector`, `podSelector`, `ports`). |
| `route.enabled` | `false` | Create a Route (OpenShift mode only). |
| `route.host` | `""` | Route host; empty = `<fullname>-<namespace>.<apps domain>`. The first DNS label (`<fullname>-<namespace>`) must stay within 63 characters, or OpenShift rejects the Route; with long release or project names set `route.host` (or a shorter `fullnameOverride`). |
| `route.path` | `""` | Path prefix (not with passthrough). |
| `route.annotations` | `{}` | Route annotations, e.g. `haproxy.router.openshift.io/timeout`. |
| `route.tls.termination` | `edge` | `edge`, `reencrypt` or `passthrough`. |
| `route.tls.insecureEdgeTerminationPolicy` | `Redirect` | Plain HTTP: `Redirect`, `None` or `Allow` (not with passthrough). |
| `route.tls.certificate` / `key` / `caCertificate` | `""` | PEM certificate, key and chain for the host (edge, reencrypt); empty = the router's default certificate. |
| `route.tls.destinationCACertificate` | `""` | PEM CA of the pod's certificate (reencrypt); empty = the service CA. |
