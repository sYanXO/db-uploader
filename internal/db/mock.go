package db

import (
	"sync/atomic"
	"time"

	"db_uploader/internal/models"
)

type MockDB struct {
	insertedCount int64
}

func NewMockDB() *MockDB {
	return &MockDB{}
}

func (db *MockDB) InsertBatch(users []models.User) error {
	// Simulate network latency
	time.Sleep(50 * time.Millisecond)

	// Simulate DB work
	count := len(users)
	atomic.AddInt64(&db.insertedCount, int64(count))

	// Optional: Print progress every 1000 records or so if needed, 
	// but main loop will handle logging usually.
	return nil
}

func (db *MockDB) GetTotalInserted() int64 {
	return atomic.LoadInt64(&db.insertedCount)
}
