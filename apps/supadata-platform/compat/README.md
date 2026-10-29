# GoLara official SDK contracts

This package uses the unmodified official `@supabase/supabase-js` **2.118.0** against an explicit GoLara target. The initial suite covers representative Auth, Database/RPC, Storage, Realtime, and Functions flows. Passing this suite proves those cases; full SDK parity requires the versioned capability inventory and further contract cases.

Install with `npm ci --prefix apps/supadata-platform/compat --workspaces=false`. Run `npm test --prefix apps/supadata-platform/compat` for configuration checks.

Set `GOLARA_CONFIG_FILE` to a protected JSON file outside the Docker context and Git. Required properties are `publicURL` (origin only), `project`, `namespace` (starts with `golara_sdk_`), `anonKey`, and `serviceRoleKey`. Storage cases require the project-owned `bucket`. Browser and function deployment checks also require `studioUsername` and `studioPassword`. Never commit this file or print its contents.

Before running writes, render the fixture SQL with `node apps/supadata-platform/compat/prepare.mjs --sql` and apply it to the explicitly selected disposable database. This creates only the named test table and RPC and grants access to `service_role`. Then run `node apps/supadata-platform/compat/prepare.mjs` to deploy the disposable echo function through the real Studio API.

Run `npm run test:sdk --prefix apps/supadata-platform/compat` against local or Stable. Tests use unique records and remove users, rows, objects, and channels they create. The named table/RPC/function fixtures remain available for repeated runs. Check test output for cleanup failures before running again.

Run `npm run test:browser --prefix apps/supadata-platform/compat` for desktop/mobile rendering, real sign-in, protected app navigation, runtime errors, and public URL checks. Screenshots go to `output/playwright/golara-stable`; traces and videos stay disabled because authentication requests contain credentials. Do not publish unreviewed artifacts.

Stable testing is explicitly authorized for this project. Use the dedicated fixture namespace and service credentials only for the configured target. Never point the fixture SQL or failure/load tests at unrelated application data.

The browser suite now checks 12 Studio feature pages in addition to sign-in/session flows. Page navigation success does not establish complete interaction parity.

`npm run test:recreate --prefix apps/supadata-platform/compat` is an opt-in replacement test for the fixed disposable local harness at `http://127.0.0.1:18080`. In addition to `GOLARA_CONFIG_FILE`, set `GOLARA_LOCAL_ENV_FILE` to its protected local environment file. It replaces only `golara-sdk-slim`, requires the loopback port and `golara-sdk-app-data` mount, keeps browser cookies in memory, and restores the previous test container on failure. Never use this workflow on a live deployment.

For local REST/RPC comparisons, `bench-local.mjs` requires the same protected local configuration/environment and the fixed `golara-sdk-slim`, `golara-sdk-reference` (PostgREST 14.12), and PostgreSQL harness network. Render the 2,000-row disposable fixture with `prepare.mjs --sql-benchmark` and apply it to the local fixture database first. It checks matching response data before comparing 200 requests per workload at concurrency 1 and 10, over four rounds with alternating target order. Keep the machine quiet, run multiple trials, and record resource limits and versions. This measures selected direct REST/RPC endpoints; it does not establish whole-Supabase performance.
