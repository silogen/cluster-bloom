package config

import (
	"strings"
	"testing"
)

func TestApplyDeprecationsStripsKeyAndWarns(t *testing.T) {
	cfg := Config{"OIDC_URL": "https://old.example.com", "DOMAIN": "example.com"}

	warnings := ApplyDeprecations(cfg)

	if _, present := cfg["OIDC_URL"]; present {
		t.Error("deprecated OIDC_URL must be stripped from cfg so Validate does not hard-fail on it")
	}
	if cfg["DOMAIN"] != "example.com" {
		t.Errorf("non-deprecated keys must be untouched, DOMAIN = %v", cfg["DOMAIN"])
	}
	if len(warnings) != 1 {
		t.Fatalf("want 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "OIDC_URL") || !strings.Contains(warnings[0], "DOMAIN") {
		t.Errorf("warning should name the key and its migration path, got %q", warnings[0])
	}
}

func TestApplyDeprecationsNoDeprecatedKeysIsNoOp(t *testing.T) {
	cfg := Config{"DOMAIN": "example.com", "GPU_NODE": true}

	if warnings := ApplyDeprecations(cfg); len(warnings) != 0 {
		t.Errorf("clean config must produce no warnings, got %v", warnings)
	}
	if len(cfg) != 2 {
		t.Errorf("clean config must be left intact, got %v", cfg)
	}
}

func TestApplyDeprecationsWarningsAreDeterministic(t *testing.T) {
	// Every known deprecated key at once — output must be sorted/stable.
	cfg := Config{}
	for key := range deprecatedKeys {
		cfg[key] = "x"
	}

	warnings := ApplyDeprecations(cfg)

	if len(warnings) != len(deprecatedKeys) {
		t.Fatalf("want %d warnings, got %d", len(deprecatedKeys), len(warnings))
	}
	for i := 1; i < len(warnings); i++ {
		if warnings[i-1] > warnings[i] {
			t.Errorf("warnings not sorted: %q came before %q", warnings[i-1], warnings[i])
		}
	}
	if len(cfg) != 0 {
		t.Errorf("all deprecated keys must be stripped, remaining: %v", cfg)
	}
}

// TestDeprecatedKeysAreNotAlsoCurrentKeys guards against a key being listed as
// both deprecated and valid — which would make ApplyDeprecations silently strip
// a still-supported setting.
func TestDeprecatedKeysAreNotAlsoCurrentKeys(t *testing.T) {
	valid := map[string]bool{}
	for _, arg := range Schema() {
		valid[arg.Key] = true
	}
	for key := range deprecatedKeys {
		if valid[key] {
			t.Errorf("%q is in both the deprecation registry and the current schema", key)
		}
	}
}
