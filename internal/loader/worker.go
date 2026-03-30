package loader

import (
	"fmt"
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
	Metrics    *MetricsCollector
}

func NewWorkerPool(database db.BatchInserter, numWorkers int, batchSize int, maxRetries int, retryDelay time.Duration) *WorkerPool {
	return &WorkerPool{
		DB:         database,
		NumWorkers: numWorkers,
		BatchSize:  batchSize,
		MaxRetries: maxRetries,
		RetryDelay: retryDelay,
		Metrics:    NewMetricsCollector(),
	}
}

func (wp *WorkerPool) Start(userChan <-chan models.User) error {
	var wg sync.WaitGroup
	errChan := make(chan error, wp.NumWorkers)

	for i := 0; i < wp.NumWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			if err := wp.workerLoop(workerID, userChan); err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	failedBatches := 0
	for err := range errChan {
		if err != nil {
			failedBatches++
		}
	}
	if failedBatches > 0 {
		return fmt.Errorf("%d worker batch(es) failed permanently", failedBatches)
	}

	return nil
}

func (wp *WorkerPool) workerLoop(workerID int, userChan <-chan models.User) error {
	batch := make([]models.User, 0, wp.BatchSize)
	failedBatches := 0

	for user := range userChan {
		batch = append(batch, user)

		if len(batch) >= wp.BatchSize {
			if err := wp.insertWithRetry(workerID, batch); err != nil {
				failedBatches++
			}
			batch = batch[:0]
		}
	}

	if len(batch) > 0 {
		if err := wp.insertWithRetry(workerID, batch); err != nil {
			failedBatches++
		}
	}

	if failedBatches > 0 {
		return fmt.Errorf("worker %d had %d failed batch(es)", workerID, failedBatches)
	}

	return nil
}

func (wp *WorkerPool) insertWithRetry(workerID int, batch []models.User) error {
	attempts := wp.MaxRetries + 1
	if attempts <= 0 {
		attempts = 1
	}
	batchStartedAt := time.Now()

	for attempt := 1; attempt <= attempts; attempt++ {
		execStartedAt := time.Now()
		err := wp.DB.InsertBatch(batch)
		wp.Metrics.RecordExec(time.Since(execStartedAt))
		if err == nil {
			wp.Metrics.RecordBatchSuccess(len(batch), time.Since(batchStartedAt), attempt-1)
			return nil
		}

		if attempt == attempts {
			log.Printf("worker=%d batch_size=%d failed after %d attempts: %v", workerID, len(batch), attempt, err)
			wp.Metrics.RecordBatchFailure(len(batch), time.Since(batchStartedAt), attempt-1)
			return err
		}

		log.Printf("worker=%d batch_size=%d insert failed (attempt %d/%d): %v", workerID, len(batch), attempt, attempts, err)

		delay := wp.RetryDelay
		if delay <= 0 {
			delay = 100 * time.Millisecond
		}
		time.Sleep(delay * time.Duration(attempt))
	}

	return nil
}

func (wp *WorkerPool) MetricsSnapshot() MetricsSnapshot {
	return wp.Metrics.Snapshot()
}
