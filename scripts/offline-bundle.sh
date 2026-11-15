#!/usr/bin/env bash
# Assembles kubebench-offline-<version>.tar.gz (+ .sha256) for air-gapped
# installs from the GoReleaser output and the Helm chart:
#
#   kubebench-offline-<version>/
#     OFFLINE-README.md   air-gapped steps for Kubernetes and VMs
#     LICENSE             the GPL-3.0 license text
#     install.sh          VM installer (install.sh --offline <this bundle>)
#     SHA256SUMS          checksums of every file in the bundle
#     packages/           Linux archives, .deb/.rpm and their checksums.txt
#     kubernetes/         Helm chart .tgz and plain manifests (kubebench.yaml)
#     images/             container image tarballs given with --image-tar
#
# Usage: scripts/offline-bundle.sh [options] [version]
#   version              release version (default: read from the archive names)
#   --release-dir DIR    GoReleaser output (default: build/release)
#   --chart FILE         packaged chart to include (default: helm package charts/kubebench)
#   --image-tar FILE     image tarball from `docker save` or buildx type=docker; repeatable.
#                        The script never builds or pulls images itself.
#   --output DIR         destination directory (default: build/extra)
#
# Requires helm (unless --chart is given), GNU tar (TAR=gtar on macOS) and sha256sum.
set -Eeuo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
release_dir="$root/build/release"
output_dir="$root/build/extra"
chart_tgz=""
version=""
image_tars=()
tar="${TAR:-tar}"

die() { printf 'offline-bundle: %s\n' "$*" >&2; exit 1; }
usage() { awk 'NR > 1 { if (!/^#/) exit; sub(/^# ?/, ""); print }' "${BASH_SOURCE[0]}"; }

while (($#)); do
  case "$1" in
    --release-dir) release_dir="${2:?--release-dir needs a value}"; shift 2 ;;
    --chart) chart_tgz="${2:?--chart needs a value}"; shift 2 ;;
    --image-tar) image_tars+=("${2:?--image-tar needs a value}"); shift 2 ;;
    --output) output_dir="${2:?--output needs a value}"; shift 2 ;;
    -h | --help) usage; exit 0 ;;
    -*) die "unknown option: $1 (see --help)" ;;
    *) [[ -z "$version" ]] || die "only one version argument"; version="${1#v}"; shift ;;
  esac
done

"$tar" --version 2>/dev/null | grep -q 'GNU tar' || die "GNU tar is required (set TAR=gtar)"
[[ -d "$release_dir" ]] || die "$release_dir not found; run make snapshot (or goreleaser release) first"
[[ -f "$release_dir/checksums.txt" ]] || die "$release_dir/checksums.txt not found"
[[ -n "$chart_tgz" ]] || command -v helm >/dev/null 2>&1 || die "helm is required (or pass --chart)"
for image in "${image_tars[@]}"; do
  [[ -f "$image" ]] || die "image tarball $image not found"
done

if [[ -z "$version" ]]; then
  versions="$(find "$release_dir" -maxdepth 1 -name 'kubebench_*_linux_*.tar.gz' -printf '%f\n' |
    sed 's/^kubebench_\(.*\)_linux_[^_]*\.tar\.gz$/\1/' | sort -u)"
  [[ -n "$versions" ]] || die "no kubebench_*_linux_*.tar.gz in $release_dir"
  [[ "$(wc -l <<<"$versions")" -eq 1 ]] || die "several versions in $release_dir; pass one explicitly"
  version="$versions"
fi

name="kubebench-offline-$version"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
stage="$work/$name"
mkdir -p "$stage/packages" "$stage/kubernetes"

# Linux archives and packages, with their lines of the release checksums.
shopt -s nullglob
packages=("$release_dir/kubebench_${version}_linux_"*.tar.gz "$release_dir"/*.deb "$release_dir"/*.rpm)
shopt -u nullglob
((${#packages[@]})) || die "no Linux archives or packages for $version in $release_dir"
for file in "${packages[@]}"; do
  cp "$file" "$stage/packages/"
  NAME="$(basename "$file")" awk '{ n = $2; sub(/^\*/, "", n) } n == ENVIRON["NAME"]' \
    "$release_dir/checksums.txt" >>"$stage/packages/checksums.txt"
done
(cd "$stage/packages" && sha256sum --quiet -c checksums.txt) ||
  die "packages do not match $release_dir/checksums.txt"
for arch in amd64 arm64; do
  [[ -f "$stage/packages/kubebench_${version}_linux_${arch}.tar.gz" ]] ||
    printf 'offline-bundle: warning: no linux/%s archive in the bundle\n' "$arch" >&2
done

if [[ -n "$chart_tgz" ]]; then
  [[ -f "$chart_tgz" ]] || die "chart $chart_tgz not found"
  cp "$chart_tgz" "$stage/kubernetes/"
else
  helm package "$root/charts/kubebench" --version "$version" --app-version "$version" \
    --destination "$stage/kubernetes" >/dev/null
fi
chart_file="$(basename "$(find "$stage/kubernetes" -name '*.tgz' -print -quit)")"
"$root/scripts/render-manifests.sh" --version "$version" --output "$stage/kubernetes/kubebench.yaml"

