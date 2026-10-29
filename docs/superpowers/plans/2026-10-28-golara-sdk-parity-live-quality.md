# GoLara SDK Parity, Live Quality, and Performance Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Execution override (2026-10-28):** The user approved starting the entire plan directly on Stable. All live validation targets `https://golara.craftmatrix.org`, deployed through `https://dev.craftmatrix.org`; Canary references below describe the original proposal and are superseded. Use uniquely named disposable test resources within Stable, preserve application data, and verify rollback. Execute inline in the current checkout; Stable deployment and necessary pushes are authorized.

**Goal:** Make GoLara reliably usable through the pinned official Supabase JavaScript SDK, prove the real Studio and public API behavior locally and on a live Canary, and optimize measured workloads against a fair Supabase baseline before promoting accepted changes to Stable.

**Architecture:** Keep the existing Go data plane, Laravel Studio/control plane, NGINX routing, and PostgreSQL authority. Build an SDK-versioned capability matrix and black-box contract suites, run them against deterministic local fixtures and an isolated live Canary, patch failures and rerun affected gates, then promote only accepted revisions to Stable. Treat Auth, PostgREST/RPC, Realtime, Storage, Functions, GraphQL, and Studio/control-plane behavior as separate compatibility areas with explicit prerequisites and evidence.

**Tech Stack:** Go, PostgreSQL, SeaweedFS/S3-compatible storage, Laravel/PHP, React/TanStack/Vite, NGINX, Node.js, pinned `@supabase/supabase-js`, Playwright, Docker, and Coolify.

**Spec:** User request (2026-10-28); current implementation boundaries in `apps/supadata-platform/README.md`, `COMPATIBILITY.md`, `KNOWN_ISSUES.md`, `TESTING.md`, and `DEPLOYMENT.md`.

## Global Constraints

- Pin the exact `@supabase/supabase-js` release under test and keep the capability inventory tied to that version.
- Canary is the default live validation target (`https://go-alpha.craftmatrix.org`); Stable (`https://golara.craftmatrix.org`) is promoted to only after the agreed gates pass.
- Never run destructive compatibility or load tests against shared production data; use disposable projects, schemas, users, buckets, and channels with cleanup.
- No credentials or real user data in source control, test output, screenshots, or benchmark reports.
- A capability is `PASS` only with executable local and live evidence for the pinned SDK version; unsupported and prerequisite-blocked items remain explicit.
- Publicly rendered pages, client configuration, redirects, API responses, and browser network destinations must use the real public origin and contain no `localhost` URL/value.
- Do not claim GoLara is faster overall than Supabase; report only reproducible, matched-workload results with environment and resource limits recorded.
- Preserve the existing Stable deployment and rollback path while repairing and validating Canary.

## Review Focus

- Missing or stale live dependencies (including the currently unresolved SeaweedFS hostname) must be identified before the rollout gate; add a live dependency/readiness probe and a canary deploy check.
- Environment-specific public URL fallback must never leak `localhost` into browser-visible config or responses; add an assertion to browser and HTTP smoke checks.
- SDK errors, headers, pagination/count behavior, and auth/session edge cases must match expected client behavior; bind differential assertions to each feature slice.
- Cross-project JWT, database, storage, and realtime access must fail closed; preserve and extend existing two-project isolation tests.
- Load and failure tests must not damage shared state or obscure rollback; verify disposable resource naming and cleanup before enabling live write/load suites.

---

## File Map

- `apps/supadata-platform/COMPATIBILITY.md` — authoritative per-capability status and evidence.
- `apps/supadata-platform/TESTING.md` — local, integration, browser, live, security, and performance test entry points.
- `apps/supadata-platform/DEPLOYMENT.md` — Canary/Stable promotion and rollback gates.
- `apps/supadata-platform/compat/` — pinned official SDK black-box suite and disposable fixtures (create during implementation).
- `apps/supadata-platform/internal/**` — Go API behavior and focused unit/integration tests, changed only for a failing capability.
- `apps/studio-laravel/**` and `apps/studio/**` — Studio/control-plane behavior, changed only for a demonstrated contract gap.
- `e2e/studio/golara-canary/**` — live browser verification, visible Studio content, public URL checks, console/network errors, and screenshot evidence.
- `apps/supadata-platform/BENCHMARKS.md` — reproducible baseline, measurements, optimizations, and before/after results.
- Coolify Canary configuration — live dependency/DNS, environment, healthcheck, exact revision, and readback evidence; do not expose secrets in notes.

## Execution Plan

### Task 1: Establish exact scope and known-good baselines

**Files:**
- Modify: `apps/supadata-platform/COMPATIBILITY.md`
- Modify: `apps/supadata-platform/TESTING.md`
- Evidence: `apps/supadata-platform/DEPLOYMENT.md`

