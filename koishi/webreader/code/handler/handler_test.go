package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webreader/config"
	"webreader/pool"
	"webreader/provider"
)

type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) Fetch(ctx context.Context, opts provider.FetchOptions) (*provider.FetchResult, error) {
	return &provider.FetchResult{
		URL:         opts.URL,
		Title:       "Test Article Title",
		Description: "A short description",
		Content:     "# Test Article Title\n\nThis is sample markdown content.",
		Provider:    m.Name(),
		StatusCode:  http.StatusOK,
	}, nil
}

func setupTestHandler() *Handler {
	cfg := &config.Config{
		Port:                  "8080",
		DefaultProvider:       "test-mock",
		DefaultTimeoutSecs:    10,
		MaxTimeoutSecs:        30,
		MaxConcurrentRequests: 1,
		MaxRequestsPerMinute:  60,
	}
	reg := provider.NewRegistry("test-mock")
	reg.Register(&mockProvider{name: "test-mock"})
	limiter := pool.NewLimiter(cfg.MaxConcurrentRequests, cfg.MaxRequestsPerMinute)
	return NewHandler(cfg, reg, limiter)
}

func TestStatusHandler(t *testing.T) {
	h := setupTestHandler()
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
	h := setupTestHandler()
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
	h := setupTestHandler()
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
	if resp.Default != "test-mock" || len(resp.Providers) != 1 {
		t.Errorf("unexpected providers response: %+v", resp)
	}
}

func TestGetMarkdownMethodNotAllowed(t *testing.T) {
	h := setupTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/markdown?url=https://example.com/test", nil)
	rec := httptest.NewRecorder()

	h.MarkdownHandler(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status 405 Method Not Allowed, got %d", rec.Code)
	}
}

func TestPostMarkdown(t *testing.T) {
	h := setupTestHandler()
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
}

func TestInvalidURL(t *testing.T) {
	h := setupTestHandler()
	payload := `{"url": "invalid-url"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/markdown", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.MarkdownHandler(rec, req)

	if rec.Code != http.StatusBadGateway && rec.Code != http.StatusBadRequest {
		t.Fatalf("expected error status code, got %d", rec.Code)
	}
}
