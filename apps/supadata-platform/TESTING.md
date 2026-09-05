# Testing

The implementation follows vertical test-first slices:

1. Write one contract test.
2. Run it and verify the expected failure.
3. Implement the smallest behavior.
4. Run the focused test and the full Go suite.
5. Run `go test -race ./...` before marking a slice complete.

Later stages add PostgreSQL/SeaweedFS integration, reference-Supabase compatibility fixtures, security/failure tests, benchmarks, stress and soak tests, Docker tests, and live Studio verification.
