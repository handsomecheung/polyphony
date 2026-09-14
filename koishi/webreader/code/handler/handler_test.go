package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webreader/cache"
	"webreader/config"
	"webreader/pool"
	"webreader/provider"
)

type mockProvider struct {
	name        string
	lastOptions provider.FetchOptions
	calls       int
}

type memoryCache struct {
	entries map[string]*cache.Entry
}

func (c *memoryCache) Get(_ context.Context, key string) (*cache.Entry, error) {
	entry, ok := c.entries[key]
	if !ok {
		return nil, cache.ErrMiss
	}
	return entry, nil
}

func (c *memoryCache) Set(_ context.Context, key string, entry *cache.Entry) error {
	if entry == nil {
		return errors.New("entry is nil")
	}
	c.entries[key] = entry
	return nil
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Fetch(ctx context.Context, opts provider.FetchOptions) (*provider.FetchResult, error) {
	m.calls++
	m.lastOptions = opts
	return &provider.FetchResult{
		URL:         opts.URL,
		Title:       "Test Article Title",
		Description: "A short description",
		Content:     "# Test Article Title\n\nThis is sample markdown content.",
		Provider:    m.Name(),
		StatusCode:  http.StatusOK,
	}, nil
}

func setupTestHandler() (*Handler, *mockProvider) {
	cfg := &config.Config{
		Port:                  "8080",
		DefaultLanguage:       "en",
		DefaultTimeoutSecs:    10,
		MaxTimeoutSecs:        30,
		MaxConcurrentRequests: 1,
		MaxRequestsPerMinute:  60,
	}
	mock := &mockProvider{name: "jina"}
	reg := provider.NewRegistry("jina")
	reg.Register(mock)
	limiter := pool.NewLimiter(cfg.MaxConcurrentRequests, cfg.MaxRequestsPerMinute)
	return NewHandler(cfg, reg, limiter, &memoryCache{entries: make(map[string]*cache.Entry)}), mock
}

func TestStatusHandler(t *testing.T) {
	h, _ := setupTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	rec := httptest.NewRecorder()

	h.StatusHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var stats pool.Stats
	if err := json.NewDecoder(rec.Body).Decode(&stats); err != nil {
		t.Fatalf("failed to decode stats response: %v", err)
	}
	if stats.MaxConcurrent != 1 || stats.RequestsPerMinute != 60 {
		t.Errorf("unexpected stats: %+v", stats)
	}
}

func TestHealthCheck(t *testing.T) {
	h, _ := setupTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	h.HealthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got %s", resp["status"])
	}
}

func TestProvidersList(t *testing.T) {
	h, _ := setupTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/providers", nil)
	rec := httptest.NewRecorder()

	h.ProvidersHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp ProvidersResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Default != "jina" || len(resp.Providers) != 1 {
		t.Errorf("unexpected providers response: %+v", resp)
	}
}

func TestGetMarkdownMethodNotAllowed(t *testing.T) {
	h, _ := setupTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/markdown?url=https://example.com/test", nil)
	rec := httptest.NewRecorder()

	h.MarkdownHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405 Method Not Allowed, got %d", rec.Code)
	}
}

func TestPostMarkdown(t *testing.T) {
	h, mock := setupTestHandler()
	payload := `{"url": "https://example.com/post-test"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.MarkdownHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res provider.FetchResult
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.URL != "https://example.com/post-test" {
		t.Errorf("expected URL 'https://example.com/post-test', got '%s'", res.URL)
	}
	if res.Title != "Test Article Title" {
		t.Errorf("expected title 'Test Article Title', got '%s'", res.Title)
	}
	if !strings.Contains(res.Content, "# Test Article Title") {
		t.Errorf("content does not match: %s", res.Content)
	}
	if mock.lastOptions.Language != "en" {
		t.Errorf("expected default language 'en', got '%s'", mock.lastOptions.Language)
	}
}

func TestPostMarkdownCacheHit(t *testing.T) {
	h, mock := setupTestHandler()
	payload := `{"url": "https://example.com/cache-test", "cache": "on"}`

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payload))
		rec := httptest.NewRecorder()
		h.MarkdownHandler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected status 200, got %d: %s", i, rec.Code, rec.Body.String())
		}
		var res provider.FetchResult
		if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
			t.Fatal(err)
		}
		cached, _ := res.Metadata["cached"].(bool)
		if cached != (i == 1) {
			t.Fatalf("request %d: cached=%v, want %v", i, cached, i == 1)
		}
		if i == 1 && res.Metadata["cached_at"] == nil {
			t.Fatal("cache hit did not include cached_at")
		}
	}
	if mock.calls != 1 {
		t.Fatalf("expected one provider fetch for a cache miss followed by a hit, got %d", mock.calls)
	}
}

func TestPostMarkdownCacheFalseRefreshes(t *testing.T) {
	h, mock := setupTestHandler()
	for _, payload := range []string{
		`{"url": "https://example.com/refresh-test", "cache": "on"}`,
		`{"url": "https://example.com/refresh-test", "cache": "off"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payload))
		rec := httptest.NewRecorder()
		h.MarkdownHandler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var res provider.FetchResult
		_ = json.NewDecoder(rec.Body).Decode(&res)
		if res.Metadata["cached"] != false {
			t.Fatalf("cache false should fetch fresh content, got metadata: %+v", res.Metadata)
		}
	}
	if mock.calls != 2 {
		t.Fatalf("cache false should force a second provider fetch, got %d calls", mock.calls)
	}
}

