#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
VERSION="${VERSION:-}"
PLG_PATH="${PLG_PATH:-$ROOT_DIR/podman-manager.plg}"
TXZ_PATH="${TXZ_PATH:-}"

python3 - "$VERSION" "$PLG_PATH" "$TXZ_PATH" <<'PY'
from hashlib import sha256
from pathlib import Path
import re
import sys

version_arg, plg_arg, txz_arg = sys.argv[1:4]
plg_path = Path(plg_arg)

if not plg_path.is_file():
    raise SystemExit(f"PLG file not found: {plg_path}")

content = plg_path.read_text()

version_match = re.search(r'<!ENTITY\s+version\s+"([^"]+)">', content)
github_match = re.search(r'<!ENTITY\s+github\s+"([^"]+)">', content)
url_match = re.search(r'<URL>([^<]+)</URL>', content)
sha_match = re.search(r'<SHA256>([0-9a-f]{64})</SHA256>', content)

if not version_match or not github_match or not url_match or not sha_match:
    raise SystemExit("Failed to parse version, github repo, URL, or SHA256 from podman-manager.plg")

version = version_match.group(1)
github_repo = github_match.group(1)
url = url_match.group(1)
manifest_sha = sha_match.group(1)

if version_arg and version != version_arg:
    raise SystemExit(f"Version mismatch: expected {version_arg}, found {version}")

expected_url = f"https://github.com/{github_repo}/releases/download/v{version}/podman-manager-{version}-x86_64-1.txz"
expanded_url = (url
    .replace('&github;', github_repo)
    .replace('&version;', version)
    .replace('&name;', 'podman-manager'))
if expanded_url != expected_url:
    raise SystemExit(f"Release URL mismatch: expected {expected_url}, found {expanded_url}")

if txz_arg:
    txz_path = Path(txz_arg)
else:
    txz_path = plg_path.parent / 'dist' / f'podman-manager-{version}-x86_64-1.txz'

if not txz_path.is_file():
    raise SystemExit(f"Package not found: {txz_path}")

file_sha = sha256(txz_path.read_bytes()).hexdigest()
if file_sha != manifest_sha:
    raise SystemExit(f"SHA mismatch: manifest {manifest_sha}, file {file_sha}")

print(f"Verified release manifest for {version}")
print(f"PLG: {plg_path}")
print(f"TXZ: {txz_path}")
PY
