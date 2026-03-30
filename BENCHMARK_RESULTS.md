# Benchmark Results

## Resume Summary

- Upgraded the PostgreSQL ingest path from `lib/pq` plus batched `INSERT ... VALUES` to `pgx` plus `COPY`
- Improved measured throughput by `43%` to `53%` on real Neon benchmarks
- Reduced `1m` row upload time from `34.28s` to `23.89s`
- Reduced `1m` batch latency tails from `p95=503ms / p99=741ms` to `p95=309ms / p99=523ms`
- Identified the remaining scaling limit as Neon project storage quota rather than Go worker concurrency

## Scope

These benchmarks were run against a real Neon Postgres database using the uploader worker pool and database ingest path.

This is a DB-ingest benchmark, not a JSON file-read benchmark.

The repository has now been benchmarked in two configurations:

- original path: `lib/pq` plus multi-row `INSERT ... VALUES`
- upgraded path: `pgx` plus `COPY`

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

This configuration gave the best throughput without inflating latency tails as badly as higher concurrency settings for both the old and upgraded runs.

## Before vs After

| Rows | Old Time | New Time | Time Change | Old Throughput | New Throughput | Throughput Change |
|---|---:|---:|---:|---:|---:|---:|
| 100k | 4.11s | 2.69s | -34.5% (0.65x time) | 24.3k rows/s | 37.2k rows/s | +53.1% (1.53x) |
| 500k | 17.34s | 12.03s | -30.6% (0.69x time) | 28.8k rows/s | 41.6k rows/s | +44.4% (1.44x) |
| 1m | 34.28s | 23.89s | -30.3% (0.70x time) | 29.2k rows/s | 41.9k rows/s | +43.5% (1.43x) |

### Detailed Comparison

#### 100k

- Old: `4.11s`, `24.3k rows/s`
- New: `2.69s`, `37.2k rows/s`
- Time improvement: `34.5%` faster, or `0.65x` the previous runtime
- Throughput improvement: `53.1%`, or `1.53x`

#### 500k

- Old: `17.34s`, `28.8k rows/s`
- New: `12.03s`, `41.6k rows/s`
- Time improvement: `30.6%` faster, or `0.69x` the previous runtime
- Throughput improvement: `44.4%`, or `1.44x`

#### 1m

- Old: `34.28s`, `29.2k rows/s`
- New: `23.89s`, `41.9k rows/s`
- Time improvement: `30.3%` faster, or `0.70x` the previous runtime
- Throughput improvement: `43.5%`, or `1.43x`

## Current Results

| Rows | Time | Throughput | Batch p50 | Batch p95 | Batch p99 | Retries | Result |
|---|---:|---:|---:|---:|---:|---:|---|
| 100k | 2.69s | 37.2k rows/s | 180ms | 443ms | 550ms | 0 | completed |
| 500k | 12.03s | 41.6k rows/s | 183ms | 365ms | 541ms | 0 | completed |
| 1m | 23.89s | 41.9k rows/s | 184ms | 309ms | 523ms | 0 | completed |
| 2m | 47.25s | 42.3k rows/s | 186ms | 269ms | 489ms | 0 | completed |
| 5m | blocked | blocked | n/a | n/a | n/a | many | failed on Neon storage cap |
| 10m | not runnable | not runnable | n/a | n/a | n/a | n/a | impossible on current Neon cap |

## Findings

- Throughput scales cleanly through at least `2m` rows on the upgraded path.
- `100k` is still slower because startup costs are not fully amortized.
- From `500k` onward, throughput stabilized around `41.6k` to `42.3k rows/s`.
- The `pgx` plus `COPY` upgrade improved throughput by about `43%` to `53%` on the measured sizes.
- Tail latency improved materially. The old `1m` run had batch `p95=503ms` and `p99=741ms`; the upgraded `1m` run dropped to `p95=309ms` and `p99=523ms`.
- `16` workers performed better than `8`.
- `32` workers still performed worse than `16`, with higher latency and no throughput gain.
- Batch size `500` still performed better than `2000` on this Neon setup.
- The new `pgxpool` stats showed no pool saturation signal; the DB path remains the main limiting factor.

## System Design Constraints Observed

- The old `lib/pq` plus Neon pooler path intermittently produced:
  - `pq: unnamed prepared statement does not exist`
  - bind/statement shape errors when changing batch sizes between runs
- The `pgx` plus `COPY` path eliminated those protocol-level retry errors in the completed benchmark runs.
- The `5m` run hit Neon's hard storage limit:
  - `pq: could not extend file because project size limit (512 MB) has been exceeded`
- Because of that external limit, `5m` and `10m` could not be benchmarked to completion on the current Neon project.

## Estimated Larger-Run Timing

These are estimates inferred from the completed upgraded runs, assuming similar sustained throughput and no storage cap:

- `5m`: about `118s`
- `10m`: about `236s`

These are estimates, not measured results.

## Optimization Recommendations

1. Keep the `pgx` plus `COPY` path as the primary Postgres ingest implementation.
2. Add a second benchmark pass against a direct Neon connection if you want to isolate pooler overhead from database ingest cost.
3. Keep concurrency near `16` for this database shape unless the DB plan or schema changes.
4. Increase Neon storage quota before attempting `5m+` row benchmarks on the current schema.
5. If you want a stronger systems story, add benchmark result export and chart generation to compare runs automatically.

## Validation

Before running the Neon benchmarks:

```bash
go test ./...
```

The test suite passed.
