package loader

import (
	"errors"
	"testing"
	"time"

	"db_uploader/internal/models"
)

type stubBatchInserter struct {
	inserted int64
	fail     bool
}

func (s *stubBatchInserter) InsertBatch(users []models.User) error {
	if s.fail {
		return errors.New("insert failed")
	}
	s.inserted += int64(len(users))
	return nil
}

func (s *stubBatchInserter) GetTotalInserted() int64 {
	return s.inserted
}

func TestWorkerPoolStartReturnsErrorOnPermanentBatchFailure(t *testing.T) {
	t.Parallel()

	wp := NewWorkerPool(&stubBatchInserter{fail: true}, 1, 2, 1, time.Millisecond)
	userChan := make(chan models.User, 2)
	userChan <- models.User{ID: 1}
	userChan <- models.User{ID: 2}
	close(userChan)

	err := wp.Start(userChan)
	if err == nil {
		t.Fatal("expected worker pool to return an error")
	}
}

func TestWorkerPoolStartSucceedsWhenBatchesInsert(t *testing.T) {
	t.Parallel()

	db := &stubBatchInserter{}
	wp := NewWorkerPool(db, 2, 2, 1, time.Millisecond)
	userChan := make(chan models.User, 3)
	userChan <- models.User{ID: 1}
	userChan <- models.User{ID: 2}
	userChan <- models.User{ID: 3}
	close(userChan)

	err := wp.Start(userChan)
	if err != nil {
		t.Fatalf("expected worker pool to succeed, got error: %v", err)
	}
	if got := db.GetTotalInserted(); got != 3 {
		t.Fatalf("expected 3 inserted records, got %d", got)
	}
}
