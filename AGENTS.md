# Agent Guidelines for Podman Manager

## Commit Messages

- No AI references in commit messages
- No co-author trailers for AI agents
- Follow conventional commit format: `type: short description`
- Types: feat, fix, build, docs, chore, refactor, test

## Project Context

Podman Manager is a multi-host Podman container management tool with:

- **Shared Go backend** (`backend/`) — REST API server connecting to remote hosts via SSH
- **Unraid plugin** (`unraid-plugin/`) — PHP/jQuery UI for the Unraid WebGUI (Dynamix framework)
- **Web application** (`webapp/`) — Modern React+Vite standalone web interface

Both frontends consume the same Go backend API at `localhost:18734`.
