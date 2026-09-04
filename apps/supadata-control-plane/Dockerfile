FROM node:22-bookworm-slim

# The registry provisions project stacks through the Docker socket. The
# docker-compose v1 executable is intentional: it is the default command used
# by src/server.mjs and remains available alongside the Docker CLI.
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates curl docker.io docker-compose \
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
ENV SUPADATA_COMPOSE_COMMAND=docker-compose
ENV PORT=8090
ENV SUPADATA_DATA_DIR=/var/lib/supadata
VOLUME ["/var/lib/supadata"]
EXPOSE 8090
HEALTHCHECK --interval=10s --timeout=5s --retries=3 CMD curl --fail --silent http://127.0.0.1:8090/health || exit 1

CMD ["node", "src/server.mjs"]
