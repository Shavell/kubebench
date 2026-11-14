# KubeBench with plain manifests

`kubebench.yaml` installs KubeBench without Helm. It is rendered from the
Helm chart by `scripts/render-manifests.sh` (`make manifests`) with
[`values.yaml`](values.yaml) and contains:

| Object | Notes |
|---|---|
| Namespace `kubebench` | Pod Security `restricted` (enforce, warn, audit) |
| ServiceAccount `kubebench` | registered as the `in-cluster` cluster on first start |
| ClusterRoleBinding `kubebench` | to the built-in `cluster-admin` ClusterRole |
| PersistentVolumeClaim `kubebench-data` | 1Gi on the default StorageClass, holds the SQLite database |
| Service `kubebench` | ClusterIP, port 80 |
| Deployment `kubebench` | one replica, `Recreate`, non-root, read-only root filesystem |

There is **no password Secret**: KubeBench creates the `admin` user with the
password `admin` and asks for a new one at the first sign-in. Sign in right
after installing, before anyone else can reach the UI. Use the Helm chart
for a generated password, an existing Secret, other RBAC presets, an Ingress
or a NetworkPolicy.

## Install

```bash
kubectl apply -f https://github.com/Shavell/kube-bench/releases/download/v0.5.0/kubebench.yaml
# or from a checkout / the offline bundle:
kubectl apply -f deploy/kubernetes/kubebench.yaml
kubectl -n kubebench rollout status deploy/kubebench
```

## Open the UI

```bash
kubectl -n kubebench port-forward svc/kubebench 8080:80
# http://localhost:8080 — sign in as admin / admin, then choose a new password
```

To expose it permanently, add an Ingress (WebSocket shells and SSE log
streams need long proxy timeouts) and set `KUBEBENCH_TRUST_PROXY=true` on
the Deployment so cookies and audit entries use the forwarded scheme and
client address:

```bash
kubectl -n kubebench set env deploy/kubebench KUBEBENCH_TRUST_PROXY=true
kubectl -n kubebench create ingress kubebench --class=traefik \
  --rule="kubebench.example.com/*=kubebench:80,tls=kubebench-tls"
```

A `LoadBalancer` or `NodePort` Service works too
(`kubectl -n kubebench patch svc kubebench -p '{"spec":{"type":"LoadBalancer"}}'`);
leave `KUBEBENCH_TRUST_PROXY` off in that case.

## Change the image or version

```bash
kubectl -n kubebench set image deploy/kubebench kubebench=ghcr.io/shavell/kube-bench:0.5.1
# private registry (air-gapped):
sed 's#ghcr.io/shavell/kube-bench#registry.example.internal/kube-bench#' kubebench.yaml | kubectl apply -f -
```

Upgrading is applying the `kubebench.yaml` of the new release; the
`Recreate` strategy stops the old pod before the new one opens the database.

## Uninstall

`kubectl delete -f kubebench.yaml` deletes everything, **including the
PersistentVolumeClaim with the database**. To keep the data, delete the other
objects only:

```bash
kubectl -n kubebench delete deploy,svc,sa kubebench
kubectl delete clusterrolebinding kubebench
# later, to remove the data too:
kubectl delete namespace kubebench
```

## Another namespace

The objects can be rendered for any namespace from a checkout:

```bash
scripts/render-manifests.sh --namespace tools --output kubebench-tools.yaml
kubectl apply -f kubebench-tools.yaml
```

## Regenerate

`kubebench.yaml` is generated; change the chart or `values.yaml`, then run
`make manifests` and commit the result. CI fails when the committed file is
stale (`scripts/render-manifests.sh --check`).
