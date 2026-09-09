package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseEnvironmentValuePreservesSchemaTypes(t *testing.T) {
	t.Run("boolean false", func(t *testing.T) {
		got, err := parseEnvironmentValue(Argument{Key: "FIX_DNS", Type: "bool"}, "false")
		if err != nil {
			t.Fatalf("parse boolean: %v", err)
		}
		if got != false {
			t.Fatalf("got %#v (%T), want bool false", got, got)
		}
	})

	t.Run("comma separated array", func(t *testing.T) {
		got, err := parseEnvironmentValue(Argument{Key: "DNS_SERVERS", Type: "array"}, "8.8.8.8, 1.1.1.1")
		if err != nil {
			t.Fatalf("parse array: %v", err)
		}
		want := []any{"8.8.8.8", "1.1.1.1"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("yaml array", func(t *testing.T) {
		got, err := parseEnvironmentValue(Argument{Key: "DNS_SERVERS", Type: "array"}, `["8.8.8.8", "1.1.1.1"]`)
		if err != nil {
			t.Fatalf("parse YAML array: %v", err)
		}
		want := []any{"8.8.8.8", "1.1.1.1"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	// Without a "map" case the value reached Ansible as a raw string. That did
	// work end to end - the task normalizes strings - but it meant an exported
	// bloom.yaml round-tripped the value as a folded string rather than a
	// mapping, and a typo was only caught later by the validator.
	t.Run("yaml mapping", func(t *testing.T) {
		got, err := parseEnvironmentValue(
			Argument{Key: "CILIUM_HELM_VALUES", Type: "map"},
			"hubble:\n  enabled: true\n",
		)
		if err != nil {
			t.Fatalf("parse YAML mapping: %v", err)
		}
		want := map[string]any{"hubble": map[string]any{"enabled": true}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("json mapping", func(t *testing.T) {
		got, err := parseEnvironmentValue(
			Argument{Key: "CILIUM_HELM_VALUES", Type: "map"},
			`{"operator": {"replicas": 2}}`,
		)
		if err != nil {
			t.Fatalf("parse JSON mapping: %v", err)
		}
		want := map[string]any{"operator": map[string]any{"replicas": 2}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})

	t.Run("blank mapping", func(t *testing.T) {
		got, err := parseEnvironmentValue(Argument{Key: "CILIUM_HELM_VALUES", Type: "map"}, "   ")
		if err != nil {
			t.Fatalf("parse blank mapping: %v", err)
		}
		if want := map[string]any{}; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %#v, want %#v", got, want)
		}
	})
}

func TestParseEnvironmentValueRejectsNonMappings(t *testing.T) {
	for name, value := range map[string]string{
		"scalar":        "hubble",
		"list":          "[a, b]",
		"malformed":     "hubble:\n\tenabled: true",
		"nested scalar": "42",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseEnvironmentValue(
				Argument{Key: "CILIUM_HELM_VALUES", Type: "map"}, value,
			); err == nil {
				t.Fatalf("expected an error for %q", value)
			}
		})
	}
}

func TestParseEnvironmentValueRejectsInvalidBoolean(t *testing.T) {
	if _, err := parseEnvironmentValue(Argument{Key: "GPU_NODE", Type: "bool"}, "not-a-bool"); err == nil {
		t.Fatal("expected invalid boolean error")
	}
}

// Config is `type Config map[string]any` - a *named* map type. yaml.v3
// decodes a nested mapping node using the destination's static type when
// that type is `any`, but when the destination is itself a named map type
// it recurses using that same named type instead of the plain
// map[string]any every other map[string]any-typed value in this package
// gets. So unmarshaling a bloom.yaml straight into *Config (as LoadConfig
// used to) turned CILIUM_HELM_VALUES's nested map into config.Config, not
// map[string]any - and every `value.(map[string]any)` assertion downstream
// (validator, generator, ansible var encoding) failed as if the field were
// some unsupported type. Constructing a Config literal by hand in a test,
// as the other tests in this package do, never goes through yaml.v3 and so
// never observes this: only a real LoadConfig(file) round trip does.
func TestLoadConfigCiliumHelmValuesSurvivesValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bloom.yaml")
	yamlContent := `
FIRST_NODE: true
GPU_NODE: false
DOMAIN: cluster.example.com
CLUSTER_SIZE: small
NO_DISKS_FOR_CLUSTER: true
CERT_OPTION: generate
CILIUM_HELM_VALUES:
  hubble:
    enabled: true
    relay:
      enabled: true
    ui:
      enabled: true
`
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("write bloom.yaml: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}

	value, ok := cfg["CILIUM_HELM_VALUES"].(map[string]any)
	if !ok {
		t.Fatalf("CILIUM_HELM_VALUES is %T, want map[string]any", cfg["CILIUM_HELM_VALUES"])
	}
	want := map[string]any{
		"hubble": map[string]any{
			"enabled": true,
			"relay":   map[string]any{"enabled": true},
			"ui":      map[string]any{"enabled": true},
		},
	}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("got %#v, want %#v", value, want)
	}

	if errors := Validate(cfg); len(errors) > 0 {
		t.Fatalf("expected a real CILIUM_HELM_VALUES mapping loaded from YAML to validate cleanly, got: %v", errors)
	}
}
