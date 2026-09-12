package pool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLimiterConcurrency(t *testing.T) {
	// Worker pool concurrency = 1
	limiter := NewLimiter(1, 0)

	var activeCount int32
	var maxActive int32

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			release, err := limiter.Acquire(context.Background())
			if err != nil {
				t.Errorf("failed to acquire: %v", err)
				return
			}
			defer release()

			curr := atomic.AddInt32(&activeCount, 1)
			// Track maximum concurrent executions observed
			for {
				max := atomic.LoadInt32(&maxActive)
				if curr <= max || atomic.CompareAndSwapInt32(&maxActive, max, curr) {
					break
				}
			}

			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&activeCount, -1)
		}()
	}

	wg.Wait()

	if maxActive > 1 {
		t.Errorf("expected maximum concurrent execution of 1, got %d", maxActive)
	}
}

func TestLimiterContextCancel(t *testing.T) {
	limiter := NewLimiter(1, 0)

	// Acquire the single worker slot
	release, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer release()

	// Second request with short timeout should fail with DeadlineExceeded
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err = limiter.Acquire(ctx)
	if err == nil {
		t.Fatal("expected context cancellation error, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestLimiterRateLimitRPM(t *testing.T) {
	// Allow 2 requests per minute
	limiter := NewLimiter(5, 2)

	// First 2 should succeed immediately
	rel1, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	defer rel1()

	rel2, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatalf("second acquire failed: %v", err)
	}
	defer rel2()

	// Third request should block because 2/min limit is hit
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, err = limiter.Acquire(ctx)
	if err != context.DeadlineExceeded {
		t.Errorf("expected rate limit to block and hit DeadlineExceeded, got %v", err)
	}
}

func TestLimiterStats(t *testing.T) {
	limiter := NewLimiter(1, 30)

	stats := limiter.GetStats()
	if stats.MaxConcurrent != 1 || stats.RequestsPerMinute != 30 {
		t.Errorf("unexpected initial stats: %+v", stats)
	}

	release, err := limiter.Acquire(context.Background())
	if err != nil {
		t.Fatalf("failed to acquire: %v", err)
	}

	stats = limiter.GetStats()
	if stats.ActiveRequests != 1 {
		t.Errorf("expected 1 active request, got %d", stats.ActiveRequests)
	}

	release()

	stats = limiter.GetStats()
	if stats.ActiveRequests != 0 {
		t.Errorf("expected 0 active requests after release, got %d", stats.ActiveRequests)
	}
}
