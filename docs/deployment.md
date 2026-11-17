# Deploying KubeBench

KubeBench is a single Go binary with the web UI embedded. It keeps its state
(users, sessions, the cluster inventory with stored kubeconfigs, the audit
log) in one SQLite database and talks to your clusters through the
Kubernetes API. Run it inside a cluster, on a VM, in Docker, or on your own
machine:

| Where | How | Section |
|---|---|---|
| Kubernetes | Helm chart (OCI or `.tgz`) or plain manifests | [Kubernetes](#kubernetes) |
| Linux VM / server | `install.sh`, `.deb`, `.rpm` or the release tarball | [Virtual machines](#virtual-machines) |
| Container host | `ghcr.io/shavell/kubebench` | [Docker](#docker) |
| Laptop (Linux, macOS, Windows) | release archive + your kubeconfig | [Local app](#local-app) |
| No internet access | offline bundle | [Offline and air-gapped](#offline-and-air-gapped) |

Every release on [GitHub](https://github.com/Shavell/kubebench/releases)
carries all of these artifacts with a `checksums.txt` (SHA-256); see
[releasing.md](releasing.md#what-a-release-contains).

**First sign-in.** The `admin` user is created on the very first start with
`KUBEBENCH_ADMIN_PASSWORD`. Without it the password is `admin`, so sign in
right after installing. The Helm chart and `install.sh` generate a random
password instead. Either way KubeBench asks for a new password at the first
sign-in: the bootstrap one stays readable where it was configured (the
chart's Secret, the env file).

## Kubernetes

Inside a cluster, KubeBench registers that cluster as `in-cluster` through
its ServiceAccount, so the ServiceAccount's RBAC decides what it can do
there. Add more clusters in the Clusters view.

Assign the `operator` role on `in-cluster` only to people you would make
KubeBench administrators. With the default `cluster-admin` preset, an
operator there can change KubeBench's own namespace like any other: run a
debug container next to KubeBench and read its database (sessions and the
credentials of every registered cluster), or create workloads that do.
Narrow the ServiceAccount's RBAC (`rbac.preset`) if operators must not
reach that namespace.

### Helm

```bash
helm install kubebench oci://ghcr.io/shavell/charts/kubebench --version 0.5.0 \
  --namespace kubebench --create-namespace
# or the chart attached to the release / in this repository:
helm install kubebench ./kubebench-0.5.0.tgz -n kubebench --create-namespace
helm install kubebench charts/kubebench -n kubebench --create-namespace

kubectl -n kubebench get secret kubebench-auth -o jsonpath='{.data.admin-password}' | base64 -d; echo
kubectl -n kubebench port-forward svc/kubebench 8080:80     # http://localhost:8080
```

The chart README documents every value: admin password handling, RBAC
presets (`cluster-admin`, `read-only`, `custom`), Ingress, NetworkPolicy,
persistence and security settings:
[charts/kubebench/README.md](../charts/kubebench/README.md).

### Plain manifests

```bash
kubectl apply -f https://github.com/Shavell/kubebench/releases/download/v0.5.0/kubebench.yaml
kubectl -n kubebench port-forward svc/kubebench 8080:80     # admin / admin, then a new password
```

`deploy/kubernetes/kubebench.yaml` is rendered from the chart (namespace
`kubebench` with Pod Security `restricted`, `cluster-admin` binding, 1Gi PVC)
and has no password Secret. Customizing, exposing and uninstalling:
[deploy/kubernetes/README.md](../deploy/kubernetes/README.md).

## Virtual machines

### Installer

```bash
curl -fsSL https://raw.githubusercontent.com/Shavell/kubebench/main/install.sh | sudo bash
curl -fsSL https://raw.githubusercontent.com/Shavell/kubebench/main/install.sh | sudo bash -s -- --version v0.5.0
```

The installer picks the `.deb` (dpkg), `.rpm` (rpm) or the tarball for the
host's architecture (amd64, arm64), verifies it against the release's
`checksums.txt`, installs it, writes `/etc/kubebench/kubebench.env` and
starts `kubebench.service`. On a fresh install it generates a 20-character
admin password and prints it once. Running it again upgrades in place and
keeps the configuration and the data.

| Option | Effect |
|---|---|
| `--version vX.Y.Z` | release to install (default: the latest) |
| `--method auto\|deb\|rpm\|tarball` | force a format (default: the existing install's, else deb, rpm, tarball) |
| `--offline PATH` | install from an offline bundle (file or extracted directory), a `.deb`/`.rpm`, or `kubebench_<v>_linux_<arch>.tar.gz` |
| `--port N`, `--listen HOST:PORT` | set `PORT` / `KUBEBENCH_LISTEN_ADDR` |
| `--admin-password PW` | bootstrap password instead of a generated one |
| `--kubeconfig FILE` | copy to `/etc/kubebench/kubeconfig` (`root:kubebench 0640`) and seed the inventory from it |
| `--data-dir DIR` | data directory other than `/var/lib/kubebench` (writes a systemd drop-in) |
| `--no-start` | install and configure without starting |
| `--uninstall [--purge] [--yes]` | remove; `--purge` also deletes configuration, data and the user |
| `--dry-run` | print the steps without changing anything |

### Packages

```bash
sudo apt install ./kubebench_0.5.0_amd64.deb          # or: sudo dpkg -i ...
sudo dnf install ./kubebench-0.5.0-1.x86_64.rpm       # or: sudo rpm -i ...
```

Both install `/usr/bin/kubebench`, `kubebench.service`, the documented
`/etc/kubebench/kubebench.env` (kept across upgrades) and a `kubebench`
system user that owns `/var/lib/kubebench`. With systemd running, the
service is enabled and started right away with the default `admin`/`admin`
bootstrap (set `KUBEBENCH_ADMIN_PASSWORD` and restart *before* the first
sign-in if the host is reachable by others, or use `install.sh`, which
writes the password before the first start). Details:
[packaging/README.md](../packaging/README.md).

### Tarball

The `kubebench_<version>_linux_<arch>.tar.gz` archive holds the binary, the
systemd unit and the env file (`packaging/`). `install.sh --method tarball`
installs them like the packages, with the binary in `/usr/local/bin`. By
hand:

```bash
tar -xzf kubebench_0.5.0_linux_amd64.tar.gz
sudo install -m 0755 kubebench /usr/local/bin/kubebench
sudo useradd --system --home-dir /var/lib/kubebench --shell /usr/sbin/nologin kubebench
sudo install -d -m 0750 -o kubebench -g kubebench /var/lib/kubebench
sudo install -D -m 0640 -g kubebench packaging/kubebench.env /etc/kubebench/kubebench.env
sed 's#/usr/bin/kubebench#/usr/local/bin/kubebench#' packaging/systemd/kubebench.service |
  sudo tee /etc/systemd/system/kubebench.service >/dev/null
sudo systemctl daemon-reload && sudo systemctl enable --now kubebench
```

### Without systemd

In containers or minimal systems, the packages and `install.sh` skip the
service steps and print the foreground command:

```bash
runuser -u kubebench -- sh -c 'set -a; KUBEBENCH_DATA_DIR=/var/lib/kubebench \
  KUBEBENCH_DB=/var/lib/kubebench/kubebench.db; . /etc/kubebench/kubebench.env;
  cd /var/lib/kubebench && exec /usr/bin/kubebench'
```

## Docker

```bash
docker volume create kubebench-data
docker run -d --name kubebench --restart unless-stopped -p 9876:9876 \
  -v kubebench-data:/data -e KUBEBENCH_ADMIN_PASSWORD='choose-a-password' \
  ghcr.io/shavell/kubebench:0.5.0
docker inspect --format '{{.State.Health.Status}}' kubebench    # HEALTHCHECK: kubebench healthcheck
```

The image (distroless, UID/GID 65532, multi-arch) stores everything in
`/data` (`KUBEBENCH_DATA_DIR`, `KUBEBENCH_DB=/data/kubebench.db`). A named
volume inherits the right owner; a bind mount must be writable by UID 65532
(`sudo chown 65532:65532 ./data`). To seed a cluster from a kubeconfig, mount
one that UID 65532 can read and set `KUBECONFIG`
(`-v ./kubeconfig:/etc/kubebench/kubeconfig:ro -e KUBECONFIG=/etc/kubebench/kubeconfig`);
an API server address such as `https://127.0.0.1:6443` must be reachable
from inside the container (`--network host`, or the host's address). You
can also paste kubeconfigs in the Clusters view later.

Build the image yourself with `make image` (local platform) or
`docker buildx build --platform linux/amd64,linux/arm64 ...` (see the
`Dockerfile` header).

## Local app

The release archives for Linux, macOS (`darwin`) and Windows contain the
same binary. Point it at your kubeconfig and open http://localhost:9876:

```bash
tar -xzf kubebench_0.5.0_darwin_arm64.tar.gz
KUBEBENCH_LISTEN_ADDR=127.0.0.1:9876 KUBECONFIG=~/.kube/config ./kubebench
```

It seeds the inventory with the kubeconfig's current context on the first
run and keeps its database (`kubebench.db`) in the working directory unless
`KUBEBENCH_DB`/`KUBEBENCH_DATA_DIR` say otherwise.

## Offline and air-gapped

Each release has `kubebench-offline-<version>.tar.gz` (+ `.sha256`):
Linux archives, `.deb`/`.rpm`, image tarballs for amd64 and arm64, the Helm
chart `.tgz`, the plain manifests, `install.sh`, `SHA256SUMS` and an
`OFFLINE-README.md` with the detailed steps.

```bash
sha256sum -c kubebench-offline-0.5.0.tar.gz.sha256
tar -xzf kubebench-offline-0.5.0.tar.gz && cd kubebench-offline-0.5.0
sha256sum -c SHA256SUMS
```

- **VM**: `sudo bash install.sh --offline .` (or pass the `.tar.gz` itself);
  the installer verifies the package against `packages/checksums.txt` and
  turns the update check off (`KUBEBENCH_UPDATE_MODE=off`).
- **Kubernetes**: load `images/kubebench_<version>_linux_<arch>.tar` on every
  node (`docker load -i`, `ctr -n k8s.io images import`, `k3s ctr images import`)
  or push it to an internal registry, then
  `helm install kubebench kubernetes/kubebench-<version>.tgz -n kubebench --create-namespace --set config.updateMode=off`
  (add `--set image.repository=<registry>/kubebench` for a registry) or
  `kubectl apply -f kubernetes/kubebench.yaml` followed by
  `kubectl -n kubebench set env deployment/kubebench KUBEBENCH_UPDATE_MODE=off`
  ([update checks](#update-checks)).
- **Helm charts inside KubeBench**: without internet access, use a chart
  repository inside the network or upload a chart `.tgz` (Helm > Install).

Build a bundle yourself from a GoReleaser output with
`make snapshot offline-bundle IMAGE_TARS="a.tar b.tar"`.

## Cloud clusters (EKS, AKS, GKE)

Kubeconfigs from `aws eks update-kubeconfig`, `gcloud container clusters
get-credentials` and `az aks get-credentials` (also after `kubelogin
convert-kubeconfig`) sign in through exec plugins: `aws eks get-token`,
`aws-iam-authenticator`, `gke-gcloud-auth-plugin` and `kubelogin`.
KubeBench does not run them (they are not in the image): it recognises
these stanzas and gets the tokens itself, so such a kubeconfig can be
pasted without `KUBEBENCH_ALLOW_KUBECONFIG_EXEC`. A pasted one needs
per-cluster credentials unless `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY` is
on (see below). Any other exec command
still needs that setting. The Clusters view shows the detected provider
(EKS, AKS, GKE, k3s, …) and warns when the Kubernetes version leaves the
provider's standard support. Details:
[design §7.12](design-v0.5.md#712-cloud-providers-core).

Which identity signs in, per cluster:

- **Per-cluster credentials**, entered by an administrator in the Add or
  Edit cluster dialog: an AWS access key (with an optional session token)
  or an AWS profile, a GCP service-account key (JSON), or an Azure service
  principal (tenant id, client id, client secret). They are stored in the
  database next to the kubeconfig and never shown again.
- Otherwise **the server's own cloud identity**, for kubeconfigs added by
  path or seeded from `KUBECONFIG`, and for pasted ones only with
  `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY=true` (chart:
  `config.allowAmbientCloudIdentity`):
  - AWS: the SDK's default chain (`AWS_*` variables, `~/.aws` files of the
    server's user and the profile the kubeconfig names, IRSA web identity,
    EKS Pod Identity, ECS, EC2 instance profile). A `--role-arn` in the
    kubeconfig is assumed on top.
  - GCP: Application Default Credentials (`GOOGLE_APPLICATION_CREDENTIALS`,
    gcloud's ADC file, GKE Workload Identity or the VM's service account).
  - Azure: `kubelogin --login spn` uses the `AZURE_*` variables, `msi` the
    managed identity, `workloadidentity` Azure Workload Identity; the
    interactive modes (`devicecode`, `azurecli`, …) fall back to those
    three in that order. The Azure CLI is never run.

**Security: with `KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY` on, an
administrator can make KubeBench use the server's ambient cloud identity
for any cluster they add**, simply by pasting a kubeconfig with one of
these stanzas, and can obtain its token by naming an API server of their
own. Turn it on only if every admin may hold that identity, and give it
only the cloud access KubeBench should have (for EKS, also an access entry
or `aws-auth` mapping with the Kubernetes permissions you intend).
Otherwise leave it off and use per-cluster credentials.

In Kubernetes, bind the chart's ServiceAccount to a cloud identity with
`serviceAccount.annotations` (and `podLabels` for Azure), and set
`config.allowAmbientCloudIdentity: true`:

```yaml
# EKS: IAM roles for service accounts (IRSA)
serviceAccount:
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/kubebench
# (EKS Pod Identity needs no annotation: create a pod identity association
#  for the ServiceAccount instead.)

# GKE: Workload Identity
serviceAccount:
  annotations:
    iam.gke.io/gcp-service-account: kubebench@my-project.iam.gserviceaccount.com

# AKS: Azure Workload Identity
serviceAccount:
  annotations:
    azure.workload.identity/client-id: 00000000-0000-0000-0000-000000000000
podLabels:
  azure.workload.identity/use: "true"
```

On a VM, the instance profile, the VM's service account or its managed
identity serve the same purpose; environment variables go into
`/etc/kubebench/kubebench.env`.

## Configuration

KubeBench reads its configuration from the environment once at startup
and refuses to start with an invalid value (every problem is reported at
once). Every variable, its default and its meaning:
[configuration.md](configuration.md).

Where each deployment sets them:

- **Helm chart**: `config.*` values map to the variables above, `extraEnv`
  adds or overrides any of them, and `PORT=9876`, `KUBEBENCH_DATA_DIR=/data`
  are fixed ([values](../charts/kubebench/README.md#values)).
- **Packages / installer**: `/etc/kubebench/kubebench.env` (every variable
  documented in place), then `sudo systemctl restart kubebench`. The unit
  sets `KUBEBENCH_DATA_DIR=/var/lib/kubebench` and
  `KUBEBENCH_DB=/var/lib/kubebench/kubebench.db`.
- **Docker**: `-e` flags; the image sets `PORT`, `KUBEBENCH_DATA_DIR` and `KUBEBENCH_DB`.

## TLS and reverse proxies

KubeBench serves plain HTTP by default. Put TLS in front of it, or let it
terminate TLS itself. Shells use WebSockets and logs/live updates use
Server-Sent Events, so proxies need long timeouts and no response buffering.
State-changing requests must carry an `Origin` that matches the `Host`
header, so a proxy has to pass the original `Host` through.

**Kubernetes**: enable the chart's Ingress with a TLS secret (for example
from cert-manager); `config.trustProxy` then defaults to `true`. For
ingress-nginx add `proxy-read-timeout`/`proxy-send-timeout` (`"3600"`) and
`proxy-buffering: "off"` annotations; Traefik needs nothing extra. With
`trustProxy`, KubeBench believes `X-Forwarded-For`, so only the ingress
controller should reach the pod: set `networkPolicy.enabled=true` with
`networkPolicy.ingressFrom` naming the controller's pods
([chart README](../charts/kubebench/README.md#networkpolicy)); otherwise any
pod in the cluster could forge the header and evade login throttling.

**VM behind a reverse proxy**: bind KubeBench to loopback and trust the proxy:

```bash
# /etc/kubebench/kubebench.env
KUBEBENCH_LISTEN_ADDR=127.0.0.1:9876
KUBEBENCH_TRUST_PROXY=true
```

```nginx
map $http_upgrade $connection_upgrade { default upgrade; '' close; }
server {
    listen 443 ssl;
    server_name kubebench.example.com;
    # ssl_certificate ...; ssl_certificate_key ...;
    location / {
        proxy_pass http://127.0.0.1:9876;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_read_timeout 3600s;
        proxy_buffering off;
    }
}
```

Caddy does all of this with `reverse_proxy 127.0.0.1:9876`.

**Native TLS**: set `KUBEBENCH_TLS_CERT` and `KUBEBENCH_TLS_KEY` to files the
`kubebench` user can read (e.g. `/etc/kubebench/tls.{crt,key}`,
`root:kubebench 0640`); the session cookie becomes `Secure` automatically.
The service has no capabilities, so listen on a port above 1023, or allow
port 443 with a drop-in (`systemctl edit kubebench`):

```ini
[Service]
AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
```

## Upgrades

KubeBench migrates its database forward when it starts; an older version
refuses to open a newer database, so **back up before upgrading** and do not
downgrade without restoring that backup.

| Install | Upgrade |
|---|---|
| Helm | `helm upgrade kubebench oci://ghcr.io/shavell/charts/kubebench --version <new> -n kubebench --reset-then-reuse-values` |
| Manifests | `kubectl apply -f` the new release's `kubebench.yaml` |
| Installer | run `install.sh` again (`--version vX.Y.Z` or latest) |
| Packages | `sudo apt install ./kubebench_<new>_amd64.deb` / `sudo dnf upgrade ./kubebench-<new>.rpm` |
| Docker | `docker pull` the new tag, then recreate the container with the same volume |

The chart and manifests use the `Recreate` strategy, so the old pod stops
before the new one opens the database. Package upgrades keep a modified
`/etc/kubebench/kubebench.env` and restart a running service.
`--reset-then-reuse-values` (Helm 3.14+) keeps your chart values on top of the
new chart's defaults; `--reuse-values` would keep the old defaults too.
Upgrading the Helm chart from 0.1.0 renames its resources; see the
[chart README](../charts/kubebench/README.md#upgrading).

### Update checks

KubeBench looks for new releases on GitHub (`KUBEBENCH_UPDATE_MODE`,
default `stable`). Administrators see "Update available: vX.Y.Z" in the
status bar, with the release notes and the upgrade command for each
install method; everyone sees the latest version in Help > About. After an
upgrade, every user is shown the release's What's new once (Help: What's
New shows the whole changelog at any time).

- The check runs when the UI asks for it, at most every 6 hours (failed
  checks are retried after 15 minutes, rate limits after their reset), and
  never delays the start. It sends no data about the installation beyond
  the request itself (`User-Agent: KubeBench/<version>`).
- It uses `HTTPS_PROXY`/`HTTP_PROXY`/`NO_PROXY`. Behind a chart
  NetworkPolicy, allow `api.github.com:443` in
  `networkPolicy.extraEgressCIDRs`.
- **Air-gapped installs**: set `off`, which makes no requests at all.
  `install.sh --offline` writes `KUBEBENCH_UPDATE_MODE=off` unless the env
  file sets a mode; with Helm use `--set config.updateMode=off`. Or mirror
  the releases JSON inside the network and set `KUBEBENCH_UPDATE_URL`.
- Development and snapshot builds (`dev`, `0.5.0-3-gabc1234`, `-dirty`,
  `-snapshot-`) show the latest release but never claim an update.

## Backup and restore

Everything lives in the data directory: `kubebench.db` (plus
`kubebench.db-wal`/`-shm` while it runs), the Helm repository cache, chart
uploads and external plugins. The database holds stored kubeconfigs in
plain text; keep backups as protected as the clusters themselves. Stop
KubeBench for a consistent copy.

**VM** (packages, installer):

```bash
sudo systemctl stop kubebench
sudo tar -czf kubebench-backup.tgz -C /var/lib/kubebench .
sudo systemctl start kubebench
# restore
sudo systemctl stop kubebench
sudo rm -rf /var/lib/kubebench/* && sudo tar -xzf kubebench-backup.tgz -C /var/lib/kubebench
sudo chown -R kubebench:kubebench /var/lib/kubebench && sudo systemctl start kubebench
```

**Docker**:

```bash
docker stop kubebench
docker run --rm -v kubebench-data:/data -v "$PWD":/backup busybox \
  tar -czf /backup/kubebench-backup.tgz -C /data .
docker start kubebench
```

**Kubernetes**: the image has no shell or `tar`, so copy the volume through
a helper pod (compatible with the `restricted` Pod Security profile) while
the Deployment is scaled to zero:

```bash
kubectl -n kubebench scale deploy/kubebench --replicas 0
kubectl -n kubebench apply -f - <<'EOF'
apiVersion: v1
kind: Pod
metadata:
  name: kubebench-backup
spec:
  securityContext:
    runAsNonRoot: true
    runAsUser: 65532
    runAsGroup: 65532
    seccompProfile: {type: RuntimeDefault}
  containers:
    - name: backup
      image: busybox:1.37
      command: [sleep, "3600"]
      securityContext:
        allowPrivilegeEscalation: false
        capabilities: {drop: [ALL]}
      volumeMounts: [{name: data, mountPath: /data}]
  volumes:
    - name: data
      persistentVolumeClaim: {claimName: kubebench-data}
EOF
kubectl -n kubebench wait --for=condition=Ready pod/kubebench-backup
kubectl -n kubebench cp kubebench-backup:/data ./kubebench-backup
# restore instead: kubectl -n kubebench cp ./kubebench-backup/. kubebench-backup:/data/
kubectl -n kubebench delete pod kubebench-backup
kubectl -n kubebench scale deploy/kubebench --replicas 1
```

The Helm chart names the PVC `<fullname>-data`, where the fullname is the
release name, followed by `-kubebench` unless it already contains
`kubebench`: `kubebench-data` for the release `kubebench` (and for the plain
manifests), `kb-kubebench-data` for a release `kb`.

## Uninstall

| Install | Remove | Also delete the data |
|---|---|---|
| Helm | `helm uninstall kubebench -n kubebench` | `kubectl -n kubebench delete pvc/kubebench-data secret/kubebench-auth` (kept by the chart) |
| Manifests | see [deploy/kubernetes/README.md](../deploy/kubernetes/README.md#uninstall) | `kubectl delete -f kubebench.yaml` removes the PVC as well |
| Installer | `sudo bash install.sh --uninstall` | `sudo bash install.sh --uninstall --purge` |
| `.deb` | `sudo apt remove kubebench` | `sudo apt purge kubebench` (data, configuration, user) |
| `.rpm` | `sudo dnf remove kubebench` | `sudo rm -rf /var/lib/kubebench /etc/kubebench && sudo userdel kubebench` |
| Docker | `docker rm -f kubebench` | `docker volume rm kubebench-data` |

## Development

```bash
make run                     # embedded build on :9876, state in build/run/ (honours KUBECONFIG)
npm run server:go            # backend from source on :9876
npm run dev                  # Vite on :5173 with hot reload, proxying /api to :9876
```

`make help` lists the build, test, packaging and release targets;
[releasing.md](releasing.md) explains how releases are cut.
