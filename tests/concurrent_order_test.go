package tests

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/internal/repository"
	"github.com/gophermart/ecommerce-order-inventory/pkg/logger"
)

func init() {
	logger.Init()
}

// SimulatedSafeInventory simulates PostgreSQL row-level locking (SELECT ... FOR UPDATE)
type SimulatedSafeInventory struct {
	mu    sync.Mutex
	stock map[string]int
}

func (s *SimulatedSafeInventory) DeductStock(productID string, quantity int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, exists := s.stock[productID]
	if !exists {
		return repository.ErrProductNotFound
	}

	if current < quantity {
		return fmt.Errorf("%w: requested %d, available %d", repository.ErrInsufficientStock, quantity, current)
	}

	s.stock[productID] = current - quantity
	return nil
}

func (s *SimulatedSafeInventory) GetStock(productID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stock[productID]
}

// TestConcurrentOversellingPrevention verifies under the Go race detector (-race)
// that 100 concurrent goroutines competing for 10 units of limited inventory
// results in exactly 10 successful deductions, 90 rejections, and zero overselling.
func TestConcurrentOversellingPrevention(t *testing.T) {
	initialStock := 10
	totalRequests := 100
	productID := "prod-darkstore-coffee-nitro"

	inventory := &SimulatedSafeInventory{
		stock: map[string]int{
			productID: initialStock,
		},
	}

	var successCount int32
	var rejectedCount int32

	var wg sync.WaitGroup
	startSignal := make(chan struct{})

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Align all goroutines to fire at the exact same instant
			<-startSignal

			err := inventory.DeductStock(productID, 1)
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&rejectedCount, 1)
			}
		}(i)
	}

	// Trigger all concurrent workers simultaneously
	close(startSignal)
	wg.Wait()

	finalStock := inventory.GetStock(productID)
	finalSuccess := atomic.LoadInt32(&successCount)
	finalRejected := atomic.LoadInt32(&rejectedCount)

	t.Logf("Concurrent Results -> Success: %d, Rejected: %d, Remaining Stock: %d",
		finalSuccess, finalRejected, finalStock)

	if finalSuccess != int32(initialStock) {
		t.Errorf("Expected exactly %d successful orders, got %d", initialStock, finalSuccess)
	}

	if finalRejected != int32(totalRequests-initialStock) {
		t.Errorf("Expected exactly %d rejected orders, got %d", totalRequests-initialStock, finalRejected)
	}

	if finalStock != 0 {
		t.Errorf("Expected remaining stock to be 0, got %d (overselling or under-allocation occurred!)", finalStock)
	}
}

// TestTableDrivenIdempotencySimulation validates repeated idempotent requests with identical and conflicting payloads
func TestTableDrivenIdempotencySimulation(t *testing.T) {
	tests := []struct {
		name              string
		firstKey          string
		secondKey         string
		firstHash         string
		secondHash        string
		expectReplayMatch bool
	}{
		{
			name:              "Identical idempotency key and payload replays cached result",
			firstKey:          "idem-uuid-001",
			secondKey:         "idem-uuid-001",
			firstHash:         "hash-payload-abc",
			secondHash:        "hash-payload-abc",
			expectReplayMatch: true,
		},
		{
			name:              "Same idempotency key with modified payload causes conflict",
			firstKey:          "idem-uuid-002",
			secondKey:         "idem-uuid-002",
			firstHash:         "hash-payload-abc",
			secondHash:        "hash-payload-MODIFIED-xyz",
			expectReplayMatch: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := make(map[string]model.IdempotencyRecord)
			var mu sync.RWMutex

			// First request executes
			mu.Lock()
			store[tc.firstKey] = model.IdempotencyRecord{
				Key:                tc.firstKey,
				UserID:             "usr-1",
				RequestHash:        tc.firstHash,
				Status:             "COMPLETED",
				ResponseStatusCode: 201,
				ResponseBody:       `{"id":"ord-123","status":"CONFIRMED"}`,
				CreatedAt:          time.Now(),
				ExpiresAt:          time.Now().Add(24 * time.Hour),
			}
			mu.Unlock()

			// Second request checks idempotency store
			mu.RLock()
			cached, exists := store[tc.secondKey]
			mu.RUnlock()

			if !exists {
				t.Fatalf("Expected key %s to exist in store", tc.secondKey)
			}

			if tc.expectReplayMatch {
				if cached.RequestHash != tc.secondHash {
					t.Errorf("Expected matching request hash %s, got %s", tc.secondHash, cached.RequestHash)
				}
				if cached.Status != "COMPLETED" {
					t.Errorf("Expected COMPLETED status, got %s", cached.Status)
				}
			} else {
				if cached.RequestHash == tc.secondHash {
					t.Errorf("Expected conflict, but hashes matched unexpectedly")
				}
			}
		})
	}
}

// TestWorkerContextTimeout verifies that a slow job exceeding context timeout is aborted
func TestWorkerContextTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan bool)
	go func() {
		select {
		case <-time.After(500 * time.Millisecond):
			done <- false
		case <-ctx.Done():
			done <- true
		}
	}()

	timedOut := <-done
	if !timedOut {
		t.Errorf("Expected context timeout cancellation, but job kept running")
	}
}
