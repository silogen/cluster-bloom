package config

import "testing"

func TestParseCPUInfoForEPYC(t *testing.T) {
	tests := []struct {
		name     string
		cpuinfo  string
		detected bool
		model    string
	}{
		{
			name: "physical EPYC",
			cpuinfo: "vendor_id : AuthenticAMD\n" +
				"model name : AMD EPYC 9124 16-Core Processor\n",
			detected: true,
			model:    "AMD EPYC 9124 16-Core Processor",
		},
		{
			name: "virtualized EPYC without vendor",
			cpuinfo: "model name : AMD EPYC 9J14\n",
			detected: true,
			model:    "AMD EPYC 9J14",
		},
		{
			name: "non EPYC AMD CPU",
			cpuinfo: "vendor_id : AuthenticAMD\n" +
				"model name : AMD Ryzen 9 9950X\n",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detected, model := ParseCPUInfoForEPYC(test.cpuinfo)
			if detected != test.detected || model != test.model {
				t.Fatalf("got (%t, %q), want (%t, %q)",
					detected, model, test.detected, test.model)
			}
		})
	}
}
