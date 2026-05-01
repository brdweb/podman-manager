# Podman Manager Agent Install

This directory contains the Quadlet templates and `curl | bash` installer for running the Podman Manager agent as a Podman container managed by systemd.

## Prerequisites

- Linux host with systemd
- Podman installed and configured
- `systemctl` available
- Rootful install: root access or passwordless/interactive `sudo`
- Rootless install: a non-root user with rootless Podman configured, plus `loginctl` for enabling linger

## Usage

Rootful install, using the default agent image:

```bash
curl -sL https://manager:18734/api/agent/install.sh | bash -s -- \
  --manager-url https://manager:18735 \
  --token abc123
```

Rootless install for the current user:

```bash
curl -sL https://manager:18734/api/agent/install.sh | bash -s -- \
  --manager-url https://manager:18735 \
  --token abc123 \
  --rootless
```

Install with a custom image:

```bash
curl -sL https://manager:18734/api/agent/install.sh | bash -s -- \
  --manager-url https://manager:18735 \
  --token abc123 \
  --image registry.example.com/podman-manager/agent:v1.2.3
```

Preview the install without writing files or starting services:

```bash
curl -sL https://manager:18734/api/agent/install.sh | bash -s -- \
  --manager-url https://manager:18735 \
  --token abc123 \
  --dry-run
```

## Rootful vs rootless

Rootful mode installs the Quadlet file at `/etc/containers/systemd/podman-manager-agent.container` and manages `podman-manager-agent.service` through the system systemd instance. It mounts the rootful Podman socket from `/run/podman/podman.sock`.

Rootless mode installs the Quadlet file at `$HOME/.config/containers/systemd/podman-manager-agent.container` and manages `podman-manager-agent.service` through `systemctl --user`. It mounts the rootless Podman socket from `%t/podman/podman.sock` and enables systemd linger so the user service can start on boot without an active login session.

Use rootful mode when the agent should manage rootful Podman containers. Use rootless mode when the agent should manage containers owned by the current user.

## Files

- `quadlet-rootful.container` - template for rootful system installs
- `quadlet-rootless.container` - template for rootless user installs
- `install.sh` - installer that renders the appropriate template and starts the systemd service

The installer writes the rendered Quadlet as `podman-manager-agent.container` and starts `podman-manager-agent.service`.

## Updating

Run the same install command again with the new manager URL, token, or image. The installer detects an existing Quadlet file and offers to update it when run interactively. In non-interactive mode, such as `curl | bash`, it updates the existing installation automatically.

## Uninstall

Rootful uninstall:

```bash
sudo systemctl disable --now podman-manager-agent.service
sudo rm -f /etc/containers/systemd/podman-manager-agent.container
sudo systemctl daemon-reload
```

Rootless uninstall:

```bash
systemctl --user disable --now podman-manager-agent.service
rm -f "$HOME/.config/containers/systemd/podman-manager-agent.container"
systemctl --user daemon-reload
```

Optional rootless cleanup if the user no longer needs lingering services:

```bash
loginctl disable-linger "$USER"
```

## Troubleshooting

Check service status:

```bash
sudo systemctl status podman-manager-agent.service --no-pager
```

For rootless installs:

```bash
systemctl --user status podman-manager-agent.service --no-pager
```

View logs:

```bash
sudo journalctl -u podman-manager-agent.service -f
```

For rootless installs:

```bash
journalctl --user -u podman-manager-agent.service -f
```

Common issues:

- `podman: command not found`: install Podman before running the installer.
- `systemctl --user` cannot connect to the bus: log in as the target user, ensure `XDG_RUNTIME_DIR` is set, and retry the rootless install.
- Agent service starts and then exits: check the manager URL, token, and Podman socket path in the rendered Quadlet file.
- Rootless agent does not start after reboot: confirm linger is enabled with `loginctl show-user "$USER" -p Linger`.
