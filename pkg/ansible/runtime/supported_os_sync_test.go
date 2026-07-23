package runtime

import (
	"testing"

	"github.com/silogen/cluster-bloom/pkg/config"
	"gopkg.in/yaml.v3"
)

// TestSupportedUbuntuVersionsMatchPlaybook guards against drift between the Go
// single source of truth (config.SupportedOSes) and the fallback
// `supported_ubuntu_versions` default baked into the embedded playbook. bloom
// cli injects the Go value, but raw `ansible-playbook` runs use the playbook
// default, so the two must stay identical.
func TestSupportedUbuntuVersionsMatchPlaybook(t *testing.T) {
	data, err := embeddedPlaybooks.ReadFile("playbooks/cluster-bloom.yaml")
	if err != nil {
		t.Fatalf("read embedded playbook: %v", err)
	}

	// The playbook is a list of plays; the first play holds the vars block.
	var plays []map[string]any
	if err := yaml.Unmarshal(data, &plays); err != nil {
		t.Fatalf("parse playbook: %v", err)
	}
	if len(plays) == 0 {
		t.Fatal("playbook has no plays")
	}

	vars, ok := plays[0]["vars"].(map[string]any)
	if !ok {
		t.Fatal("first play has no vars map")
	}
	rawList, ok := vars["supported_ubuntu_versions"].([]any)
	if !ok {
		t.Fatalf("supported_ubuntu_versions missing or not a list: %T", vars["supported_ubuntu_versions"])
	}

	playbookVersions := make([]string, 0, len(rawList))
	for _, v := range rawList {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("supported_ubuntu_versions entry is not a string: %T (%v)", v, v)
		}
		playbookVersions = append(playbookVersions, s)
	}

	want := config.SupportedUbuntuVersions()
	if len(playbookVersions) != len(want) {
		t.Fatalf("supported_ubuntu_versions drift:\n  playbook: %v\n  go:       %v", playbookVersions, want)
	}
	for i := range want {
		if playbookVersions[i] != want[i] {
			t.Fatalf("supported_ubuntu_versions drift at index %d:\n  playbook: %v\n  go:       %v", i, playbookVersions, want)
		}
	}
}
