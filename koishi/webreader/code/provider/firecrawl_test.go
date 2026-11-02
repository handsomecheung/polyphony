package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestRemoveMarkdownImageURLs(t *testing.T) {
	content := "[![Next page](/icons/next.svg)](/articles?page=2)\n\n![Diagram](https://example.com/diagram.png)"
	want := "[Next page](/articles?page=2)\n\nDiagram"

	if got := removeMarkdownImageURLs(content); got != want {
		t.Fatalf("removeMarkdownImageURLs() = %q, want %q", got, want)
	}
}

func TestFirecrawlUserAgentHeaders(t *testing.T) {
	tests := []struct {
		name          string
		customHeaders map[string]string
		wantBodyUA    string
		wantReqUA     string
	}{
		{
			name:          "default user agent",
			customHeaders: nil,
			wantBodyUA:    DefaultUserAgent,
			wantReqUA:     DefaultUserAgent,
		},
		{
			name: "custom user agent override",
			customHeaders: map[string]string{
				"User-Agent": "CustomBrowser/2.0",
			},
			wantBodyUA: "CustomBrowser/2.0",
			wantReqUA:  DefaultUserAgent,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotReqUA string
			var gotBodyUA string

			client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				gotReqUA = req.Header.Get("User-Agent")
				bodyBytes, _ := io.ReadAll(req.Body)
				var body firecrawlScrapeRequest
				_ = json.Unmarshal(bodyBytes, &body)
				if body.Headers != nil {
					gotBodyUA = body.Headers["User-Agent"]
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"success":true,"data":{"markdown":"ok","metadata":{"title":"test"}}}`)),
					Header:     make(http.Header),
				}, nil
			})}

			p := &FirecrawlProvider{httpClient: client}
			_, err := p.Fetch(context.Background(), FetchOptions{
				URL:           "https://example.com",
				CustomHeaders: tc.customHeaders,
			})
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			if gotReqUA != tc.wantReqUA {
				t.Errorf("request User-Agent = %q, want %q", gotReqUA, tc.wantReqUA)
			}
			if gotBodyUA != tc.wantBodyUA {
				t.Errorf("scrape body User-Agent = %q, want %q", gotBodyUA, tc.wantBodyUA)
			}
		})
	}
}
