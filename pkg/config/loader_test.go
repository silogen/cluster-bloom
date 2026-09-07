package config

import (
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
