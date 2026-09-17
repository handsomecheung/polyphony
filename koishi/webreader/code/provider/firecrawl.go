package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"
)

const firecrawlBaseURL = "https://api.firecrawl.dev/v2"

var markdownImagePattern = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)

// FirecrawlProvider implements the Provider interface using Firecrawl (api.firecrawl.dev).
// Unlike Jina, Firecrawl uses a headless browser to execute JavaScript before
// extracting content, making it suitable for single-page applications and
// dynamically rendered pages.
type FirecrawlProvider struct {
	apiKey     string
	httpClient *http.Client
}

// NewFirecrawlProvider creates a new FirecrawlProvider instance.
func NewFirecrawlProvider(apiKey string) *FirecrawlProvider {
	return &FirecrawlProvider{
		apiKey: apiKey,
		httpClient: &http.Client{
			// Individual request timeout is governed by context
			Timeout: 0,
		},
	}
}

func (f *FirecrawlProvider) Name() string {
	return "firecrawl"
}

// firecrawlScrapeRequest models the POST /v2/scrape request body.
type firecrawlScrapeRequest struct {
	URL         string                   `json:"url"`
	Formats     []string                 `json:"formats"`
	ExcludeTags []string                 `json:"excludeTags,omitempty"`
	Headers     map[string]string        `json:"headers,omitempty"`
	Location    *firecrawlLocation       `json:"location,omitempty"`
	Actions     []map[string]interface{} `json:"actions,omitempty"`
}

// firecrawlLocation sets the proxy location and browser language/timezone emulation.
type firecrawlLocation struct {
	Languages []string `json:"languages,omitempty"`
}

// firecrawlScrapeResponse models the POST /v2/scrape JSON response.
type firecrawlScrapeResponse struct {
	Success bool `json:"success"`
	Data    struct {
		Markdown string                 `json:"markdown"`
		Metadata map[string]interface{} `json:"metadata"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

func (f *FirecrawlProvider) Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error) {
	reqBody := firecrawlScrapeRequest{
		URL:     opts.URL,
		Formats: []string{"markdown"},
	}

	// Forward browser actions (e.g. click, wait, write, executeJavascript)
	if len(opts.Actions) > 0 {
		reqBody.Actions = opts.Actions
	}

	if opts.RemoveMedia {
		// Do not exclude img, picture, or svg before Markdown conversion: an
		// image-only pagination link would otherwise become empty and be dropped.
		reqBody.ExcludeTags = []string{"video", "audio", "iframe", "embed", "object"}
	}

	// Set language preference via Firecrawl's location.languages
	if opts.Language != "" {
		reqBody.Location = &firecrawlLocation{
			Languages: []string{opts.Language},
		}
	}

	// Forward custom headers directly to the target page
	if len(opts.CustomHeaders) > 0 {
		reqBody.Headers = opts.CustomHeaders
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal firecrawl request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, firecrawlBaseURL+"/scrape", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create firecrawl request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "webreader/1.0 (koishi-cluster)")
	if f.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+f.apiKey)
	}

	startTime := time.Now()
	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("firecrawl request failed: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read firecrawl response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("firecrawl returned error status %d: %s", resp.StatusCode, string(rawBody))
	}

	var fcResp firecrawlScrapeResponse
	if err := json.Unmarshal(rawBody, &fcResp); err != nil {
		return nil, fmt.Errorf("failed to parse firecrawl response: %w", err)
	}
	if !fcResp.Success {
		return nil, fmt.Errorf("firecrawl scrape failed: %s", fcResp.Error)
	}

	metadata := make(map[string]interface{})
	for k, v := range fcResp.Data.Metadata {
		metadata[k] = v
	}
	metadata["duration_ms"] = time.Since(startTime).Milliseconds()

	// Extract well-known metadata fields for top-level response fields
	title, _ := fcResp.Data.Metadata["title"].(string)
	description, _ := fcResp.Data.Metadata["description"].(string)
	sourceURL, _ := fcResp.Data.Metadata["sourceURL"].(string)
	if sourceURL == "" {
		sourceURL = opts.URL
	}

	content := fcResp.Data.Markdown
	if opts.RemoveMedia {
		content = removeMarkdownImageURLs(content)
	}

	return &FetchResult{
		URL:         sourceURL,
		Title:       title,
		Description: description,
		Content:     content,
		Provider:    f.Name(),
		StatusCode:  resp.StatusCode,
		Metadata:    metadata,
	}, nil
}

// removeMarkdownImageURLs preserves image alt text while removing image URLs.
// This retains the label and destination of links whose only child is an image.
func removeMarkdownImageURLs(content string) string {
	return markdownImagePattern.ReplaceAllString(content, "$1")
}
