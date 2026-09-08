package config

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// generateAndParse asserts that what GenerateYAML wrote is still YAML, and
// returns it parsed. Substring assertions are not enough here: a top-level map
// used to render as Go's `map[operator:map[replicas:1]]`, which contains every
// substring a reader would think to check for and is not YAML at all.
func generateAndParse(t *testing.T, cfg Config) map[string]any {
	t.Helper()

	out, err := GenerateYAML(cfg)
	if err != nil {
		t.Fatalf("GenerateYAML: %v", err)
	}
	var got map[string]any
	if err := yaml.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("GenerateYAML produced invalid YAML: %v\n---\n%s", err, out)
	}
	return got
}

func TestGenerateYAMLRoundTripsANestedMap(t *testing.T) {
	values := map[string]any{
		"hubble": map[string]any{
			"enabled": true,
			"relay":   map[string]any{"enabled": true},
			"ui":      map[string]any{"enabled": true},
		},
		"operator": map[string]any{"replicas": 1},
	}

	got := generateAndParse(t, Config{
		"FIRST_NODE":         true,
		"GPU_NODE":           true,
		"DOMAIN":             "cluster.example.com",
		"CILIUM_HELM_VALUES": values,
	})

	if !reflect.DeepEqual(got["CILIUM_HELM_VALUES"], values) {
		t.Errorf("CILIUM_HELM_VALUES did not survive the round-trip\n got: %#v\nwant: %#v",
			got["CILIUM_HELM_VALUES"], values)
	}
}

// bloom.yaml is regenerated on every web-UI save and every `--export`. A map
// iterated in Go's randomized order would rewrite the file with the same
// content in a different key order each time, producing a diff on every run.
func TestGenerateYAMLIsDeterministic(t *testing.T) {
	cfg := Config{
		"FIRST_NODE": true,
		"GPU_NODE":   true,
		"DOMAIN":     "cluster.example.com",
		"CILIUM_HELM_VALUES": map[string]any{
			"zeta":     map[string]any{"enabled": true},
			"alpha":    map[string]any{"enabled": false},
			"middle":   map[string]any{"n": 3},
			"operator": map[string]any{"replicas": 1},
		},
	}

	first, err := GenerateYAML(cfg)
	if err != nil {
		t.Fatalf("GenerateYAML: %v", err)
	}
	for i := 0; i < 20; i++ {
		out, err := GenerateYAML(cfg)
		if err != nil {
			t.Fatalf("GenerateYAML: %v", err)
		}
		if out != first {
			t.Fatalf("GenerateYAML is not deterministic (run %d)\n--- first ---\n%s\n--- run %d ---\n%s",
				i, first, i, out)
		}
	}
}

// escapeString only escapes `"`, so a multi-line value used to be emitted as a
// double-quoted scalar containing literal newlines - which is not valid YAML.
// RKE2_EXTRA_CONFIG round-tripped through the web UI came back corrupt.
func TestGenerateYAMLRoundTripsAMultiLineString(t *testing.T) {
	extra := "node-taint:\n  - \"node.cilium.io/agent-not-ready=true:NoExecute\"\nkubelet-arg: max-pods=250\n"

	got := generateAndParse(t, Config{
		"FIRST_NODE":        true,
		"GPU_NODE":          true,
		"DOMAIN":            "cluster.example.com",
		"RKE2_EXTRA_CONFIG": extra,
	})

	if got["RKE2_EXTRA_CONFIG"] != extra {
		t.Errorf("RKE2_EXTRA_CONFIG did not survive the round-trip\n got: %q\nwant: %q",
			got["RKE2_EXTRA_CONFIG"], extra)
	}
}

// An untouched map (or a web-UI textarea left blank) is the schema default and
// must not be written out, or every generated bloom.yaml grows a `{}` line for
// a key the operator never set.
func TestGenerateYAMLOmitsAnEmptyMap(t *testing.T) {
	for name, value := range map[string]any{
		"empty map":    map[string]any{},
		"empty string": "",
		"blank string": "   \n",
	} {
		t.Run(name, func(t *testing.T) {
			out, err := GenerateYAML(Config{
				"FIRST_NODE":         true,
				"GPU_NODE":           true,
				"DOMAIN":             "cluster.example.com",
				"CILIUM_HELM_VALUES": value,
			})
			if err != nil {
				t.Fatalf("GenerateYAML: %v", err)
			}
			if strings.Contains(out, "CILIUM_HELM_VALUES") {
				t.Errorf("default CILIUM_HELM_VALUES was written to bloom.yaml:\n%s", out)
			}
		})
	}
}

// A value the YAML encoder cannot render must abort the whole file rather than
// silently vanish from it. GenerateYAML used to skip any field that came back
// empty, so an encode failure would drop the key with no error anywhere: the
// operator exports a bloom.yaml, reads it, and their setting is simply gone.
// Config values are normally plain decoded data, so this is a dormant branch -
// which is exactly why the failure mode matters.
//
// A chan reaches yaml.v3's panic path rather than its error return, so this
// also pins the recover in marshalYAMLField.
func TestGenerateYAMLFailsLoudlyOnAnUnrenderableValue(t *testing.T) {
	out, err := GenerateYAML(Config{
		"FIRST_NODE":         true,
		"GPU_NODE":           true,
		"DOMAIN":             "cluster.example.com",
		"CILIUM_HELM_VALUES": map[string]any{"operator": make(chan int)},
	})
	if err == nil {
		t.Fatalf("expected an error for an unrenderable value, got:\n%s", out)
	}
	if !strings.Contains(err.Error(), "CILIUM_HELM_VALUES") {
		t.Errorf("error should name the offending key, got: %v", err)
	}
}
