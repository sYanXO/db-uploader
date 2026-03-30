# DB Uploader

`db-uploader` is a Go bulk-ingest project for loading user records into PostgreSQL with concurrent workers, bounded batches, and stream-based JSON processing.

It supports two workflows:

- file-based uploads from a JSON array
- direct benchmark runs that generate records in memory

## Performance Highlights

The PostgreSQL path was upgraded from `lib/pq` plus batched `INSERT ... VALUES` to `pgx` plus `COPY`.

| Rows | Old Throughput | New Throughput | Improvement |
|---|---:|---:|---:|
| 100k | 24.3k rows/s | 37.2k rows/s | 1.53x |
| 500k | 28.8k rows/s | 41.6k rows/s | 1.44x |
| 1m | 29.2k rows/s | 41.9k rows/s | 1.43x |

- Throughput improved by `43%` to `53%`.
- `1m` upload time dropped from `34.28s` to `23.89s`.
- `1m` latency tails improved from `p95=503ms / p99=741ms` to `p95=309ms / p99=523ms`.
- The current scaling limit is Neon storage quota, not Go worker concurrency.

Detailed benchmark data lives in [`BENCHMARK_RESULTS.md`](BENCHMARK_RESULTS.md).

## What It Does

- Streams JSON without loading the full file into memory
- Inserts records with configurable worker concurrency and batch size
- Retries failed batch writes with linear backoff
- Reports throughput, latency percentiles, retry counts, and pool stats
- Provides a benchmark runner for database-only ingest testing

## Repo Layout

- [`cmd/uploader`](cmd/uploader) runs file-based uploads
- [`cmd/generate`](cmd/generate) generates JSON test data
- [`cmd/benchmark`](cmd/benchmark) runs direct PostgreSQL benchmarks
- [`internal/loader`](internal/loader) contains batching, retries, and metrics
- [`internal/db`](internal/db) contains mock and PostgreSQL backends

## Quick Start

Validate the repo:

```bash
go test ./...
```

Generate test data:

```bash
go run ./cmd/generate -count 100000 -output data.json
```

Run the mock backend:

```bash
go run ./cmd/uploader -driver mock -file data.json -workers 10 -batch 100
```

## PostgreSQL Upload

Expected table:

```sql
CREATE TABLE IF NOT EXISTS users (
    id BIGINT PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    age INT NOT NULL,
    city TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);
```

Run an upload:

```bash
export DATABASE_URL='postgres://user:pass@localhost:5432/mydb?sslmode=disable'

go run ./cmd/uploader \
  -driver postgres \
  -dsn "$DATABASE_URL" \
  -table users \
  -file data.json \
  -workers 16 \
  -batch 500 \
  -max-open-conns 16 \
  -max-idle-conns 16 \
  -progress-interval 1
```

## Benchmark Mode

The benchmark command skips file I/O and generates users in memory, which makes it better for isolating database ingest behavior.

```bash
export DATABASE_URL='postgres://user:pass@localhost:5432/mydb?sslmode=disable'

go run ./cmd/benchmark \
  -dsn "$DATABASE_URL" \
  -table users_benchmark \
  -count 100000 \
  -workers 16 \
  -batch 500 \
  -max-open-conns 16 \
  -max-idle-conns 16 \
  -progress-interval 0
```

The benchmark command creates the table if needed, truncates it before each run, uploads generated rows, and prints a JSON summary.

## Metrics

Both uploader mode and benchmark mode report:

- total rows inserted
- elapsed time and throughput
- successful and failed batch counts
- retry count
- batch latency: avg, p50, p95, p99, max
- DB exec latency: avg, p50, p95, p99, max
- `pgxpool` connection stats

## Important Flags

- `-workers`: worker goroutines
- `-batch`: rows per batch
- `-retries`: retries for failed batch writes
- `-retry-delay-ms`: base retry delay
- `-progress-interval`: seconds between progress logs
- `-dsn`: Postgres connection string
- `-table`: target table
- `-max-open-conns`: pool max connections
- `-max-idle-conns`: pool minimum warm connections
- `-conn-max-lifetime-s`: connection lifetime in seconds
- `-conn-max-idle-time-s`: idle connection timeout in seconds

Uploader-only:

- `-file`
- `-driver`

Benchmark-only:

- `-count`

## Notes

- PostgreSQL now uses `pgx` plus `COPY`
- The old `lib/pq` path had Neon pooler protocol issues that the new path avoids in current benchmark runs
- Runs above roughly the current Neon storage cap are blocked by the database plan, not the Go pipeline
- MySQL is not implemented yet
