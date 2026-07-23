package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfigEmptyFile guards against the nil-map panic that occurred when an
// empty (or comment-only) bloom.yaml unmarshalled to a nil map and applyDefaults
// tried to assign into it.
func TestLoadConfigEmptyFile(t *testing.T) {
	cases := map[string]string{
		"empty":        "",
		"comment_only": "# just a comment\n",
		"whitespace":   "\n  \n",
	}

	for name, contents := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "bloom.yaml")
			if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
				t.Fatalf("write temp config: %v", err)
			}

			cfg, err := LoadConfig(path)
			if err != nil {
				t.Fatalf("LoadConfig(%q) returned error: %v", name, err)
			}
			if cfg == nil {
				t.Fatalf("LoadConfig(%q) returned nil config", name)
			}
		})
	}
}

// TestValidateOptionalSkipsRequired verifies that node-local diagnostic runs can
// validate an empty config (with schema defaults applied, as in the real load
// path) without hard-failing on required cluster fields, while full Validate
// still enforces them.
func TestValidateOptionalSkipsRequired(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bloom.yaml")
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	if errs := Validate(cfg); len(errs) == 0 {
		t.Fatalf("Validate(empty+defaults) = no errors, want required-field errors")
	}
	if errs := ValidateOptional(cfg); len(errs) != 0 {
		t.Errorf("ValidateOptional(empty+defaults) = %v, want no errors", errs)
	}
}

// TestValidateOptionalStillFlagsUnknownKeys ensures relaxed validation still
// catches typos / removed keys.
func TestValidateOptionalStillFlagsUnknownKeys(t *testing.T) {
	cfg := Config{"FAKE": "value"}

	errs := ValidateOptional(cfg)
	found := false
	for _, e := range errs {
		if e == "Unknown configuration key: FAKE" {
			found = true
		}
	}
	if !found {
		t.Errorf("ValidateOptional(unknown key) = %v, want unknown-key error", errs)
	}
}
