#!/bin/sh
set -eu

data_dir="${SUPADATA_DATA_DIR:-/var/lib/supadata}"
mkdir -p "$data_dir/sessions" "$data_dir/cache"
chown -R www-data:www-data "$data_dir" 2>/dev/null || true
chmod 2770 "$data_dir" 2>/dev/null || true
chmod 2770 "$data_dir/sessions" 2>/dev/null || true
chmod 2770 "$data_dir/cache" 2>/dev/null || true

if [ "${DB_CONNECTION:-sqlite}" = "sqlite" ]; then
  studio_database="${DB_DATABASE:-$data_dir/studio.sqlite}"
  if [ "$studio_database" != ":memory:" ]; then
    mkdir -p "$(dirname "$studio_database")"
    touch "$studio_database"
    chown www-data:www-data "$studio_database"
    chmod 0660 "$studio_database"
  fi
fi

if [ "${SUPADATA_RUN_MIGRATIONS:-true}" = "true" ]; then
  cd /var/www/studio
  php artisan migrate --force --no-interaction
  chown -R www-data:www-data storage bootstrap/cache "$data_dir" 2>/dev/null || true
fi

/usr/local/bin/supadata-platform &
go_pid=$!

php-fpm -F &
php_pid=$!

cleanup() {
  kill "$go_pid" "$php_pid" "${nginx_pid:-}" 2>/dev/null || true
  wait "$go_pid" 2>/dev/null || true
  wait "$php_pid" 2>/dev/null || true
  if [ -n "${nginx_pid:-}" ]; then wait "$nginx_pid" 2>/dev/null || true; fi
}
trap cleanup INT TERM EXIT

nginx -g 'daemon off;' &
nginx_pid=$!

while kill -0 "$go_pid" 2>/dev/null && kill -0 "$php_pid" 2>/dev/null && kill -0 "$nginx_pid" 2>/dev/null; do
  sleep 1
done

exit 1
