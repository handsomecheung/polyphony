package pool

import (
	"context"
	"sync"
	"time"
)

// Limiter manages the worker pool concurrency and requests-per-minute rate limiting.
type Limiter struct {
	mu              sync.Mutex
	sem             chan struct{}
	rpmLimit        int
	requestTimes    []time.Time
	activeRequests  int
	waitingRequests int
}

// Stats represents the current state and metrics of the worker pool and rate limiter.
type Stats struct {
	MaxConcurrent     int `json:"max_concurrent"`
	RequestsPerMinute int `json:"requests_per_minute"`
	ActiveRequests    int `json:"active_requests"`
	WaitingRequests   int `json:"waiting_requests"`
}

// NewLimiter creates a new Limiter with specified max concurrency and RPM limit.
// If maxConcurrent <= 0, it defaults to 1 (serialized execution).
// If requestsPerMinute <= 0, RPM rate limiting is disabled.
func NewLimiter(maxConcurrent int, requestsPerMinute int) *Limiter {
	if maxConcurrent <= 0 {
		maxConcurrent = 1
	}

	return &Limiter{
		sem:          make(chan struct{}, maxConcurrent),
		rpmLimit:     requestsPerMinute,
		requestTimes: make([]time.Time, 0),
	}
}

// GetStats returns current pool metrics.
func (l *Limiter) GetStats() Stats {
	l.mu.Lock()
	defer l.mu.Unlock()

	return Stats{
		MaxConcurrent:     cap(l.sem),
		RequestsPerMinute: l.rpmLimit,
		ActiveRequests:    l.activeRequests,
		WaitingRequests:   l.waitingRequests,
	}
}

// Acquire waits for rate limit availability and an available worker in the pool.
// It returns a release function that MUST be called when the task completes.
func (l *Limiter) Acquire(ctx context.Context) (func(), error) {
	l.mu.Lock()
	l.waitingRequests++
	l.mu.Unlock()

	defer func() {
		l.mu.Lock()
		l.waitingRequests--
		l.mu.Unlock()
	}()

	// 1. Rate limiter check / wait
	if l.rpmLimit > 0 {
		if err := l.waitForRateLimit(ctx); err != nil {
			return nil, err
		}
	}

	// 2. Worker pool concurrency acquisition
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case l.sem <- struct{}{}:
		l.mu.Lock()
		l.activeRequests++
		l.mu.Unlock()

		var once sync.Once
		release := func() {
			once.Do(func() {
				<-l.sem
				l.mu.Lock()
				l.activeRequests--
				l.mu.Unlock()
			})
		}
		return release, nil
	}
}

func (l *Limiter) waitForRateLimit(ctx context.Context) error {
	for {
		l.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-1 * time.Minute)

		// Remove expired timestamps
		validIdx := 0
		for i, t := range l.requestTimes {
			if t.After(cutoff) {
				validIdx = i
				break
			}
			if i == len(l.requestTimes)-1 {
				validIdx = len(l.requestTimes)
			}
		}
		if validIdx > 0 {
			l.requestTimes = l.requestTimes[validIdx:]
		}

		if len(l.requestTimes) < l.rpmLimit {
			// Slot available, record start time and proceed
			l.requestTimes = append(l.requestTimes, now)
			l.mu.Unlock()
			return nil
		}

		// Wait until the oldest recorded request rolls out of the 1-minute window
		oldest := l.requestTimes[0]
		waitDuration := oldest.Add(1 * time.Minute).Sub(now)
		l.mu.Unlock()

		if waitDuration <= 0 {
			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDuration):
			// Loop to re-verify slot availability
		}
	}
}
