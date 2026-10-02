package worker

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/gophermart/ecommerce-order-inventory/internal/model"
	"github.com/gophermart/ecommerce-order-inventory/pkg/logger"
)

var (
	ErrPoolClosed = errors.New("worker pool is stopped and cannot accept new jobs")
	ErrQueueFull  = errors.New("worker job queue is full")
)

type JobHandler func(ctx context.Context, event model.OrderEvent) error

type Job struct {
	Event      model.OrderEvent
	Handler    JobHandler
	MaxRetries int
	Timeout    time.Duration
}

type Pool struct {
	workerCount int
	jobQueue    chan Job
	wg          sync.WaitGroup
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.RWMutex
	closed      bool
}

func NewPool(workerCount, queueSize int) *Pool {
	ctx, cancel := context.WithCancel(context.Background())
	return &Pool{
		workerCount: workerCount,
		jobQueue:    make(chan Job, queueSize),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start launches the fixed bounded number of worker goroutines
func (p *Pool) Start() {
	for i := 1; i <= p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
	logger.Info("Worker pool started", "workers", p.workerCount, "queue_capacity", cap(p.jobQueue))
}

func (p *Pool) worker(id int) {
	defer p.wg.Done()
	logger.Debug("Worker started", "worker_id", id)

	for job := range p.jobQueue {
		p.processJobWithRetry(id, job)
	}

	logger.Debug("Worker stopped", "worker_id", id)
}

func (p *Pool) processJobWithRetry(workerID int, job Job) {
	timeout := job.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}

	for {
		jobCtx, jobCancel := context.WithTimeout(p.ctx, timeout)
		err := job.Handler(jobCtx, job.Event)
		jobCancel()

		if err == nil {
			logger.Info("Order event processed successfully",
				"worker_id", workerID,
				"order_id", job.Event.OrderID,
				"status", job.Event.Status,
				"attempts", job.Event.RetryCount+1,
			)
			return
		}

		job.Event.RetryCount++
		if job.Event.RetryCount > job.MaxRetries {
			logger.Error("Order event permanently failed after exceeding max retries",
				"worker_id", workerID,
				"order_id", job.Event.OrderID,
				"status", job.Event.Status,
				"error", err.Error(),
				"retries", job.Event.RetryCount,
			)
			return
		}

		// Exponential backoff: base 50ms * 2^(retry-1)
		backoffDuration := time.Duration(float64(50*time.Millisecond) * math.Pow(2, float64(job.Event.RetryCount-1)))
		logger.Warn("Order event failed, retrying with exponential backoff",
			"worker_id", workerID,
			"order_id", job.Event.OrderID,
			"retry", job.Event.RetryCount,
			"backoff_ms", backoffDuration.Milliseconds(),
			"error", err.Error(),
		)

		select {
		case <-time.After(backoffDuration):
			// retry loop continue
		case <-p.ctx.Done():
			logger.Warn("Worker pool context cancelled during backoff sleep", "order_id", job.Event.OrderID)
			return
		}
	}
}

// Submit queues a job to the bounded worker pool
func (p *Pool) Submit(job Job) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.closed {
		return ErrPoolClosed
	}

	select {
	case p.jobQueue <- job:
		return nil
	default:
		return fmt.Errorf("%w: capacity %d", ErrQueueFull, cap(p.jobQueue))
	}
}

// Stop initiates graceful shutdown, waiting for in-flight tasks to complete
func (p *Pool) Stop(timeout time.Duration) error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	close(p.jobQueue)
	p.mu.Unlock()

	done := make(chan struct{})
	go func() {
		p.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info("Worker pool gracefully shut down all workers")
		p.cancel()
		return nil
	case <-time.After(timeout):
		p.cancel()
		return errors.New("worker pool graceful shutdown timed out; forced cancellation")
	}
}

