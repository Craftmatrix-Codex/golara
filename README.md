<p align="center">
  <img src="assets/golara-mascot.png" alt="GoLara mascot" width="520" />
</p>

<h1 align="center">GoLara</h1>

<p align="center">
  A fast, project-scoped backend platform with a Supabase-compatible SDK.
</p>

<p align="center">
  <a href="https://github.com/Craftmatrix-Codex/supabase/tree/canary">Canary</a>
  ·
  <a href="https://github.com/Craftmatrix-Codex/supabase/issues">Issues</a>
  ·
  <a href="DEVELOPERS.md">Developers</a>
</p>

## What is GoLara?

GoLara is a developer platform for building applications on PostgreSQL. It provides a Go data plane, a React-based dashboard, project isolation, authentication boundaries, generated APIs, GraphQL, storage, realtime adapters, and a control plane for managing projects.

GoLara is designed to work with the **Supabase-compatible SDK ecosystem**, allowing existing client applications to connect through familiar API conventions while GoLara owns the runtime, routing, deployment, and project infrastructure.

## Platform

- [x] PostgreSQL-backed projects
- [x] Project-scoped database routing
- [x] Authentication and authorization boundaries
- [x] REST and RPC API foundation
- [x] GraphQL through native PostgreSQL resolution
- [x] React/TanStack dashboard
- [x] Project and database metadata APIs
- [x] Storage compatibility foundation
- [x] Realtime compatibility foundation
- [ ] Complete Storage API parity
- [ ] Complete Realtime protocol parity
- [ ] Edge Functions runtime
- [ ] Full provisioning automation

## Supabase-compatible SDK

GoLara exposes familiar client-facing contracts so applications can use a Supabase-compatible SDK without replacing their data-access patterns.

Supported foundations include:

- API-key and JWT authentication headers
- Project-scoped REST requests
- PostgreSQL RPC calls
- GraphQL requests at `/graphql/v1`
- Variables, operation names, and extensions for GraphQL
- Database roles and JWT claim propagation
- PostgreSQL Row Level Security enforcement

Example GraphQL request:

```bash
curl -X POST "https://go-alpha.craftmatrix.org/graphql/v1" \
  -H "Content-Type: application/json" \
  -H "apikey: $GOLARA_ANON_KEY" \
  --data '{"query":"{ __typename }"}'
```

Expected response:

```json
{"data":{"__typename":"Query"}}
```

GraphQL is resolved by PostgreSQL through the `pg_graphql` extension. GoLara provides the authenticated and project-scoped HTTP boundary; it does not emulate a static schema.

## Architecture

```text
Supabase-compatible SDK / browser
              |
              v
         NGINX / HTTPS
          |          \
          |           \-- GoLara dashboard and control plane
          v
       Go data plane
          |
          +-- Authentication and project scope
          +-- REST / RPC / GraphQL
          +-- Storage and Realtime adapters
          |
          v
       PostgreSQL
          |
          \-- pg_graphql / graphql.resolve(...)
```

## Environments

| Environment | Purpose | Endpoint |
|---|---|---|
| Canary | Active development and acceptance | `https://go-alpha.craftmatrix.org` |
| Stable | Promoted production revision | `https://go-stable.craftmatrix.org` |

Canary is the working branch. Stable is changed only after Canary behavior has been tested and explicitly promoted.

## Repository layout

| Directory | Responsibility |
|---|---|
| `apps/supadata-platform` | Go client-facing data plane |
| `apps/studio` | React/TanStack dashboard |
| `apps/studio-laravel` | Laravel control plane and metadata APIs |
| `apps/docs` | Documentation application |
| `docker` | Local and deployment container configuration |
| `packages` | Shared frontend and platform packages |
| `assets` | GoLara branding and mascot assets |

## Development

Run Go data-plane checks from the service directory:

```bash
cd apps/supadata-platform
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

For frontend and local platform setup, see [`DEVELOPERS.md`](DEVELOPERS.md).

The detailed Go service contract is documented in [`apps/supadata-platform/README.md`](apps/supadata-platform/README.md).

## Canary workflow

1. Make changes on `canary`.
2. Run focused tests and repository checks.
3. Push the commit and deploy Canary.
4. Verify the exact deployed commit and public HTTPS routes.
5. Exercise authenticated and unauthenticated flows.
6. Promote to Stable only after acceptance passes.

A healthy container or successful build is not, by itself, proof that a feature works. Public behavior must be exercised and read back.

## Current boundaries

Implemented foundations are documented above. The following require additional parity work and dedicated acceptance coverage:

- complete Storage behavior;
- complete Realtime websocket behavior;
- Edge Functions execution;
- full project provisioning lifecycle;
- production-scale load, backup, and disaster-recovery validation.

## License

See [`LICENSE`](LICENSE) and the license files for the individual components included in this repository.
