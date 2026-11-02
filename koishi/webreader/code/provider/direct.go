package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	markdown "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
)

const maxDirectBodyBytes = 10 << 20

// DirectProvider fetches and converts HTML within the service, without a hosted reader.
type DirectProvider struct{ httpClient *http.Client }

func NewDirectProvider() *DirectProvider {
	// Ignore proxy environment variables so this mode uses the container's own route.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &DirectProvider{httpClient: &http.Client{Transport: transport}}
}

func (d *DirectProvider) Name() string { return "direct" }

func (d *DirectProvider) Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error) {
	if len(opts.Actions) > 0 {
		return nil, fmt.Errorf("browser actions are not supported in static.direct mode")
	}
	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("create direct request: %w", err)
	}
	req.Header.Set("User-Agent", DefaultUserAgent)
	req.Header.Set("Accept", "text/html, application/xhtml+xml, text/plain;q=0.9")
	if opts.Language != "" {
		req.Header.Set("Accept-Language", opts.Language)
	}
	for k, v := range opts.CustomHeaders {
		req.Header.Set(k, v)
	}
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("direct request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("target returned HTTP status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxDirectBodyBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read direct response: %w", err)
	}
	if len(body) > maxDirectBodyBytes {
		return nil, fmt.Errorf("direct response exceeds %d bytes", maxDirectBodyBytes)
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, fmt.Errorf("invalid target content type: %w", err)
	}
	if mediaType != "text/html" && mediaType != "application/xhtml+xml" && mediaType != "text/plain" {
		return nil, fmt.Errorf("unsupported direct content type %q: expected HTML or plain text", mediaType)
	}
	reader, err := charset.NewReader(bytes.NewReader(body), contentType)
	if err != nil {
		return nil, fmt.Errorf("decode direct response: %w", err)
	}
	result := &FetchResult{URL: resp.Request.URL.String(), Provider: d.Name(), StatusCode: resp.StatusCode, Metadata: map[string]interface{}{"content_type": contentType}}
	if mediaType == "text/plain" {
		text, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		result.Content = string(text)
	} else {
		doc, err := goquery.NewDocumentFromReader(reader)
		if err != nil {
			return nil, fmt.Errorf("parse target HTML: %w", err)
		}
		result.Title = strings.TrimSpace(doc.Find("title").First().Text())
		result.Description, _ = doc.Find(`meta[name="description"]`).First().Attr("content")
		baseURL := resp.Request.URL
		if href, ok := doc.Find("base[href]").First().Attr("href"); ok {
			if base, err := url.Parse(href); err == nil {
				baseURL = baseURL.ResolveReference(base)
			}
		}
		doc.Find("head, script, style, noscript, template").Remove()
		doc.Find("[href], [src]").Each(func(_ int, s *goquery.Selection) {
			for _, attr := range []string{"href", "src"} {
				if value, ok := s.Attr(attr); ok {
					if ref, err := url.Parse(value); err == nil {
						s.SetAttr(attr, baseURL.ResolveReference(ref).String())
					}
				}
			}
		})
		if opts.RemoveMedia {
			doc.Find("img").Each(func(_ int, s *goquery.Selection) {
				alt, _ := s.Attr("alt")
				s.BeforeHtml("<span></span>")
				s.Prev().SetText(alt)
				s.Remove()
			})
			doc.Find("video, audio, iframe, embed, object, picture source, svg").Remove()
		}
		converter := markdown.NewConverter("", true, nil)
		converter.Use(plugin.GitHubFlavored())
		result.Content = converter.Convert(doc.Selection)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result.Metadata["duration_ms"] = time.Since(start).Milliseconds()
	return result, nil
}
