package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

type blockingProvider struct {
	name    string
	started chan struct{}
	release chan struct{}
}

func (p *blockingProvider) Name() string { return p.name }

func (p *blockingProvider) Fetch(_ context.Context, opts provider.FetchOptions) (*provider.FetchResult, error) {
	p.started <- struct{}{}
	<-p.release
	return &provider.FetchResult{URL: opts.URL, Provider: p.name, StatusCode: http.StatusOK}, nil
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

func TestPostMarkdownCacheOffBypassesReadsAndDoesNotWrite(t *testing.T) {
	h, mock := setupTestHandler()
	for i, payload := range []string{
		`{"url": "https://example.com/refresh-test", "cache": "on"}`,
		`{"url": "https://example.com/refresh-test", "cache": "off"}`,
		`{"url": "https://example.com/refresh-test", "cache": "on"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payload))
		rec := httptest.NewRecorder()
		h.MarkdownHandler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var res provider.FetchResult
		_ = json.NewDecoder(rec.Body).Decode(&res)
		if res.Metadata["cached"] != (i == 2) {
			t.Fatalf("request %d: cached=%v", i, res.Metadata["cached"])
		}
	}
	if mock.calls != 2 {
		t.Fatalf("cache off should not replace the existing entry, got %d provider fetches", mock.calls)
	}
}

func TestPostMarkdownCacheOmittedReadsButDoesNotWrite(t *testing.T) {
	h, mock := setupTestHandler()
	url := "https://example.com/cache-omitted-test"
	for i, payload := range []string{
		`{"url": "https://example.com/cache-omitted-test"}`,
		`{"url": "https://example.com/cache-omitted-test"}`,
		`{"url": "https://example.com/cache-omitted-test", "cache": "on"}`,
		`{"url": "https://example.com/cache-omitted-test"}`,
	} {
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
		if res.Metadata["cached"] != (i == 3) {
			t.Fatalf("request %d: cached=%v", i, res.Metadata["cached"])
		}
	}
	if mock.calls != 3 {
		t.Fatalf("omitted cache should not store fresh results for %s, got %d provider fetches", url, mock.calls)
	}
}

func TestPostMarkdownRejectsInvalidCacheMode(t *testing.T) {
	h, _ := setupTestHandler()
	for _, value := range []string{"invalid", "skip_write"} {
		req := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(`{"url":"https://example.com", "cache":"`+value+`"}`))
		rec := httptest.NewRecorder()
		h.MarkdownHandler(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("cache=%q: expected status 400, got %d: %s", value, rec.Code, rec.Body.String())
		}
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

func TestPostMarkdownRemoveMedia(t *testing.T) {
	h, mock := setupTestHandler()
	payload := `{"url": "https://example.com/remove-media", "remove_media": "on"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payload))
	rec := httptest.NewRecorder()

	h.MarkdownHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !mock.lastOptions.RemoveMedia {
		t.Fatal("expected remove_media to be forwarded to the provider")
	}
}

func TestPostMarkdownRemoveMediaCacheKey(t *testing.T) {
	h, mock := setupTestHandler()
	for _, payload := range []string{
		`{"url": "https://example.com/cache-media", "cache": "on"}`,
		`{"url": "https://example.com/cache-media", "cache": "on", "remove_media": "on"}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payload))
		rec := httptest.NewRecorder()
		h.MarkdownHandler(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
		}
	}
	if mock.calls != 2 {
		t.Fatalf("expected media mode to create a distinct cache entry, got %d provider calls", mock.calls)
	}
}

func TestPostMarkdownRejectsInvalidRemoveMediaMode(t *testing.T) {
	h, _ := setupTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(`{"url":"https://example.com", "remove_media":"true"}`))
	rec := httptest.NewRecorder()
	h.MarkdownHandler(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestParseRemoveMediaMode(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  bool
	}{
		{value: "", want: false},
		{value: "off", want: false},
		{value: "on", want: true},
	} {
		got, err := parseRemoveMediaMode(tc.value)
		if err != nil || got != tc.want {
			t.Fatalf("parseRemoveMediaMode(%q) = (%v, %v), want (%v, nil)", tc.value, got, err, tc.want)
		}
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

func TestDashboardShowsCachedResultAndParameters(t *testing.T) {
	h, _ := setupTestHandler()
	payload := `{"url":"https://example.com/dashboard-test","cache":"on","language":"ja","remove_media":"on"}`
	request := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payload))
	h.MarkdownHandler(httptest.NewRecorder(), request)

	rec := httptest.NewRecorder()
	h.DashboardDataHandler(rec, httptest.NewRequest(http.MethodGet, "/v1/dashboard", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected dashboard status 200, got %d", rec.Code)
	}
	var response struct {
		History []RequestRecord `json:"history"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.History) != 1 {
		t.Fatalf("expected one dashboard record, got %d", len(response.History))
	}
	record := response.History[0]
	if record.URL != "https://example.com/dashboard-test" || record.Parameters.Language != "ja" || record.Parameters.RemoveMedia != "on" {
		t.Fatalf("unexpected dashboard record: %+v", record)
	}
	if !record.Cached || record.Result == nil || record.State != RequestStateCompleted {
		t.Fatalf("expected completed cached result, got %+v", record)
	}
}

func TestDashboardDoesNotExposeUncachedResult(t *testing.T) {
	h, _ := setupTestHandler()
	request := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(`{"url":"https://example.com/dashboard-uncached"}`))
	h.MarkdownHandler(httptest.NewRecorder(), request)

	history, _ := h.history.snapshot()
	if len(history) != 1 || history[0].Result != nil || history[0].Cached {
		t.Fatalf("uncached request unexpectedly exposed a result: %+v", history)
	}
}

