![Podman Manager](images/podman-manager.png)

# Podman Manager

Multi-host Podman container management with agent-based architecture

Podman Manager provides a unified dashboard to monitor and control Podman containers across multiple remote hosts. It uses a lightweight containerized agent installed on each managed host, communicating via gRPC bidirectional streaming for real-time operations, multi-user RBAC, and persistent authentication.

## Features

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
curl -sSL https://raw.githubusercontent.com/brdweb/podman-manager/main/agent/install/install.sh | sudo bash -s -- --token YOUR_ENROLLMENT_TOKEN --manager manager.example.com:18735

# Rootless installation
curl -sSL https://raw.githubusercontent.com/brdweb/podman-manager/main/agent/install/install.sh | bash -s -- --token YOUR_ENROLLMENT_TOKEN --manager manager.example.com:18735
```

The installer:
1. Creates a Quadlet `.container` file at `/etc/containers/systemd/podman-agent.container`
2. Mounts the Podman socket (auto-detects rootful or rootless path)
3. Starts the agent as a systemd-managed container
4. The agent connects to the manager and enrolls using the provided token

## Configuration

The backend uses a YAML configuration file to define the API server settings and authentication.

```yaml
# Podman Manager Configuration

server:
  # Port for the REST API server
  port: 18734
  # Bind address: 127.0.0.1 for local-only deployments
  bind: "127.0.0.1"

# Agent gRPC server port
agent:
  port: 18735

# SQLite auth database
auth:
  enabled: true
  db_path: "/etc/podman-manager/auth.db"

# Enable real-time event streaming via WebSocket
enable_events_stream: true

# Optional local authentication (legacy single-user)
local_auth:
  enabled: false
  username: ""
  password_hash: ""
```

### Configuration Sections

- **server**: Defines the API port and bind address. Use `127.0.0.1` if the frontend is on the same machine.
- **agent**: gRPC server port for agent connections (default 18735).
- **auth**: SQLite-backed multi-user authentication.
  - `db_path`: Path to the SQLite database (default `/etc/podman-manager/auth.db`).
- **enable_events_stream**: Enable WebSocket-based real-time container events.
- **local_auth**: Legacy single-user authentication (deprecated, use multi-user auth instead).

## User Roles

| Role | Permissions |
|------|------------|
| **admin** | Full access: manage users, config, hosts, containers, volumes, networks |
| **operator** | Manage containers, volumes, networks, images (no user/config management) |
| **viewer** | Read-only: view containers, images, events, logs |

## Project Structure

```
podman-manager/
├── backend/                     # Go REST API server
│   ├── cmd/podman-manager/      # Entry point
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
podman build -f webapp/Dockerfile -t podman-manager .
podman run --rm -p 8080:80 \
  -v ./webapp/config.yaml:/etc/podman-manager/config.yaml:ro \
  -v ~/.ssh/id_ed25519:/root/.ssh/id_ed25519:ro \
  podman-manager
```

This builds and starts a single container image that runs both the Go backend and nginx-served webapp on port 8080.

## Development

### Prerequisites

- Go 1.26.2+
- Node.js 20+ (for webapp)

### Building

Build the standalone container image:
```bash
podman build -f webapp/Dockerfile -t podman-manager .
```

### Running Tests

```bash
cd backend && go test ./...
cd backend && go vet ./...
```

## Versioning

Podman Manager uses date-based versioning (YYYY.MM.DD format). The version is:

- Embedded in both the manager and agent binaries at build time via `-ldflags`
- Displayed in the webapp header
- Printed with `podman-manager -version` and `podman-agent -version`

## Contributing

Issues and pull requests are welcome. Please ensure any changes follow the project's coding style and include appropriate tests.

## License

GPL-3.0
