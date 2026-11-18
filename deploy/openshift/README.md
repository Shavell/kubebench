# KubeBench on OpenShift with plain manifests

`kubebench.yaml` installs KubeBench on OpenShift without Helm. It is
rendered from the Helm chart by `scripts/render-manifests.sh` (`make
manifests`) with [`values.yaml`](values.yaml) and holds the objects of
[the Kubernetes manifests](../kubernetes/README.md) plus a Route:

| Object | Notes |
|---|---|
| Namespace `kubebench` | Pod Security `restricted` (enforce, warn, audit) |
| ServiceAccount `kubebench` | registered as the `in-cluster` cluster on first start |
| ClusterRoleBinding `kubebench` | to the built-in `cluster-admin` ClusterRole |
| PersistentVolumeClaim `kubebench-data` | 1Gi on the default StorageClass, holds the SQLite database |
| Service `kubebench` | ClusterIP, port 80 |
| Deployment `kubebench` | one replica, `Recreate`, pinned to the `restricted-v2` SCC, no fixed UIDs |
| Route `kubebench` | edge TLS on the router's default certificate, HTTP redirected to HTTPS |

The pod sets no `runAsUser`, `runAsGroup` or `fsGroup`: the SCC assigns
them from the project's ranges and the image runs with any UID in group 0.
The `openshift.io/required-scc: restricted-v2` annotation keeps the pod on
`restricted-v2` even though the `cluster-admin` binding would allow every
SCC. There is **no password Secret**: sign in as `admin` / `admin` right
after installing; KubeBench asks for a new password at once. Use the Helm
chart for a generated password, the `read-only` RBAC preset, your own Route
host or certificate, or a NetworkPolicy
([docs/deployment.md](../../docs/deployment.md#openshift)).

## Install

Applying needs a user who may create the ClusterRoleBinding (a cluster
administrator):

```bash
oc apply -f https://github.com/Shavell/kubebench/releases/download/v0.8.1/kubebench-openshift.yaml
# or from a checkout / the offline bundle:
oc apply -f deploy/openshift/kubebench.yaml
oc -n kubebench rollout status deploy/kubebench
oc -n kubebench get pod -l app.kubernetes.io/name=kubebench \
  -o jsonpath='{.items[*].metadata.annotations.openshift\.io/scc}'   # restricted-v2
```

To use a project created with `oc new-project kubebench` instead, the
Namespace in the file is applied onto it (the labels are added).

## Open the UI

```bash
echo "https://$(oc -n kubebench get route kubebench -o jsonpath='{.spec.host}')"
```

The host is `kubebench-kubebench.<apps domain>`. For your own host name,
patch the Route (`oc -n kubebench patch route kubebench --type merge -p
'{"spec":{"host":"kubebench.example.com"}}'`) or install with the Helm chart
and `route.host`.

## Change the image or version

```bash
oc -n kubebench set image deploy/kubebench kubebench=ghcr.io/shavell/kubebench:0.7.1
```

Upgrading is applying the `kubebench-openshift.yaml` of the new release.

## Uninstall

`oc delete -f kubebench.yaml` deletes everything, **including the
PersistentVolumeClaim with the database**. To keep the data, delete the
other objects only:

```bash
oc -n kubebench delete route,deploy,svc,sa kubebench
oc delete clusterrolebinding kubebench
# later, to remove the data too:
oc delete project kubebench
```

## Another namespace

```bash
scripts/render-manifests.sh --variant openshift --namespace tools --output kubebench-tools.yaml
oc apply -f kubebench-tools.yaml
```

## Regenerate

`kubebench.yaml` is generated; change the chart or `values.yaml`, then run
`make manifests` and commit the result. CI fails when the committed file is
stale (`scripts/render-manifests.sh --check`).
