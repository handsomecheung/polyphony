package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestJinaFetchUsesUnfilteredMarkdown(t *testing.T) {
	client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		if got := req.Header.Get("X-Respond-With"); got != "markdown" {
			t.Errorf("X-Respond-With = %q, want %q", got, "markdown")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"data":{"url":"https://example.com","content":"content"}}`)),
			Header:     make(http.Header),
		}, nil
	})}
	provider := &JinaProvider{httpClient: client}

	_, err := provider.Fetch(context.Background(), FetchOptions{
		URL:           "https://example.com",
		CustomHeaders: map[string]string{"X-Respond-With": "text"},
	})
	if err != nil {
		t.Fatalf("Fetch() error = %v", err)
	}
}

func TestJinaUserAgentHeaders(t *testing.T) {
	tests := []struct {
		name          string
		customHeaders map[string]string
		wantUA        string
		wantXUA       string
	}{
		{
			name:          "default user agent",
			customHeaders: nil,
			wantUA:        DefaultUserAgent,
			wantXUA:       DefaultUserAgent,
		},
		{
			name: "custom user agent forwards to x-user-agent",
			customHeaders: map[string]string{
				"User-Agent": "CustomBrowser/1.0",
			},
			wantUA:  "CustomBrowser/1.0",
			wantXUA: "CustomBrowser/1.0",
		},
		{
			name: "explicit x-user-agent preserved",
			customHeaders: map[string]string{
				"User-Agent":   "OuterUA/1.0",
				"X-User-Agent": "TargetUA/1.0",
			},
			wantUA:  "OuterUA/1.0",
			wantXUA: "TargetUA/1.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotUA, gotXUA string
			client := &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				gotUA = req.Header.Get("User-Agent")
				gotXUA = req.Header.Get("X-User-Agent")
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"data":{"url":"https://example.com","content":"ok"}}`)),
					Header:     make(http.Header),
				}, nil
			})}
			p := &JinaProvider{httpClient: client}
			_, err := p.Fetch(context.Background(), FetchOptions{
				URL:           "https://example.com",
				CustomHeaders: tc.customHeaders,
			})
			if err != nil {
				t.Fatalf("Fetch() error = %v", err)
			}
			if gotUA != tc.wantUA {
				t.Errorf("User-Agent = %q, want %q", gotUA, tc.wantUA)
			}
			if gotXUA != tc.wantXUA {
				t.Errorf("X-User-Agent = %q, want %q", gotXUA, tc.wantXUA)
			}
		})
	}
}
