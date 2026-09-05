# Security

Security is a release blocker. The Go implementation must validate identity and authorization server-side, use constant-time secret comparison, avoid secret logging or registry responses, enforce project isolation, and treat malformed input as untrusted.

Required gates include authentication and authorization bypass tests, JWT validation tests, RLS isolation tests, SQL/path/command injection tests, upload validation, WebSocket authorization, rate-limit behavior, secret-safe logs, dependency review, container hardening, and race detection.

No production credential values belong in this repository or its documentation.
