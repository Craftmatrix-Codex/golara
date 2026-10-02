<p align="center">
  <img src="https://user-images.githubusercontent.com/8291514/213727234-cda046d6-28c6-491a-b284-b86c5cede25d.png#gh-light-mode-only" alt="Supabase" />
  <img src="https://user-images.githubusercontent.com/8291514/213727225-56186826-bee8-43b5-9b15-86e839d89393.png#gh-dark-mode-only" alt="Supabase" />
</p>

<h1 align="center">Supabase</h1>

<p align="center">
  The open-source Postgres development platform — maintained and extended on the <code>canary</code> branch.
</p>

<p align="center">
  <a href="https://github.com/Craftmatrix-Codex/supabase/tree/canary">Canary</a>
  ·
  <a href="https://supabase.com/docs">Documentation</a>
  ·
  <a href="https://github.com/Craftmatrix-Codex/supabase/issues">Issues</a>
</p>

## What is Supabase?

Supabase is a Postgres development platform. It combines a powerful database with authentication, auto-generated APIs, file storage, realtime features, functions, and a web dashboard.

This repository is a Supabase-compatible platform implementation. The active development environment is **Canary**; Stable is updated only after Canary verification and explicit promotion.

## Platform capabilities

- [x] Hosted PostgreSQL database
- [x] Authentication and authorization boundaries
- [x] Project-scoped REST and RPC APIs
- [x] Native PostgreSQL-backed GraphQL through `pg_graphql`
- [x] Studio GraphiQL integration
- [x] File Storage compatibility foundation
- [x] Realtime compatibility foundation
- [ ] Complete Storage API parity
- [ ] Complete Realtime protocol parity
- [ ] Edge Functions runtime parity
- [ ] Full upstream provisioning parity

### API compatibility

- [x] REST API foundation
- [x] RPC routing
- [x] GraphQL HTTP boundary at `/graphql/v1`
- [x] GraphQL resolution through PostgreSQL `graphql.resolve(...)`
- [x] API-key and JWT authorization checks
- [x] Project-scoped database routing

### Dashboard and Studio

- [x] Existing React/TanStack Studio interface preserved
- [x] Laravel control-plane APIs
- [x] Project metadata and database metadata routes
- [x] Studio GraphiQL proxy to the Go data plane
- [x] Secure masked connection details

## Architecture

```text
Supabase client / browser
            |
            v
      NGINX / HTTPS
        |       \
        |        \-- Laravel Studio and control plane
        v
   Go compatibility data plane
        |
        +-- Auth and project scope
        +-- REST / RPC / GraphQL
        +-- Storage and Realtime adapters
        |
        v
     PostgreSQL
        |
        \-- pg_graphql / graphql.resolve(...)
```

The public Canary endpoint is:

```text
https://go-alpha.craftmatrix.org
```

The public Stable endpoint is:

```text
https://go-stable.craftmatrix.org
```

## GraphQL

GraphQL requests are resolved by the PostgreSQL `pg_graphql` extension. The Go service provides the authenticated, project-scoped HTTP boundary; it does not return a fabricated static schema.

```bash
curl -X POST "https://go-alpha.craftmatrix.org/graphql/v1" \
  -H "Content-Type: application/json" \
  -H "apikey: $SUPABASE_ANON_KEY" \
  --data '{"query":"{ __typename }"}'
```

Expected response:

```json
{"data":{"__typename":"Query"}}
```

The database must provide `pg_graphql`:

```sql
CREATE EXTENSION IF NOT EXISTS pg_graphql;
```

See the complete data-plane contract in [`apps/supadata-platform/README.md`](apps/supadata-platform/README.md).

## Repository layout

| Directory | Responsibility |
|---|---|
| `apps/supadata-platform` | Go client-facing data plane |
| `apps/studio` | React/TanStack Studio interface |
| `apps/studio-laravel` | Laravel Studio and control plane |
| `apps/docs` | Documentation application |
| `docker` | Local and deployment container configuration |
| `packages` | Shared frontend and platform packages |

## Development

Requirements depend on the workspace being changed. Go data-plane validation can be run with:

```bash
cd apps/supadata-platform
go test ./...
go test -race ./...
go vet ./...
git diff --check
```

For frontend, Studio, and local platform setup, start with [`DEVELOPERS.md`](DEVELOPERS.md).

## Canary workflow

Canary is the integration and acceptance branch:

1. Make the change on `canary`.
2. Run focused tests and repository checks.
3. Push the commit and deploy Canary.
4. Verify the exact deployed commit and public route.
5. Exercise the real authenticated and unauthenticated flows.
6. Promote to Stable only after acceptance passes.

A green build or healthy container alone is not considered feature verification.

## Documentation

- [Supabase documentation](https://supabase.com/docs)
- [Developer setup](DEVELOPERS.md)
- [Go data-plane contract](apps/supadata-platform/README.md)
- [Database guides](https://supabase.com/docs/guides/database)
- [Authentication guides](https://supabase.com/docs/guides/auth)
- [API guides](https://supabase.com/docs/guides/api)
- [GraphQL guides](https://supabase.com/docs/guides/graphql)
- [Storage guides](https://supabase.com/docs/guides/storage)

## Community and support

- [GitHub Discussions](https://github.com/Craftmatrix-Codex/supabase/discussions) — questions and design discussion
- [GitHub Issues](https://github.com/Craftmatrix-Codex/supabase/issues) — reproducible bugs
- [Discord](https://discord.supabase.com) — community discussion

## License

This project follows the licenses declared by the upstream Supabase components and the individual projects included in this repository. See [`LICENSE`](LICENSE) and each component's license for details.
