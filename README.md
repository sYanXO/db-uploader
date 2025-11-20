# DB Uploader

A high-performance Go application designed to efficiently read large JSON datasets and upload them to a database using concurrent workers.

## 🚀 Features

### Current Features
- **High-Performance Loading**: Stream-based JSON reading to handle files larger than available RAM.
- **Concurrency**: Configurable worker pool to process records in parallel.
- **Batch Processing**: efficient database insertion using configurable batch sizes.
- **Data Generator**: Built-in utility to generate massive test datasets.
- **Mock Database**: Currently integrates with a mock database for testing throughput and logic.

### 🗺️ Roadmap
The following features are currently in development or planned for future releases:
- [ ] **Real Database Integration**: Support for PostgreSQL and MySQL.
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
```

## 📂 Project Structure

- `cmd/uploader`: Main entry point for the data uploader.
- `cmd/generate`: Utility to generate random test data.
- `internal/loader`: Core logic for file reading and worker pool management.
- `internal/db`: Database interface and mock implementation.
- `internal/models`: Data structures.
