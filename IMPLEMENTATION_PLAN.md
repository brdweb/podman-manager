# Homelab Control Implementation Plan

## Goals

Stabilize the current Homelab Control codebase, align documented behavior with implemented behavior, and prepare the standalone app for iterative feature work.

## Phase 0: Local environment and verification baseline

Acceptance criteria:

- `node`, `npm`, TypeScript build, ESLint, PHP page lint, Go build/test tooling, and LSP diagnostics are available or limitations are documented.
- Docker or another OCI-compatible local builder is available for container image verification.
- Webapp dependencies install from `package-lock.json`.
- Existing build/lint/test commands are run before feature work begins.

Tasks:

1. Install or enable Go and `gopls` for backend diagnostics.
2. Install TypeScript language server support for editor/LSP diagnostics.
3. Confirm local OCI builder availability and use `docker build` or equivalent for image verification.
4. Capture baseline results for `npm run build`, `npm run lint`, backend build/test, dependency audit, and image build.

## Phase 1: Correctness, security, and release hygiene

Acceptance criteria:

- Documented configuration keys load correctly.
- CI/release Go version matches the module/toolchain requirements.
- Fixable npm advisories are resolved without breaking `npm run build` or `npm run lint`.
- Release artifacts use consistent version naming.

Tasks:

1. Support documented `ssh.strict_host_key_checking` while preserving backward compatibility with `ssh.ssh_strict_host_key_checking`.
2. Align README, example configs, release workflow, and Dockerfile around the same Go toolchain requirement.
3. Run safe npm dependency updates that clear current audit findings.
4. Normalize release version handling so `vYYYY.MM.DD` tags do not create mixed `v`/non-`v` package names.

## Phase 2: Standalone webapp parity with documented features

Acceptance criteria:

- Webapp implements or docs explicitly defer advertised standalone features.
- All frontend API calls honor `VITE_API_BASE_URL` consistently.
- Container tables support sorting and bulk actions.
- Log viewer uses the streaming endpoint with pause/resume and auto-scroll.
- Admin config editor provides YAML validation feedback before saving.

Tasks:

1. Route version fetching through the shared API client.
2. Implement live log streaming with reconnect/error handling.
3. Add sortable headers and bulk lifecycle actions to `ContainerTable`.
4. Replace blocking `alert`/`confirm` flows in image management with in-app dialogs/status messages.
5. Upgrade Admin config editing from a plain textarea to a validated YAML editing experience.
6. Remove or reuse dead frontend code such as unused log hooks/components after verification.

## Phase 3: Backend robustness and tests

Acceptance criteria:

- Core backend packages have unit coverage for Docker Compose inventory parsing, config, auth, reload behavior, and command/action validation.
- Docker and DockMon become the primary runtime assumptions; legacy Podman paths are isolated behind compatibility boundaries or explicitly deferred.
- CORS/WebSocket origin behavior is configurable and safe by default for standalone deployments.
- `/api/v1/*` exposes the Git-backed Docker Compose desired state from the `homelab-docker` repository before live DockMon mutations are enabled.

Tasks:

1. Add tests for config defaults, YAML key compatibility, and validation errors.
2. Add tests and implementation for Docker Compose stack inventory parsing from `homelab-docker/stacks/<host>/<stack>/compose.yaml`.
3. Add tests for session/auth behavior and config reload side effects.
4. Make CORS and WebSocket origin policy configurable.
5. Replace Podman-specific update reconstruction with Git-backed Compose/DockMon action records and explicit approval flow.
6. Add diagnostics endpoints for Docker host reachability, DockMon availability, Compose source freshness, and permission errors.
7. Rename user-facing Podman terminology to Docker/Homelab Control while preserving compatibility where needed.

## Phase 4: CI and release confidence

Acceptance criteria:

- Pull requests and releases fail fast on lint/build/test/security regressions.
- OCI image outputs are reproducible enough for release validation.

Tasks:

1. Add CI jobs for backend test/build, frontend lint/build/audit, and release verification.
2. Keep release workflow focused on publishing artifacts; avoid brittle generated-file commit-back where possible.
3. Document local release dry-run steps using Docker or another OCI-compatible builder.
