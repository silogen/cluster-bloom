package runtime

import (
	"bytes"
	"testing"

	"gopkg.in/yaml.v3"
)

// The Longhorn defaults live as a YAML document nested inside a ConfigMap
// string, so bad indentation there is silently ignored by Longhorn rather
// than rejected. Parse both levels.
func longhornDefaultSettings(t *testing.T) map[string]any {
	t.Helper()

	raw, err := longhornManifests.ReadFile("manifests/longhorn/longhorn.yaml")
	if err != nil {
		t.Fatalf("read embedded longhorn.yaml: %v", err)
	}

	dec := yaml.NewDecoder(bytes.NewReader(raw))
	for {
		var doc struct {
			Kind     string                `yaml:"kind"`
			Metadata struct{ Name string } `yaml:"metadata"`
			Data     map[string]string     `yaml:"data"`
		}
		err := dec.Decode(&doc)
		if err != nil {
			break
		}
		if doc.Kind != "ConfigMap" || doc.Metadata.Name != "longhorn-default-setting" {
			continue
		}

		settings := map[string]any{}
		if err := yaml.Unmarshal([]byte(doc.Data["default-setting.yaml"]), &settings); err != nil {
			t.Fatalf("default-setting.yaml is not valid YAML: %v", err)
		}
		return settings
	}

	t.Fatal("no longhorn-default-setting ConfigMap in the embedded manifest")
	return nil
}

// Every StorageClass the manifest ships provisions one replica, so Longhorn's
// own block-if-contains-last-replica default would make every node holding a
// replica permanently undrainable.
func TestLonghornDrainPolicyAllowsStoppedReplicas(t *testing.T) {
	settings := longhornDefaultSettings(t)

	if got := settings["node-drain-policy"]; got != "allow-if-replica-is-stopped" {
		t.Errorf("node-drain-policy = %v, want allow-if-replica-is-stopped", got)
	}
}
