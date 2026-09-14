package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds runtime configuration for the webreader service.
type Config struct {
	Port                  string
	DefaultLanguage       string
	JinaAPIKey            string
	FirecrawlAPIKey       string
	DefaultTimeoutSecs    int
	MaxTimeoutSecs        int
	MaxConcurrentRequests int
	MaxRequestsPerMinute  int
}

// LoadConfig loads configuration from environment variables with sensible defaults.
// It returns an error if any required configuration is missing.
func LoadConfig() (*Config, error) {
	port := getEnv("PORT", "8080")
	defaultLanguage := getEnv("DEFAULT_LANGUAGE", getEnv("LANGUAGE", ""))
	if strings.TrimSpace(defaultLanguage) == "" {
		return nil, fmt.Errorf("environment variable DEFAULT_LANGUAGE is required")
	}

	jinaAPIKey := getEnv("JINA_API_KEY", "")
	firecrawlAPIKey := getEnv("FIRECRAWL_APIKEY", "")
	if strings.TrimSpace(firecrawlAPIKey) == "" {
		return nil, fmt.Errorf("environment variable FIRECRAWL_APIKEY is required")
	}

	defaultTimeout := getEnvAsInt("DEFAULT_TIMEOUT_SECONDS", 45)
	maxTimeout := getEnvAsInt("MAX_TIMEOUT_SECONDS", 180)
	maxConcurrent := getEnvAsInt("MAX_CONCURRENT_REQUESTS", 1)
	maxRPM := getEnvAsInt("MAX_REQUESTS_PER_MINUTE", 60)

	return &Config{
		Port:                  port,
		DefaultLanguage:       defaultLanguage,
		JinaAPIKey:            jinaAPIKey,
		FirecrawlAPIKey:       firecrawlAPIKey,
		DefaultTimeoutSecs:    defaultTimeout,
		MaxTimeoutSecs:        maxTimeout,
		MaxConcurrentRequests: maxConcurrent,
		MaxRequestsPerMinute:  maxRPM,
	}, nil
}

// GetTimeout returns the effective timeout duration based on request parameter and server bounds.
func (c *Config) GetTimeout(reqTimeoutSeconds int) time.Duration {
	if reqTimeoutSeconds <= 0 {
		return time.Duration(c.DefaultTimeoutSecs) * time.Second
	}
	if reqTimeoutSeconds > c.MaxTimeoutSecs {
		return time.Duration(c.MaxTimeoutSecs) * time.Second
	}
	return time.Duration(reqTimeoutSeconds) * time.Second
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := getEnv(key, "")
	if val, err := strconv.Atoi(valStr); err == nil && val > 0 {
		return val
	}
	return defaultVal
}
