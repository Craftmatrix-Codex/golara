# Compatibility Matrix

Status values are `NOT STARTED`, `PARTIAL`, or `PASS`. No feature is marked `PASS` without executable tests and verification evidence.

| Feature | Status | Evidence |
|---|---|---|
| Health endpoint | PASS | `go test ./...`, `go test -race ./...`, and live binary smoke test |
| Bearer authentication | PASS | Auth unit tests, protected HTTP tests, and live unauthenticated `401` check |
| Project list/current/create/select contract | PASS | Registry persistence tests, HTTP tests, race tests, and live binary smoke test |
| Auth API key gating, settings, signup/login/refresh/logout | PARTIAL | Unit/HTTP tests plus live PostgreSQL repository round-trip; email delivery, OAuth, MFA, admin, recovery, and full GoTrue error parity remain unimplemented |
| JWT validation and Postgres role mapping | PARTIAL | HS256 sign/verify tests, persisted-user lookup, and live RLS claim propagation; asymmetric/JWKS keys, full claim surface, and refresh/session parity remain unimplemented |
| REST SELECT/INSERT/UPDATE/DELETE | PARTIAL | Parameterized SELECT/INSERT/UPDATE/DELETE builders, API-key/JWT gate, request-claim transaction propagation, RLS integration test, identifier-injection tests, and live PostgreSQL GET integration test; upsert, embeds...[truncated]
| RPC | PARTIAL | POST `/rest/v1/rpc/{function}` with validated named PostgreSQL arguments, API-key/JWT/RLS transaction path, and sqlmock handler coverage; overloaded functions, scalar/object return parity, and exact PostgREST errors remain unimplemented |
| Realtime WebSocket protocol | PARTIAL | `/realtime/v1/websocket` API-key handshake, optional JWT validation, Phoenix join/leave/heartbeat replies, public-topic authorization, WebSocket integration tests, and NGINX upgrade routing; broadcast, presence, PostgreSQL changes, tenant authorization, and exact protocol parity remain unimplemented |
| Storage upload/download/list/delete | PARTIAL | API-key authorization, service/user mutation checks, path traversal rejection, size limits, S3-compatible SeaweedFS round-trip, and handler tests; bucket metadata, signed URLs, policies, resumable uploads, and exact Storage error parity remain unimplemented |
| Database metadata/admin API | NOT STARTED | — |
| Edge functions | NOT STARTED | — |
| Supabase client compatibility | NOT STARTED | — |
| Single-image Docker deployment | NOT STARTED | — |
