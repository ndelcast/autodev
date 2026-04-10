package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/outlined/autodev/internal/store"
)

// mockProcessor records calls and can simulate work duration.
type mockProcessor struct {
	mu       sync.Mutex
	calls    []int64 // generation IDs in order of start
	active   int32   // currently active (atomic)
	maxSeen  int32   // max concurrent seen (atomic)
	duration time.Duration
}

func (m *mockProcessor) Process(_ context.Context, gen *store.Generation) error {
	m.mu.Lock()
	m.calls = append(m.calls, gen.ID)
	m.mu.Unlock()

	cur := atomic.AddInt32(&m.active, 1)
	// Track max concurrency
	for {
		old := atomic.LoadInt32(&m.maxSeen)
		if cur <= old || atomic.CompareAndSwapInt32(&m.maxSeen, old, cur) {
			break
		}
	}

	if m.duration > 0 {
		time.Sleep(m.duration)
	}

	atomic.AddInt32(&m.active, -1)
	return nil
}

func TestScheduler_BasicEnqueueAndProcess(t *testing.T) {
	proc := &mockProcessor{duration: 10 * time.Millisecond}
	sched := New(proc, nil, 3)

	gen := &store.Generation{ID: 1, ProjectID: 100}
	sched.Enqueue(gen)
	sched.Shutdown()

	if len(proc.calls) != 1 || proc.calls[0] != 1 {
		t.Fatalf("expected gen 1 processed, got %v", proc.calls)
	}
}

func TestScheduler_SameProjectSequential(t *testing.T) {
	// Two generations for the same project should run sequentially
	proc := &mockProcessor{duration: 50 * time.Millisecond}
	sched := New(proc, nil, 3)

	sched.Enqueue(&store.Generation{ID: 1, ProjectID: 100})
	sched.Enqueue(&store.Generation{ID: 2, ProjectID: 100})
	sched.Shutdown()

	if len(proc.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(proc.calls))
	}
	// Max concurrency should be 1 since same project
	if atomic.LoadInt32(&proc.maxSeen) > 1 {
		t.Fatalf("same project ran in parallel: maxSeen=%d", proc.maxSeen)
	}
}

func TestScheduler_DifferentProjectsParallel(t *testing.T) {
	// Two generations for different projects should run in parallel
	proc := &mockProcessor{duration: 100 * time.Millisecond}
	sched := New(proc, nil, 3)

	sched.Enqueue(&store.Generation{ID: 1, ProjectID: 100})
	sched.Enqueue(&store.Generation{ID: 2, ProjectID: 200})
	sched.Shutdown()

	if len(proc.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(proc.calls))
	}
	if atomic.LoadInt32(&proc.maxSeen) < 2 {
		t.Fatalf("different projects should run in parallel: maxSeen=%d", proc.maxSeen)
	}
}

func TestScheduler_SemaphoreRespected(t *testing.T) {
	// With max_concurrent=2, only 2 should run at once even with 4 different projects
	proc := &mockProcessor{duration: 100 * time.Millisecond}
	sched := New(proc, nil, 2)

	sched.Enqueue(&store.Generation{ID: 1, ProjectID: 100})
	sched.Enqueue(&store.Generation{ID: 2, ProjectID: 200})
	sched.Enqueue(&store.Generation{ID: 3, ProjectID: 300})
	sched.Enqueue(&store.Generation{ID: 4, ProjectID: 400})
	sched.Shutdown()

	if len(proc.calls) != 4 {
		t.Fatalf("expected 4 calls, got %d", len(proc.calls))
	}
	if atomic.LoadInt32(&proc.maxSeen) > 2 {
		t.Fatalf("semaphore exceeded: maxSeen=%d, expected max 2", proc.maxSeen)
	}
}

func TestScheduler_MixedProjectsSameAndDifferent(t *testing.T) {
	// Project A: 2 gens (sequential), Project B: 1 gen — A and B can run in parallel
	proc := &mockProcessor{duration: 50 * time.Millisecond}
	sched := New(proc, nil, 3)

	sched.Enqueue(&store.Generation{ID: 1, ProjectID: 100})
	sched.Enqueue(&store.Generation{ID: 2, ProjectID: 200})
	sched.Enqueue(&store.Generation{ID: 3, ProjectID: 100}) // same as gen 1
	sched.Shutdown()

	if len(proc.calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(proc.calls))
	}
	// Should have seen at least 2 concurrent (project 100 + 200)
	if atomic.LoadInt32(&proc.maxSeen) < 2 {
		t.Fatalf("expected parallel execution: maxSeen=%d", proc.maxSeen)
	}
}
