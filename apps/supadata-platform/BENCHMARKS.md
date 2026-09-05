# Benchmarks

Measured on 2026-09-05, Linux amd64, AMD EPYC Processor with IBPB, Go 1.22.2:

- `BenchmarkSignHS256-4`: 342,243 ops, 4,108 ns/op, 1,936 B/op, 19 allocs/op.
- `BenchmarkBuildSelectQuery-4`: 479,826 ops, 2,743 ns/op, 784 B/op, 27 allocs/op.
- Single-image Docker size: 432,470,579 bytes (about 412 MiB decimal image metadata; `docker inspect` size).
- The Docker build ran Studio's Vite/TanStack build and its smoke server; this is not a full platform performance benchmark.

Not measured yet: reference-vs-Go throughput/latency, startup/RSS under equivalent configuration, Auth/REST end-to-end load, Realtime, Storage, database connections, progressive concurrency, soak, and peak memory. No speed or efficiency claim is made until those measurements exist.
