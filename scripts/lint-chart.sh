#!/usr/bin/env bash
# Lints charts/kubebench and validates what it renders.
#
# Every scenario (the defaults, each charts/kubebench/ci/*-values.yaml and
# deploy/{kubernetes,openshift}/values.yaml) goes through `helm lint --strict`
# and `helm template`; the renders plus deploy/*/kubebench.yaml are then
# checked with `kubeconform -strict` for each Kubernetes version. kubeconform
# has no schema for the OpenShift Route, so Routes are skipped there. A
# NetworkPolicy without API server CIDRs, and a Route outside OpenShift mode,
# must fail to render offline.
#
# Usage: scripts/lint-chart.sh
#   KUBE_VERSIONS="1.30.0 1.35.0"  Kubernetes schema versions to validate against
#   KUBECONFORM_CACHE=DIR          cache downloaded schemas in DIR
#
# Requires helm and kubeconform (which downloads schemas from GitHub).
set -Eeuo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
chart="$root/charts/kubebench"
read -r -a kube_versions <<<"${KUBE_VERSIONS:-1.30.0 1.35.0}"

die() { printf 'lint-chart: %s\n' "$*" >&2; exit 1; }
for tool in helm kubeconform; do
  command -v "$tool" >/dev/null 2>&1 || die "$tool is required"
done

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

scenarios=(default "$chart"/ci/*-values.yaml "$root"/deploy/{kubernetes,openshift}/values.yaml)
for scenario in "${scenarios[@]}"; do
  args=()
  name=default
  if [[ "$scenario" != default ]]; then
    args=(--values "$scenario")
    name="$(basename "$(dirname "$scenario")")-$(basename "$scenario" .yaml)"
  fi
  printf '==> helm lint/template: %s\n' "$name"
  helm lint --strict --quiet "$chart" "${args[@]}"
  helm template kubebench "$chart" --namespace kubebench "${args[@]}" >"$work/$name.yaml"
done

# Offline, a NetworkPolicy without API server CIDRs cannot be confined and
# must not render (it would allow every destination on the API ports).
printf '==> helm template: networkPolicy without apiServer.cidrs fails\n'
if helm template kubebench "$chart" --set networkPolicy.enabled=true >/dev/null 2>&1; then
  die "networkPolicy.enabled without apiServer.cidrs rendered offline; it must fail"
fi

# A Route needs OpenShift mode, which an offline render detects only when forced.
printf '==> helm template: route.enabled without OpenShift mode fails\n'
if helm template kubebench "$chart" --set route.enabled=true >/dev/null 2>&1; then
  die "route.enabled rendered without OpenShift mode; it must fail"
fi

kubeconform_args=(-strict -summary -skip Route)
if [[ -n "${KUBECONFORM_CACHE:-}" ]]; then
  mkdir -p "$KUBECONFORM_CACHE"
  kubeconform_args+=(-cache "$KUBECONFORM_CACHE")
fi
for version in "${kube_versions[@]}"; do
  printf '==> kubeconform: Kubernetes %s\n' "$version"
  kubeconform "${kubeconform_args[@]}" -kubernetes-version "$version" \
    "$work"/*.yaml "$root"/deploy/{kubernetes,openshift}/kubebench.yaml
done
