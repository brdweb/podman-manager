#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

VERSION="${VERSION:-$(date +%Y.%m.%d)}"
GITHUB_REPO="${GITHUB_REPO:-brdweb/podman-manager}"
TXZ_PATH="${TXZ_PATH:-$ROOT_DIR/dist/podman-manager-${VERSION}-x86_64-1.txz}"
OUTPUT_PATH="${OUTPUT_PATH:-$ROOT_DIR/podman-manager.plg}"
TEMPLATE_PATH="$ROOT_DIR/podman-manager.plg.in"

if [ ! -f "$TEMPLATE_PATH" ]; then
  echo "Template not found: $TEMPLATE_PATH" >&2
  exit 1
fi

if [ ! -f "$TXZ_PATH" ]; then
  echo "Package not found: $TXZ_PATH" >&2
  exit 1
fi

TXZ_SHA256="$(sha256sum "$TXZ_PATH" | awk '{print $1}')"

sed \
  -e "s|@VERSION@|$VERSION|g" \
  -e "s|@GITHUB_REPO@|$GITHUB_REPO|g" \
  -e "s|@TXZ_SHA256@|$TXZ_SHA256|g" \
  "$TEMPLATE_PATH" > "$OUTPUT_PATH"

echo "Generated: $OUTPUT_PATH"
echo "Version:   $VERSION"
echo "Repo:      $GITHUB_REPO"
echo "SHA256:    $TXZ_SHA256"
