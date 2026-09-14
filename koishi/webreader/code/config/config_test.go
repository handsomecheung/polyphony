package config

import (
	"os"
	"testing"
)

func TestLoadConfigSuccess(t *testing.T) {
	_ = os.Setenv("DEFAULT_LANGUAGE", "en")
	defer os.Unsetenv("DEFAULT_LANGUAGE")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.DefaultLanguage != "en" {
		t.Errorf("expected default language 'en', got '%s'", cfg.DefaultLanguage)
	}
}

func TestLoadConfigMissingLanguageError(t *testing.T) {
	_ = os.Unsetenv("DEFAULT_LANGUAGE")
	_ = os.Unsetenv("LANGUAGE")

	cfg, err := LoadConfig()
	if err == nil {
		t.Fatalf("expected error when DEFAULT_LANGUAGE is missing, got cfg: %+v", cfg)
	}
}
