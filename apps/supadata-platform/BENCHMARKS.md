# Benchmarks

Measured on 2026-09-05, Linux amd64, AMD EPYC Processor with IBPB, Go 1.22.2:

- `BenchmarkSignHS256-4`: 184,704 ops, 6,964 ns/op, 1,936 B/op, 19 allocs/op.
- `BenchmarkBuildSelectQuery-4`: 388,436 ops, 2,774 ns/op, 784 B/op, 27 allocs/op.
- Single-image Docker size: 433,475,516 bytes (about 413 MiB decimal image metadata; `docker inspect` size).
- The Docker build ran Studio's Vite/TanStack build and its smoke server; this is not a full platform performance benchmark.

Startup/RSS under equivalent configuration, end-to-end Auth load, Realtime, Storage, progressive concurrency, soak, and peak memory remain unmeasured. The selected REST comparison below does not establish full-platform superiority.


## Selected REST/RPC comparison — 2026-10-29

GoLara runtime changes through `601d31ee74` versus the repository-pinned `postgrest/postgrest:v14.12`, using the same local PostgreSQL 17 database and 2,000-row disposable fixture. Both data planes are reached directly over the same Docker network. Each workload uses 20 warmup requests, then 200 measured requests at concurrency 1 or 10, across four rounds with alternating target order. Matching response data and exact HEAD counts are required before measuring.

Values below are medians across four rounds. The latency column is the median of each round's p95, not a pooled percentile.

| Workload | Concurrency | GoLara requests/s | PostgREST requests/s | GoLara p95 ms | PostgREST p95 ms |
| --- | ---: | ---: | ---: | ---: | ---: |
| read_100 | 1 | 521 | 372 | 2.77 | 4.08 |
| read_100 | 10 | 1883 | 1518 | 10.94 | 15.98 |
| filtered_read | 1 | 731 | 642 | 2.47 | 2.56 |
| filtered_read | 10 | 3209 | 3108 | 6.01 | 6.80 |
| head_exact_count | 1 | 412 | 338 | 4.18 | 5.88 |
| head_exact_count | 10 | 1553 | 1344 | 9.63 | 11.19 |
| rpc | 1 | 858 | 655 | 1.58 | 2.02 |
| rpc | 10 | 3254 | 2764 | 5.50 | 6.29 |

All 12,800 measured requests completed with the expected response status. These are service-role REST/RPC workloads only; GoLara recognizes its configured API key, while PostgREST uses a signed service-role JWT. Other projects were active on this shared host, so absolute results can vary. No CPU/memory limits were applied to isolate these containers. This does not compare Auth, Storage, Realtime, Functions, Studio, or an entire Supabase installation, and does not establish broad performance superiority.

Reproduce with `compat/bench-local.mjs` and the protected disposable local harness described in `compat/README.md`. Further acceptance requires broader workloads, equivalent resource budgets, longer runs, and an isolated host.
