# Deployment

KubeBench ships as a single container: the Go binary with the built frontend
embedded (`go:embed`). It stores users, sessions and the cluster inventory in
SQLite on a PVC.

## Image

```bash
docker build -t ghcr.io/your-org/kubebench:0.1.0 .
docker push ghcr.io/your-org/kubebench:0.1.0
```

Multi-stage build: `node` builds the frontend → `golang` builds the binary
with `dist/` embedded → `distroless/static` runtime (~38 MB, non-root).

Run locally:

```bash
docker run -p 9876:9876 -v kubebench-data:/data \
  -e KUBEBENCH_ADMIN_PASSWORD=changeme \
  ghcr.io/your-org/kubebench:0.1.0
```

## Helm (in-cluster)

```bash
helm install kubebench charts/kubebench \
  --namespace kubebench --create-namespace \
  --set image.repository=ghcr.io/your-org/kubebench \
  --set auth.adminPassword=changeme
```

The chart creates:

- **ServiceAccount** + **ClusterRoleBinding** — by default to `cluster-admin`
  so the workbench can manage the cluster (helm, exec, delete, CRDs). Narrow
  it with `rbac.clusterRole: ""` + `rbac.rules: [...]` for least privilege.
- **PVC** on `persistence.storageClass` (default **`local-path`** for k3s) —
  holds `/data/kubebench.db`. The Deployment uses the `Recreate` strategy and
  one replica because SQLite + RWO must not have two writers.
- **Secret** with the bootstrap admin password (kept across upgrades via
  `helm.sh/resource-policy: keep`; use `auth.existingSecret` to bring your own).
- **Service** (+ optional **Ingress**).

The pod's ServiceAccount is auto-registered in the inventory as the
`in-cluster` cluster and becomes admin's active cluster. Add remote clusters
from the **Clusters** module (paste a kubeconfig) and grant them to users from
the **Users** module.

### First login

```bash
kubectl -n kubebench get secret kubebench-auth \
  -o jsonpath='{.data.admin-password}' | base64 -d
kubectl -n kubebench port-forward svc/kubebench 8080:80
open http://localhost:8080   # sign in as admin
```

## Local development

```bash
# Backend against a kubeconfig (seeds it as a cluster, creates admin/admin):
KUBECONFIG=~/.kube/config npm run server:go    # :9876

# Frontend with hot reload, proxying /api to the Go backend:
npm run dev                                     # :5173
```

Mock mode still works with no backend running — the frontend detects the
missing `/api` and skips auth entirely.

## Configuration

| Env | Default | Purpose |
|---|---|---|
| `PORT` | `9876` | HTTP listen port |
| `KUBEBENCH_DB` | `kubebench.db` (`/data/...` in chart) | SQLite path |
| `KUBEBENCH_ADMIN_PASSWORD` | `admin` | bootstrap admin password (first run only) |
| `KUBECONFIG` | — | dev convenience: seed a cluster from this kubeconfig |

## Security notes

- Sessions are DB-backed HttpOnly cookies (7-day TTL).
- Stored kubeconfigs are unencrypted in SQLite — protect the volume;
  at-rest encryption is a planned follow-up.
- Roles: **admin** manages users + the whole inventory; **user** sees only
  assigned clusters. Kubernetes-side authorization is whatever each cluster
  credential grants.
