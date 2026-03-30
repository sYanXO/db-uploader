# Benchmark Results

## Scope

These benchmarks were run against a real Neon Postgres database using the uploader worker pool and database insert path.

This is a DB-ingest benchmark, not a JSON file-read benchmark.

## Instrumentation Added

- [`cmd/benchmark/main.go`](cmd/benchmark/main.go) adds a dedicated benchmark runner.
- [`internal/loader/metrics.go`](internal/loader/metrics.go) adds batch and DB exec latency metrics with percentile reporting.
- [`internal/db/postgres.go`](internal/db/postgres.go) adds Postgres pool controls and benchmark table helpers.
- [`cmd/uploader/main.go`](cmd/uploader/main.go) now prints the same metrics for normal runs.

## Best Configuration Found

- `workers=16`
- `batch=500`
- `max-open-conns=16`
- `max-idle-conns=16`

This configuration gave the best throughput without inflating latency tails as badly as higher concurrency settings.

## Results

| Rows | Time | Throughput | Batch p50 | Batch p95 | Batch p99 | Retries | Result |
|---|---:|---:|---:|---:|---:|---:|---|
| 100k | 4.11s | 24.3k rows/s | 199ms | 816ms | 968ms | 1 | completed |
| 500k | 17.34s | 28.8k rows/s | 245ms | 539ms | 812ms | 0 | completed |
| 1m | 34.28s | 29.2k rows/s | 245ms | 503ms | 741ms | 0 | completed |
| 5m | blocked | blocked | n/a | n/a | n/a | many | failed on Neon storage cap |
| 10m | not runnable | not runnable | n/a | n/a | n/a | n/a | impossible on current Neon cap |

## Findings

- Throughput scales cleanly through `1m` rows.
- `100k` is slower because startup costs are not amortized.
- From `500k` onward, throughput stabilized around `28.8k` to `29.2k rows/s`.
- `16` workers performed better than `8`.
- `32` workers performed worse than `16`, with higher latency and no throughput gain.
- Batch size `500` performed better than `1000` and `2000` on this Neon setup.
- `database/sql` pool wait count stayed at `0`, so the client-side pool was not the main bottleneck.

## System Design Constraints Observed

- The current `lib/pq` plus Neon pooler path intermittently produced:
  - `pq: unnamed prepared statement does not exist`
  - bind/statement shape errors when changing batch sizes between runs
- Those errors were transient in smaller runs and were recovered by retries, but they indicate a real protocol-level incompatibility risk for this workload.
- The `5m` run hit Neon's hard storage limit:
  - `pq: could not extend file because project size limit (512 MB) has been exceeded`
- Because of that external limit, `5m` and `10m` could not be benchmarked to completion on the current Neon project.

## Estimated Larger-Run Timing

These are estimates inferred from the completed `500k` and `1m` runs, assuming similar sustained throughput and no storage cap:

- `5m`: about `170s`
- `10m`: about `340s`

These are estimates, not measured results.

## Optimization Recommendations

1. Replace multi-row `INSERT ... VALUES` with Postgres `COPY` for bulk ingest.
2. Move from `lib/pq` to `pgx` for this benchmark path.
3. Prefer a direct Neon connection or simple-protocol path if the pooler continues to emit prepared-statement errors.
4. Keep concurrency near `16` for this database shape unless the DB plan or schema changes.
5. Increase Neon storage quota before attempting `5m+` row benchmarks on the current schema.

## Validation

Before running the Neon benchmarks:

```bash
go test ./...
```

The test suite passed.