image_files=()
if ((${#image_tars[@]})); then
  mkdir -p "$stage/images"
  for image in "${image_tars[@]}"; do
    cp "$image" "$stage/images/"
    image_files+=("images/$(basename "$image")")
  done
fi

cp "$root/install.sh" "$stage/install.sh"
chmod 0755 "$stage/install.sh"
cp "$root/LICENSE" "$stage/LICENSE"

# OFFLINE-README.md: the image section lists what this bundle really holds.
if ((${#image_files[@]})); then
  # shellcheck disable=SC2016 # Markdown code spans, not command substitution
  image_list="$(printf -- '- `%s`\n' "${image_files[@]}")"
  example_image="${image_files[0]}"
else
  image_list="- none: this bundle was built without image tarballs. Save the image on a
  connected machine with \`docker pull ghcr.io/shavell/kube-bench:$version\` and
  \`docker save -o kube-bench_${version}_linux_amd64.tar ghcr.io/shavell/kube-bench:$version\`."
  example_image="images/kube-bench_${version}_linux_amd64.tar"
fi
cat >"$stage/OFFLINE-README.md" <<EOF
# KubeBench $version offline bundle

Everything needed to install KubeBench $version without internet access:
Linux archives and .deb/.rpm packages, the installer, the Helm chart, plain
Kubernetes manifests and the container image tarballs listed below.

Verify the bundle before copying it into the air-gapped network, and the
files after extracting it:

\`\`\`bash
sha256sum -c $name.tar.gz.sha256
tar -xzf $name.tar.gz && cd $name
sha256sum -c SHA256SUMS
\`\`\`

## Virtual machines and bare-metal servers

\`\`\`bash
sudo bash install.sh --offline .                 # detects .deb / .rpm / tarball
sudo bash install.sh --offline . --method tarball --port 8080
\`\`\`

The installer verifies the package against \`packages/checksums.txt\`,
creates the \`kubebench\` user, writes \`/etc/kubebench/kubebench.env\` with a
generated admin password (printed once) and \`KUBEBENCH_UPDATE_MODE=off\` (see
[Update checks](#update-checks)) and starts \`kubebench.service\`.
Without the installer: \`sudo dpkg -i packages/kubebench_*_amd64.deb\` or
\`sudo rpm -i packages/kubebench-*.x86_64.rpm\`, set \`KUBEBENCH_UPDATE_MODE=off\`
in \`/etc/kubebench/kubebench.env\`, restart the service, then sign in as
admin/admin and change the password.

## Kubernetes

Image tarballs in this bundle:

$image_list

### 1. Make the image available to the cluster

Pick the tarball that matches the node architecture
(\`kubectl get nodes -L kubernetes.io/arch\`), then either load it on **every
node** that may run the pod (the chart uses \`imagePullPolicy: IfNotPresent\`):

\`\`\`bash
image=$example_image
docker load -i "\$image"                        # nodes running Docker
sudo ctr -n k8s.io images import "\$image"      # containerd (kubeadm, ...)
sudo k3s ctr images import "\$image"            # k3s; it also imports tarballs
                                               # from /var/lib/rancher/k3s/agent/images/ at startup
\`\`\`

or push it to a registry inside the network:

\`\`\`bash
docker load -i "\$image"
docker tag ghcr.io/shavell/kube-bench:$version registry.example.internal/kube-bench:$version
docker push registry.example.internal/kube-bench:$version
# without Docker:
skopeo copy "docker-archive:\$image" docker://registry.example.internal/kube-bench:$version
\`\`\`

### 2a. Install with Helm

\`\`\`bash
helm install kubebench kubernetes/$chart_file \\
  --namespace kubebench --create-namespace --set config.updateMode=off
# images pushed to a private registry:
#   --set image.repository=registry.example.internal/kube-bench
\`\`\`

Read the generated admin password with the command printed by Helm (NOTES).

### 2b. Install with kubectl

\`\`\`bash
# with a private registry, rewrite the image first:
#   sed -i 's#ghcr.io/shavell/kube-bench#registry.example.internal/kube-bench#' kubernetes/kubebench.yaml
kubectl apply -f kubernetes/kubebench.yaml
kubectl -n kubebench set env deployment/kubebench KUBEBENCH_UPDATE_MODE=off
\`\`\`

Then \`kubectl -n kubebench port-forward svc/kubebench 8080:80\`, open
http://localhost:8080, sign in as admin/admin and set a new password.

## Update checks

KubeBench asks GitHub for new releases unless \`KUBEBENCH_UPDATE_MODE\` is
\`off\`. Without internet access the check can only fail, so the steps above
turn it off: \`install.sh --offline\` writes it to the env file, the Helm
command sets \`config.updateMode=off\`, the kubectl steps set it on the
Deployment. Administrators see the mode in Settings > Updates. A mirror of
the releases list inside the network works too: set
\`KUBEBENCH_UPDATE_URL\` to it and the mode to \`stable\`.

## Notes

- Helm charts installed from the KubeBench UI need a chart repository inside
  the network, or an uploaded chart \`.tgz\` (Helm > Install > Upload).
- Documentation: https://github.com/Shavell/kube-bench/blob/main/docs/deployment.md
EOF

(cd "$stage" && find . -type f -printf '%P\n' | LC_ALL=C sort | xargs sha256sum) >"$work/SHA256SUMS"
mv "$work/SHA256SUMS" "$stage/SHA256SUMS"

# Reproducible archive: fixed order, owners, modes and timestamps. The modes
# also make files readable that arrive as 0600 (`docker save -o` writes those).
epoch="${SOURCE_DATE_EPOCH:-$(git -C "$root" log -1 --format=%ct 2>/dev/null || date +%s)}"
mkdir -p "$output_dir"
bundle="$output_dir/$name.tar.gz"
"$tar" --sort=name --owner=0 --group=0 --numeric-owner --mode='u+rwX,go+rX,go-w' --mtime="@$epoch" \
  -C "$work" -cf - "$name" | gzip -n -9 >"$bundle"
(cd "$output_dir" && sha256sum "$name.tar.gz" >"$name.tar.gz.sha256")
printf 'offline-bundle: wrote %s (%s)\n' "${bundle#"$root"/}" "$(du -h "$bundle" | cut -f1)"
