# GoLara official SDK contracts

This package uses the unmodified official `@supabase/supabase-js` **2.118.0** against an explicit GoLara target. The initial suite covers representative Auth, Database/RPC, Storage, Realtime, and Functions flows. Passing this suite proves those cases; full SDK parity requires the versioned capability inventory and further contract cases.

Install with `npm ci --prefix apps/supadata-platform/compat --workspaces=false`. Run `npm test --prefix apps/supadata-platform/compat` for configuration checks.

Set `GOLARA_CONFIG_FILE` to a protected JSON file outside the Docker context and Git. Required properties are `publicURL` (origin only), `project`, `namespace` (starts with `golara_sdk_`), `anonKey`, and `serviceRoleKey`. Storage cases require the project-owned `bucket`. Browser and function deployment checks also require `studioUsername` and `studioPassword`. Never commit this file or print its contents.

Before running writes, render the fixture SQL with `node apps/supadata-platform/compat/prepare.mjs --sql` and apply it to the explicitly selected disposable database. This creates only the named test table and RPC and grants access to `service_role`. Then run `node apps/supadata-platform/compat/prepare.mjs` to deploy the disposable echo function through the real Studio API.

Run `npm run test:sdk --prefix apps/supadata-platform/compat` against local or Stable. Tests use unique records and remove users, rows, objects, and channels they create. The named table/RPC/function fixtures remain available for repeated runs. Check test output for cleanup failures before running again.

Run `npm run test:browser --prefix apps/supadata-platform/compat` for desktop/mobile rendering, real sign-in, protected app navigation, runtime errors, and public URL checks. Screenshots go to `output/playwright/golara-stable`; traces and videos stay disabled because authentication requests contain credentials. Do not publish unreviewed artifacts.

Stable testing is explicitly authorized for this project. Use the dedicated fixture namespace and service credentials only for the configured target. Never point the fixture SQL or failure/load tests at unrelated application data.
