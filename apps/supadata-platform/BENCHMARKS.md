# Benchmarks

Measured on 2026-09-05, Linux amd64, AMD EPYC Processor with IBPB, Go 1.22.2:

- `BenchmarkSignHS256-4`: 184,704 ops, 6,964 ns/op, 1,936 B/op, 19 allocs/op.
- `BenchmarkBuildSelectQuery-4`: 388,436 ops, 2,774 ns/op, 784 B/op, 27 allocs/op.
- Single-image Docker size: 432,468,963 bytes (about 412 MiB decimal image metadata; `docker inspect` size).
- The Docker build ran Studio's Vite/TanStack build and its smoke server; this is not a full platform performance benchmark.

Not measured yet: reference-vs-Go throughput/latency, startup/RSS under equivalent configuration, Auth/REST end-to-end load, Realtime, Storage, database connections, progressive concurrency, soak, and peak memory. No speed or efficiency claim is made until those measurements exist.
