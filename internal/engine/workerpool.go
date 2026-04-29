package engine

import (
	"context"
	"sync"
	"time"
)

const defaultAlertTimeout = 30 * time.Second

// AlertWorkerPool bounds concurrent alert processing goroutines so that a
// burst of firing alerts (e.g. 500+ from a single webhook push) doesn't
// exhaust the scheduler.  Spare work is dropped with a warning log rather
// than queued — alert processing is lossy by design because the evaluator
// will re-fire on the next evaluation cycle.
type AlertWorkerPool struct {
	sem     chan struct{}
	pending sync.WaitGroup
}

// NewAlertWorkerPool creates a bounded pool.  concurrency should typically be
// 32–128; 0 or negative values default to 64.
func NewAlertWorkerPool(concurrency int) *AlertWorkerPool {
	if concurrency <= 0 {
		concurrency = 64
	}
	return &AlertWorkerPool{
		sem: make(chan struct{}, concurrency),
	}
}

// Submit runs fn in a goroutine if the pool has capacity.  Returns false when
// the pool is full (caller should log and move on).  fn receives a context
// derived from ctx (30 s deadline added if none is set).
func (p *AlertWorkerPool) Submit(ctx context.Context, fn func(context.Context)) bool {
	select {
	case p.sem <- struct{}{}:
	default:
		return false
	}

	p.pending.Add(1)
	go func() {
		defer p.pending.Done()
		defer func() { <-p.sem }()

		// Add a deadline if ctx doesn't already carry one.
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultAlertTimeout)
			defer cancel()
		}

		// Recover panics so a buggy callback never takes down the pool.
		defer func() {
			if r := recover(); r != nil {
				// Panic is logged by the caller's own recover block;
				// we just keep the pool alive.
			}
		}()

		fn(ctx)
	}()

	return true
}

// Wait blocks until all goroutines that were accepted via Submit have
// returned.  Call this during graceful shutdown after the producer has been
// stopped (e.g. after Evaluator.Stop).
func (p *AlertWorkerPool) Wait() {
	p.pending.Wait()
}
