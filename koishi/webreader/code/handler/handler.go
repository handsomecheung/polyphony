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
	"webreader/provider"
)

// Handler handles incoming HTTP requests.
type Handler struct {
	cfg      *config.Config
	registry *provider.Registry
}

// NewHandler creates a new HTTP handler.
func NewHandler(cfg *config.Config, registry *provider.Registry) *Handler {
	return &Handler{
		cfg:      cfg,
		registry: registry,
	}
}

// MarkdownRequestBody represents the JSON request payload for POST /v1/markdown.
type MarkdownRequestBody struct {
	URL                string            `json:"url"`
	Provider           string            `json:"provider,omitempty"`
	TimeoutSeconds     int               `json:"timeout_seconds,omitempty"`
	WithImagesSummary  bool              `json:"with_images_summary,omitempty"`
	WithLinksSummary   bool              `json:"with_links_summary,omitempty"`
	NoCache            bool              `json:"no_cache,omitempty"`
	WaitForSelector    string            `json:"wait_for_selector,omitempty"`
	TargetSelector     string            `json:"target_selector,omitempty"`
	CustomHeaders      map[string]string `json:"custom_headers,omitempty"`
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

// RawMarkdownHandler fetches a URL and returns raw Markdown content directly.
func (h *Handler) RawMarkdownHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	targetURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if targetURL == "" {
		http.Error(w, "Missing 'url' query parameter", http.StatusBadRequest)
		return
	}

	providerName := r.URL.Query().Get("provider")
	timeoutSecs := 0
	opts := provider.FetchOptions{
		URL:               targetURL,
		WithImagesSummary: r.URL.Query().Get("with_images_summary") == "true",
		WithLinksSummary:  r.URL.Query().Get("with_links_summary") == "true",
		NoCache:           r.URL.Query().Get("no_cache") == "true",
		WaitForSelector:   r.URL.Query().Get("wait_for_selector"),
		TargetSelector:    r.URL.Query().Get("target_selector"),
	}

	result, err := h.executeFetch(r, providerName, timeoutSecs, opts)
	if err != nil {
		log.Printf("[ERROR] raw fetch failed for %s: %v", targetURL, err)
		http.Error(w, fmt.Sprintf("Failed to fetch markdown: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(result.Content))
}

// MarkdownHandler handles both GET and POST requests for /v1/markdown.
func (h *Handler) MarkdownHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGetMarkdown(w, r)
	case http.MethodPost:
		h.handlePostMarkdown(w, r)
	default:
		h.writeJSONError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "")
	}
}

func (h *Handler) handleGetMarkdown(w http.ResponseWriter, r *http.Request) {
	targetURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if targetURL == "" {
		h.writeJSONError(w, http.StatusBadRequest, "Missing required query parameter: 'url'", "")
		return
	}

	providerName := r.URL.Query().Get("provider")
	opts := provider.FetchOptions{
		URL:               targetURL,
		WithImagesSummary: r.URL.Query().Get("with_images_summary") == "true",
		WithLinksSummary:  r.URL.Query().Get("with_links_summary") == "true",
		NoCache:           r.URL.Query().Get("no_cache") == "true",
		WaitForSelector:   r.URL.Query().Get("wait_for_selector"),
		TargetSelector:    r.URL.Query().Get("target_selector"),
	}

	result, err := h.executeFetch(r, providerName, 0, opts)
	if err != nil {
		log.Printf("[ERROR] fetch failed for %s: %v", targetURL, err)
		h.writeJSONError(w, http.StatusBadGateway, "Failed to scrape target URL", err.Error())
		return
	}

	h.respondResult(w, r, result)
}

func (h *Handler) handlePostMarkdown(w http.ResponseWriter, r *http.Request) {
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
		URL:               body.URL,
		WithImagesSummary: body.WithImagesSummary,
		WithLinksSummary:  body.WithLinksSummary,
		NoCache:           body.NoCache,
		WaitForSelector:   body.WaitForSelector,
		TargetSelector:    body.TargetSelector,
		CustomHeaders:     body.CustomHeaders,
	}

	result, err := h.executeFetch(r, body.Provider, body.TimeoutSeconds, opts)
	if err != nil {
		log.Printf("[ERROR] fetch failed for %s: %v", body.URL, err)
		h.writeJSONError(w, http.StatusBadGateway, "Failed to scrape target URL", err.Error())
		return
	}

	h.respondResult(w, r, result)
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

func (h *Handler) respondResult(w http.ResponseWriter, r *http.Request, result *provider.FetchResult) {
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/markdown") || strings.Contains(accept, "text/plain") {
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(result.Content))
		return
	}

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
