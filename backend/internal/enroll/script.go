package enroll

const installScript = `#!/usr/bin/env bash
set -euo pipefail

MANAGER_URL="__MANAGER_URL__"
TOKEN=""
CONFIG_PATH="/etc/podman-agent/config.yaml"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --manager-url)
      MANAGER_URL="$2"
      shift 2
      ;;
    --token)
      TOKEN="$2"
      shift 2
      ;;
    --config)
      CONFIG_PATH="$2"
      shift 2
      ;;
    *)
      echo "Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

if [[ -z "$TOKEN" ]]; then
  echo "Enrollment token is required. Pass --token <token>." >&2
  exit 1
fi

install -d -m 0755 "$(dirname "$CONFIG_PATH")"
cat > "$CONFIG_PATH" <<EOF
manager:
  address: "$MANAGER_URL"
  tls: false
  tls_insecure: false
agent:
  id: ""
  credential: ""
  token: "$TOKEN"
podman:
  socket_path: "/run/podman/podman.sock"
  timeout: 10s
heartbeat:
  interval: 15s
  timeout: 5s
reconnect:
  initial_backoff: 1s
  max_backoff: 1m
  multiplier: 2.0
log:
  level: info
  format: json
EOF
chmod 0600 "$CONFIG_PATH"

echo "Podman Manager agent configuration written to $CONFIG_PATH"
echo "Install and start the podman-agent binary with: podman-agent --config $CONFIG_PATH --token $TOKEN"
`
