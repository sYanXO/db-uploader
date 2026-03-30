package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"db_uploader/internal/db"
	"db_uploader/internal/loader"
	"db_uploader/internal/models"
)

func main() {
	filePath := flag.String("file", "data.json", "Path to JSON file")
	workers := flag.Int("workers", 10, "Number of concurrent workers")
	batchSize := flag.Int("batch", 100, "Batch size for DB insertion")
	driver := flag.String("driver", "mock", "Database backend: mock|postgres")
	dsn := flag.String("dsn", "", "Database connection string (required for postgres)")
	table := flag.String("table", "users", "Destination database table name")
	maxOpenConns := flag.Int("max-open-conns", 0, "Maximum open DB connections for postgres (0 uses driver default)")
	maxIdleConns := flag.Int("max-idle-conns", 0, "Maximum idle DB connections for postgres (0 uses driver default)")
	connMaxLifetimeS := flag.Int("conn-max-lifetime-s", 0, "Maximum connection lifetime in seconds for postgres (0 disables)")
	connMaxIdleTimeS := flag.Int("conn-max-idle-time-s", 0, "Maximum connection idle time in seconds for postgres (0 disables)")
	maxRetries := flag.Int("retries", 3, "Retries for failed DB batch inserts")
	retryDelayMs := flag.Int("retry-delay-ms", 100, "Base retry delay in milliseconds")
	progressInterval := flag.Int("progress-interval", 2, "Progress log interval in seconds (0 disables)")
	flag.Parse()

	if err := validateConfig(*workers, *batchSize, *progressInterval, *maxOpenConns, *maxIdleConns, *connMaxLifetimeS, *connMaxIdleTimeS); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	start := time.Now()

	database, err := buildDatabase(*driver, db.PostgresConfig{
		DSN:             *dsn,
		Table:           *table,
		MaxOpenConns:    *maxOpenConns,
		MaxIdleConns:    *maxIdleConns,
		ConnMaxLifetime: time.Duration(*connMaxLifetimeS) * time.Second,
		ConnMaxIdleTime: time.Duration(*connMaxIdleTimeS) * time.Second,
	})
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	if closer, ok := database.(interface{ Close() error }); ok {
		defer func() {
			if err := closer.Close(); err != nil {
				log.Printf("Error closing database connection: %v", err)
			}
		}()
	}

	workerPool := loader.NewWorkerPool(database, *workers, *batchSize, *maxRetries, time.Duration(*retryDelayMs)*time.Millisecond)
	userChan := make(chan models.User, *workers*2)

	stopProgress := make(chan struct{})
	if *progressInterval > 0 {
		go logProgress(database, start, time.Duration(*progressInterval)*time.Second, stopProgress)
	}

	readErrChan := make(chan error, 1)
	go func() {
		log.Printf("Reading from %s...", *filePath)
		readErrChan <- loader.ReadUsers(*filePath, userChan)
	}()

	log.Printf("Starting %d workers...", *workers)
	workerErr := workerPool.Start(userChan)
	readErr := <-readErrChan

	close(stopProgress)

	if readErr != nil {
		log.Fatalf("Error reading file: %v", readErr)
	}
	if workerErr != nil {
		log.Fatalf("Upload failed: %v", workerErr)
	}

	elapsed := time.Since(start)
	total := database.GetTotalInserted()
	rps := float64(total) / elapsed.Seconds()
	metrics := workerPool.MetricsSnapshot()

	fmt.Printf("\nUpload complete!\n")
	fmt.Printf("Total Records: %d\n", total)
	fmt.Printf("Time Elapsed: %s\n", elapsed)
	fmt.Printf("Throughput: %.2f records/sec\n", rps)
	fmt.Printf("Successful Batches: %d\n", metrics.SuccessfulBatches)
	fmt.Printf("Failed Batches: %d\n", metrics.FailedBatches)
	fmt.Printf("Retries: %d\n", metrics.RetryCount)
	fmt.Printf("Batch Latency: avg=%s p50=%s p95=%s p99=%s max=%s\n", metrics.BatchLatencyAvg, metrics.BatchLatencyP50, metrics.BatchLatencyP95, metrics.BatchLatencyP99, metrics.BatchLatencyMax)
	fmt.Printf("DB Exec Latency: avg=%s p50=%s p95=%s p99=%s max=%s\n", metrics.ExecLatencyAvg, metrics.ExecLatencyP50, metrics.ExecLatencyP95, metrics.ExecLatencyP99, metrics.ExecLatencyMax)
	printDBStats(database)
}

func buildDatabase(driver string, config db.PostgresConfig) (db.BatchInserter, error) {
	switch strings.ToLower(driver) {
	case "mock":
		return db.NewMockDB(), nil
	case "postgres":
		return db.NewPostgresDBWithConfig(config)
	default:
		return nil, fmt.Errorf("unsupported driver %q (use mock or postgres)", driver)
	}
}

func validateConfig(workers int, batchSize int, progressInterval int, maxOpenConns int, maxIdleConns int, connMaxLifetimeS int, connMaxIdleTimeS int) error {
	if workers <= 0 {
		return fmt.Errorf("workers must be greater than 0")
	}
	if batchSize <= 0 {
		return fmt.Errorf("batch size must be greater than 0")
	}
	if progressInterval < 0 {
		return fmt.Errorf("progress interval must be 0 or greater")
	}
	if maxOpenConns < 0 {
		return fmt.Errorf("max open conns must be 0 or greater")
	}
	if maxIdleConns < 0 {
		return fmt.Errorf("max idle conns must be 0 or greater")
	}
	if connMaxLifetimeS < 0 {
		return fmt.Errorf("conn max lifetime must be 0 or greater")
	}
	if connMaxIdleTimeS < 0 {
		return fmt.Errorf("conn max idle time must be 0 or greater")
	}

	return nil
}

func logProgress(database db.BatchInserter, start time.Time, interval time.Duration, stop <-chan struct{}) {
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

func printDBStats(database db.BatchInserter) {
	statsProvider, ok := database.(interface{ GetSQLStats() sql.DBStats })
	if !ok {
		return
	}

	stats := statsProvider.GetSQLStats()
	fmt.Printf("DB Pool: max_open=%d open=%d in_use=%d idle=%d wait_count=%d wait_duration=%s max_idle_closed=%d max_idle_time_closed=%d max_lifetime_closed=%d\n",
		stats.MaxOpenConnections,
		stats.OpenConnections,
		stats.InUse,
		stats.Idle,
		stats.WaitCount,
		stats.WaitDuration,
		stats.MaxIdleClosed,
		stats.MaxIdleTimeClosed,
		stats.MaxLifetimeClosed,
	)
}
