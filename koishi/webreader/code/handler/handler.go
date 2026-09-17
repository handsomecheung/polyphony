package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"webreader/cache"
	"webreader/config"
	"webreader/pool"
	"webreader/provider"
)

// Handler handles incoming HTTP requests.
type Handler struct {
	cfg      *config.Config
	registry *provider.Registry
	limiter  *pool.Limiter
	cache    cache.Store
}

// NewHandler creates a new HTTP handler.
func NewHandler(cfg *config.Config, registry *provider.Registry, limiter *pool.Limiter, cacheStore cache.Store) *Handler {
	return &Handler{
		cfg:      cfg,
		registry: registry,
		limiter:  limiter,
		cache:    cacheStore,
	}
}

// FetchMode specifies how a page should be fetched.
// "static" (default) fetches the raw HTML without JavaScript rendering.
// "rendered" fetches the page after JavaScript execution (e.g. via Firecrawl).
type FetchMode string

const (
	FetchModeStatic   FetchMode = "static"
	FetchModeRendered FetchMode = "rendered"
)

// MarkdownRequestBody represents the JSON request payload for POST /v1/markdown.
type MarkdownRequestBody struct {
	URL            string                   `json:"url"`
	Mode           FetchMode                `json:"mode,omitempty"`
	Language       string                   `json:"language,omitempty"`
	RemoveMedia    string                   `json:"remove_media,omitempty"`
	TimeoutSeconds int                      `json:"timeout_seconds,omitempty"`
	Cache          string                   `json:"cache,omitempty"`
	CustomHeaders  map[string]string        `json:"custom_headers,omitempty"`
	Actions        []map[string]interface{} `json:"actions,omitempty"`
}

type cacheMode int

const (
	cacheUse cacheMode = iota
	cacheRefresh
	cacheSkipWrite
)

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

	language := strings.TrimSpace(body.Language)
	if language == "" {
		language = h.cfg.DefaultLanguage
	}
	removeMedia, err := parseRemoveMediaMode(body.RemoveMedia)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid 'remove_media' value", err.Error())
		return
	}

	opts := provider.FetchOptions{
		URL:           body.URL,
		Language:      language,
		RemoveMedia:   removeMedia,
		CustomHeaders: body.CustomHeaders,
		Actions:       body.Actions,
	}

	providerName, err := modeToProviderName(body.Mode)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid 'mode' value", err.Error())
		return
	}
	if err := validateURL(opts.URL); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid 'url' value", err.Error())
		return
	}
	cacheMode, err := parseCacheMode(body.Cache)
	if err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "Invalid 'cache' value", err.Error())
		return
	}

	effectiveMode := string(body.Mode)
	if effectiveMode == "" {
		effectiveMode = string(FetchModeStatic)
	}
	cacheAllowed := len(body.CustomHeaders) == 0
	cacheKey := cache.Key(h.cfg.CacheKeyPrefix, opts.URL, language, effectiveMode, opts.RemoveMedia, opts.Actions)
	if cacheAllowed && cacheMode == cacheUse {
		entry, err := h.cache.Get(r.Context(), cacheKey)
		switch {
		case err == nil:
			setCacheMetadata(entry.Result, true, entry.CachedAt)
			log.Printf("[CACHE] URL=%s Status=HIT", opts.URL)
			h.respondResult(w, entry.Result)
			return
		case !errors.Is(err, cache.ErrMiss):
			log.Printf("[WARN] cache read failed for %s: %v", opts.URL, err)
		default:
			log.Printf("[CACHE] URL=%s Status=MISS", opts.URL)
		}
	}

	result, err := h.executeFetch(r, providerName, body.TimeoutSeconds, opts)
	if err != nil {
		log.Printf("[ERROR] fetch failed for %s: %v", body.URL, err)
		h.writeJSONError(w, http.StatusBadGateway, "Failed to scrape target URL", err.Error())
		return
	}
	setCacheMetadata(result, false, time.Time{})
	if cacheAllowed && cacheMode != cacheSkipWrite {
		entry := &cache.Entry{Result: result, CachedAt: time.Now().UTC()}
		if err := h.cache.Set(r.Context(), cacheKey, entry); err != nil {
			log.Printf("[WARN] cache write failed for %s: %v", opts.URL, err)
		} else {
			log.Printf("[CACHE] URL=%s Status=STORED", opts.URL)
		}
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

// modeToProviderName maps a client-facing FetchMode to an internal provider name.
// This decouples the API surface from backend implementation details,
// so callers never need to know which backend serves a given mode.
// An empty mode defaults to FetchModeStatic.
func modeToProviderName(mode FetchMode) (string, error) {
	if mode == "" {
		mode = FetchModeStatic
	}
	switch mode {
	case FetchModeStatic:
		return "jina", nil
	case FetchModeRendered:
		return "firecrawl", nil
	default:
		return "", fmt.Errorf("unknown mode %q: valid values are %q and %q", mode, FetchModeStatic, FetchModeRendered)
	}
}

func (h *Handler) executeFetch(r *http.Request, providerName string, timeoutSecs int, opts provider.FetchOptions) (*provider.FetchResult, error) {
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

func validateURL(rawURL string) error {
	parsedURL, err := url.ParseRequestURI(rawURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return fmt.Errorf("invalid URL: must be an absolute http or https URL")
	}
	return nil
}

func parseCacheMode(value string) (cacheMode, error) {
	switch value {
	case "", "on":
		return cacheUse, nil
	case "off":
		return cacheRefresh, nil
	case "skip_write":
		return cacheSkipWrite, nil
	default:
		return 0, fmt.Errorf("must be \"on\", \"off\", or \"skip_write\"")
	}
}

func parseRemoveMediaMode(value string) (bool, error) {
	switch value {
	case "", "off":
		return false, nil
	case "on":
		return true, nil
	default:
		return false, fmt.Errorf("must be \"on\" or \"off\"")
	}
}

func setCacheMetadata(result *provider.FetchResult, cached bool, cachedAt time.Time) {
	if result.Metadata == nil {
		result.Metadata = make(map[string]interface{})
	}
	result.Metadata["cached"] = cached
	if cached {
		result.Metadata["cached_at"] = cachedAt.UTC().Format(time.RFC3339)
		return
	}
	delete(result.Metadata, "cached_at")
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
