package config

import "testing"

func TestParseAmdSmiROCmVersion(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "typical header",
			output: "AMDSMI Tool: 25.3.0 | AMDSMI Library version: 25.3.0 | ROCm version: 7.2.3\n",
			want:   "7.2.3",
		},
		{
			name:   "header on second line",
			output: "usage: amd-smi ...\nROCm version: 7.13.0\n",
			want:   "7.13.0",
		},
		{
			name:   "no version present",
			output: "amd-smi: command help\n",
			want:   "",
		},
		{
			name:   "empty",
			output: "",
			want:   "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseAmdSmiROCmVersion(tc.output); got != tc.want {
				t.Fatalf("parseAmdSmiROCmVersion(%q) = %q, want %q", tc.output, got, tc.want)
			}
		})
	}
}

func TestNormalizeROCmVersion(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"deb build suffix", "7.2.3-70203\n", "7.2.3"},
		{"preview suffix", "7.13.0-preview", "7.13.0"},
		{"plain", "7.2.4", "7.2.4"},
		{"whitespace", "  7.2.3  \n", "7.2.3"},
		{"garbage", "unknown", ""},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeROCmVersion(tc.content); got != tc.want {
				t.Fatalf("normalizeROCmVersion(%q) = %q, want %q", tc.content, got, tc.want)
			}
		})
	}
}
