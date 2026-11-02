package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDirectFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gotUA := r.Header.Get("User-Agent"); gotUA != DefaultUserAgent {
			t.Errorf("User-Agent = %q, want %q", gotUA, DefaultUserAgent)
		}
		if r.Header.Get("Accept-Language") != "ja-JP" || r.Header.Get("X-Test") != "value" {
			t.Error("missing target headers")
		}
		if r.URL.Path == "/start" {
			http.Redirect(w, r, "/concert/index.html", http.StatusFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><head><title>公演一覧</title><meta name="description" content="Concerts"><base href="/events/"></head><body><h1>Concerts</h1><p><strong>Welcome</strong></p><a href="detail"><img src="poster.jpg" alt="公演詳細"></a><script>secret_script()</script><style>secret_style</style><video src="movie.mp4"></video><table><tr><th>Date</th></tr><tr><td>November</td></tr></table></body></html>`)
	}))
	defer server.Close()
	p := NewDirectProvider()
	for _, removeMedia := range []bool{false, true} {
		result, err := p.Fetch(context.Background(), FetchOptions{URL: server.URL + "/start", Language: "ja-JP", CustomHeaders: map[string]string{"X-Test": "value"}, RemoveMedia: removeMedia})
		if err != nil {
			t.Fatal(err)
		}
		if result.URL != server.URL+"/concert/index.html" || result.Title != "公演一覧" || result.Description != "Concerts" || result.Provider != "direct" || result.StatusCode != 200 {
			t.Fatalf("unexpected result: %+v", result)
		}
		for _, want := range []string{"# Concerts", "**Welcome**", "公演詳細", server.URL + "/events/detail", "November"} {
			if !strings.Contains(result.Content, want) {
				t.Errorf("missing %q in %s", want, result.Content)
			}
		}
		if strings.Contains(result.Content, "secret_") {
			t.Errorf("script/style leaked: %s", result.Content)
		}
		if strings.Contains(result.Content, "poster.jpg") == removeMedia {
			t.Errorf("image removal=%v: %s", removeMedia, result.Content)
		}
		if removeMedia && strings.Contains(result.Content, "movie.mp4") {
			t.Error("video retained")
		}
	}

	// Verify custom User-Agent override
	overrideServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if gotUA := r.Header.Get("User-Agent"); gotUA != "CustomBrowser/3.0" {
			t.Errorf("User-Agent = %q, want %q", gotUA, "CustomBrowser/3.0")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<html><head><title>OK</title></head><body><p>OK</p></body></html>`)
	}))
	defer overrideServer.Close()
	if _, err := p.Fetch(context.Background(), FetchOptions{URL: overrideServer.URL, CustomHeaders: map[string]string{"User-Agent": "CustomBrowser/3.0"}}); err != nil {
		t.Fatalf("Fetch() with custom UA error = %v", err)
	}
}

func TestDirectErrors(t *testing.T) {
	for _, tc := range []struct {
		name        string
		status      int
		contentType string
		body        string
		want        string
	}{
		{"http error", 403, "text/html", "blocked", "403"},
		{"binary", 200, "application/pdf", "%PDF-1.4", "unsupported"},
		{"oversized", 200, "text/html", strings.Repeat("x", maxDirectBodyBytes+1), "exceeds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.contentType)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()
			_, err := NewDirectProvider().Fetch(context.Background(), FetchOptions{URL: server.URL})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestDirectCharsetAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=Shift_JIS")
		w.Write([]byte("<html><head><title>\x93\xfa\x96\x7b</title></head><body><p>\x93\xfa\x96\x7b</p></body></html>"))
	}))
	defer server.Close()
	p := NewDirectProvider()
	result, err := p.Fetch(context.Background(), FetchOptions{URL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	if result.Title != "日本" || !strings.Contains(result.Content, "日本") {
		t.Fatalf("charset decode failed: %+v", result)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	if _, err := p.Fetch(ctx, FetchOptions{URL: server.URL}); err == nil {
		t.Fatal("expected cancellation")
	}
}
