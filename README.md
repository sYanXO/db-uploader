# DB Uploader

`db-uploader` is a Go bulk-ingest tool for loading user records into either a mock backend or PostgreSQL with concurrent workers and batched inserts.

It supports two main workflows:

- file-driven uploads from a JSON array
- direct synthetic benchmark runs against PostgreSQL

## What It Does

- Streams large JSON files without loading the entire dataset into memory
- Inserts records in batches with configurable worker concurrency
- Retries failed batch inserts with linear backoff
- Reports throughput and latency metrics for upload runs
- Supports a dedicated PostgreSQL benchmark mode

## Current State

This repo is functional for concurrent batch inserts, but it is still a benchmark-oriented project rather than a production-ready ingestion system.

Important constraints from real testing:

- PostgreSQL bulk ingest now uses `pgx` plus `COPY`
- The earlier `lib/pq` path showed intermittent Neon pooler protocol issues, which the upgraded path avoids in current benchmark runs
- Larger runs were blocked by the Neon project storage cap, not by the Go worker pool

Benchmark details are captured in [`BENCHMARK_RESULTS.md`](BENCHMARK_RESULTS.md).

## Repo Layout

- [`cmd/uploader`](cmd/uploader) runs file-based uploads
- [`cmd/generate`](cmd/generate) generates large JSON datasets
- [`cmd/benchmark`](cmd/benchmark) runs direct PostgreSQL ingest benchmarks without JSON files
- [`internal/loader`](internal/loader) contains the worker pool, retry logic, and metrics collection
- [`internal/db`](internal/db) contains the database interface plus mock and PostgreSQL implementations
- [`internal/models`](internal/models) contains the `User` model

## Requirements

- Go `1.22+`
- For PostgreSQL mode: a reachable Postgres database and a DSN

Verify your toolchain:

```bash
go version
```

Validate the repo:

```bash
go test ./...
```

## Data Model

The uploader expects records matching this shape:

```json
{
  "id": 1,
  "name": "User 1",
  "email": "user1@example.com",
  "age": 30,
  "city": "London",
  "created_at": "2026-03-30T08:00:00Z"
}
```

For PostgreSQL, the target table must match:

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

## File-Based Uploads

### 1. Generate Test Data

Generate a JSON array file:

```bash
go run ./cmd/generate -count 100000 -output data.json
```

Defaults:

- output file: `data.json`
- row count: `100000`

### 2. Run Against Mock DB

Use the mock driver to test the worker pool without a real database:

```bash
go run ./cmd/uploader -driver mock -file data.json -workers 10 -batch 100 -progress-interval 1
```

### 3. Run Against PostgreSQL

Set your DSN and upload to a real table:

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

The benchmark runner bypasses JSON files and generates records in memory, which is better when you want to isolate database ingest behavior.

Example:

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

The benchmark runner:

- creates the target benchmark table if needed
- truncates it before each run
- uploads generated records
- prints a JSON result summary

## Metrics Reported

Both uploader mode and benchmark mode now report:

- total rows inserted
- elapsed time
- average throughput
- successful batch count
- failed batch count
- retry count
- batch latency: avg, p50, p95, p99, max
- DB exec latency: avg, p50, p95, p99, max
- `pgxpool` connection stats

## Important Flags

### Shared Upload Flags

- `-workers`: number of worker goroutines
- `-batch`: rows per insert batch
- `-retries`: retries for failed batch inserts
- `-retry-delay-ms`: base retry delay
- `-progress-interval`: seconds between progress logs, `0` disables

### PostgreSQL Flags

- `-dsn`: Postgres connection string
- `-table`: destination table
- `-max-open-conns`: `database/sql` max open connections
- `-max-idle-conns`: `database/sql` max idle connections
- `-conn-max-lifetime-s`: connection lifetime in seconds
- `-conn-max-idle-time-s`: connection idle timeout in seconds

### File Uploader Only

- `-file`: input JSON file path
- `-driver`: `mock` or `postgres`

### Benchmark Only

- `-count`: number of generated rows

## Failure Behavior

This project now fails loudly on real ingest failures.

- Permanent batch failures cause the run to exit non-zero
- File read failures are propagated back to the main process
- Invalid worker and batch configurations are rejected up front

This matters because earlier versions could silently report success while dropping failed batches.

## Observed Performance

On the tested Neon setup, the best measured configuration was:

- `workers=16`
- `batch=500`
- `max-open-conns=16`
- `max-idle-conns=16`

Measured DB-ingest results on the upgraded `pgx` plus `COPY` path:

| Rows | Time | Throughput |
|---|---:|---:|
| 100k | 2.69s | 37.2k rows/s |
| 500k | 12.03s | 41.6k rows/s |
| 1m | 23.89s | 41.9k rows/s |
| 2m | 47.25s | 42.3k rows/s |

For the full write-up, see [`BENCHMARK_RESULTS.md`](BENCHMARK_RESULTS.md).

## Known Limitations

- The project does not yet expose Prometheus metrics
- There is no config file support yet
- MySQL is not implemented
- The current benchmark command generates rows in memory rather than reading files, by design

## Recommended Next Steps

If you want to push this toward production-grade ingest performance:

1. Benchmark against a direct Postgres endpoint if you want to isolate pooler overhead
2. Export benchmark results as JSON or CSV and generate comparison charts
3. Add cancellation and context propagation through reader, workers, and DB calls
4. Keep worker and connection counts aligned with real DB capacity instead of increasing them blindly

## Quick Start

```bash
go test ./...
go run ./cmd/generate -count 100000 -output data.json
go run ./cmd/uploader -driver mock -file data.json -workers 10 -batch 100
```
