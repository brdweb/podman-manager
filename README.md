![Homelab Control](images/homelab-control.png)

# Homelab Control

Homelab Control is the renamed and refactored successor to the original Podman Manager codebase: a desktop-first homelab cockpit that keeps the existing Go/React/RBAC foundation while adding Docker-first control-plane APIs, a PostgreSQL-capable state store, Homepage-style launchpad links, and integration points for Git-backed Compose, metrics, and OpenClaw.

The current implementation keeps the legacy Podman transport available during the transition, but the new `/api/v1/*` surface and UI are shaped around the Homelab Control plan.

## Features

### Homelab Control Foundation
- **Launchpad links** — Homepage-style grouped visual cards with icon slugs, full icon URLs, favicon fallback, admin CRUD, import, and YAML/JSON export
- **State and audit store** — PostgreSQL-ready state storage with SQLite fallback for local development
- **v1 API surface** — overview, inventory, services, stacks, links, actions, action streams, and OpenClaw proxy endpoints
- **Ops cockpit UI** — Overview, Launchpad, Services, Stacks, Wall, and admin link management routes
- **Homepage import** — imports existing `homepage-config/bookmarks.yaml` and link-style `services.yaml` entries while preserving groups and order

### Agent-Based Architecture
- **Containerized agent** — lightweight Podman container installed via Quadlet, zero host dependencies
- **gRPC bidirectional streaming** — real-time commands, logs, and events over persistent connections
- **Reverse connections** — agents connect outbound to the manager, no inbound firewall rules needed
- **Auto-reconnect** — exponential backoff with heartbeat monitoring
- **Token-based enrollment** — secure one-time tokens for agent registration
- **Rootful and rootless** — auto-detects Podman socket, supports both modes

### Multi-User RBAC
- **SQLite-backed auth** — persistent user accounts and sessions survive restarts
- **Three roles** — admin (full access), operator (manage containers), viewer (read-only)
- **Per-endpoint enforcement** — every API route protected by role middleware
- **User management UI** — create users, assign roles, reset passwords from the web interface

### Container Management
- **Multi-host dashboard** — manage containers across unlimited remote Podman hosts
- **Full container lifecycle** — create, start, stop, restart, and remove containers from the UI
- **Multi-step creation wizard** — configure image, networking, storage, and advanced options
- **Management method detection** — identifies Quadlet (systemd), Docker Compose, and standalone containers
- **Quadlet (systemd) support** — proper lifecycle management for Quadlet containers
- **Inline container details** — expand rows for IPs, ports, volumes, networks
- **Container logs** — real-time streaming log viewer with pause/resume and auto-scroll
- **Bulk actions** — checkbox selection with bulk start/stop/restart
- **Sortable columns** — click headers to sort by container name or host

### Volume & Network Management
- **Volume management** — list, create, and delete volumes on any host
- **Network management** — list, create, and delete networks with subnet configuration
- **Host-scoped UI** — each host has its own volumes and networks pages

### Image Management
- **List images** — view all images across all hosts with size and tag information
- **Pull images** — pull new images from any configured registry
- **Remove images** — delete images with force option for in-use images
- **Prune images** — clean up dangling/unused images across all hosts

### Real-Time Events
- **Live event dashboard** — WebSocket-streamed Podman events across all hosts
- **Filter by type and host** — container, image, volume, network events
- **Pause/resume** — control the event stream without disconnecting
- **Auto-reconnect** — resilient connection with exponential backoff

### Configuration & UX
- **CodeMirror YAML editor** — syntax-highlighted config editing in the browser
- **Toast notifications** — success/error/info notifications for all actions
- **Error boundaries** — graceful error handling with reload capability
- **404 page** — friendly "not found" page for unknown routes
- **Hot reload** — configuration changes apply without restart

### CI/CD
- **GitHub Actions** — automated testing, linting, and building on every push/PR
- **Multi-binary releases** — both manager and agent binaries published with GitHub releases
- **Multi-arch Docker** — container images built for multiple architectures

## Architecture

```
                    ┌─────────────────────────────────────────┐
                    │         Go REST API Backend              │
                    │         (localhost:18734)                │
                    ├─────────────────────────────────────────┤
                    │         gRPC Server (port 18735)         │
                    ├──────────┬──────────┬────────────────────┤
                    │  gRPC    │  gRPC    │  gRPC              │
                    │  ◄─────► │  ◄─────► │  ◄─────►           │
                    ▼          ▼          ▼                   │
              host-alpha    host-beta    host-gamma            │
              ┌──────────┐  ┌──────────┐  ┌──────────┐        │
              │  Agent   │  │  Agent   │  │  Agent   │        │
              │ Container│  │ Container│  │ Container│        │
              └──────────┘  └──────────┘  └──────────┘        │
                    └─────────────────────────────────────────┘
                              ▲
                              │
                    ┌─────────┴─────────┐
                    │   React+Vite      │
                    │     Web App       │
                    └───────────────────┘
```

