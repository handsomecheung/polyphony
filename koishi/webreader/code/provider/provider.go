package provider

import (
	"context"
	"fmt"
	"sync"
)

// FetchOptions contains options passed to a Provider for scraping/reading a URL.
type FetchOptions struct {
	URL           string                   `json:"url"`
	Language      string                   `json:"language,omitempty"`
	CustomHeaders map[string]string        `json:"custom_headers,omitempty"`
	Actions       []map[string]interface{} `json:"actions,omitempty"`
}

// FetchResult represents the standardized output of a webpage reader extraction.
type FetchResult struct {
	URL         string                 `json:"url"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Content     string                 `json:"content"`
	Provider    string                 `json:"provider"`
	StatusCode  int                    `json:"status_code"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Provider defines the interface that all webpage reader backends must satisfy.
type Provider interface {
	Name() string
	Fetch(ctx context.Context, opts FetchOptions) (*FetchResult, error)
}

// Registry manages available reader providers.
type Registry struct {
	mu          sync.RWMutex
	providers   map[string]Provider
	defaultName string
}

// NewRegistry creates a new Provider Registry.
func NewRegistry(defaultName string) *Registry {
	return &Registry{
		providers:   make(map[string]Provider),
		defaultName: defaultName,
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.Name()] = p
}

// Get returns the provider by name, or the default provider if name is empty.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	targetName := name
	if targetName == "" {
		targetName = r.defaultName
	}

	p, ok := r.providers[targetName]
	if !ok {
		return nil, fmt.Errorf("provider '%s' not found", targetName)
	}
	return p, nil
}

// ListNames returns the list of registered provider names and the default name.
func (r *Registry) ListNames() (defaultName string, names []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names = make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return r.defaultName, names
}
