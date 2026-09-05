# Architecture

## Target

```text
Single public URL
        |
      NGINX
     /     \
 Studio   Go platform
             |
       PostgreSQL + object storage
```

The final deployment uses one multi-stage Dockerfile. Studio static assets and the Go binary are shipped in one runtime image. PostgreSQL remains PostgreSQL and is not reimplemented.

## Modular monolith

The Go process will contain explicit internal boundaries:

- `auth`: JWT and session behavior.
- `rest`: PostgREST-compatible database API behavior.
- `rpc`: stored procedure invocation.
- `realtime`: WebSocket protocol and change propagation.
- `storage`: object API and SeaweedFS integration.
- `database`: PostgreSQL pools, roles, RLS context, migrations.
- `admin`: metadata and platform administration.
- `functions`: edge-function compatibility boundary.
- `jobs`: asynchronous lifecycle work.
- `middleware`: request IDs, authorization, limits, and recovery.
- `compatibility`: reference fixtures and contract tests.

## Migration boundary

The current Node control plane is not modified or removed until the Go implementation has passed focused tests, integration tests, race tests, security checks, Docker checks, and live compatibility verification. Native Studio routes remain the compatibility consumer throughout migration.

## Public URL decision

The current deployment has separate Studio and control-plane hostnames for safety during migration. The target architecture consolidates public traffic behind one URL; the old split remains only as a rollback path until the single-image deployment is verified.
