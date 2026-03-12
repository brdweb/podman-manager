# Podman Manager

Multi-host Podman container management with a shared Go backend, deployable as an Unraid plugin or a standalone web application.

## Overview

Podman Manager provides a unified dashboard to monitor and control Podman containers across multiple remote hosts. It connects to each host via SSH and executes Podman commands, exposing the results through a REST API consumed by either frontend.

### Features

- List all containers across multiple Podman hosts (running and stopped)
- Start, stop, and restart containers
- View container details: IP addresses, port mappings, volume mounts, networks
- View container logs
- Host health monitoring with connectivity status
- Grouped-by-host display with per-host status indicators

### Deployment Options

| Target | Description |
|--------|-------------|
| **Unraid Plugin** | Native Unraid WebGUI tab using PHP/jQuery (Dynamix framework) |
| **Web App** | Modern React+Vite standalone web interface |

Both share the same Go backend binary.

## Architecture

```
                    ┌─────────────────────────────────────┐
                    │         Go REST API Backend          │
                    │         (localhost:18734)             │
                    ├──────────┬──────────┬────────────────┤
                    │  SSH     │  SSH     │  SSH           │
                    ▼          ▼          ▼                │
              host-alpha    host-beta    host-gamma        │
              (rootful)     (rootful)    (rootless)        │
                    └─────────────────────────────────────┘
                         ▲                    ▲
                         │                    │
                  ┌──────┴──────┐    ┌───────┴───────┐
                  │ Unraid      │    │ React+Vite    │
                  │ Plugin UI   │    │ Web App       │
                  │ (PHP/jQuery)│    │ (standalone)  │
                  └─────────────┘    └───────────────┘
```

## Project Structure

```
podman-manager/
├── backend/                 # Go REST API server (shared)
│   ├── cmd/podman-manager/  # Entry point
│   ├── internal/api/        # HTTP handlers + router
│   ├── internal/podman/     # SSH + Podman client
│   ├── internal/config/     # YAML config loading
│   └── configs/             # Example configuration
├── unraid-plugin/           # Unraid plugin files
│   ├── podman-manager.plg   # Plugin installer manifest
│   └── src/                 # Plugin source (PHP, JS, events)
└── webapp/                  # React+Vite standalone web UI
    ├── src/api/             # Type-safe API client
    ├── src/components/      # Reusable UI components
    ├── src/pages/           # Dashboard, host detail views
    ├── Dockerfile           # Multi-stage production build
    └── docker-compose.yaml  # Dev environment
```

## Quick Start

### Build the backend

```bash
cd backend
make build
```

### Configure hosts

```bash
cp backend/configs/config.example.yaml config.yaml
# Edit config.yaml with your host details
```

### Run

```bash
./backend/bin/podman-manager --config config.yaml
```

The API is available at `http://localhost:18734/api/`.

## Unraid Plugin Workflow

### Local testing

Use the repo directly while iterating:

```bash
cd unraid-plugin
make package
```

That builds the `.txz` package for manual testing on Unraid.

### Release / Community Apps preparation

Community Apps expects the plugin to be installed from a raw `.plg` URL, and that `.plg` should point to a versioned release asset.

This repo is set up for that flow:

```bash
cd unraid-plugin
make release VERSION=2026.03.12 GITHUB_REPO=brdweb/podman-manager
```

That does three things:

1. Builds the versioned `.txz` package
2. Computes the package SHA256
3. Generates `unraid-plugin/podman-manager.plg` from `unraid-plugin/podman-manager.plg.in`

On Unraid or Slackware, the plugin package is built with native `makepkg`.
On non-Slackware development machines, the Makefile falls back to a `tar.xz`
archive so local packaging, checksum generation, and manifest generation still work.
For an actual Community Apps release, build the final `.txz` on Unraid/Slackware.

For CA publication later, the intended flow is:

1. Create a tagged GitHub release
2. Upload the generated `.txz` asset
3. Commit the generated `unraid-plugin/podman-manager.plg`
4. Submit the raw `.plg` URL to Community Apps

So the current repo supports both:

- fast local testing now
- proper CA-compatible releases later

## Web App

### Development

```bash
cd webapp
npm install
npm run dev
```

The dev server starts at `http://localhost:5173` and proxies `/api` requests to the backend at `localhost:18734`.

### Production (Docker)

```bash
cd webapp
docker compose up --build
```

This starts both the Go backend and the webapp behind nginx on port 8080.

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Backend health + host connectivity |
| GET | `/api/hosts` | List configured hosts with status |
| GET | `/api/hosts/{host}/containers` | List containers on a host |
| GET | `/api/hosts/{host}/containers/{id}` | Inspect container details |
| POST | `/api/hosts/{host}/containers/{id}/start` | Start a container |
| POST | `/api/hosts/{host}/containers/{id}/stop` | Stop a container |
| POST | `/api/hosts/{host}/containers/{id}/restart` | Restart a container |
| GET | `/api/hosts/{host}/containers/{id}/logs` | Container logs |
| GET | `/api/overview` | Aggregated view of all hosts |

## License

GNU General Public License v3.0
