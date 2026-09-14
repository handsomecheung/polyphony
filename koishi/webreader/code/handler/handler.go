package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"webreader/config"
	"webreader/pool"
	"webreader/provider"
)

// Handler handles incoming HTTP requests.
type Handler struct {
	cfg      *config.Config
	registry *provider.Registry
	limiter  *pool.Limiter
}

// NewHandler creates a new HTTP handler.
func NewHandler(cfg *config.Config, registry *provider.Registry, limiter *pool.Limiter) *Handler {
	return &Handler{
		cfg:      cfg,
		registry: registry,
		limiter:  limiter,
	}
}

// MarkdownRequestBody represents the JSON request payload for POST /v1/markdown.
type MarkdownRequestBody struct {
	URL              string            `json:"url"`
	Provider         string            `json:"provider,omitempty"`
	TimeoutSeconds   int               `json:"timeout_seconds,omitempty"`
	WithLinksSummary bool              `json:"with_links_summary,omitempty"`
	WaitForSelector  string            `json:"wait_for_selector,omitempty"`
	TargetSelector   string            `json:"target_selector,omitempty"`
	CustomHeaders    map[string]string `json:"custom_headers,omitempty"`
}

// ErrorResponse represents an error response JSON payload.
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// ProvidersResponse represents the response for GET /v1/providers.
type ProvidersResponse struct {
	Default   string   `json:"default"`
	Providers []string `json:"providers"`
}

// HealthHandler handles health check requests.
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// ProvidersHandler lists all registered providers.
func (h *Handler) ProvidersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	defaultName, list := h.registry.ListNames()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ProvidersResponse{
		Default:   defaultName,
		Providers: list,
	})
}

// MarkdownHandler handles POST requests for /v1/markdown.
func (h *Handler) MarkdownHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Only POST method is supported")
		return
	}

	var body MarkdownRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid JSON request body", err.Error())
		return
	}

	body.URL = strings.TrimSpace(body.URL)
	if body.URL == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Field 'url' is required in request body", "")
		return
	}

	opts := provider.FetchOptions{
		URL:              body.URL,
		WithLinksSummary: body.WithLinksSummary,
		WaitForSelector:  body.WaitForSelector,
		TargetSelector:   body.TargetSelector,
		CustomHeaders:    body.CustomHeaders,
	}

	result, err := h.executeFetch(r, body.Provider, body.TimeoutSeconds, opts)
	if err != nil {
		log.Printf("[ERROR] fetch failed for %s: %v", body.URL, err)
		h.writeJSONError(w, http.StatusBadGateway, "Failed to scrape target URL", err.Error())
		return
	}

	h.respondResult(w, result)
}

// StatusHandler returns the current worker pool and rate limit metrics.
func (h *Handler) StatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := h.limiter.GetStats()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

func (h *Handler) executeFetch(r *http.Request, providerName string, timeoutSecs int, opts provider.FetchOptions) (*provider.FetchResult, error) {
	// Validate URL format
	parsedURL, err := url.ParseRequestURI(opts.URL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return nil, fmt.Errorf("invalid URL: must be an absolute http or https URL")
	}

	p, err := h.registry.Get(providerName)
	if err != nil {
		return nil, err
	}

	timeout := h.cfg.GetTimeout(timeoutSecs)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Acquire slot from worker pool & rate limiter queue
	release, err := h.limiter.Acquire(ctx)
	if err != nil {
		log.Printf("[QUEUE] URL=%s Error acquiring worker slot: %v", opts.URL, err)
		return nil, fmt.Errorf("queue or rate limit error: %w", err)
	}
	defer release()

	start := time.Now()
	res, err := p.Fetch(ctx, opts)
	duration := time.Since(start)

	if err != nil {
		log.Printf("[FETCH] URL=%s Provider=%s Duration=%s Status=ERROR Error=%v", opts.URL, p.Name(), duration, err)
		return nil, err
	}

	log.Printf("[FETCH] URL=%s Provider=%s Duration=%s Status=OK", opts.URL, p.Name(), duration)
	return res, nil
}

func (h *Handler) respondResult(w http.ResponseWriter, result *provider.FetchResult) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func (h *Handler) writeJSONError(w http.ResponseWriter, statusCode int, message string, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   message,
		Details: details,
	})
}
