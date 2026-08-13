package cmd

import "testing"

func TestShouldGateAIMHardwareFamily(t *testing.T) {
	tests := []struct {
		name   string
		dryRun bool
		tags   string
		want   bool
	}{
		{name: "full install", want: true},
		{name: "ClusterForge tag", tags: "deploy_clusterforge", want: true},
		{name: "ClusterForge among tags", tags: "validate_node, deploy_clusterforge", want: true},
		{name: "dry run", dryRun: true, want: false},
		{name: "dry run ClusterForge", dryRun: true, tags: "deploy_clusterforge", want: false},
		{name: "validation only", tags: "validate_node", want: false},
		{name: "certificate update only", tags: "update_cert", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := shouldGateAIMHardwareFamily(test.dryRun, test.tags); got != test.want {
				t.Fatalf("shouldGateAIMHardwareFamily(%t, %q) = %t, want %t",
					test.dryRun, test.tags, got, test.want)
			}
		})
	}
}
