package db

import "db_uploader/internal/models"

// BatchInserter defines the behavior needed by the worker pool.
type BatchInserter interface {
	InsertBatch(users []models.User) error
	GetTotalInserted() int64
}
