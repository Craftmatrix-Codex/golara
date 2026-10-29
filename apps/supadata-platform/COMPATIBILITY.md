# Compatibility Matrix

Status values are `NOT STARTED`, `PARTIAL`, or `PASS`. No feature is marked `PASS` without executable tests and verification evidence.

| Feature | Status | Evidence |
|---|---|---|
| Health endpoint | PASS | `go test ./...`, `go test -race ./...`, and live binary smoke test |
| Bearer authentication | PASS | Auth unit tests, protected HTTP tests, and live unauthenticated `401` check |
| Project list/current/create/select contract | PASS | Registry persistence tests, HTTP tests, race tests, and live binary smoke test |
| Request-scoped project routing and database binding | PARTIAL | Strict fail-closed project scope, explicit shared/per-project DSN routing, real two-PostgreSQL binary isolation (`alpha.test`/`beta.test`), RLS project claim propagation, Storage/Realtime topic/token boundaries, and project-bound JWT rejection; dynamic per-project database creation and API-key-to-project mapping remain unimplemented |
| Studio control-plane metadata/content contracts | PARTIAL | Native OpenAPI REST schema, vector-bucket/function/log-drain metadata, atomic project-scoped SQL snippet/folder CRUD, CSRF-safe Basic Auth API POSTs, Laravel feature tests, and unified-image round-trip; SQL execution/history and full pg-meta schema introspection remain unimplemented |
| Auth API key gating, settings, signup/login/refresh/logout/admin users | PARTIAL | Unit/HTTP tests plus live PostgreSQL repository round-trip; email delivery, OAuth, MFA, recovery, broader admin/provider APIs, and full GoTrue error parity remain unimplemented |
| JWT validation and Postgres role mapping | PARTIAL | HS256 sign/verify tests, persisted-user lookup, and live RLS claim propagation; asymmetric/JWKS keys, full claim surface, and refresh/session parity remain unimplemented |
| REST SELECT/INSERT/UPSERT/UPDATE/DELETE | PARTIAL | Parameterized SELECT/INSERT/UPSERT/UPDATE/DELETE builders, `in`/`not`/`or` filter slices, API-key/JWT gate, request-claim transaction propagation, RLS integration test, identifier-injection tests, and live PostgreSQL GET integration test; embeds, remaining filter grammar, headers/count/preferences, schema exposure, exact status/error parity, and complete write semantics remain unimplemented |
| RPC | PARTIAL | POST `/rest/v1/rpc/{function}` with validated named PostgreSQL arguments, API-key/JWT/RLS transaction path, and sqlmock handler coverage; overloaded functions, scalar/object return parity, and exact PostgREST errors remain unimplemented |
| Realtime WebSocket protocol | PARTIAL | `/realtime/v1/websocket` API-key handshake, optional JWT validation, Phoenix join/leave/heartbeat replies, public-topic authorization, WebSocket integration tests, and NGINX upgrade routing; broadcast, presence, PostgreSQL changes, tenant authorization, and exact protocol parity remain unimplemented |
| Storage upload/download/list/delete | PARTIAL | API-key authorization, service/user mutation checks, path traversal rejection, size limits, S3-compatible SeaweedFS round-trip, handler tests, and Laravel project-scoped bucket metadata listing with native search/sort/pagination tests; bucket CRUD, signed URLs, policies, resumable uploads, database-backed bucket metadata, and exact Storage error parity remain unimplemented |
| Database metadata/admin API | PARTIAL | Native empty pg-meta entity contract and OpenAPI schema are served; live schema introspection, SQL execution/history, migrations, and full admin parity remain unimplemented |
| Edge functions | PARTIAL | Registry-backed deployment and real Deno invocation pass through the official SDK locally; functions receive a project-scoped public SDK URL and public request URL. Per-function secrets, complete logs, and runtime lifecycle parity remain unverified |
| Supabase client compatibility | PARTIAL | Official `@supabase/supabase-js` 2.118.0 contract package in `compat/`; 13 representative contracts pass locally and on Stable revision 737dd97145. Broader behavioral coverage remains required. |
| Single-image Docker deployment | PARTIAL | Unified Docker build, NGINX/PHP-FPM/Go startup, health, Basic Auth/CSRF boundary, and content round-trip pass locally; Stable 737dd97145 is healthy in Coolify with a verified named data mount, 13 passing live SDK contracts, and live Studio access. Optimization and broader lifecycle validation remain. |

## Versioned SDK evidence

`compat/sdk-inventory.json` is generated from the pinned official SDK declarations and lists 223 public callable surfaces across 20 areas, including Auth passkeys/custom providers and alpha vector/analytics Storage APIs. It is an inventory of signatures, not a behavioral coverage percentage. Its unverified entries remain unverified until individual contract evidence is linked.

The initial executable contracts cover invalid password handling, admin listing/signup/create/delete, password/session/refresh/logout, database CRUD/upsert/single/maybeSingle/HEAD count/range, RPC, singleton mutation rollback, anonymous database privilege rejection, Storage upload/list/download/remove, public Realtime channel lifecycle, and a deployed Deno function. Browser checks cover desktop/mobile sign-in, session-based Studio access, public project metadata, runtime failures, and public-origin destinations. See `compat/README.md` for target configuration and fixture setup.
