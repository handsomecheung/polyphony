package cache

import (
	"testing"
)

func TestKeyDeterminism(t *testing.T) {
	k1 := Key("https://example.com", "en", "static", nil)
	k2 := Key("https://example.com", "en", "static", nil)
	if k1 != k2 {
		t.Fatalf("expected identical keys for identical inputs, got %q vs %q", k1, k2)
	}
}

func TestKeyActionsImpact(t *testing.T) {
	kWithoutActions := Key("https://example.com", "en", "rendered", nil)
	actions1 := []map[string]interface{}{
		{"type": "click", "selector": "#btn1"},
	}
	actions2 := []map[string]interface{}{
		{"type": "click", "selector": "#btn2"},
	}

	kWithActions1 := Key("https://example.com", "en", "rendered", actions1)
	kWithActions2 := Key("https://example.com", "en", "rendered", actions2)

	if kWithoutActions == kWithActions1 {
		t.Fatalf("expected key with actions to differ from key without actions")
	}
	if kWithActions1 == kWithActions2 {
		t.Fatalf("expected different actions to produce different keys")
	}
}
