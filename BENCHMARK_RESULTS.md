# Benchmark Results

## Resume Summary

- Upgraded the PostgreSQL ingest path from `lib/pq` plus batched `INSERT ... VALUES` to `pgx` plus `COPY`
- Improved measured throughput by `43%` to `53%`
- Reduced `1m` row upload time from `34.28s` to `23.89s`
- Reduced `1m` batch latency tails from `p95=503ms / p99=741ms` to `p95=309ms / p99=523ms`
- Identified the main scaling limit as Neon storage quota rather than Go worker concurrency

## Scope

These numbers are from DB-ingest benchmarks against a real Neon PostgreSQL database.

This is not a file-read benchmark. The benchmark command generates rows in memory and measures the database path.

## Best Configuration

- `workers=16`
- `batch=500`
- `max-open-conns=16`
- `max-idle-conns=16`

## Before vs After

| Rows | Old Time | New Time | Time Change | Old Throughput | New Throughput | Throughput Change |
|---|---:|---:|---:|---:|---:|---:|
| 100k | 4.11s | 2.69s | -34.5% | 24.3k rows/s | 37.2k rows/s | +53.1% / 1.53x |
| 500k | 17.34s | 12.03s | -30.6% | 28.8k rows/s | 41.6k rows/s | +44.4% / 1.44x |
| 1m | 34.28s | 23.89s | -30.3% | 29.2k rows/s | 41.9k rows/s | +43.5% / 1.43x |

## Current Results

| Rows | Time | Throughput | Batch p50 | Batch p95 | Batch p99 | Result |
|---|---:|---:|---:|---:|---:|---|
| 100k | 2.69s | 37.2k rows/s | 180ms | 443ms | 550ms | completed |
| 500k | 12.03s | 41.6k rows/s | 183ms | 365ms | 541ms | completed |
| 1m | 23.89s | 41.9k rows/s | 184ms | 309ms | 523ms | completed |
| 2m | 47.25s | 42.3k rows/s | 186ms | 269ms | 489ms | completed |
| 5m | blocked | blocked | n/a | n/a | n/a | Neon storage cap |
| 10m | not runnable | not runnable | n/a | n/a | n/a | Neon storage cap |

## Key Takeaways

- Throughput scales cleanly through at least `2m` rows on the upgraded path
- `pgx` plus `COPY` removed the protocol-level retry problems seen with the old `lib/pq` pooler path
- `16` workers beat `8`, and `32` was worse than `16`
- Batch size `500` beat `2000` on this setup
- The next real bottleneck is database/storage limits, not application concurrency

## Constraints

- The old `lib/pq` plus Neon pooler path intermittently produced prepared-statement and statement-shape errors
- The upgraded `pgx` plus `COPY` path did not show those errors in the completed runs
- The `5m` run hit Neon’s `512 MB` project storage limit

## Estimated Larger Runs

Assuming similar sustained throughput and no storage cap:

- `5m`: about `118s`
- `10m`: about `236s`

## Validation

```bash
go test ./...
```

The test suite passed before the benchmark runs.
