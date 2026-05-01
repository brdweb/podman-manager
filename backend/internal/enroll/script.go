package enroll

const installScript = `#!/usr/bin/env bash
set -euo pipefail

INSTALLER_URL="${PODMAN_MANAGER_AGENT_INSTALLER_URL:-https://raw.githubusercontent.com/brdweb/podman-manager/main/agent/install/install.sh}"

if command -v curl >/dev/null 2>&1; then
  curl -fsSL "$INSTALLER_URL" | bash -s -- "$@"
elif command -v wget >/dev/null 2>&1; then
  wget -qO- "$INSTALLER_URL" | bash -s -- "$@"
else
  echo "curl or wget is required to download the Podman Manager agent installer" >&2
  exit 1
fi
`
