#!/usr/bin/env bash
# Checks that the tree is ready to be released as <version>:
#   - package.json "version", Chart.yaml version and appVersion equal it
#   - CHANGELOG.md has a dated "## [<version>] - YYYY-MM-DD" section with entries
#   - deploy/kubernetes/kubebench.yaml matches a fresh render of the chart
# Every problem is reported before exiting 1.
#
# Usage: scripts/check-release.sh <version>   (with or without "v")
set -Eeuo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if (($# != 1)); then
  echo "usage: scripts/check-release.sh <version>" >&2
  exit 1
fi
version="${1#v}"
problems=0

fail() {
  printf 'check-release: %s\n' "$*" >&2
  problems=$((problems + 1))
}

# First "version" key of package.json (the top-level one), as in version.sh.
pkg="$(sed -n '/^[[:space:]]*"version":/{s/^[[:space:]]*"version":[[:space:]]*"\([^"]*\)".*/\1/p;q;}' "$root/package.json")"
[[ "$pkg" == "$version" ]] || fail "package.json version is \"$pkg\", expected \"$version\""

for key in version appVersion; do
  value="$(sed -n "s/^$key:[[:space:]]*\"\{0,1\}\([^\"]*\)\"\{0,1\}[[:space:]]*$/\1/p" "$root/charts/kubebench/Chart.yaml")"
  [[ "$value" == "$version" ]] || fail "charts/kubebench/Chart.yaml $key is \"$value\", expected \"$version\""
done

if ! grep -Eq "^## \[${version//./\\.}\] - [0-9]{4}-[0-9]{2}-[0-9]{2}\$" "$root/CHANGELOG.md"; then
  fail "CHANGELOG.md needs a heading \"## [$version] - YYYY-MM-DD\""
fi
"$root/scripts/extract-changelog.sh" "$version" >/dev/null || problems=$((problems + 1))
"$root/scripts/render-manifests.sh" --check || problems=$((problems + 1))

if ((problems)); then
  printf 'check-release: %d problem(s); not ready to release %s\n' "$problems" "$version" >&2
  exit 1
fi
printf 'check-release: ready to release %s\n' "$version"
