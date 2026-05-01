#!/usr/bin/env bash

set -euo pipefail

AGENT_NAME="podman-manager-agent"
SERVICE_NAME="${AGENT_NAME}.service"
CONTAINER_FILE="${AGENT_NAME}.container"
DEFAULT_IMAGE="ghcr.io/brdweb/podman-manager/agent:latest"

MANAGER_URL=""
TOKEN=""
IMAGE="$DEFAULT_IMAGE"
ROOTLESS=false
DRY_RUN=false

if [[ -t 1 ]]; then
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  BLUE='\033[0;34m'
  NC='\033[0m'
else
  RED=''
  GREEN=''
  YELLOW=''
  BLUE=''
  NC=''
fi

info() {
  printf '%b==>%b %s\n' "$BLUE" "$NC" "$*"
}

success() {
  printf '%bSuccess:%b %s\n' "$GREEN" "$NC" "$*"
}

warn() {
  printf '%bWarning:%b %s\n' "$YELLOW" "$NC" "$*" >&2
}

error() {
  printf '%bError:%b %s\n' "$RED" "$NC" "$*" >&2
}

die() {
  error "$*"
  exit 1
}

usage() {
  cat <<'EOF'
Install the Podman Manager agent as a Quadlet-managed Podman container.

Usage:
  install.sh --manager-url URL --token TOKEN [--rootless] [--image IMAGE] [--dry-run]

Options:
  --manager-url URL  Manager URL the agent should connect to. Required.
  --token TOKEN      Agent registration/authentication token. Required.
  --rootless         Install as the current user with systemd --user.
  --image IMAGE      Agent image to run. Defaults to ghcr.io/brdweb/podman-manager/agent:latest.
  --dry-run          Print planned actions and rendered Quadlet without changing the system.
  -h, --help         Show this help text.
EOF
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --manager-url)
        [[ $# -ge 2 ]] || die "--manager-url requires a value"
        MANAGER_URL="$2"
        shift 2
        ;;
      --token)
        [[ $# -ge 2 ]] || die "--token requires a value"
        TOKEN="$2"
        shift 2
        ;;
      --rootless)
        ROOTLESS=true
        shift
        ;;
      --image)
        [[ $# -ge 2 ]] || die "--image requires a value"
        IMAGE="$2"
        shift 2
        ;;
      --dry-run)
        DRY_RUN=true
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "Unknown argument: $1"
        ;;
    esac
  done
}

validate_args() {
  [[ -n "$MANAGER_URL" ]] || die "--manager-url is required"
  [[ -n "$TOKEN" ]] || die "--token is required"
  [[ "$MANAGER_URL" != *$'\n'* ]] || die "--manager-url cannot contain newlines"
  [[ "$TOKEN" != *$'\n'* ]] || die "--token cannot contain newlines"
  [[ "$IMAGE" != *$'\n'* ]] || die "--image cannot contain newlines"
}

require_command() {
  local name="$1"
  command -v "$name" >/dev/null 2>&1 || die "Required command not found: $name"
}

check_prerequisites() {
  require_command podman
  require_command systemctl

  if [[ "$ROOTLESS" == false && "${EUID:-$(id -u)}" -ne 0 ]]; then
    require_command sudo
  fi

  if [[ "$ROOTLESS" == true ]]; then
    require_command loginctl
    [[ "${EUID:-$(id -u)}" -ne 0 ]] || die "Rootless install must be run as the target non-root user"
    [[ -n "${HOME:-}" ]] || die "HOME is not set; cannot determine rootless Quadlet directory"
    [[ -n "${USER:-}" ]] || die "USER is not set; cannot enable linger for the rootless install"
  fi
}

rootful_template() {
  cat <<'EOF'
[Unit]
Description=Podman Manager Agent
After=podman.service
Requires=podman.service

[Container]
Image=ghcr.io/brdweb/podman-manager/agent:latest
ContainerName=podman-manager-agent
Volume=/etc/podman-agent:/etc/podman-agent:Z
Volume=/run/podman/podman.sock:/run/podman/podman.sock
Volume=/etc/containers/systemd:/etc/containers/systemd:ro
Volume=/proc:/host/proc:ro
Volume=/sys:/host/sys:ro
Volume=/dev:/host/dev:ro
Volume=/etc/os-release:/host/etc/os-release:ro
Environment=AGENT_MANAGER_URL=__MANAGER_URL__
Environment=AGENT_TOKEN=__TOKEN__
Environment=AGENT_TLS=false
Environment=AGENT_LOG_LEVEL=info
Environment=AGENT_LOG_FORMAT=json

[Service]
Restart=always

[Install]
WantedBy=multi-user.target
EOF
}

rootless_template() {
  cat <<'EOF'
[Unit]
Description=Podman Manager Agent (Rootless)
After=podman.service

[Container]
Image=ghcr.io/brdweb/podman-manager/agent:latest
ContainerName=podman-manager-agent
Volume=$HOME/.config/podman-agent:/etc/podman-agent:Z
Volume=%t/podman/podman.sock:/run/podman/podman.sock
Volume=$HOME/.config/containers/systemd:/etc/containers/systemd:ro
Volume=/proc:/host/proc:ro
Volume=/sys:/host/sys:ro
Environment=AGENT_MANAGER_URL=__MANAGER_URL__
Environment=AGENT_TOKEN=__TOKEN__
Environment=AGENT_TLS=false
Environment=AGENT_LOG_LEVEL=info
Environment=AGENT_LOG_FORMAT=json

[Service]
Restart=always

[Install]
WantedBy=default.target
EOF
}

render_quadlet() {
  local template

  if [[ "$ROOTLESS" == true ]]; then
    template="$(rootless_template)"
  else
    template="$(rootful_template)"
  fi

  template="${template//__MANAGER_URL__/$MANAGER_URL}"
  template="${template//__TOKEN__/$TOKEN}"
  template="${template//__IMAGE__/$IMAGE}"
  template="${template//Image=$DEFAULT_IMAGE/Image=$IMAGE}"
  printf '%s\n' "$template"
}

redacted_quadlet() {
  render_quadlet | sed 's/^Environment=AGENT_TOKEN=.*/Environment=AGENT_TOKEN=<redacted>/'
}

run_cmd() {
  if [[ "$DRY_RUN" == true ]]; then
    printf '[dry-run]'
    printf ' %q' "$@"
    printf '\n'
  else
    "$@"
  fi
}

run_rootful() {
  if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    run_cmd "$@"
  else
    run_cmd sudo "$@"
  fi
}

systemctl_user() {
  run_cmd systemctl --user "$@"
}

systemctl_rootful() {
  run_rootful systemctl "$@"
}

enable_linger() {
  if [[ "$ROOTLESS" != true ]]; then
    return 0
  fi

  info "Enabling systemd linger for $USER so the rootless agent can start at boot"
  if [[ "$DRY_RUN" == true ]]; then
    run_cmd loginctl enable-linger "$USER"
    return 0
  fi

  if loginctl enable-linger "$USER" 2>/dev/null; then
    return 0
  fi

  if command -v sudo >/dev/null 2>&1; then
    sudo loginctl enable-linger "$USER"
  else
    die "Failed to enable linger for $USER. Re-run with permissions to execute: loginctl enable-linger $USER"
  fi
}

confirm_update() {
  local quadlet_path="$1"

  if [[ ! -e "$quadlet_path" ]]; then
    return 0
  fi

  warn "Existing agent installation found at $quadlet_path"

  if [[ "$DRY_RUN" == true ]]; then
    info "Dry run would update the existing Quadlet file"
    return 0
  fi

  if [[ -t 0 && -r /dev/tty ]]; then
    local answer
    printf 'Update the existing Podman Manager agent installation? [y/N] ' > /dev/tty
    read -r answer < /dev/tty || answer=""
    case "$answer" in
      y|Y|yes|YES) return 0 ;;
      *) die "Installation cancelled" ;;
    esac
  fi

  info "Non-interactive mode detected; updating the existing installation"
}

