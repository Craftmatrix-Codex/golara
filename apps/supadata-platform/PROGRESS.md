# Progress

## Current execution state

The Go rewrite is being developed beside the existing Node control plane. The existing production path remains the rollback path until the Go implementation passes the full compatibility and deployment gates.

## Verified

- Go 1.22 module at `apps/supadata-platform`.
- Core configuration loading.
- Constant-time Bearer authentication for the control-plane API.
- Atomic durable project registry.
- Project list/current/create/select HTTP contract.
- Graceful HTTP shutdown.
- `go test ./...` passes.
- `go test -race ./...` passes.
- `go vet ./...` passes.
- Stripped Go binary builds.
- Local binary smoke test verified health, unauthorized access, project creation, selection, and current-project persistence.
- Auth password hashing and verification use bcrypt; plaintext is never stored.
- Auth signup, password token, refresh-token rotation, logout/session revocation, admin user list/delete, API-key gating, disable-signup, GoTrue health/settings response shapes, and persisted-user lookup are covered by executed Go tests.
- PostgreSQL Auth repository round-trip and refresh-token rotation passed against disposable PostgreSQL 16.
- REST has parameterized SELECT/INSERT/PATCH/DELETE builders and API-key/JWT gating; handler tests passed with sqlmock and live PostgreSQL SELECT integration.
- Storage has API-key authorization, service/user mutation checks, safe bucket/object paths, bounded uploads, list/download/delete handlers, and an S3-compatible backend. A disposable SeaweedFS 3.80 put/get/list/delete round-trip passed.
- Realtime has API-key/JWT-gated `/realtime/v1/websocket` handling, Phoenix join/leave/heartbeat replies, public-topic authorization, WebSocket tests, and NGINX upgrade routing. Broadcast, presence, database changes, and exact protocol parity remain.
- A clean BuildKit single-image build passed: native Studio Vite/TanStack build, Go test/race/vet/build stage, NGINX, and runtime entrypoint. Image size reported by Docker is 433,469,391 bytes.
- Image smoke passed through NGINX for `/health`, `/auth/v1/health`, and authenticated `/api/projects`; `nginx -t` passed. The disposable container was removed.

## Active work

Continuing from the verified Auth/REST/Storage/Realtime slices into full PostgREST semantics, RPC, metadata/admin APIs, Functions/jobs, client compatibility, and production-safe deployment gates. The unified image is locally buildable and smoke-tested, but it is not yet a complete Supabase replacement.

## Required next sequence

1. Expand REST to full PostgREST semantics and request-scoped PostgreSQL roles/RLS.
2. Implement RPC and database metadata/admin contracts.
3. Expand Storage/SeaweedFS and implement Realtime/WebSocket, Functions, and jobs.
4. Run differential supabase-js/Laravel/PDO compatibility suites.
5. Run security, fuzz, failure, stress, soak, profiling, optimization, and regression suites.
6. Deploy only an isolated verified canary; preserve Stable and require public NGINX/Studio proof before any cutover.

## Resume commands

```bash
cd /root/supabase/apps/supadata-platform
go test ./...
go test -race ./...
go vet ./...
go build -trimpath -ldflags='-s -w' -o /tmp/supadata-go ./cmd/server
```

## Completion rule

Do not mark the platform complete while any required compatibility matrix entry is `NOT STARTED`, `PARTIAL`, `UNKNOWN`, `UNWIRED`, or `UNTESTED`.
