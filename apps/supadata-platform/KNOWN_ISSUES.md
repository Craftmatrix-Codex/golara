# Known issues

Stable is deployed at `https://golara.craftmatrix.org`. Passing the current contract suite establishes the tested behaviors, not full Supabase compatibility.

- The official `@supabase/supabase-js` 2.118.0 inventory contains 223 callable surfaces. Many remain unverified; representative contract counts are not a parity percentage.
- One live browser run reported an intermittent uncaught `Canceled` exception when loading Logs; a subsequent run passed. Its cause remains unconfirmed and page-error capture now includes a stack for future diagnosis.
- Live multi-project isolation is not yet verified; two separate project databases were tested locally.
- Email delivery, SMS, and OAuth validation/configuration are deferred by the user (2026-10-29); these remain unverified.
- Explicit soft deletion currently marks and retains the account; GoTrue-style anonymization and complete soft-delete semantics remain incomplete. Default admin deletion now removes the user record. Existing retained records are not purged automatically.
- Existing active sessions created before project tagging can make global/other-session logout return a conflict if their project cannot be determined. They require a separately validated migration; default-project local logout remains available.
- Auth still needs broader admin updates, OAuth/SSO, OTP/recovery/email/SMS delivery, anonymous accounts, MFA, passkeys, identity management, and complete GoTrue session/error semantics.
- REST/RPC still need complete PostgREST grammar, embedding, schema/profile negotiation, return-shape/error parity, and preference semantics.
- Storage private access currently uses coarse authenticated/project checks. Per-object RLS policies, full SDK bucket management, signed URLs, resumable uploads, transforms, vector/analytics APIs, and exact error parity remain incomplete.
- Realtime public channel subscription is verified. Broadcast, presence, PostgreSQL changes, private-channel authorization, and reconnection behavior need individual official SDK contracts.
- Deno deployment/invocation and project/public URL propagation are verified. Full secrets/logs/runtime lifecycle parity and sandbox isolation remain open.
- API-key-to-project mapping, shared-database account isolation, and complete control-plane SQL/database routing require further negative testing.
- Studio Advisors currently receives an empty lint result; its “no issues” display is not evidence that security or performance checks actually ran.
- Broad feature navigation checks establish rendering and required request success; they do not prove every button, editor operation, or integration works.
- Production-scale load, soak, backup restoration, failover, and matched reference-Supabase benchmarks are outstanding. No claim that GoLara is faster than Supabase has been established.
- Coolify requires a named mount at `/var/lib/supadata`; a Dockerfile `VOLUME` alone creates anonymous volumes and does not preserve state across Coolify replacement. Stable now uses `poslsmflcfylqkhdl131pj6o-golara-stable-data`; prior volumes and root-only recovery snapshots are retained.
