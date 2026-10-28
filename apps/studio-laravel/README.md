# Laravel Studio server

This application replaces the Supabase Studio Next.js/TanStack server runtime with Laravel while retaining the existing React Studio UI.

## Runtime boundary

- `apps/studio` remains the source of truth for the native React/Vite Studio UI.
- `scripts/sync-studio-client.sh` builds the existing Studio client with `SUPADATA_STUDIO_BUILDING=1` and copies only `dist/client` into Laravel `public/`.
- Laravel serves the static React shell and owns Studio control-plane routes under `/api/platform/*`, `/api/v1/*`, and Studio utility routes under `/api/*`.
- Go remains authoritative for Supabase client/data-plane routes: Auth, REST/RPC, Storage, Functions, and Phoenix-compatible Realtime.
- Nginx serves hashed assets, forwards Laravel SPA/API requests to PHP-FPM, and proxies Go data-plane routes to the Go process.
- Blade and Livewire are not used. The default Laravel Blade welcome view has been removed.

## Local verification

```bash
# Build the existing React/Vite Studio client into Laravel public assets
apps/studio-laravel/scripts/sync-studio-client.sh

# Run Laravel tests through the pinned PHP container
docker run --rm -v "$PWD:/app" -w /app/apps/studio-laravel php:8.4-cli php artisan test

# Build the unified Go + Laravel/PHP-FPM + Nginx image
docker buildx build --load -t supadata-platform:laravel-local -f apps/supadata-platform/Dockerfile .
```

The image build runs Go unit/race/vet/build gates and the Studio Vite build. The Laravel feature suite covers authentication, project contracts, pg-meta/content empty-state contracts, Storage/Functions/log surfaces, settings, bootstrap metadata, analytics, and secret-free configuration responses.

## Authentication

Browser visits to Studio routes redirect to `/sign-in` when unauthenticated. The sign-in page verifies the deployment credentials configured by `STUDIO_AUTH_USERNAME` and `STUDIO_AUTH_PASSWORD`, then establishes a Laravel session. The production image uses encrypted cookie-backed sessions and file-backed cache for sign-in throttling; set a stable `APP_KEY` so sessions remain valid across container restarts. Session cookies are secure by default in production. Existing HTTP Basic Auth clients and the optional `SUPADATA_CONTROL_PLANE_TOKEN` bearer token remain supported for API access. Secrets are never returned by project metadata adapters; API-key responses remain empty until project-scoped key storage is connected to the Go registry.

## Current compatibility status

The initial migration provides successful native empty-state contracts for the Studio surfaces that previously produced 404/500 responses. PostgreSQL-backed pg-meta execution, persisted project provisioning, Logflare/analytics data, API-key issuance, Edge Function management, and project-specific secret/key storage remain follow-up implementation work. The UI is intentionally retained unchanged so those contracts can be filled without another frontend rewrite.