write_quadlet() {
  local quadlet_dir="$1"
  local quadlet_path="$2"
  local agent_config_dir
  local tmp_file

  confirm_update "$quadlet_path"

  info "Writing Quadlet file to $quadlet_path"
  if [[ "$ROOTLESS" == true ]]; then
    agent_config_dir="$HOME/.config/podman-agent"
  else
    agent_config_dir="/etc/podman-agent"
  fi

  if [[ "$DRY_RUN" == true ]]; then
    if [[ "$ROOTLESS" == true ]]; then
      run_cmd install -d -m 0755 "$quadlet_dir"
      run_cmd install -d -m 0700 "$agent_config_dir"
    else
      run_rootful install -d -m 0755 "$quadlet_dir"
      run_rootful install -d -m 0700 "$agent_config_dir"
    fi
    printf '%s\n' '--- rendered Quadlet (token redacted) ---'
    redacted_quadlet
    printf '%s\n' '--- end rendered Quadlet ---'
    return 0
  fi

  if [[ "$ROOTLESS" == true ]]; then
    install -d -m 0755 "$quadlet_dir"
    install -d -m 0700 "$agent_config_dir"
    tmp_file="$(mktemp)"
    render_quadlet > "$tmp_file"
    install -m 0600 "$tmp_file" "$quadlet_path"
    rm -f "$tmp_file"
  else
    tmp_file="$(mktemp)"
    render_quadlet > "$tmp_file"
    run_rootful install -d -m 0755 "$quadlet_dir"
    run_rootful install -d -m 0700 "$agent_config_dir"
    run_rootful install -m 0600 "$tmp_file" "$quadlet_path"
    rm -f "$tmp_file"
  fi
}

