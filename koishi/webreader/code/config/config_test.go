package config

import (
	"os"
	"testing"
)

func TestLoadConfigSuccess(t *testing.T) {
	_ = os.Setenv("DEFAULT_LANGUAGE", "en")
	defer os.Unsetenv("DEFAULT_LANGUAGE")
	_ = os.Setenv("FIRECRAWL_APIKEY", "test-key")
	defer os.Unsetenv("FIRECRAWL_APIKEY")
	_ = os.Setenv("REDIS_URL", "redis://localhost:6379")
	defer os.Unsetenv("REDIS_URL")
	_ = os.Setenv("REDIS_CACHE_TTL_SECONDS", "60")
	defer os.Unsetenv("REDIS_CACHE_TTL_SECONDS")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.DefaultLanguage != "en" {
		t.Errorf("expected default language 'en', got '%s'", cfg.DefaultLanguage)
	}
	if cfg.FirecrawlAPIKey != "test-key" {
		t.Errorf("expected firecrawl api key 'test-key', got '%s'", cfg.FirecrawlAPIKey)
	}
	if cfg.CacheKeyPrefix != "webreader" {
		t.Errorf("expected default cache key prefix 'webreader', got %q", cfg.CacheKeyPrefix)
	}
}

func TestLoadConfigCustomCacheKeyPrefix(t *testing.T) {
	t.Setenv("DEFAULT_LANGUAGE", "en")
	t.Setenv("FIRECRAWL_APIKEY", "test-key")
	t.Setenv("REDIS_URL", "redis://localhost:6379")
	t.Setenv("REDIS_CACHE_TTL_SECONDS", "60")
	t.Setenv("CACHE_KEY_PREFIX", "webreader-test")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.CacheKeyPrefix != "webreader-test" {
		t.Errorf("expected custom cache key prefix, got %q", cfg.CacheKeyPrefix)
	}
}

func TestLoadConfigMissingCacheTTLError(t *testing.T) {
	_ = os.Setenv("DEFAULT_LANGUAGE", "en")
	defer os.Unsetenv("DEFAULT_LANGUAGE")
	_ = os.Setenv("FIRECRAWL_APIKEY", "test-key")
	defer os.Unsetenv("FIRECRAWL_APIKEY")
	_ = os.Setenv("REDIS_URL", "redis://localhost:6379")
	defer os.Unsetenv("REDIS_URL")
	_ = os.Unsetenv("REDIS_CACHE_TTL_SECONDS")
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected error when REDIS_CACHE_TTL_SECONDS is missing")
	}
}

func TestLoadConfigMissingLanguageError(t *testing.T) {
	_ = os.Unsetenv("DEFAULT_LANGUAGE")
	_ = os.Unsetenv("LANGUAGE")
	_ = os.Setenv("FIRECRAWL_APIKEY", "test-key")
	defer os.Unsetenv("FIRECRAWL_APIKEY")

	cfg, err := LoadConfig()
	if err == nil {
		t.Fatalf("expected error when DEFAULT_LANGUAGE is missing, got cfg: %+v", cfg)
	}
}

func TestLoadConfigMissingFirecrawlKeyError(t *testing.T) {
	_ = os.Setenv("DEFAULT_LANGUAGE", "en")
	defer os.Unsetenv("DEFAULT_LANGUAGE")
	_ = os.Unsetenv("FIRECRAWL_APIKEY")

	cfg, err := LoadConfig()
	if err == nil {
		t.Fatalf("expected error when FIRECRAWL_APIKEY is missing, got cfg: %+v", cfg)
	}
}