## Installation

### Manager (Standalone)

1. Configure your hosts and place the file at `webapp/config.yaml`.
2. Start the standalone container:
   ```bash
   cd webapp
   docker compose up --build
   ```
3. Open the UI:
   ```bash
   http://localhost:8080
   ```

### Agent (Remote Hosts)

Install the agent on each Podman host you want to manage:

```bash
# Rootful installation
curl -sSL https://raw.githubusercontent.com/brdweb/homelab-control/main/agent/install/install.sh | sudo bash -s -- --token YOUR_ENROLLMENT_TOKEN --manager-url manager.example.com:18735

# Rootless installation
curl -sSL https://raw.githubusercontent.com/brdweb/homelab-control/main/agent/install/install.sh | bash -s -- --token YOUR_ENROLLMENT_TOKEN --manager-url manager.example.com:18735
```

The installer:
1. Creates a Quadlet `.container` file at `/etc/containers/systemd/homelab-control-agent.container`
2. Mounts the Podman socket (auto-detects rootful or rootless path)
3. Starts the agent as a systemd-managed container
4. The agent connects to the manager and enrolls using the provided token

## Configuration

The backend uses a YAML configuration file to define API server settings, SSH defaults, authentication, and managed hosts.

```yaml
# Homelab Control Configuration

server:
  # Port for the REST API server
  port: 18734
  # Bind address: 127.0.0.1 for local-only deployments
  bind: "127.0.0.1"
  # Optional browser origins for a separate standalone webapp
  allowed_origins:
    - "https://homelab-control.example.com"

ssh:
  key_path: "~/.ssh/id_ed25519"
  connect_timeout: "5s"
  keepalive_interval: "30s"
  strict_host_key_checking: "accept-new"

# Snapshot cache TTL for SSH polling
cache_ttl: "3s"

docker:
  # Git-backed Docker Compose desired-state repository
  compose_repo_path: "../homelab-docker"
  # Optional DockMon dashboard URL surfaced by Docker diagnostics
  dockmon_url: "https://dockmon.brdweb.com"

# Optional login for the standalone web application
auth:
  enabled: false
  username: "admin"
  password_hash: ""
  session_ttl: "12h"

# Enable real-time event streaming via WebSocket
enable_events_stream: true

hosts:
  - name: "host-alpha"
    address: "10.0.0.101"
    port: 22
    user: "your-user"
    mode: "rootful"
```

### Configuration Sections

- **server**: Defines the API port, bind address, and optional cross-origin browser clients. Same-host browser requests are always allowed. Add exact `http` or `https` origins to `allowed_origins` when the standalone webapp is served from a different origin. Use `"*"` only on trusted networks.
- **docker**: Git-backed Docker Compose desired-state source. `/api/v1/stacks` reads `stacks/<host>/<stack>/compose.yaml` from `compose_repo_path`; `/api/v1/diagnostics/docker` reports source availability, stack count, hosts, and the configured DockMon URL.
- **ssh**: Shared SSH settings retained for legacy host endpoints, including `strict_host_key_checking` values of `strict`, `accept-new`, or `off`.
- **cache_ttl**: Snapshot cache duration used to reduce SSH polling.
- **auth**: Optional standalone webapp login settings.
- **enable_events_stream**: Enable WebSocket-based real-time container events.
- **hosts**: Docker host inventory for the current control plane, with legacy runtime fields retained until old SSH endpoints are replaced by Docker/DockMon APIs.

## User Roles

| Role | Permissions |
|------|------------|
| **admin** | Full access: manage users, config, hosts, containers, volumes, networks |
| **operator** | Manage containers, volumes, networks, images (no user/config management) |
| **viewer** | Read-only: view containers, images, events, logs |

## Project Structure

```
homelab-control/
├── backend/                     # Go REST API server
│   ├── cmd/homelab-control/      # Entry point
│   ├── internal/api/            # HTTP handlers, router, RBAC middleware
│   ├── internal/agent/          # gRPC server, agent registry, transport bridge
│   ├── internal/auth/           # SQLite user/session store
│   ├── internal/enroll/         # Token-based agent enrollment
│   ├── internal/host/           # Transport abstraction (SSH + Agent)
│   ├── internal/podman/         # Podman client, cache, events
│   ├── internal/config/         # YAML config loading
│   └── configs/                 # Example configuration
├── agent/                       # Containerized host agent
│   ├── cmd/agent/               # Entry point
│   ├── internal/podman/         # Podman REST API client (Unix socket)
│   ├── internal/config/         # Agent configuration
│   ├── internal/quadlet.go      # Quadlet discovery
│   ├── install/                 # Quadlet install scripts
│   └── proto/                   # gRPC protocol definitions
└── webapp/                      # React+Vite standalone web UI
    ├── src/api/                 # Type-safe API client
    ├── src/components/          # Reusable UI components (Toast, ErrorBoundary)
    ├── src/pages/               # Dashboard, containers, volumes, networks, events, users
    ├── src/hooks/               # TanStack Query hooks
    ├── Dockerfile               # Multi-stage production build
    └── docker-compose.yaml      # Dev environment
```

