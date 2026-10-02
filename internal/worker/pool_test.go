package worker

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/pkg/logger"
)

func init() {
	logger.Init()
}

func TestWorkerPoolTableDriven(t *testing.T) {
	tests := []struct {
		name         string
		workerCount  int
		queueSize    int
		jobCount     int
		shouldFail   bool
		failUntil    int32
		maxRetries   int
		expectedDone int32
	}{
		{
			name:         "All jobs succeed on first attempt",
			workerCount:  3,
			queueSize:    10,
			jobCount:     10,
			shouldFail:   false,
			maxRetries:   2,
			expectedDone: 10,
		},
		{
			name:         "Jobs retry with exponential backoff and eventually succeed",
			workerCount:  2,
			queueSize:    5,
			jobCount:     3,
			shouldFail:   true,
			failUntil:    2, // fails 2 times then succeeds
			maxRetries:   3,
			expectedDone: 3,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pool := NewPool(tc.workerCount, tc.queueSize)
			pool.Start()

			var completedJobs int32
			var attemptsPerJob sync.Map

			for i := 0; i < tc.jobCount; i++ {
				orderID := string(rune('A' + i))
				job := Job{
					Event: model.OrderEvent{
						OrderID:   orderID,
						UserID:    "user-1",
						Status:    model.OrderStatusConfirmed,
						Timestamp: time.Now(),
					},
					MaxRetries: tc.maxRetries,
					Timeout:    2 * time.Second,
					Handler: func(ctx context.Context, evt model.OrderEvent) error {
						val, _ := attemptsPerJob.LoadOrStore(evt.OrderID, new(int32))
						countPtr := val.(*int32)
						currentAttempt := atomic.AddInt32(countPtr, 1)

						if tc.shouldFail && currentAttempt <= tc.failUntil {
							return errors.New("simulated transient failure")
						}
						atomic.AddInt32(&completedJobs, 1)
						return nil
					},
				}

				if err := pool.Submit(job); err != nil {
					t.Fatalf("Failed to submit job %d: %v", i, err)
				}
			}

			// Graceful shutdown with 5s timeout
			if err := pool.Stop(5 * time.Second); err != nil {
				t.Fatalf("Stop error: %v", err)
			}

			finalCompleted := atomic.LoadInt32(&completedJobs)
			if finalCompleted != tc.expectedDone {
				t.Errorf("Expected %d completed jobs, got %d", tc.expectedDone, finalCompleted)
			}
		})
	}
}
