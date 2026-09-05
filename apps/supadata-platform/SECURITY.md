# Security

Security is a release blocker. The Go implementation validates identity and authorization server-side, uses constant-time secret comparison, avoids secret logging or registry responses, enforces project-ID/SQL-identifier validation, and treats malformed input as untrusted.

Executed checks:

- Bearer token tests cover missing, malformed, wrong, and valid credentials.
- HS256 tests cover tampering, wrong secrets, non-HS256 algorithms, expiry, issuer/audience validation, and a 10-second fuzz run with 178,314 executions.
- REST tests cover API-key gating, invalid bearer rejection, identifier injection rejection, parameterized values, and a 10-second query-builder fuzz run with 165,004 executions.
- `go test -race ./...` and `go vet ./...` pass.
- Registry tests cover project-ID validation, atomic writes, and secret-free project metadata.
- Docker image smoke verification passed through NGINX; no secrets were included in the image build command or report.

Remaining release gates: request-scoped PostgreSQL roles/RLS differential tests, SQL/RPC authorization, Storage path/upload abuse, WebSocket authorization, rate limits/resource exhaustion, dependency vulnerability review, full failure/fuzz/stress/soak suites, and production canary verification. No production credential values belong in this repository or its documentation.
