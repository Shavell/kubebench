#!/usr/bin/env bash
# Prints the KubeBench version used for builds, without a leading "v".
#
#   $VERSION set           -> that value               (e.g. 0.5.0)
#   reachable v* git tag   -> git describe             (0.5.0, 0.5.0-3-gabc1234, 0.5.0-dirty)
#   git, but no tag yet    -> <package.json>-<sha>     (0.5.0-abc1234, 0.5.0-abc1234-dirty)
#   no git at all          -> package.json "version"   (0.5.0)
#
# Usage: scripts/version.sh
set -Eeuo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

package_version() {
  # First top-level "version" key; portable between GNU and BSD sed.
  sed -n '/^[[:space:]]*"version":/{s/^[[:space:]]*"version":[[:space:]]*"\([^"]*\)".*/\1/p;q;}' \
    "$root/package.json" 2>/dev/null || true
}

if [[ -n "${VERSION:-}" ]]; then
  printf '%s\n' "${VERSION#v}"
  exit 0
fi

if command -v git >/dev/null 2>&1 && git -C "$root" rev-parse --git-dir >/dev/null 2>&1; then
  if desc="$(git -C "$root" describe --tags --always --dirty --match 'v[0-9]*' 2>/dev/null)"; then
    if [[ "$desc" =~ ^v[0-9] ]]; then
      printf '%s\n' "${desc#v}"
    else
      pkg="$(package_version)"
      printf '%s-%s\n' "${pkg:-0.0.0}" "$desc"
    fi
    exit 0
  fi
fi

pkg="$(package_version)"
printf '%s\n' "${pkg:-dev}"
