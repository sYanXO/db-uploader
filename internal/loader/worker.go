package loader

import (
	"log"
	"sync"
	"time"

	"db_uploader/internal/db"
	"db_uploader/internal/models"
)

type WorkerPool struct {
	DB         db.BatchInserter
	NumWorkers int
	BatchSize  int
	MaxRetries int
	RetryDelay time.Duration
}

func NewWorkerPool(database db.BatchInserter, numWorkers int, batchSize int, maxRetries int, retryDelay time.Duration) *WorkerPool {
	return &WorkerPool{
		DB:         database,
		NumWorkers: numWorkers,
		BatchSize:  batchSize,
		MaxRetries: maxRetries,
		RetryDelay: retryDelay,
	}
}

func (wp *WorkerPool) Start(userChan <-chan models.User) {
	var wg sync.WaitGroup

	for i := 0; i < wp.NumWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			wp.workerLoop(workerID, userChan)
		}(i)
	}

	wg.Wait()
}

func (wp *WorkerPool) workerLoop(workerID int, userChan <-chan models.User) {
	batch := make([]models.User, 0, wp.BatchSize)

	for user := range userChan {
		batch = append(batch, user)

		if len(batch) >= wp.BatchSize {
			wp.insertWithRetry(workerID, batch)
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		wp.insertWithRetry(workerID, batch)
	}
}

func (wp *WorkerPool) insertWithRetry(workerID int, batch []models.User) {
	attempts := wp.MaxRetries + 1
	if attempts <= 0 {
		attempts = 1
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		err := wp.DB.InsertBatch(batch)
		if err == nil {
			return
		}

		if attempt == attempts {
			log.Printf("worker=%d batch_size=%d failed after %d attempts: %v", workerID, len(batch), attempt, err)
			return
		}

		log.Printf("worker=%d batch_size=%d insert failed (attempt %d/%d): %v", workerID, len(batch), attempt, attempts, err)

		delay := wp.RetryDelay
		if delay <= 0 {
			delay = 100 * time.Millisecond
		}
		time.Sleep(delay * time.Duration(attempt))
	}
}
