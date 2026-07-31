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
}

func TestParseEnvironmentValueRejectsInvalidBoolean(t *testing.T) {
	if _, err := parseEnvironmentValue(Argument{Key: "GPU_NODE", Type: "bool"}, "not-a-bool"); err == nil {
		t.Fatal("expected invalid boolean error")
	}
}