func TestPostMarkdownRejectsInvalidCacheMode(t *testing.T) {
	h, _ := setupTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(`{"url":"https://example.com", "cache":"invalid"}`))
	rec := httptest.NewRecorder()
	h.MarkdownHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPostMarkdownCustomLanguage(t *testing.T) {
	h, mock := setupTestHandler()
	payload := `{"url": "https://example.com/post-test-lang", "language": "ja"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.MarkdownHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if mock.lastOptions.Language != "ja" {
		t.Errorf("expected custom language 'ja', got '%s'", mock.lastOptions.Language)
	}
}

func TestInvalidURL(t *testing.T) {
	h, _ := setupTestHandler()
	payload := `{"url": "invalid-url"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.MarkdownHandler(rec, req)

	if rec.Code != http.StatusBadGateway && rec.Code != http.StatusBadRequest {
		t.Fatalf("expected error status code, got %d", rec.Code)
	}
}

func TestPostMarkdownActions(t *testing.T) {
	h, mock := setupTestHandler()
	payload := `{
		"url": "https://example.com/actions-test",
		"mode": "static",
		"actions": [
			{"type": "click", "selector": "#btn"},
			{"type": "wait", "milliseconds": 1000}
		]
	}`
	req := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.MarkdownHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if len(mock.lastOptions.Actions) != 2 {
		t.Fatalf("expected 2 actions forwarded, got %d", len(mock.lastOptions.Actions))
	}
	if mock.lastOptions.Actions[0]["type"] != "click" || mock.lastOptions.Actions[0]["selector"] != "#btn" {
		t.Errorf("unexpected action 0: %+v", mock.lastOptions.Actions[0])
	}
	if mock.lastOptions.Actions[1]["type"] != "wait" {
		t.Errorf("unexpected action 1: %+v", mock.lastOptions.Actions[1])
	}
}

func TestPostMarkdownActionsCacheKey(t *testing.T) {
	h, mock := setupTestHandler()
	payloadWithoutActions := `{"url": "https://example.com/cache-actions", "cache": "on"}`
	payloadWithActions := `{
		"url": "https://example.com/cache-actions",
		"cache": "on",
		"actions": [{"type": "click", "selector": "#btn"}]
	}`

	// 1. Fetch without actions -> cache miss -> store
	req1 := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payloadWithoutActions))
	rec1 := httptest.NewRecorder()
	h.MarkdownHandler(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("request 1 failed: %d", rec1.Code)
	}

	// 2. Fetch with actions -> should be a cache miss (different key) -> store
	req2 := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payloadWithActions))
	rec2 := httptest.NewRecorder()
	h.MarkdownHandler(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("request 2 failed: %d", rec2.Code)
	}

	// 3. Fetch with actions again -> should hit cache
	req3 := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payloadWithActions))
	rec3 := httptest.NewRecorder()
	h.MarkdownHandler(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("request 3 failed: %d", rec3.Code)
	}

	var res3 provider.FetchResult
	_ = json.NewDecoder(rec3.Body).Decode(&res3)
	if res3.Metadata["cached"] != true {
		t.Fatalf("expected request 3 to be a cache hit, got cached=%v", res3.Metadata["cached"])
	}

	// Total provider calls should be 2 (one for without actions, one for with actions)
	if mock.calls != 2 {
		t.Fatalf("expected 2 provider calls, got %d", mock.calls)
	}
}
