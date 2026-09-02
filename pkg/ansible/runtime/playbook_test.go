package runtime

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Every value bloom holds reaches ansible-playbook as `-e '{"KEY": ...}'`.
// ansible-playbook parses a -e value that starts with `{` with a YAML loader,
// so the argument has to survive *both* parsers and come back unchanged.
//
// The old implementation formatted strings with fmt and no escaping, so a value
// containing a quote produced invalid JSON and a multi-line value had its
// newlines folded into spaces. RKE2_EXTRA_CONFIG and CILIUM_HELM_VALUES are
// both multi-line by design, which is how that stayed hidden: the failure was
// silent config loss, not a crash.
func TestConfigToAnsibleVarsSurvivesBothParsers(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value any
	}{
		{"bool", "FIRST_NODE", true},
		{"plain string", "DOMAIN", "cluster.example.com"},
		{"string with a double quote", "RKE2_EXTRA_CONFIG", `node-label: "role=gpu"`},
		{"string with a backslash", "TLS_KEY", `C:\certs\key.pem`},
		{"multi-line string", "RKE2_EXTRA_CONFIG", "node-taint:\n  - \"a=b:NoExecute\"\nkubelet-arg: x\n"},
		{"string with an ampersand", "CLUSTERFORGE_REPO", "https://example.com/r?a=1&b=2"},
		{"empty map", "CILIUM_HELM_VALUES", map[string]any{}},
		{"nested map", "CILIUM_HELM_VALUES", map[string]any{
			"hubble": map[string]any{
				"enabled": true,
				"relay":   map[string]any{"enabled": true},
			},
		}},
		{"list", "DNS_SERVERS", []any{"8.8.8.8", "1.1.1.1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vars, err := ConfigToAnsibleVars(map[string]any{tt.key: tt.value})
			if err != nil {
				t.Fatalf("ConfigToAnsibleVars: %v", err)
			}
			if len(vars) != 1 {
				t.Fatalf("expected 1 extravar, got %d: %q", len(vars), vars)
			}
			arg := vars[0]

			if strings.Contains(arg, "\n") {
				t.Errorf("extravar contains a literal newline; ansible would see a truncated argument: %q", arg)
			}

			for parser, unmarshal := range map[string]func([]byte, any) error{
				"json": json.Unmarshal,
				"yaml": func(b []byte, v any) error { return yaml.Unmarshal(b, v) },
			} {
				var got map[string]any
				if err := unmarshal([]byte(arg), &got); err != nil {
					t.Errorf("%s cannot parse %q: %v", parser, arg, err)
					continue
				}
				want := map[string]any{tt.key: tt.value}
				if !reflect.DeepEqual(normalize(t, got), normalize(t, want)) {
					t.Errorf("%s round-trip changed the value\n got: %#v\nwant: %#v", parser, got, want)
				}
			}
		})
	}
}

// normalize runs a value through the JSON encoder and back so that the YAML and
// JSON parsers' differing numeric types (int vs float64) do not make a
// value-preserving round-trip look like a failure.
func normalize(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("normalize: %v", err)
	}
	return out
}

func TestConfigToAnsibleVarsEmitsOneArgPerKey(t *testing.T) {
	vars, err := ConfigToAnsibleVars(map[string]any{
		"FIRST_NODE":   true,
		"DOMAIN":       "a.example.com",
		"CLUSTER_SIZE": "small",
	})
	if err != nil {
		t.Fatalf("ConfigToAnsibleVars: %v", err)
	}
	if len(vars) != 3 {
		t.Fatalf("expected 3 extravars, got %d: %q", len(vars), vars)
	}
}
