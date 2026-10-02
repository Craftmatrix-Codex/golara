# Supadata Platform

Supadata is a project-scoped, Supabase-compatible data plane with the native Supabase Studio interface.

This application contains the Go service that owns client-facing API compatibility. Laravel remains the Studio and control-plane server. PostgreSQL remains authoritative for database behavior, including `pg_graphql` schema resolution and RLS.

## Architecture

```text
Browser / Supabase client
          |
          v
    NGINX :8080
      |       \
      |        \-- Laravel Studio and control-plane APIs
      v
  Go data plane :8090
      |
      +-- Auth and JWT/API-key validation
      +-- Project-scoped REST/RPC
      +-- GraphQL HTTP boundary
      +-- Storage and Realtime adapters
      +-- Project database resolver
      |
      v
   PostgreSQL
      |
      \-- pg_graphql / graphql.resolve(...)
```

The public application is deployed separately for each branch:

- **Canary**: `https://go-alpha.craftmatrix.org`
- **Stable**: `https://golara.craftmatrix.org`

Canary is the validation environment. Stable is updated only through an explicit promotion.

## Supabase-compatible routes

| Capability | Route | Status |
|---|---|---|
| Health | `GET /health` | Available |
| Auth health | `GET /auth/v1/health` | Available |
| REST | `/rest/v1/*` | Project-scoped implementation |
| RPC | `/rest/v1/rpc/*` | Project-scoped implementation |
| GraphQL | `POST /graphql/v1` | `pg_graphql` backed |
| Studio GraphiQL | `POST /api/projects/{project-ref}/api/graphql` | Laravel bridge to Go |
| Storage | `/storage/v1/*` | Partial compatibility |
| Realtime | `/realtime/v1/*` | Adapter implementation |

All client-facing routes are subject to API-key, JWT, and project-scope checks where applicable.

## GraphQL compatibility

GraphQL is resolved by PostgreSQL rather than a fabricated Go schema. The Go handler calls:

```sql
graphql.resolve($1, $2::jsonb, $3, $4::jsonb)
```

This preserves PostgreSQL-owned behavior for:

- introspection;
- relationships and reflected schema;
- queries and mutations;
- variables and named operations;
- JWT claims and database roles;
- Row Level Security policies.

### Request

```bash
curl -X POST "https://go-alpha.craftmatrix.org/graphql/v1" \
  -H "Content-Type: application/json" \
  -H "apikey: $SUPABASE_ANON_KEY" \
  --data '{"query":"{ __typename }"}'
```

Expected response when the database is configured:

```json
{"data":{"__typename":"Query"}}
```

The request may also include:

```http
Authorization: Bearer <access-token>
x-graphql-authorization: Bearer <role-token>
```

`operationName` is sent as SQL `NULL` when omitted, as required by `pg_graphql`.

### PostgreSQL prerequisite

The target database must provide and install `pg_graphql`:

```sql
CREATE EXTENSION IF NOT EXISTS pg_graphql;
```

The standard Supabase roles must also be able to use the GraphQL schema and functions:

```sql
GRANT USAGE ON SCHEMA graphql TO anon, authenticated, service_role;
GRANT EXECUTE ON ALL FUNCTIONS IN SCHEMA graphql TO anon, authenticated, service_role;
```

The application returns an explicit service/database error when this prerequisite is unavailable. It does not silently emulate a schema or claim GraphQL support from a static response.

## Project isolation

Projects are resolved through the control-plane registry and mapped to database connections before data-plane execution. A project-scoped request must not fall back to an unrelated default database when project isolation is required.

The isolation contract covers:

- project-aware database resolution;
- request JWT claim propagation;
- database role setup inside a transaction;
- REST, RPC, and GraphQL project routing;
- separate project storage metadata;
- explicit Canary and Stable deployment environments.

## Configuration

Secrets must be supplied through the deployment secret manager or protected runtime environment. Never commit them to this repository.

Common runtime variables include:

| Variable | Purpose |
|---|---|
| `DATABASE_URL` | Runtime PostgreSQL connection URL |
| `SUPADATA_DATABASE_URL` | Explicit Supadata database connection URL |
| `SUPADATA_PUBLIC_DATABASE_HOST` | Safe public database host metadata |
| `SUPADATA_PUBLIC_DATABASE_PORT` | Safe public database port metadata |
| `SUPADATA_PUBLIC_DATABASE_NAME` | Safe public database name metadata |
| `SUPADATA_PUBLIC_DATABASE_USER` | Safe public database user metadata |
| `ANON_KEY` | Anonymous Supabase-compatible API key |
| `SERVICE_ROLE_KEY` | Service-role API key |
| `JWT_SECRET` | HS256 access-token verification secret |
| `SUPADATA_PUBLIC_HOST` | Public application hostname |
| `SUPADATA_ALLOWED_ORIGIN` | Allowed browser origin |
| `SUPADATA_REQUIRE_PROJECT_SCOPE` | Enforces project-scoped routing |

Connection strings returned to Studio are password-masked. Credentials must never appear in logs, screenshots, test fixtures, or documentation.

## Local validation

Run Go commands from this directory:

```bash
go test ./...
go test -race ./...
go vet ./...
gofmt -w .
git diff --check
```

The production Dockerfile also runs the Go test, race, vet, and build checks before producing the runtime image.

## Deployment workflow

1. Develop and validate on `canary`.
2. Push the Canary commit.
3. Deploy the Canary Coolify application.
4. Verify the exact deployment commit, container health, and public HTTPS route.
5. Exercise authenticated and unauthenticated API boundaries.
6. Test the requested feature through the real Studio/client path.
7. Fast-forward `canary` into `stable` only after Canary acceptance passes.
8. Deploy Stable explicitly and repeat the live verification.

A Git merge is not a deployment. A healthy container is not proof that the requested route or database feature works; both require public readback.

## Current parity boundary

Implemented and verified:

- project-scoped Go runtime routing;
- API-key and JWT authorization boundaries;
- REST/RPC compatibility foundations;
- native PostgreSQL-backed GraphQL HTTP compatibility;
- Studio GraphiQL routing;
- persistent control-plane metadata and telemetry contracts;
- separate Canary and Stable deployments.

Still requiring separate parity work or acceptance coverage:

- complete Storage API parity;
- complete Realtime protocol parity and websocket behavior;
- Edge Functions runtime parity;
- advanced Supabase project-management behavior;
- full upstream provisioning and lifecycle automation;
- complete production-scale load, backup, and disaster-recovery validation.

Do not describe an item in the second list as complete until its live behavior has been exercised and read back.
