# DB Uploader

A high-performance Go application designed to efficiently read large JSON datasets and upload them to a database using concurrent workers.

## 🚀 Features

### Current Features
- **High-Performance Loading**: Stream-based JSON reading to handle files larger than available RAM.
- **Concurrency**: Configurable worker pool to process records in parallel.
- **Batch Processing**: efficient database insertion using configurable batch sizes.
- **Retry Handling**: Configurable retry and backoff for transient batch insert failures.
- **Progress Logs**: Periodic throughput and inserted-record logging during long uploads.
- **Data Generator**: Built-in utility to generate massive test datasets.
- **Pluggable Backends**: `mock` and `postgres` database drivers.

### 🗺️ Roadmap
The following features are currently in development or planned for future releases:
- [x] **PostgreSQL Integration**: Upload directly into a live PostgreSQL table via DSN.
- [ ] **MySQL Integration**: Add MySQL backend support.
- [ ] **Error Handling**: Robust retry mechanisms for failed batches.
- [ ] **Progress Visualization**: Real-time progress bar and ETA.
- [ ] **Configuration**: YAML/Env file support for easier configuration.
- [ ] **Docker Support**: Containerization for easy deployment.
- [ ] **Metrics**: Prometheus metrics for monitoring upload performance.

## 🛠️ Usage

### 1. Generate Test Data
First, generate a large JSON file to test the uploader.

```bash
# Generate 100,000 records (default)
go run cmd/generate/main.go

# Generate 1 million records to a specific file
go run cmd/generate/main.go -count 1000000 -output large_data.json
```

### 2. Run the Uploader
Run the uploader to process the generated file.

```bash
# Run with default settings (10 workers, batch size 100)
go run cmd/uploader/main.go

# Run with custom settings for higher performance
go run cmd/uploader/main.go -file large_data.json -workers 50 -batch 1000

# Run with custom retry + progress settings
go run cmd/uploader/main.go -file large_data.json -workers 50 -batch 1000 -retries 5 -retry-delay-ms 200 -progress-interval 1

# Upload to PostgreSQL (set your DSN first)
export DATABASE_URL='postgres://user:pass@localhost:5432/mydb?sslmode=disable'
go run cmd/uploader/main.go -driver postgres -dsn "$DATABASE_URL" -table users -file large_data.json -workers 20 -batch 500
```

### PostgreSQL Schema
Create a destination table before running:

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

## 📂 Project Structure

- `cmd/uploader`: Main entry point for the data uploader.
- `cmd/generate`: Utility to generate random test data.
- `internal/loader`: Core logic for file reading and worker pool management.
- `internal/db`: Database interface and mock implementation.
- `internal/models`: Data structures.
