#!/usr/bin/env bash
# Prints the body of one version's section of CHANGELOG.md (Keep a Changelog
# format: "## [0.5.0] - 2026-11-20" up to the next "## [" heading or the link
# references), e.g. as GitHub release notes. Exits 1 when the section is
# missing or empty.
#
# Usage: scripts/extract-changelog.sh <version> [changelog]   (version with or without "v")
set -Eeuo pipefail

die() { printf 'extract-changelog: %s\n' "$*" >&2; exit 1; }

(($# >= 1 && $# <= 2)) || die "usage: scripts/extract-changelog.sh <version> [changelog]"
version="${1#v}"
changelog="${2:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/CHANGELOG.md}"
[[ -f "$changelog" ]] || die "$changelog not found"

# Collects the section, then prints it without leading/trailing blank lines.
body="$(HEADING="## [$version]" awk '
  /^## \[/ {
    if (found) exit
    if (index($0, ENVIRON["HEADING"]) == 1) { found = 1; next }
  }
  found && /^\[[^]]+\]: / { exit }
  found { lines[++n] = $0 }
  END {
    first = 1
    while (first <= n && lines[first] ~ /^[[:space:]]*$/) first++
    last = n
    while (last >= first && lines[last] ~ /^[[:space:]]*$/) last--
    for (i = first; i <= last; i++) print lines[i]
  }' "$changelog")"

[[ -n "$body" ]] || die "no entries for version $version in $changelog"
printf '%s\n' "$body"
