# Progress

## Current execution state — 2026-10-29

The user authorized direct Stable deployment and iterative local/live testing. Stable is `https://golara.craftmatrix.org`; Coolify is `https://dev.craftmatrix.org`. The runtime consists of Go, Laravel/PHP-FPM, NGINX, the native React/TanStack Studio client, PostgreSQL, authenticated SeaweedFS, and Deno.

## Verified evidence

- `197f150865` repaired Studio login/session handling and core official SDK contracts; Coolify deployment `ezthr7pe60brble0mjeefx9l` completed healthy.
- `737dd97145` enforces private Storage download/list access, checks public bucket metadata, and gives functions the invoked project's public SDK and request URLs. Coolify deployment `wkqcizzz3ltxwfpaz1w1c6eb` is healthy.
- **13/13 official SDK contracts pass on Stable** with unmodified `@supabase/supabase-js` **2.118.0**. They cover representative Auth, database/RPC, rollback/RLS, Storage privacy/bytes, Realtime subscription, and real Deno invocation, including query-string/public URL propagation.
- Desktop/mobile sign-in, authenticated Studio navigation, metadata, and logout passed live. An expanded navigation run passed 14/15 cases; its desktop failure was `net::ERR_NETWORK_CHANGED` on the test machine and needs a clean rerun. All 12 feature navigation cases passed live.
- Go tests, race checks, vet, and stripped builds pass. Laravel passes **42 tests / 241 assertions**. Studio's production client build, smoke server, and typecheck pass. Changed root files have no ESLint errors and one existing `no-explicit-any` warning.
- Stable uses the named volume `poslsmflcfylqkhdl131pj6o-golara-stable-data`. Prior data was recovered; old anonymous volumes and root-only snapshots are retained. The actual running container's mount was read back.
- Runtime credentials are no longer injected as Dockerfile build arguments. Selected credential values were absent from the running image's history.
- The live trusted `service_role` was missing `BYPASSRLS`; its standard Supabase role behavior is restored. Anonymous/authenticated roles retain RLS. Installation SQL is in `scripts/bootstrap-api-roles.sql`.

## In progress

- The optimized production runtime built successfully at **663,060,101 bytes** and passed **13 SDK / 15 browser** cases locally. It reuses the previously built native Studio artifact, whose source is unchanged; the new Go/PHP/Debian runtime was rebuilt. Stable rollout is pending. The earlier repair overlay measured **1,683,672,338 bytes**; the original recovered Stable image measured approximately 1.58 GB. These are image-size measurements, not whole-platform performance results.
- Fresh-database Auth metadata reconciliation passes the isolated PostgreSQL listing test and the actual local Users page.
- Authenticated Studio access survives replacement by a fresh container using the named volume. Session files and server-side active-session permits persist there. A deterministic stale-session replay test reproduces the logout race and passes after explicit permit revocation; the real browser logout check also passes.
- A preliminary local REST/RPC comparison with repository-pinned PostgREST 14.12 found mixed sequential/concurrent results. It does not establish superior performance; quiet repeats and concurrent-request profiling remain.

## Remaining work

Expand the versioned contract matrix, repair each unsupported behavior, exercise full UI operations and project/permission failures, run resilience/load/soak tests, and compare matched workloads with a reference Supabase stack. See `KNOWN_ISSUES.md` and `COMPATIBILITY.md`. No full parity or superior-performance claim is established.

## Completion rule

Do not mark the platform complete while required compatibility entries are partial, unknown, unsupported, or untested. Keep direct Stable deployment evidence, rollback information, and local/live results tied to exact revisions.
