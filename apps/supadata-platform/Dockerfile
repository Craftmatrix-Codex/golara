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
RUN pnpm --filter studio deploy --prod --legacy --ignore-scripts /srv/apps/studio && \
    cd /srv/apps/studio && \
    find . -mindepth 1 -maxdepth 1 \
      ! -name node_modules ! -name package.json ! -name scripts \
      ! -name instrument.server.mjs ! -name .env \
      -exec rm -rf {} + && \
    cp -a /app/apps/studio/dist ./dist && \
    printf "import('./scripts/serve.js')\n" > server.js && \
    node scripts/smoke-server.mjs

FROM node:22-slim AS production
RUN apt-get update -qq && \
    apt-get install -y --no-install-recommends nginx ca-certificates && \
    rm -rf /var/lib/apt/lists/* && \
    mkdir -p /var/lib/supadata /app/apps/studio
COPY --from=go-build /out/supadata-platform /usr/local/bin/supadata-platform
COPY --from=studio-build /srv/apps/studio /app/apps/studio
COPY apps/supadata-platform/nginx.conf /etc/nginx/nginx.conf
COPY apps/supadata-platform/docker-entrypoint.sh /usr/local/bin/supadata-entrypoint
RUN chmod 0755 /usr/local/bin/supadata-entrypoint /usr/local/bin/supadata-platform

ENV SUPADATA_PORT=8090
ENV PORT=8082
ENV SUPADATA_DATA_DIR=/var/lib/supadata
EXPOSE 8080
VOLUME ["/var/lib/supadata"]
HEALTHCHECK --interval=10s --timeout=5s --retries=5 CMD node -e "fetch('http://127.0.0.1:8080/health').then(r => { if (r.status !== 200) throw new Error(String(r.status)) })"
ENTRYPOINT ["/usr/local/bin/supadata-entrypoint"]
