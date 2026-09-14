// Package cache provides Redis-backed storage for successful markdown fetches.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"webreader/provider"
)

const keyPrefix = "webreader:markdown:v1:"

// ErrMiss indicates that no unexpired entry exists for a key.
var ErrMiss = errors.New("cache miss")

// Entry is the value persisted for a successful provider fetch.
type Entry struct {
	Result   *provider.FetchResult `json:"result"`
	CachedAt time.Time             `json:"cached_at"`
}

// Store is the cache interface used by HTTP handlers.
type Store interface {
	Get(ctx context.Context, key string) (*Entry, error)
	Set(ctx context.Context, key string, entry *Entry) error
}

// RedisStore stores entries in Redis with a fixed TTL.
type RedisStore struct {
	client *redis.Client
	ttl    time.Duration
}

// NewRedisStore creates a store from a redis:// URL.
func NewRedisStore(redisURL string, ttl time.Duration) (*RedisStore, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_URL: %w", err)
	}
	return &RedisStore{client: redis.NewClient(opts), ttl: ttl}, nil
}

// Close releases the underlying Redis client resources.
func (s *RedisStore) Close() error {
	return s.client.Close()
}

func (s *RedisStore) Get(ctx context.Context, key string) (*Entry, error) {
	raw, err := s.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, ErrMiss
	}
	if err != nil {
		return nil, err
	}

	var entry Entry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, fmt.Errorf("decode cached result: %w", err)
	}
	if entry.Result == nil || entry.CachedAt.IsZero() {
		return nil, fmt.Errorf("decode cached result: invalid entry")
	}
	return &entry, nil
}

func (s *RedisStore) Set(ctx context.Context, key string, entry *Entry) error {
	raw, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("encode cached result: %w", err)
	}
	return s.client.Set(ctx, key, raw, s.ttl).Err()
}

// Key derives a bounded Redis key from every request component that can change
// the provider result. The raw URL is hashed to avoid Redis key size limits.
func Key(rawURL, language, mode string) string {
	sum := sha256.Sum256([]byte(rawURL + "\x00" + language + "\x00" + mode))
	return keyPrefix + hex.EncodeToString(sum[:])
}
