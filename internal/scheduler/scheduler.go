package scheduler

import (
	"context"
	"log/slog"
	"sync"

	"github.com/outlined/autodev/internal/store"
)

// Processor processes a generation. Decouples scheduler from executor.
type Processor interface {
	Process(ctx context.Context, gen *store.Generation) error
}

// Scheduler manages a FIFO queue with a global concurrency semaphore
// and per-project sequential execution.
type Scheduler struct {
	processor Processor
	store     *store.Store

	sem     chan struct{}            // global concurrency semaphore
	running map[int64]struct{}      // projects currently running
	queue   []*store.Generation     // FIFO queue
	mu      sync.Mutex              // protects running + queue
	wg      sync.WaitGroup          // tracks in-flight goroutines
}

// New creates a scheduler with the given max concurrency.
func New(proc Processor, s *store.Store, maxConcurrent int) *Scheduler {
	if maxConcurrent < 1 {
		maxConcurrent = 1
	}
	return &Scheduler{
		processor: proc,
		store:     s,
		sem:       make(chan struct{}, maxConcurrent),
		running:   make(map[int64]struct{}),
	}
}

// Enqueue adds a generation to the queue and tries to dispatch it.
func (s *Scheduler) Enqueue(gen *store.Generation) {
	s.mu.Lock()
	s.queue = append(s.queue, gen)
	s.mu.Unlock()

	s.dispatch()
}

// Shutdown waits for all in-flight generations to complete.
func (s *Scheduler) Shutdown() {
	slog.Info("scheduler shutting down, waiting for in-flight generations")
	s.wg.Wait()
	slog.Info("scheduler shutdown complete")
}

// dispatch tries to start queued generations that are eligible to run.
func (s *Scheduler) dispatch() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var remaining []*store.Generation
	for _, gen := range s.queue {
		if _, busy := s.running[gen.ProjectID]; busy {
			// Project already running — keep in queue
			remaining = append(remaining, gen)
			continue
		}

		// Try to acquire a semaphore slot (non-blocking)
		select {
		case s.sem <- struct{}{}:
			// Got a slot — mark project as running and launch
			s.running[gen.ProjectID] = struct{}{}
			s.wg.Add(1)
			go s.processGeneration(gen)
		default:
			// No slot available — keep in queue
			remaining = append(remaining, gen)
		}
	}
	s.queue = remaining
}

// processGeneration runs the processor for a generation, then releases resources.
func (s *Scheduler) processGeneration(gen *store.Generation) {
	defer s.wg.Done()

	slog.Info("scheduler: starting generation",
		"id", gen.ID,
		"project_id", gen.ProjectID,
		"ticket", gen.TicketNumber,
	)

	if err := s.processor.Process(context.Background(), gen); err != nil {
		slog.Error("scheduler: generation failed",
			"id", gen.ID,
			"error", err,
		)
	} else {
		slog.Info("scheduler: generation completed", "id", gen.ID)
	}

	// Release semaphore slot
	<-s.sem

	// Release project lock and try to dispatch next
	s.mu.Lock()
	delete(s.running, gen.ProjectID)
	s.mu.Unlock()

	s.dispatch()
}