reload_and_start() {
  info "Reloading systemd and starting $SERVICE_NAME"

  if [[ "$ROOTLESS" == true ]]; then
    systemctl_user daemon-reload
    if [[ "$DRY_RUN" == true ]]; then
      systemctl_user start "$SERVICE_NAME"
      return 0
    fi
    if systemctl --user is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
      systemctl_user restart "$SERVICE_NAME"
    else
      systemctl_user start "$SERVICE_NAME"
    fi
  else
    systemctl_rootful daemon-reload
    if [[ "$DRY_RUN" == true ]]; then
      systemctl_rootful start "$SERVICE_NAME"
      return 0
    fi
    if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
      if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        systemctl_rootful restart "$SERVICE_NAME"
      else
        systemctl_rootful start "$SERVICE_NAME"
      fi
    elif sudo systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
      systemctl_rootful restart "$SERVICE_NAME"
    else
      systemctl_rootful start "$SERVICE_NAME"
    fi
  fi
}

verify_agent() {
  info "Verifying $SERVICE_NAME status"

  if [[ "$DRY_RUN" == true ]]; then
    if [[ "$ROOTLESS" == true ]]; then
      run_cmd systemctl --user status "$SERVICE_NAME" --no-pager
    else
      run_rootful systemctl status "$SERVICE_NAME" --no-pager
    fi
    return 0
  fi

  if [[ "$ROOTLESS" == true ]]; then
    systemctl --user is-active --quiet "$SERVICE_NAME" || {
      systemctl --user status "$SERVICE_NAME" --no-pager || true
      die "$SERVICE_NAME did not become active"
    }
  else
    if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
      systemctl is-active --quiet "$SERVICE_NAME" || {
        systemctl status "$SERVICE_NAME" --no-pager || true
        die "$SERVICE_NAME did not become active"
      }
    else
      sudo systemctl is-active --quiet "$SERVICE_NAME" || {
        sudo systemctl status "$SERVICE_NAME" --no-pager || true
        die "$SERVICE_NAME did not become active"
      }
    fi
  fi
}

main() {
  parse_args "$@"
  validate_args
  check_prerequisites

  local quadlet_dir
  local quadlet_path

  if [[ "$ROOTLESS" == true ]]; then
    quadlet_dir="$HOME/.config/containers/systemd"
  else
    quadlet_dir="/etc/containers/systemd"
  fi
  quadlet_path="$quadlet_dir/$CONTAINER_FILE"

  info "Installing $AGENT_NAME using image $IMAGE"
  info "Manager URL: $MANAGER_URL"
  if [[ "$ROOTLESS" == true ]]; then
    info "Install mode: rootless user service"
  else
    info "Install mode: rootful system service"
  fi

  write_quadlet "$quadlet_dir" "$quadlet_path"
  enable_linger
  reload_and_start
  verify_agent

  success "$AGENT_NAME is installed and running"
  success "Connected manager URL: $MANAGER_URL"
}

main "$@"
