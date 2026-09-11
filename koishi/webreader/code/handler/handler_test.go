package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"webreader/config"
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
		Port:               "8080",
		DefaultProvider:    "test-mock",
		DefaultTimeoutSecs: 10,
		MaxTimeoutSecs:     30,
	}
	reg := provider.NewRegistry("test-mock")
	reg.Register(&mockProvider{name: "test-mock"})
	return NewHandler(cfg, reg)
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

func TestGetMarkdown(t *testing.T) {
	h := setupTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/markdown?url=https://example.com/test", nil)
	rec := httptest.NewRecorder()

	h.MarkdownHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res provider.FetchResult
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res.Title != "Test Article Title" {
		t.Errorf("expected title 'Test Article Title', got '%s'", res.Title)
	}
	if !strings.Contains(res.Content, "# Test Article Title") {
		t.Errorf("content does not match: %s", res.Content)
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
}

func TestRawMarkdown(t *testing.T) {
	h := setupTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/raw?url=https://example.com/raw-test", nil)
	rec := httptest.NewRecorder()

	h.RawMarkdownHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Type") != "text/markdown; charset=utf-8" {
		t.Errorf("expected Content-Type text/markdown, got %s", rec.Header().Get("Content-Type"))
	}

	body := rec.Body.String()
	if !strings.Contains(body, "# Test Article Title") {
		t.Errorf("expected markdown body, got %s", body)
	}
}

func TestInvalidURL(t *testing.T) {
	h := setupTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/v1/markdown?url=invalid-url", nil)
	rec := httptest.NewRecorder()

	h.MarkdownHandler(rec, req)

	if rec.Code != http.StatusBadGateway && rec.Code != http.StatusBadRequest {
		t.Fatalf("expected error status code, got %d", rec.Code)
	}
}
