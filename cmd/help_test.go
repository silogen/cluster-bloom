package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfigurationFieldsHelpVisibility(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantFields bool
	}{
		{name: "root", args: []string{"--help"}, wantFields: true},
		{name: "cli", args: []string{"cli", "--help"}, wantFields: true},
		{name: "cleanup", args: []string{"cleanup", "--help"}, wantFields: false},
		{name: "webui", args: []string{"webui", "--help"}, wantFields: false},
		{name: "version", args: []string{"version", "--help"}, wantFields: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rootCmd := newRootCmd()
			output := new(bytes.Buffer)
			rootCmd.SetOut(output)
			rootCmd.SetErr(output)
			rootCmd.SetArgs(test.args)

			if err := rootCmd.Execute(); err != nil {
				t.Fatalf("help command failed: %v", err)
			}

			hasFields := strings.Contains(output.String(), "CONFIGURATION FIELDS")
			if hasFields != test.wantFields {
				t.Fatalf("CONFIGURATION FIELDS visibility = %v, want %v\n%s",
					hasFields, test.wantFields, output.String())
			}
		})
	}
}

func TestCLIHelpShowsCommonWorkflows(t *testing.T) {
	rootCmd := newRootCmd()
	output := new(bytes.Buffer)
	rootCmd.SetOut(output)
	rootCmd.SetErr(output)
	rootCmd.SetArgs([]string{"cli", "--help"})

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("help command failed: %v", err)
	}

	help := output.String()
	for _, want := range []string{
		"Common workflows:",
		"sudo bloom cli bloom.yaml",
		"sudo bloom cli bloom.yaml --tags validate_node",
		"sudo bloom cli bloom.yaml --tags deploy_clusterforge",
		"sudo bloom cli cert-update.yaml --tags update_cert",
		"./bloom cli bloom.yaml --export",
		"Run only Ansible tasks matching tags (e.g. validate_node, deploy_clusterforge, update_cert)",
	} {
		if !strings.Contains(help, want) {
			t.Errorf("CLI help missing %q\n%s", want, help)
		}
	}

	for _, unwanted := range []string{
		"ClusterForge Bootstrap (deferred install only):",
		"cleanup, storage",
	} {
		if strings.Contains(help, unwanted) {
			t.Errorf("CLI help unexpectedly contains %q\n%s", unwanted, help)
		}
	}
}
