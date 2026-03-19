#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(dirname "$(readlink -f "$0")")
TARGET_HOST=${1:-yoda-podman}
SSH_USER=${2:-brdweb}
TARGET_LABEL=$TARGET_HOST

if [[ $TARGET_HOST == "yoda-podman" ]]; then
  TARGET_LABEL="yoda-podman (192.168.40.60)"
fi

read -r -p "SSH private key to deploy [~/.ssh/id_ed25519]: " SSH_KEY_SOURCE
SSH_KEY_SOURCE=${SSH_KEY_SOURCE:-~/.ssh/id_ed25519}
SSH_KEY_SOURCE=${SSH_KEY_SOURCE/#\~/$HOME}

if [[ ! -f $SSH_KEY_SOURCE ]]; then
  printf 'SSH key not found: %s\n' "$SSH_KEY_SOURCE" >&2
  exit 1
fi

REMOTE_USER_CONFIG_DIR=".config/podman-manager"
REMOTE_SYSTEM_CONFIG_DIR="/etc/podman-manager"
REMOTE_QUADLET_DIR="/etc/containers/systemd"
REMOTE_CONFIG_PATH="$REMOTE_USER_CONFIG_DIR/config.yaml"
REMOTE_KEY_PATH="$REMOTE_USER_CONFIG_DIR/id_ed25519"

printf 'Deploying Podman Manager to %s as %s\n' "$TARGET_LABEL" "$SSH_USER"

ssh "$SSH_USER@$TARGET_HOST" "mkdir -p '$REMOTE_USER_CONFIG_DIR'"

CONFIG_FILE="$SCRIPT_DIR/config.yaml"
if [[ ! -f $CONFIG_FILE ]]; then
  CONFIG_FILE="$SCRIPT_DIR/config.example.yaml"
fi
scp "$CONFIG_FILE" "$SSH_USER@$TARGET_HOST:$REMOTE_CONFIG_PATH"
scp "$SSH_KEY_SOURCE" "$SSH_USER@$TARGET_HOST:$REMOTE_KEY_PATH"

for quadlet_file in "$SCRIPT_DIR"/quadlet/*; do
  scp "$quadlet_file" "$SSH_USER@$TARGET_HOST:/tmp/$(basename "$quadlet_file")"
done

ssh "$SSH_USER@$TARGET_HOST" "sudo install -d -m 0755 '$REMOTE_SYSTEM_CONFIG_DIR' '$REMOTE_QUADLET_DIR' && sudo install -m 0644 '$REMOTE_CONFIG_PATH' '$REMOTE_SYSTEM_CONFIG_DIR/config.yaml' && sudo install -m 0600 '$REMOTE_KEY_PATH' '$REMOTE_SYSTEM_CONFIG_DIR/id_ed25519' && sudo systemctl disable --now podman-manager-backend.service podman-manager-webapp.service 2>/dev/null || true && for obsolete in podman-manager.network podman-manager-backend.container podman-manager-webapp.container; do sudo rm -f '$REMOTE_QUADLET_DIR/'\"\$obsolete\"; done && sudo install -m 0644 /tmp/podman-manager.container '$REMOTE_QUADLET_DIR/' && rm -f /tmp/podman-manager.container && sudo systemctl daemon-reload && sudo systemctl enable --now podman-manager.service && sudo systemctl restart podman-manager.service"

printf 'Deployment complete. Web UI: http://%s:18780\n' "$TARGET_HOST"
