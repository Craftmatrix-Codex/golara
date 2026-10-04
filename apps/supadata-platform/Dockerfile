# syntax=docker/dockerfile:1.7

# The final image contains the native Studio runtime, the Go platform, and
# NGINX. Build context is the repository root.

FROM golang:1.22-bookworm AS go-build
WORKDIR /src/apps/supadata-platform
COPY apps/supadata-platform/go.mod apps/supadata-platform/go.sum ./
RUN GOTOOLCHAIN=local go mod download
COPY apps/supadata-platform/ ./
RUN GOTOOLCHAIN=local go test ./... && \
    GOTOOLCHAIN=local go test -race ./... && \
    GOTOOLCHAIN=local go vet ./... && \
    CGO_ENABLED=0 GOTOOLCHAIN=local go build -trimpath -buildvcs=false -ldflags='-s -w' -o /out/supadata-platform ./cmd/server

FROM node:22-slim AS studio-base
ENV PNPM_HOME=/pnpm
ENV PATH=$PNPM_HOME:$PATH
RUN apt-get update -qq && \
    apt-get install -y --no-install-recommends ca-certificates git python3 build-essential && \
    rm -rf /var/lib/apt/lists/* && \
    npm install -g pnpm@11.13.1
WORKDIR /app

FROM studio-base AS studio-turbo
COPY . .
RUN pnpm dlx turbo@2.9.14 prune studio --docker

FROM studio-base AS studio-deps
COPY --from=studio-turbo /app/out/json ./
COPY --from=studio-turbo /app/out/pnpm-lock.yaml ./
COPY patches/ ./patches/
RUN pnpm install --frozen-lockfile

FROM studio-deps AS studio-build
COPY --from=studio-turbo /app/out/full ./
ENV SUPADATA_STUDIO_BUILDING=1
RUN NODE_OPTIONS=--max-old-space-size=4096 pnpm --filter studio run build:tanstack

FROM composer:2 AS laravel-deps
WORKDIR /app
COPY apps/studio-laravel/composer.json apps/studio-laravel/composer.lock ./
RUN composer install --no-dev --no-interaction --prefer-dist --no-progress --optimize-autoloader --no-scripts

FROM laravel-deps AS laravel-build
COPY apps/studio-laravel/ ./
COPY --from=laravel-deps /app/vendor ./vendor
RUN rm -f bootstrap/cache/*.php && \
    composer dump-autoload --no-dev --optimize --no-scripts
COPY --from=studio-build /app/apps/studio/dist/client /tmp/studio-client
RUN STUDIO_CLIENT_SOURCE=/tmp/studio-client ./scripts/sync-studio-client.sh

FROM denoland/deno:2.5.3 AS deno-runtime

FROM php:8.4-fpm-bookworm AS production
RUN apt-get update -qq && \
    apt-get install -y --no-install-recommends nginx ca-certificates curl libonig-dev libpq-dev libsqlite3-dev && \
    docker-php-ext-install mbstring pdo_pgsql pdo_sqlite opcache && \
    rm -rf /var/lib/apt/lists/* && \
    printf '\n; Pass Coolify runtime variables through to Laravel workers.\nclear_env = no\n' >> /usr/local/etc/php-fpm.d/www.conf && \
    mkdir -p /var/lib/supadata/content /var/www/studio && \
    chown -R www-data:www-data /var/lib/supadata
COPY --from=go-build /out/supadata-platform /usr/local/bin/supadata-platform
COPY --from=deno-runtime /usr/bin/deno /usr/bin/deno
COPY --from=laravel-build /app /var/www/studio
COPY apps/supadata-platform/internal/functions/runtime/function-wrapper.ts /usr/local/share/supadata/function-wrapper.ts
COPY apps/supadata-platform/nginx.conf /etc/nginx/nginx.conf
COPY apps/supadata-platform/docker-entrypoint.sh /usr/local/bin/supadata-entrypoint
RUN chown -R www-data:www-data /var/www/studio/storage /var/www/studio/bootstrap/cache && \
    chmod 0755 /usr/local/bin/supadata-entrypoint /usr/local/bin/supadata-platform

ENV SUPADATA_PORT=8090
ENV APP_ENV=production
ENV APP_DEBUG=false
ENV APP_URL=http://localhost
ENV LOG_CHANNEL=stderr
ENV SESSION_DRIVER=array
ENV CACHE_STORE=array
ENV QUEUE_CONNECTION=sync
ENV SUPADATA_DATA_DIR=/var/lib/supadata
EXPOSE 8080
VOLUME ["/var/lib/supadata"]
HEALTHCHECK --interval=10s --timeout=5s --retries=5 CMD curl -fsS http://127.0.0.1:8080/health
ENTRYPOINT ["/usr/local/bin/supadata-entrypoint"]
