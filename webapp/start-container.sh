#!/bin/sh

set -eu

CONFIG_PATH="${PODMAN_MANAGER_CONFIG:-/etc/podman-manager/config.yaml}"

backend_pid=""
nginx_pid=""

cleanup() {
  if [ -n "$backend_pid" ]; then
    kill "$backend_pid" 2>/dev/null || true
  fi
  if [ -n "$nginx_pid" ]; then
    kill "$nginx_pid" 2>/dev/null || true
  fi
}

trap cleanup INT TERM

/usr/local/bin/podman-manager --config "$CONFIG_PATH" &
backend_pid=$!

nginx -g 'daemon off;' &
nginx_pid=$!

while true; do
  if ! kill -0 "$backend_pid" 2>/dev/null; then
    wait "$backend_pid"
    status=$?
    kill "$nginx_pid" 2>/dev/null || true
    wait "$nginx_pid" 2>/dev/null || true
    exit "$status"
  fi

  if ! kill -0 "$nginx_pid" 2>/dev/null; then
    wait "$nginx_pid"
    status=$?
    kill "$backend_pid" 2>/dev/null || true
    wait "$backend_pid" 2>/dev/null || true
    exit "$status"
  fi

  sleep 1
done
