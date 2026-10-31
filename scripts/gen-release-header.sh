#!/usr/bin/env bash
# gen-release-header.sh - extract the release notes section for the version
# being released from CHANGELOG.md into dist/release-header.md, consumed by the
# CI step `gh release edit --notes-file` (goreleaser OSS cannot load the header
# from a file - `release.header.from_file` is Pro-only).
#
# The file is ALWAYS produced (empty when the section is not found), so a
# missing CHANGELOG entry degrades gracefully: the release body falls back to
# goreleaser's default changelog instead of failing the release.
#
# Usage:
#   bash scripts/gen-release-header.sh [version]
#
#   version  optional; defaults to the latest git tag with the leading "v"
#            stripped (matches goreleaser's {{ .Version }} on a tag push).
#            The .goreleaser.yaml before-hook passes {{ .Version }} explicitly
#            so the script always extracts the exact version being released.
#
# ASCII-only comments on purpose (project convention for shell scripts).
set -euo pipefail

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  VERSION="$(git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || true)"
fi
echo "[gen-release-header] version: ${VERSION:-<none>}"

mkdir -p dist

# Find the "## [<version>]" heading in CHANGELOG.md and print everything up to
# the next "## [" heading (same awk logic as the former release.yml step).
# Redirection creates the file even when nothing matches -> empty header.
awk -v v="$VERSION" '
  $0 ~ "^## \\[" v "\\]" {found=1; next}
  found && /^## \[/ {exit}
  found {print}
' CHANGELOG.md > dist/release-header.md

if [ -s dist/release-header.md ]; then
  echo "[gen-release-header] extracted CHANGELOG section -> dist/release-header.md"
else
  echo "[gen-release-header] no [$VERSION] section in CHANGELOG.md; header left empty (goreleaser changelog will backfill the body)"
fi