func TestDashboardPage(t *testing.T) {
	h, _ := setupTestHandler()
	rec := httptest.NewRecorder()
	h.DashboardHandler(rec, httptest.NewRequest(http.MethodGet, "/dashboard", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "webreader dashboard") {
		t.Fatalf("unexpected dashboard page: status=%d body=%q", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/html") {
		t.Fatalf("expected HTML content type, got %q", contentType)
	}
	if !strings.Contains(rec.Body.String(), "request-dialog") || !strings.Contains(rec.Body.String(), "showRequest") {
		t.Fatal("dashboard page does not provide a request detail dialog")
	}
	if !strings.Contains(rec.Body.String(), "overflow-wrap:anywhere") {
		t.Fatal("dashboard dialog does not wrap long URLs")
	}
}

func TestDashboardUsesEmptyArraysWhenThereAreNoRequests(t *testing.T) {
	h, _ := setupTestHandler()
	rec := httptest.NewRecorder()
	h.DashboardDataHandler(rec, httptest.NewRequest(http.MethodGet, "/v1/dashboard", nil))
	if strings.Contains(rec.Body.String(), `"queued_requests":null`) || strings.Contains(rec.Body.String(), `"history":null`) {
		t.Fatalf("dashboard must return empty arrays, got %s", rec.Body.String())
	}
}

func TestDashboardListsRequestsWaitingForWorker(t *testing.T) {
	cfg := &config.Config{DefaultLanguage: "en", DefaultTimeoutSecs: 10, MaxTimeoutSecs: 30, MaxConcurrentRequests: 1}
	provider := &blockingProvider{name: "jina", started: make(chan struct{}, 2), release: make(chan struct{}, 2)}
	registry := provider2Registry(provider)
	h := NewHandler(cfg, registry, pool.NewLimiter(1, 0), &memoryCache{entries: make(map[string]*cache.Entry)})

	done := make(chan struct{}, 2)
	for _, url := range []string{"https://example.com/first", "https://example.com/second"} {
		go func(url string) {
			h.MarkdownHandler(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(`{"url":"`+url+`"}`)))
			done <- struct{}{}
		}(url)
	}

	<-provider.started
	deadline := time.After(time.Second)
	for {
		_, queued := h.history.snapshot()
		if len(queued) == 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("expected one request to be shown as queued")
		case <-time.After(time.Millisecond):
		}
	}
	provider.release <- struct{}{}
	<-provider.started
	provider.release <- struct{}{}
	<-done
	<-done
}

func provider2Registry(p provider.Provider) *provider.Registry {
	registry := provider.NewRegistry("jina")
	registry.Register(p)
	return registry
}
