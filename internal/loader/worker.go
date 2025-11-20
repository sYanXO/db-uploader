package loader

import (
	"log"
	"sync"

	"db_uploader/internal/db"
	"db_uploader/internal/models"
)

type WorkerPool struct {
	DB        *db.MockDB
	NumWorkers int
	BatchSize  int
}

func NewWorkerPool(db *db.MockDB, numWorkers int, batchSize int) *WorkerPool {
	return &WorkerPool{
		DB:        db,
		NumWorkers: numWorkers,
		BatchSize:  batchSize,
	}
}

func (wp *WorkerPool) Start(userChan <-chan models.User) {
	var wg sync.WaitGroup

	for i := 0; i < wp.NumWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			wp.workerLoop(userChan)
		}(i)
	}

	wg.Wait()
}

func (wp *WorkerPool) workerLoop(userChan <-chan models.User) {
	batch := make([]models.User, 0, wp.BatchSize)

	for user := range userChan {
		batch = append(batch, user)

		if len(batch) >= wp.BatchSize {
			if err := wp.DB.InsertBatch(batch); err != nil {
				log.Printf("Error inserting batch: %v", err)
			}
			// Clear batch (keep capacity)
			batch = batch[:0]
		}
	}

	// Insert remaining items
	if len(batch) > 0 {
		if err := wp.DB.InsertBatch(batch); err != nil {
			log.Printf("Error inserting remaining batch: %v", err)
		}
	}
}
