# Known Issues

- The Go service is not deployed to production.
- Auth remains partial: email delivery, OAuth, MFA, admin, logout, recovery, and complete GoTrue parity are not implemented.
- REST currently supports basic parameterized SELECT/INSERT/UPDATE/DELETE/RPC, API-key/JWT checks, and authenticated RLS claim transactions; upsert, embeds, complete filters, headers, and exact PostgREST parity remain.
- Realtime, Storage, metadata/admin APIs, functions, and jobs are not implemented yet.
- The unified multi-stage Studio + Go + NGINX Dockerfile builds and has local NGINX smoke coverage; production deployment and public compatibility are not proven.
- The current Node control plane and existing Supabase runtime remain in place for rollback.
- The existing production project-provisioning PostgreSQL role/password synchronization issue remains a separate unresolved production issue until the Go path replaces or repairs that flow.
