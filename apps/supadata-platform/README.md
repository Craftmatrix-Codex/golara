# Supadata Go Platform

This directory is the migration-safe Go modular monolith for the Supadata-compatible backend.

The existing native Supabase Studio frontend remains the user interface. The Go service is developed beside the current Node.js control plane until compatibility and deployment gates pass.

## Compatibility rule

Compatibility means preserving externally observable API behavior, protocols, status codes, authorization semantics, database semantics, and client behavior. It does not mean copying upstream source line by line.

## Phase 1 boundary

The current implementation establishes the core HTTP/configuration/auth contract, basic REST/RPC behavior, and a partial Storage object API while keeping the existing Studio-facing `/api/projects` response shape. Realtime, full Storage parity, functions, metadata, and the current Docker provisioning implementation remain explicitly incomplete.
