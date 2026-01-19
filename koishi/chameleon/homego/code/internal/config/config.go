package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	// Server settings
	Port string

	// Cookie settings
	CookieSecure   bool
	CookieSameSite string
	CookieExpiry   time.Duration

	// JWT settings
	JWTAlgorithm string
	JWTExpiresIn time.Duration

	// Auth settings
	AuthClickRangeMS    int64
	AuthClickIntervalMS int64
	AuthClickOffsetMS   int64

	TOTPSecret string
}

func NewConfig() (*Config, error) {
	totpSecret := os.Getenv("TOTP_SECRET_KEY")
	if totpSecret == "" {
		return nil, fmt.Errorf("TOTP_SECRET_KEY is not set")
	}

	return &Config{
		Port:                ":8000",
		CookieSecure:        true,
		CookieSameSite:      "Lax",
		CookieExpiry:        12 * time.Hour,
		JWTAlgorithm:        "HS256",
		JWTExpiresIn:        12 * time.Hour,
		AuthClickRangeMS:    20000,
		AuthClickIntervalMS: 2000,
		AuthClickOffsetMS:   800,
		TOTPSecret:          totpSecret,
	}, nil
}
