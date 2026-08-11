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
