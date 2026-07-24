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

// TestValidateRancherPartitionThresholds covers the numeric + ordering checks on
// the configurable /var/lib/rancher size thresholds.
func TestValidateRancherPartitionThresholds(t *testing.T) {
	cases := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{"defaults_absent", Config{}, false},
		{"valid_string", Config{"RANCHER_PARTITION_MIN_GB": "100", "RANCHER_PARTITION_RECOMMENDED_GB": "500"}, false},
		{"valid_int", Config{"RANCHER_PARTITION_MIN_GB": 100, "RANCHER_PARTITION_RECOMMENDED_GB": 500}, false},
		{"equal_ok", Config{"RANCHER_PARTITION_MIN_GB": "500", "RANCHER_PARTITION_RECOMMENDED_GB": "500"}, false},
		{"min_gt_recommended", Config{"RANCHER_PARTITION_MIN_GB": "600", "RANCHER_PARTITION_RECOMMENDED_GB": "500"}, true},
		{"non_numeric", Config{"RANCHER_PARTITION_MIN_GB": "lots"}, true},
		{"negative", Config{"RANCHER_PARTITION_MIN_GB": "-5"}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateRancherPartitionThresholds(tc.cfg)
			if tc.wantErr && len(errs) == 0 {
				t.Errorf("expected an error, got none")
			}
			if !tc.wantErr && len(errs) != 0 {
				t.Errorf("expected no error, got %v", errs)
			}
		})
	}
}

// TestHostOSIsSupported checks os-release parsing and the supported-OS match.
func TestHostOSIsSupported(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "ubuntu supported version",
			content: "ID=ubuntu\nVERSION_ID=\"22.04\"\nPRETTY_NAME=\"Ubuntu 22.04.4 LTS\"\n",
			want:    true,
		},
		{
			name:    "ubuntu unsupported version",
			content: "ID=ubuntu\nVERSION_ID=\"18.04\"\n",
			want:    false,
		},
		{
			name:    "opensuse tumbleweed unsupported",
			content: "ID=\"opensuse-tumbleweed\"\nVERSION_ID=\"20260721\"\nPRETTY_NAME=\"openSUSE Tumbleweed\"\n",
			want:    false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			host := parseOSRelease(tc.content)
			if got := host.IsSupported(); got != tc.want {
				t.Errorf("IsSupported() = %v, want %v (host=%+v)", got, tc.want, host)
			}
		})
	}
}

// TestHostOSDisplayName verifies the fallback ordering for the OS label.
func TestHostOSDisplayName(t *testing.T) {
	if got := parseOSRelease("ID=\"opensuse-tumbleweed\"\nPRETTY_NAME=\"openSUSE Tumbleweed\"\n").DisplayName(); got != "openSUSE Tumbleweed" {
		t.Errorf("DisplayName() = %q, want PRETTY_NAME", got)
	}
	if got := parseOSRelease("ID=ubuntu\nVERSION_ID=\"22.04\"\n").DisplayName(); got != "ubuntu 22.04" {
		t.Errorf("DisplayName() = %q, want id+version fallback", got)
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
