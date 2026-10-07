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
