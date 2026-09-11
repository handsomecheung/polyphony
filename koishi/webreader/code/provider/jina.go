package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const jinaBaseURL = "https://r.jina.ai"

// JinaProvider implements the Provider interface using Jina Reader (r.jina.ai).
type JinaProvider struct {
	apiKey     string
	httpClient *http.Client
}

// NewJinaProvider creates a new JinaProvider instance.
func NewJinaProvider(apiKey string) *JinaProvider {
	return &JinaProvider{
		apiKey: apiKey,
		httpClient: &http.Client{
			// Individual request timeout is governed by context
			Timeout: 0,
		},
	}
}

func (j *JinaProvider) Name() string {
	return "jina"
}

// jinaJSONResponse models the standard JSON response from Jina Reader.
type jinaJSONResponse struct {
	Code   int    `json:"code"`
	Status int    `json:"status"`
	Data   struct {
		Title       string                 `json:"title"`
		Description string                 `json:"description"`
		URL         string                 `json:"url"`
		Content     string                 `json:"content"`
		Usage       map[string]interface{} `json:"usage,omitempty"`
		Images      map[string]interface{} `json:"images,omitempty"`
		Links       map[string]interface{} `json:"links,omitempty"`
	} `json:"data"`
	Message string `json:"message,omitempty"`
}

func (j *JinaProvider) Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error) {
	reqURL := fmt.Sprintf("%s/%s", jinaBaseURL, opts.URL)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create jina request: %w", err)
	}

	// Request JSON response from Jina
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "webreader/1.0 (koishi-cluster)")

	if j.apiKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", j.apiKey))
	}

	if opts.WithImagesSummary {
		req.Header.Set("X-With-Images-Summary", "true")
	}
	if opts.WithLinksSummary {
		req.Header.Set("X-With-Links-Summary", "true")
	}
	if opts.NoCache {
		req.Header.Set("X-No-Cache", "true")
	}
	if opts.WaitForSelector != "" {
		req.Header.Set("X-Wait-For-Selector", opts.WaitForSelector)
	}
	if opts.TargetSelector != "" {
		req.Header.Set("X-Target-Selector", opts.TargetSelector)
	}

	// Forward any custom headers
	for k, v := range opts.CustomHeaders {
		req.Header.Set(k, v)
	}

	startTime := time.Now()
	resp, err := j.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jina request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("jina reader returned error status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Try parsing JSON format
	var jinaResp jinaJSONResponse
	if err := json.Unmarshal(bodyBytes, &jinaResp); err == nil && (jinaResp.Data.Content != "" || jinaResp.Data.Title != "") {
		metadata := make(map[string]interface{})
		if jinaResp.Data.Usage != nil {
			metadata["usage"] = jinaResp.Data.Usage
		}
		if jinaResp.Data.Images != nil {
			metadata["images"] = jinaResp.Data.Images
		}
		if jinaResp.Data.Links != nil {
			metadata["links"] = jinaResp.Data.Links
		}
		metadata["duration_ms"] = time.Since(startTime).Milliseconds()

		finalURL := jinaResp.Data.URL
		if finalURL == "" {
			finalURL = opts.URL
		}

		return &FetchResult{
			URL:         finalURL,
			Title:       jinaResp.Data.Title,
			Description: jinaResp.Data.Description,
			Content:     jinaResp.Data.Content,
			Provider:    j.Name(),
			StatusCode:  resp.StatusCode,
			Metadata:    metadata,
		}, nil
	}

	// Fallback: If Jina returned raw text/markdown rather than JSON
	rawContent := string(bodyBytes)
	title := ""
	lines := strings.Split(rawContent, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Title:") {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "Title:"))
			break
		} else if strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			break
		}
	}

	return &FetchResult{
		URL:        opts.URL,
		Title:      title,
		Content:    rawContent,
		Provider:   j.Name(),
		StatusCode: resp.StatusCode,
		Metadata: map[string]interface{}{
			"duration_ms": time.Since(startTime).Milliseconds(),
			"fallback":    "raw_text",
		},
	}, nil
}
