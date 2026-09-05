#!/bin/sh
set -eu

mkdir -p "${SUPADATA_DATA_DIR:-/var/lib/supadata}"

/usr/local/bin/supadata-platform &
go_pid=$!

(
  cd /app/apps/studio
  exec node server.js
) &
studio_pid=$!

cleanup() {
  kill "$go_pid" "$studio_pid" 2>/dev/null || true
  wait "$go_pid" 2>/dev/null || true
  wait "$studio_pid" 2>/dev/null || true
}
trap cleanup INT TERM EXIT

nginx -g 'daemon off;' &
nginx_pid=$!

while kill -0 "$go_pid" 2>/dev/null && kill -0 "$studio_pid" 2>/dev/null && kill -0 "$nginx_pid" 2>/dev/null; do
  sleep 1
done

exit 1