- [ ] Record current branch/revision, local build/test commands, current matrix statuses, available local dependencies, and known live deployment state without changing Stable.
- [ ] Select and pin the official `@supabase/supabase-js` version; inventory the public client modules and methods from that version's official API/reference into matrix rows, including prerequisites and expected contracts.
- [ ] Separate SDK scope (Auth, Database/PostgREST/RPC, Realtime, Storage, Functions, GraphQL) from Studio/control-plane and deployment concerns; mark out-of-scope Supabase products and SDKs explicitly.
- [ ] Run and record the current local gates: Go unit/race/vet/build, Laravel feature tests, Studio build, container smoke/browser checks, and available PostgreSQL/SeaweedFS integration checks.
- [ ] Record baseline results without relabeling partial or untested behavior as passed.

**Gate:** There is a versioned feature inventory, each feature has a measurable expected result and prerequisite, and the current local/live failures are reproducible.

### Task 2: Restore a dependable local and Canary foundation

**Files:**
- Modify: `apps/supadata-platform/DEPLOYMENT.md`
- Modify: `apps/supadata-platform/TESTING.md`
- Modify: `e2e/studio/golara-canary/**`
- Configuration: Coolify Canary dependencies and runtime settings

- [ ] Reproduce the current local Docker build and run the unified image with disposable PostgreSQL and SeaweedFS services.
- [ ] Trace the failed Stable rollout's unresolved SeaweedFS hostname to its Coolify service/configuration owner; record the verified target rather than guessing or disabling storage.
- [ ] Repair the Canary runtime dependency/configuration and deploy one exact commit to `https://go-alpha.craftmatrix.org`; record deployment and container health.
- [ ] Exercise `/health`, `/sign-in`, Studio project routes, and public API URL discovery through the public HTTPS origin.
- [ ] Run Playwright and assert visible native Studio content, expected title, no page/console errors, no failed required requests, no `localhost` in browser-visible strings/config/URLs, and correct public host; save screenshot evidence.
- [ ] Confirm failed rollout rollback leaves the prior Canary available and document the rollback action.

**Gate:** Canary is healthy at the exact revision, storage dependency resolves, and a real browser sees the app using only its public origin. Stable remains unchanged.

### Task 3: Build reusable official-SDK compatibility harness

**Files:**
- Create: `apps/supadata-platform/compat/package.json`
- Create: `apps/supadata-platform/compat/README.md`
- Create: `apps/supadata-platform/compat/**` test/config/fixture files
- Modify: `apps/supadata-platform/COMPATIBILITY.md`

- [ ] Add a pinned SDK dependency and scripts for local, Canary, and optional reference-Supabase targets.
- [ ] Require explicit target URL/project credentials; fail before running if target is missing or not marked disposable for write tests.
- [ ] Provide unique test resource names and reliable cleanup for auth users, rows, files, channels, and functions.
- [ ] Emit per-test pass/fail, SDK version, target revision/origin, and sanitized diagnostics; never print secret values.
- [ ] Add tests for API-key/JWT boundaries, returned data, status/error contracts, headers, and public URL usage before expanding method coverage.

**Gate:** Harness runs the same safe smoke suite against local and Canary and clearly refuses an unsafe or incomplete write-test configuration.

### Task 4: Close SDK feature gaps in small, measurable slices

**Files:**
- Modify: feature-owned files under `apps/supadata-platform/internal/{auth,rest,realtime,storage,functions,graphql,project}/**`
- Modify: applicable tests under those packages
- Modify: `apps/supadata-platform/compat/**`
- Modify: `apps/supadata-platform/COMPATIBILITY.md`

- [ ] Prioritize one capability slice at a time using user-facing impact, dependency readiness, and current partial status.
- [ ] For each slice, add a failing official-SDK contract test, reproduce locally, patch the smallest implementation surface, then rerun the focused test and full local regression gates.
- [ ] Cover Auth flows and session behavior, PostgREST query/filter/write/count/error semantics, RPC signatures/results, Realtime websocket/channel/broadcast/presence/database-change behavior, Storage CRUD/signed/resumable flows, Functions invoke/error behavior, and GraphQL client calls as supported by the pinned SDK.
- [ ] Extend PostgreSQL and S3-compatible integration tests for actual service behavior, RLS, project isolation, and expected error boundaries.
- [ ] Keep any capability blocked by an unavailable prerequisite explicitly blocked; do not fabricate success or replace native service behavior with stubs.
- [ ] After each accepted slice, rerun the full local suite and the live Canary suite, patch regressions, and retain evidence before moving to the next slice.

**Gate:** Every claimed `PASS` has local SDK contract + required service integration + live Canary evidence; unsupported rows state the exact remaining behavior.

### Task 5: Prove the actual live app and all public origins continuously

**Files:**
- Modify: `e2e/studio/golara-canary/**`
- Modify: `apps/supadata-platform/TESTING.md`
- Modify: `apps/supadata-platform/DEPLOYMENT.md`

