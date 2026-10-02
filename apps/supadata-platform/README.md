# Supadata Go Platform

This directory is the migration-safe Go modular monolith for the Supadata-compatible backend.

The existing native Supabase Studio frontend remains the user interface. The Go service is developed beside the current Node.js control plane until compatibility and deployment gates pass.

## Compatibility rule

Compatibility means preserving externally observable API behavior, protocols, status codes, authorization semantics, database semantics, and client behavior. It does not mean copying upstream source line by line.

## Phase 1 boundary

The current implementation establishes the core HTTP/configuration/auth contract, basic REST/RPC behavior, project-scoped GraphQL HTTP compatibility through PostgreSQL `pg_graphql`/`graphql.resolve`, and a partial Storage object API while keeping the existing Studio-facing `/api/projects` response shape. Realtime, full Storage parity, functions, metadata, and the current Docker provisioning implementation remain explicitly incomplete.

## GraphQL compatibility

The service exposes both Supabase-compatible GraphQL URLs:

- `POST /graphql/v1`
- `POST /api/projects/{project-ref}/api/graphql` for the native Studio GraphiQL screen

Requests require `apikey` and accept `Authorization: Bearer ...` or `x-graphql-authorization`. The handler forwards GraphQL query text, variables, operation name, and extensions to PostgreSQL's `graphql.resolve(...)` inside the project-scoped transaction, preserving pg_graphql introspection, relationships, CRUD mutations, and RLS semantics. The target PostgreSQL image/resource must provide and install the `pg_graphql` extension; the application does not silently emulate the extension or expose a fake schema when it is unavailable.
