package main

import (
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
	maxRetries := flag.Int("retries", 3, "Retries for failed DB batch inserts")
	retryDelayMs := flag.Int("retry-delay-ms", 100, "Base retry delay in milliseconds")
	progressInterval := flag.Int("progress-interval", 2, "Progress log interval in seconds (0 disables)")
	flag.Parse()

	start := time.Now()

	database, err := buildDatabase(*driver, *dsn, *table)
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

	go func() {
		log.Printf("Reading from %s...", *filePath)
		if err := loader.ReadUsers(*filePath, userChan); err != nil {
			log.Fatalf("Error reading file: %v", err)
		}
	}()

	log.Printf("Starting %d workers...", *workers)
	workerPool.Start(userChan)

	close(stopProgress)

	elapsed := time.Since(start)
	total := database.GetTotalInserted()
	rps := float64(total) / elapsed.Seconds()

	fmt.Printf("\nUpload complete!\n")
	fmt.Printf("Total Records: %d\n", total)
	fmt.Printf("Time Elapsed: %s\n", elapsed)
	fmt.Printf("Throughput: %.2f records/sec\n", rps)
}

func buildDatabase(driver string, dsn string, table string) (db.BatchInserter, error) {
	switch strings.ToLower(driver) {
	case "mock":
		return db.NewMockDB(), nil
	case "postgres":
		return db.NewPostgresDB(dsn, table)
	default:
		return nil, fmt.Errorf("unsupported driver %q (use mock or postgres)", driver)
	}
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