- [ ] Test each relevant Studio route in a real browser and assert meaningful rendered content, correct navigation, expected title, and no blank/error shell.
- [ ] Inspect HTML, boot payloads, JavaScript runtime configuration, redirects, API responses, request destinations, websocket endpoints, and console output for `localhost` values or wrong domains.
- [ ] Exercise sign-in and sign-out using a disposable live account; verify session cookies/tokens and protected route behavior without recording credentials.
- [ ] Run the SDK suite against Canary for each completed feature slice, including public endpoint and websocket resolution.
- [ ] Save screenshots and machine-readable result summaries tagged with exact Git revision and deployment identifier.

**Gate:** The deployed revision is visibly the intended GoLara app, all browser-visible origins are public and correct, and all tested routes/services behave as expected.

### Task 6: Differential correctness, security, and resilience

**Files:**
- Create/modify: `apps/supadata-platform/compat/**`
- Modify: `apps/supadata-platform/internal/**` security/integration tests
- Modify: `apps/supadata-platform/COMPATIBILITY.md`
- Modify: `apps/supadata-platform/TESTING.md`

- [ ] Run the same safe SDK cases against GoLara and an isolated reference Supabase project using equivalent schema, data, auth settings, and service prerequisites.
- [ ] Compare data, status codes, error shapes, headers, pagination/counts, JWT/RLS outcomes, websocket events, storage metadata, and function invocation results.
- [ ] Add cross-project authorization, malformed/oversized input, rate/timeout, dependency loss, restart, and rollback tests for applicable features.
- [ ] Ensure live fault/write testing is isolated and cleanup succeeds even after a failed assertion or interrupted run.

**Gate:** Differential mismatches are fixed or recorded as known incompatibilities; security and cleanup gates pass before load testing.

### Task 7: Benchmark and optimize measured bottlenecks

**Files:**
- Create/modify: `apps/supadata-platform/**/benchmark_test.go`
- Create/modify: `apps/supadata-platform/compat/benchmarks/**`
- Modify: `apps/supadata-platform/BENCHMARKS.md`

- [ ] Define matched workloads for health, Auth token validation, REST reads/writes, RPC, GraphQL, Storage, Realtime, and Studio page load using identical SDK versions and payloads.
- [ ] Record hardware/CPU and memory limits, Go/runtime versions, PostgreSQL and SeaweedFS versions, dataset size, warmup, concurrency, repetitions, p50/p95/p99 latency, throughput, errors, CPU, memory, and image size.
- [ ] Establish baselines before changes; profile CPU, heap, allocations, SQL plans, connection pools, NGINX, PHP, and frontend bundles where measurements show cost.
- [ ] Optimize only measured hot paths; after each change rerun correctness gates and the same benchmark to demonstrate a repeatable improvement without feature regressions.
- [ ] Report per-workload results against the reference service; claim “faster” only for workloads that meet a predeclared threshold under matched conditions.

**Gate:** Each optimization has repeatable before/after results, no compatibility/security regression, and resource/latency tradeoffs are visible.

### Task 8: Stable promotion and completion decision

**Files:**
- Modify: `apps/supadata-platform/DEPLOYMENT.md`
- Modify: `apps/supadata-platform/COMPATIBILITY.md`
- Modify: `apps/supadata-platform/KNOWN_ISSUES.md`
- Modify: `apps/supadata-platform/PROGRESS.md`

- [ ] Require all release-scoped compatibility, local, security, live browser, dependency, and rollback gates to pass on one exact Canary revision.
- [ ] Promote only that accepted revision to Stable; do not merge/deploy a different unverified head.
- [ ] Repeat non-destructive health, browser, public URL, and SDK smoke checks on Stable and verify the exact running revision.
- [ ] Keep rollback available until Stable verification completes; roll back immediately if health, route, SDK, or public-origin checks fail.
- [ ] Publish the full matrix, benchmark report, remaining incompatibilities, exact deployed revisions, and live evidence; call the overall platform complete only when the promised SDK inventory is fully `PASS` or the user explicitly narrows scope.

**Gate:** Stable serves the intended visible app at its public origin, the accepted revision passes readback, and no required capability is represented as complete without evidence.

## Execution Rules

- Work in the order above; do not start feature implementation until the plan is reviewed and accepted.
- Run a focused failing test before each code fix, then the focused pass, related integration suite, and full local regression checks.
- After any deployment/configuration patch, rerun local checks first, deploy only to Canary, and repeat public browser and SDK checks before considering Stable.
- If a live prerequisite is unavailable, continue independent local work and report the specific blocker instead of weakening a health check or silently disabling the affected feature.
- Keep commits small and attributable to a compatibility slice or deployment fix; exclude unrelated/unapproved design drafts.
