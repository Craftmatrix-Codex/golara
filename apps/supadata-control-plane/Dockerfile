FROM node:22-bookworm-slim

# Compose v2 is required for current Docker image metadata.
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl docker.io \
  && mkdir -p /usr/local/lib/docker/cli-plugins \
  && curl -fsSL https://github.com/docker/compose/releases/download/v2.39.2/docker-compose-linux-x86_64 -o /usr/local/lib/docker/cli-plugins/docker-compose \
  && chmod +x /usr/local/lib/docker/cli-plugins/docker-compose \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app
# Build with the repository root as context so setup.sh and its compose
# templates remain available to the control plane at runtime.
COPY docker /app/docker
RUN printf '#!/bin/sh\nexec docker compose "$@"\n' > /usr/local/bin/supadata-compose \
    && chmod +x /usr/local/bin/supadata-compose

COPY apps/supadata-control-plane /app/apps/supadata-control-plane

WORKDIR /app/apps/supadata-control-plane
ENV NODE_ENV=production
ENV SUPADATA_COMPOSE_COMMAND=supadata-compose
ENV PORT=8090
ENV SUPADATA_DATA_DIR=/var/lib/supadata
VOLUME ["/var/lib/supadata"]
EXPOSE 8090
HEALTHCHECK --interval=10s --timeout=5s --retries=3 CMD curl --fail --silent http://127.0.0.1:8090/health || exit 1

CMD ["node", "src/server.mjs"]
