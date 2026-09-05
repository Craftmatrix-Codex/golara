# Known Issues

- The Go service is not deployed to production.
- Auth remains partial: email delivery, OAuth, MFA, admin, logout, recovery, and complete GoTrue parity are not implemented.
- REST currently supports only basic parameterized SELECT and INSERT; UPDATE, DELETE, UPSERT, embeds, complete filters, and RLS claim transaction setup are not implemented.
- RPC, Realtime, Storage, metadata/admin APIs, functions, and jobs are not implemented yet.
- The final single-image Studio + Go + NGINX Dockerfile is not ready.
- Reference Supabase compatibility, security, failure, stress, soak, benchmark, and public-route suites are not complete.
- The current Node control plane and existing Supabase runtime remain in place for rollback.
- The existing production project-provisioning PostgreSQL role/password synchronization issue remains a separate unresolved production issue until the Go path replaces or repairs that flow.