## API Reference

### Authentication
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/auth/session` | Current session info |
| POST | `/api/auth/login` | Login with credentials |
| POST | `/api/auth/logout` | Logout current session |

### Admin (admin only)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/admin/config` | Get current configuration |
| PUT | `/api/admin/config` | Update configuration |

### Users (admin only)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/users` | List all users |
| POST | `/api/users` | Create a new user |
| GET | `/api/users/{id}` | Get user details |
| PUT | `/api/users/{id}` | Update user (role, active status) |
| PUT | `/api/users/{id}/password` | Reset user password |
| DELETE | `/api/users/{id}` | Delete a user |
| GET | `/api/users/me` | Get current user profile |

### Agent Enrollment (admin only)
| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/agent/tokens` | Create enrollment token |
| GET | `/api/agent/tokens` | List active tokens |
| DELETE | `/api/agent/tokens/{id}` | Revoke enrollment token |
| GET | `/api/agent/install.sh` | Download install script |
| GET | `/api/agent/hosts` | List enrolled agent hosts |

### Hosts & Containers (operator+)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/hosts` | List configured hosts with status |
| GET | `/api/hosts/{host}/containers` | List containers on a host |
| POST | `/api/hosts/{host}/containers` | Create a container |
| GET | `/api/hosts/{host}/containers/{id}` | Inspect container details |
| POST | `/api/hosts/{host}/containers/{id}/start` | Start a container |
| POST | `/api/hosts/{host}/containers/{id}/stop` | Stop a container |
| POST | `/api/hosts/{host}/containers/{id}/restart` | Restart a container |
| DELETE | `/api/hosts/{host}/containers/{id}` | Remove a container |
| PUT | `/api/hosts/{host}/containers/{id}` | Update container settings |
| GET | `/api/hosts/{host}/containers/{id}/logs` | Container logs (static) |
| GET | `/api/hosts/{host}/containers/{id}/logs/stream` | Container logs (WebSocket stream) |

### Volumes (operator+)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/hosts/{host}/volumes` | List volumes on a host |
| POST | `/api/hosts/{host}/volumes` | Create a volume |
| DELETE | `/api/hosts/{host}/volumes/{name}` | Remove a volume |

### Networks (operator+)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/hosts/{host}/networks` | List networks on a host |
| POST | `/api/hosts/{host}/networks` | Create a network |
| DELETE | `/api/hosts/{host}/networks/{name}` | Remove a network |

### Images (operator+)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/hosts/{host}/images` | List images on a host |
| POST | `/api/hosts/{host}/images/pull` | Pull an image |
| DELETE | `/api/hosts/{host}/images/{id}` | Remove an image |
| POST | `/api/hosts/{host}/images/prune` | Prune unused images |

### General (viewer+)
| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/health` | Backend health + host connectivity |
| GET | `/api/version` | Backend version |
| GET | `/api/containers` | List all containers across hosts |
| GET | `/api/overview` | Aggregated view of all hosts |
| GET | `/api/events` | WebSocket for real-time container events |

## Web App

### Development

```bash
cd webapp
npm install
npm run dev
```

The dev server starts at `http://localhost:5173` and proxies `/api` requests to the backend at `localhost:18734`.

### Production (Podman)

```bash
podman build -f webapp/Dockerfile -t homelab-control .
podman run --rm -p 8080:80 \
  -v ./webapp/config.yaml:/etc/homelab-control/config.yaml:ro \
  -v ~/.ssh/id_ed25519:/root/.ssh/id_ed25519:ro \
  homelab-control
```

This builds and starts a single container image that runs both the Go backend and nginx-served webapp on port 8080.

## Development

### Prerequisites

- Go 1.26.2+
- Node.js 20+ (for webapp)

### Building

Build the standalone container image:
```bash
podman build -f webapp/Dockerfile -t homelab-control .
```

### Running Tests

```bash
cd backend && go test ./...
cd backend && go vet ./...
```

## Versioning

Homelab Control uses date-based versioning (YYYY.MM.DD format). The version is:

- Embedded in both the manager and agent binaries at build time via `-ldflags`
- Displayed in the webapp header
- Printed with `homelab-control -version` and `homelab-agent -version`

## Contributing

Issues and pull requests are welcome. Please ensure any changes follow the project's coding style and include appropriate tests.

## License

GPL-3.0
