package handler

import (
	"sync"
	"time"

	"webreader/provider"
)

// RequestState describes where a markdown request is in its lifecycle.
type RequestState string

const (
	RequestStateQueued    RequestState = "queued"
	RequestStateRunning   RequestState = "running"
	RequestStateCompleted RequestState = "completed"
	RequestStateFailed    RequestState = "failed"
)

// RequestRecord is the in-memory representation exposed by the dashboard.
// Result is populated only when the response is available from Redis cache.
type RequestRecord struct {
	ID          uint64                `json:"id"`
	URL         string                `json:"url"`
	Parameters  MarkdownRequestBody   `json:"parameters"`
	State       RequestState          `json:"state"`
	StartedAt   time.Time             `json:"started_at"`
	CompletedAt *time.Time            `json:"completed_at,omitempty"`
	Error       string                `json:"error,omitempty"`
	Cached      bool                  `json:"cached"`
	Result      *provider.FetchResult `json:"result,omitempty"`
}

// requestHistory retains a bounded, process-local list of recent requests.
type requestHistory struct {
	mu      sync.RWMutex
	limit   int
	nextID  uint64
	records []*RequestRecord
}

func newRequestHistory(limit int) *requestHistory {
	if limit <= 0 {
		limit = 20
	}
	return &requestHistory{limit: limit}
}

func (h *requestHistory) add(url string, params MarkdownRequestBody) *RequestRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.nextID++
	record := &RequestRecord{
		ID:         h.nextID,
		URL:        url,
		Parameters: params,
		State:      RequestStateQueued,
		StartedAt:  time.Now().UTC(),
	}
	h.records = append(h.records, record)
	if len(h.records) > h.limit {
		h.records = h.records[len(h.records)-h.limit:]
	}
	return record
}

func (h *requestHistory) setRunning(record *RequestRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()
	record.State = RequestStateRunning
}

func (h *requestHistory) complete(record *RequestRecord, cached bool, result *provider.FetchResult) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now().UTC()
	record.State = RequestStateCompleted
	record.CompletedAt = &now
	record.Cached = cached
	record.Result = result
}

func (h *requestHistory) fail(record *RequestRecord, err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now().UTC()
	record.State = RequestStateFailed
	record.CompletedAt = &now
	record.Error = err.Error()
}

func (h *requestHistory) snapshot() (history []RequestRecord, queued []RequestRecord) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	history = make([]RequestRecord, 0, len(h.records))
	queued = make([]RequestRecord, 0)
	for i := len(h.records) - 1; i >= 0; i-- {
		record := *h.records[i]
		history = append(history, record)
		if record.State == RequestStateQueued {
			queued = append(queued, record)
		}
	}
	return history, queued
}
