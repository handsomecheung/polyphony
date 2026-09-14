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
