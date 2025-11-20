package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"db_uploader/internal/db"
	"db_uploader/internal/loader"
	"db_uploader/internal/models"
)

func main() {
	filePath := flag.String("file", "data.json", "Path to JSON file")
	workers := flag.Int("workers", 10, "Number of concurrent workers")
	batchSize := flag.Int("batch", 100, "Batch size for DB insertion")
	flag.Parse()

	start := time.Now()

	// Initialize components
	mockDB := db.NewMockDB()
	workerPool := loader.NewWorkerPool(mockDB, *workers, *batchSize)
	userChan := make(chan models.User, *workers*2)

	// Start reader in a goroutine
	go func() {
		log.Printf("Reading from %s...", *filePath)
		if err := loader.ReadUsers(*filePath, userChan); err != nil {
			log.Fatalf("Error reading file: %v", err)
		}
	}()

	// Start workers (blocks until channel is closed and all workers are done)
	log.Printf("Starting %d workers...", *workers)
	workerPool.Start(userChan)

	elapsed := time.Since(start)
	total := mockDB.GetTotalInserted()
	rps := float64(total) / elapsed.Seconds()

	fmt.Printf("\nUpload complete!\n")
	fmt.Printf("Total Records: %d\n", total)
	fmt.Printf("Time Elapsed: %s\n", elapsed)
	fmt.Printf("Throughput: %.2f records/sec\n", rps)
}
