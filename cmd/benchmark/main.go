package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"db_uploader/internal/db"
	"db_uploader/internal/loader"
	"db_uploader/internal/models"
)

type BenchmarkResult struct {
	Count         int                    `json:"count"`
	Workers       int                    `json:"workers"`
	BatchSize     int                    `json:"batchSize"`
	MaxOpenConns  int                    `json:"maxOpenConns"`
	MaxIdleConns  int                    `json:"maxIdleConns"`
	Elapsed       time.Duration          `json:"elapsed"`
	RecordsPerSec float64                `json:"recordsPerSec"`
	BatchesPerSec float64                `json:"batchesPerSec"`
	Metrics       loader.MetricsSnapshot `json:"metrics"`
	DBStats       db.PoolStats           `json:"dbStats"`
	StartedAt     time.Time              `json:"startedAt"`
	CompletedAt   time.Time              `json:"completedAt"`
	Table         string                 `json:"table"`
}

func main() {
	count := flag.Int("count", 100000, "Number of records to upload")
	workers := flag.Int("workers", 10, "Number of concurrent workers")
	batchSize := flag.Int("batch", 1000, "Batch size for DB insertion")
	dsn := flag.String("dsn", "", "Postgres connection string")
	table := flag.String("table", "users_benchmark", "Destination benchmark table")
	maxOpenConns := flag.Int("max-open-conns", 10, "Maximum open DB connections")
	maxIdleConns := flag.Int("max-idle-conns", 10, "Maximum idle DB connections")
	connMaxLifetimeS := flag.Int("conn-max-lifetime-s", 0, "Maximum connection lifetime in seconds")
	connMaxIdleTimeS := flag.Int("conn-max-idle-time-s", 0, "Maximum connection idle time in seconds")
	maxRetries := flag.Int("retries", 3, "Retries for failed DB batch inserts")
	retryDelayMs := flag.Int("retry-delay-ms", 100, "Base retry delay in milliseconds")
	progressInterval := flag.Int("progress-interval", 2, "Progress log interval in seconds (0 disables)")
	flag.Parse()

	cfg := db.PostgresConfig{
		DSN:             *dsn,
		Table:           *table,
		MaxOpenConns:    *maxOpenConns,
		MaxIdleConns:    *maxIdleConns,
		ConnMaxLifetime: time.Duration(*connMaxLifetimeS) * time.Second,
		ConnMaxIdleTime: time.Duration(*connMaxIdleTimeS) * time.Second,
	}

	if err := validateBenchmarkConfig(*count, *workers, *batchSize, *progressInterval, cfg); err != nil {
		log.Fatalf("Invalid benchmark configuration: %v", err)
	}

	database, err := db.NewPostgresDBWithConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize postgres: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		}
	}()

	if err := database.EnsureBenchmarkTable(); err != nil {
		log.Fatalf("Failed to ensure benchmark table: %v", err)
	}
	if err := database.Truncate(); err != nil {
		log.Fatalf("Failed to truncate benchmark table: %v", err)
	}

	workerPool := loader.NewWorkerPool(database, *workers, *batchSize, *maxRetries, time.Duration(*retryDelayMs)*time.Millisecond)
	userChan := make(chan models.User, *workers*2)

	startedAt := time.Now()
	stopProgress := make(chan struct{})
	if *progressInterval > 0 {
		go logProgress(database, startedAt, time.Duration(*progressInterval)*time.Second, stopProgress)
	}

	go func() {
		generateUsers(*count, userChan)
	}()

	if err := workerPool.Start(userChan); err != nil {
		close(stopProgress)
		log.Fatalf("Benchmark upload failed: %v", err)
	}
	close(stopProgress)

	completedAt := time.Now()
	elapsed := completedAt.Sub(startedAt)
	total := database.GetTotalInserted()
	metrics := workerPool.MetricsSnapshot()

	result := BenchmarkResult{
		Count:         *count,
		Workers:       *workers,
		BatchSize:     *batchSize,
		MaxOpenConns:  *maxOpenConns,
		MaxIdleConns:  *maxIdleConns,
		Elapsed:       elapsed,
		RecordsPerSec: float64(total) / elapsed.Seconds(),
		BatchesPerSec: float64(metrics.SuccessfulBatches) / elapsed.Seconds(),
		Metrics:       metrics,
		DBStats:       database.GetPoolStats(),
		StartedAt:     startedAt.UTC(),
		CompletedAt:   completedAt.UTC(),
		Table:         *table,
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		log.Fatalf("Failed to encode benchmark result: %v", err)
	}

	fmt.Println(string(encoded))
}

func validateBenchmarkConfig(count int, workers int, batchSize int, progressInterval int, cfg db.PostgresConfig) error {
	if count <= 0 {
		return fmt.Errorf("count must be greater than 0")
	}
	if workers <= 0 {
		return fmt.Errorf("workers must be greater than 0")
	}
	if batchSize <= 0 {
		return fmt.Errorf("batch size must be greater than 0")
	}
	if progressInterval < 0 {
		return fmt.Errorf("progress interval must be 0 or greater")
	}
	if cfg.MaxOpenConns < 0 {
		return fmt.Errorf("max open conns must be 0 or greater")
	}
	if cfg.MaxIdleConns < 0 {
		return fmt.Errorf("max idle conns must be 0 or greater")
	}
	if cfg.ConnMaxLifetime < 0 {
		return fmt.Errorf("conn max lifetime must be 0 or greater")
	}
	if cfg.ConnMaxIdleTime < 0 {
		return fmt.Errorf("conn max idle time must be 0 or greater")
	}
	return nil
}

func generateUsers(count int, userChan chan<- models.User) {
	defer close(userChan)

	start := time.Now().UTC()
	for i := 0; i < count; i++ {
		userChan <- models.User{
			ID:        i + 1,
			Name:      fmt.Sprintf("Benchmark User %d", i+1),
			Email:     fmt.Sprintf("benchmark-user-%d@example.com", i+1),
			Age:       18 + (i % 60),
			City:      cities[i%len(cities)],
			CreatedAt: start.Add(time.Duration(i) * time.Second).Format(time.RFC3339),
		}
	}
}

var cities = []string{
	"New York",
	"London",
	"Tokyo",
	"Paris",
	"Berlin",
	"Sydney",
	"Mumbai",
	"Toronto",
}

func logProgress(database interface{ GetTotalInserted() int64 }, start time.Time, interval time.Duration, stop <-chan struct{}) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			inserted := database.GetTotalInserted()
			elapsed := time.Since(start).Seconds()
			if elapsed <= 0 {
				continue
			}
			rate := float64(inserted) / elapsed
			log.Printf("progress inserted=%d rate=%.2f records/sec", inserted, rate)
		case <-stop:
			return
		}
	}
}
