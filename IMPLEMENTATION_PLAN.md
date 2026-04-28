# Podman Manager Implementation Plan

## Goals

Stabilize the current Podman Manager codebase, align documented behavior with implemented behavior, harden the Unraid plugin bridge, and prepare both the standalone app and Unraid plugin for iterative feature work.

## Phase 0: Local environment and verification baseline

Acceptance criteria:

- `node`, `npm`, TypeScript build, ESLint, PHP page lint, Go build/test tooling, and LSP diagnostics are available or limitations are documented.
- Podman CLI or Podman Desktop is available for local container image verification.
- Webapp dependencies install from `package-lock.json`.
- Existing build/lint/test commands are run before feature work begins.

Tasks:

1. Install or enable Go and `gopls` for backend diagnostics.
2. Install TypeScript language server support for editor/LSP diagnostics.
3. Confirm Podman machine availability and use `podman build` for local image verification.
4. Capture baseline results for `npm run build`, `npm run lint`, plugin page lint, backend build/test, dependency audit, and `podman build`.

## Phase 1: Correctness, security, and release hygiene

Acceptance criteria:

- Documented configuration keys load correctly.
- CI/release Go version matches the module/toolchain requirements.
- Fixable npm advisories are resolved without breaking `npm run build` or `npm run lint`.
- Unraid plugin proxy validates CSRF for mutating plugin actions, safely forwards backend methods, and escapes dynamic UI data.
- Release artifacts use consistent version naming and validated `.plg` metadata.

Tasks:

1. Support documented `ssh.strict_host_key_checking` while preserving backward compatibility with `ssh.ssh_strict_host_key_checking`.
2. Align README, example configs, release workflow, and Dockerfile around the same Go toolchain requirement.
3. Run safe npm dependency updates that clear current audit findings.
4. Harden `unraid-plugin/.../include/Events.php`:
   - validate CSRF for backend lifecycle and key-generation actions,
   - support GET, POST, PUT, and DELETE in `api_proxy`,
   - preserve backend response status where possible,
   - quote shell commands safely.
5. Escape dynamic backend data before inserting it into Unraid plugin HTML.
6. Normalize release version handling so `vYYYY.MM.DD` tags do not create mixed `v`/non-`v` package names.

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

## Phase 3: Unraid plugin feature parity

Acceptance criteria:

- Unraid UI exposes the same safe core management capabilities as the backend.
- Plugin refresh behavior can use events or lower-impact polling.
- Settings page validates backend config before restart where practical.

Tasks:

1. Add image list, pull, remove, and prune flows to the Unraid UI.
2. Add container remove with force retry flow.
3. Add streaming logs or event-assisted refresh for active containers.
4. Surface backend health/version/config validation in settings.
5. Keep PHP/jQuery UI thin and namespaced to avoid collisions with Unraid core pages.

## Phase 4: Backend robustness and tests

Acceptance criteria:

- Core backend packages have unit coverage for parsing, config, auth, update reconstruction, and command sanitization.
- Long-lived streams handle permanent failures without unbounded retries.
- CORS/WebSocket origin behavior is configurable and safe by default for standalone deployments.

Tasks:

1. Add tests for config defaults, YAML key compatibility, and validation errors.
2. Add tests for Podman JSON parsing and update-check behavior.
3. Add tests for session/auth behavior and config reload side effects.
4. Make CORS and WebSocket origin policy configurable.
5. Improve standalone update reconstruction so more container runtime options are preserved.
6. Add diagnostics endpoints for host Podman version, socket/API availability, and permission errors.

## Phase 5: CI and release confidence

Acceptance criteria:

- Pull requests and releases fail fast on lint/build/test/security regressions.
- Plugin package and OCI image outputs are reproducible enough for release validation.

Tasks:

1. Add CI jobs for backend test/build, frontend lint/build/audit, and plugin verify.
2. Keep release workflow focused on publishing artifacts; avoid brittle generated-file commit-back where possible.
3. Add release validation for `.plg` URL, SHA256, package name, and GHCR image tag.
4. Document local release dry-run steps using Podman.
