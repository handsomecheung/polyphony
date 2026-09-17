package cache

import (
	"strings"
	"testing"
)

func TestKeyDeterminism(t *testing.T) {
	k1 := Key("webreader", "https://example.com", "en", "static", false, nil)
	k2 := Key("webreader", "https://example.com", "en", "static", false, nil)
	if k1 != k2 {
		t.Fatalf("expected identical keys for identical inputs, got %q vs %q", k1, k2)
	}
}

func TestKeyActionsImpact(t *testing.T) {
	kWithoutActions := Key("webreader", "https://example.com", "en", "rendered", false, nil)
	actions1 := []map[string]interface{}{
		{"type": "click", "selector": "#btn1"},
	}
	actions2 := []map[string]interface{}{
		{"type": "click", "selector": "#btn2"},
	}

	kWithActions1 := Key("webreader", "https://example.com", "en", "rendered", false, actions1)
	kWithActions2 := Key("webreader", "https://example.com", "en", "rendered", false, actions2)

	if kWithoutActions == kWithActions1 {
		t.Fatalf("expected key with actions to differ from key without actions")
	}
	if kWithActions1 == kWithActions2 {
		t.Fatalf("expected different actions to produce different keys")
	}
}

func TestKeyRemoveMediaImpact(t *testing.T) {
	kWithMedia := Key("webreader", "https://example.com", "en", "static", false, nil)
	kWithoutMedia := Key("webreader", "https://example.com", "en", "static", true, nil)
	if kWithMedia == kWithoutMedia {
		t.Fatal("expected remove_media to produce a distinct cache key")
	}
}

func TestKeyPrefix(t *testing.T) {
	key := Key("other-service", "https://example.com", "en", "static", false, nil)
	if !strings.HasPrefix(key, "other-service:markdown:v1:") {
		t.Fatalf("expected configured key prefix, got %q", key)
	}
}
