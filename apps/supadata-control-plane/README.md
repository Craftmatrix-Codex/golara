# Supadata control plane

API-only lifecycle service for the multi-project Supabase fork. The user-facing project selector lives inside Supabase Studio; this service owns project state and orchestration.

## Per-project contract

Creating a project runs the repository's `docker/setup.sh` into an isolated project directory, generates fresh secrets, and materializes the complete Supabase Compose graph:

- Studio, Envoy gateway, Auth, PostgREST, Realtime, Storage, imgproxy, postgres-meta, Edge Functions, PostgreSQL, and Supavisor
- isolated Compose project/network, database and pooler ports, volumes, credentials, JWT keys, and gateway URL
- shared SeaweedFS S3-compatible service with persistent data and one bucket per project (`supadata-<project-id>`)

The default database mode uses one PostgreSQL container per project for maximum isolation. For the optimized single-host deployment, set `SUPADATA_DATABASE_MODE=shared`. This creates one shared `supadata-postgres` container and one database per project while keeping each project's Supabase service group, credentials, and routing isolated.

For storage, the default is `SUPADATA_STORAGE_MODE=shared`: one SeaweedFS container/network with persistent data and a separate bucket per project. Set `SUPADATA_STORAGE_MODE=isolated` only when a separate SeaweedFS service per project is required.

## API

- `GET /health`
- `GET /api/projects`
- `GET /api/projects/current`
- `POST /api/projects` with `{ "name": "Analytics", "id": "analytics" }`
- `POST /api/projects/:id/provision`
- `POST /api/projects/:id/select`
- `DELETE /api/projects/:id`

`gatewayPort` and `publicUrl` are returned as non-secret project metadata. Secrets are never returned by the API.

## Local verification

```sh
pnpm --filter supadata-control-plane test
```

A generated project can be validated or started with:

```sh
docker compose --project-name supadata-analytics \
  --env-file .supadata/projects/analytics/.env \
  --file .supadata/projects/analytics/compose.yml config --quiet
```

The full per-project stacks are independently deployable. Studio-side dynamic backend context and production reverse-proxy hostname routing remain the next integration layer; a visible selector alone must not be treated as proof of backend switching.

## Container deployment

Build from the repository root (the image includes `docker/setup.sh` and its
Compose assets):

```sh
docker build -f apps/supadata-control-plane/Dockerfile \
  -t supadata-control-plane:local .
```

Run the control plane with access to the host Docker daemon and a persistent
data volume. The Docker socket grants the container control over the host
daemon; restrict access to this image and use an authenticated reverse proxy
before exposing the API outside a trusted network.

```sh
docker run --detach --name supadata-control-plane \
  --restart unless-stopped \
  --publish 8090:8090 \
  --volume /var/run/docker.sock:/var/run/docker.sock \
  --volume supadata-control-plane-data:/var/lib/supadata \
  supadata-control-plane:local
```

The registry generates secrets into the persistent data volume and never
returns them from its API. Do not commit or log the volume contents. Set
`SUPADATA_ALLOWED_ORIGIN`, `SUPADATA_DATABASE_MODE`, and
`SUPADATA_STORAGE_MODE` with `--env`/an external deployment secret mechanism
as appropriate for the host. No secrets are baked into the image.
