# Deployment

Stable is deployed through `https://dev.craftmatrix.org` at `https://golara.craftmatrix.org`. Runtime revision `d123a7d3eb` is healthy as of 2026-10-29. The previous Coolify deployment remains available for rollback; preserve the named data volume when rolling back.

The production target is one reproducible multi-stage Docker build containing:

- Studio frontend build artifacts.
- A statically linked Go platform binary.
- Laravel/PHP-FPM control plane and session authentication.
- Deno function runtime.
- Minimal NGINX runtime configuration.
- No Node or Go toolchain in the final runtime image.

Deploy each patch after local compatibility, security, Docker, and browser gates pass, then verify the exact image revision, health, named mount, official SDK contracts, and browser flows through the public HTTPS origin. The 2026-10-29 checkpoint passed 15 API/SDK contracts and 18 browser checks both locally and on Stable. Passing these gates covers the tested behaviors; it does not establish full Supabase parity.

## Stable runtime settings

- Keep `/var/lib/supadata` mounted from named volume `poslsmflcfylqkhdl131pj6o-golara-stable-data`. Registry, function content, Studio sessions, and authentication permits must survive container replacement. A Dockerfile `VOLUME` declaration alone is insufficient.
- Use `SESSION_DRIVER=file` and `CACHE_STORE=file`. Studio authentication permits additionally use the dedicated `STUDIO_SESSION_CACHE_STORE`, defaulting to `file`, independently of the general cache.
- Set public application/API origins to `https://golara.craftmatrix.org`. The default project's SDK URL is `https://golara.craftmatrix.org/api/projects/default`; browser settings and function URLs must advertise the actual public project path.
- Keep runtime credentials in Coolify. Set build arguments to **Managed manually in Dockerfile**; this Dockerfile needs no runtime-secret build arguments. Inspect selected image-history metadata without exposing credentials.
- Stable S3 uses the private authenticated gateway `golara-s3-stable:8333`, backed by the retained SeaweedFS data volume. Do not recreate storage data during an application rollout.
- Database API roles must match `scripts/bootstrap-api-roles.sql`: trusted `service_role` bypasses RLS; `anon` and `authenticated` do not. Inspect existing grants before applying role changes.
- `SUPADATA_DATABASE_POOL_SIZE` defaults to 10 per unique database binding. Change it only against measured database capacity and load.

Email delivery, SMS, and OAuth configuration/testing are deferred by the user. Other compatibility work continues independently.

## Required project database configuration

The Go binary fails closed when a registry contains projects without database bindings.

- `SUPADATA_REQUIRE_PROJECT_SCOPE=true` is the secure default and should remain enabled.
- `SUPADATA_DATABASE_MODE=per-project` selects isolated database connections.
- `SUPADATA_PROJECT_DATABASE_URLS` is a JSON object keyed by project ID, for example `{ "alpha": "[REDACTED]", "beta": "[REDACTED]" }`. Values must be supplied through the deployment secret manager and must not be committed.
- `SUPADATA_DATABASE_MODE=shared` is supported for controlled testing and maps every registered project to `DATABASE_URL`; it does not provide database-level isolation.
- The current runtime verifies and pings every configured project database at startup, registers it in the request router, and refuses startup when a binding is missing or unreachable.

Dynamic PostgreSQL database/role creation is not enabled. New per-project resources require an external provisioning workflow that can issue runtime credentials through a secret manager; the server intentionally does not generate or persist those credentials in the registry.
