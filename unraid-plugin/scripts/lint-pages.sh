#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PHP_IMAGE="${PHP_IMAGE:-docker.io/library/php:8.2-cli}"

run_php_lint() {
  local target="$1"

  if command -v php >/dev/null 2>&1; then
    php -l "$target"
    return
  fi

  if command -v podman >/dev/null 2>&1; then
    podman run --rm -v "$ROOT_DIR:/workspace:Z" -w /workspace "$PHP_IMAGE" php -l "${target#$ROOT_DIR/}"
    return
  fi

  if command -v docker >/dev/null 2>&1; then
    docker run --rm -v "$ROOT_DIR:/workspace" -w /workspace "$PHP_IMAGE" php -l "${target#$ROOT_DIR/}"
    return
  fi

  echo "Unable to lint PHP: install php, podman, or docker" >&2
  exit 1
}

lint_page_file() {
  local source_file="$1"
  local tmp_file
  tmp_file="$(mktemp -p "$ROOT_DIR" .lint-page-XXXXXX.php)"

  python3 - "$source_file" "$tmp_file" <<'PY'
from pathlib import Path
import sys

source = Path(sys.argv[1]).read_text()
parts = source.split('\n---\n', 1)
body = parts[1] if len(parts) == 2 else source
Path(sys.argv[2]).write_text(body)
PY

  run_php_lint "$tmp_file"
  rm -f "$tmp_file"
}

while IFS= read -r file; do
  case "$file" in
    *.page)
      echo "Linting page: ${file#$ROOT_DIR/}"
      lint_page_file "$file"
      ;;
    *.php)
      echo "Linting php: ${file#$ROOT_DIR/}"
      run_php_lint "$file"
      ;;
  esac
done < <(find "$ROOT_DIR/src/usr/local/emhttp/plugins/podman-manager" \( -name '*.page' -o -name '*.php' \) | sort)

echo "PHP/page lint passed"
