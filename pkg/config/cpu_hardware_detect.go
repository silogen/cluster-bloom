package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var cpuInfoContents = func() (string, error) {
	data, err := os.ReadFile("/proc/cpuinfo")
	return string(data), err
}

func DetectAMDEPYCCPU() (detected bool, model string, err error) {
	contents, err := cpuInfoContents()
	if err != nil {
		return false, "", fmt.Errorf("read /proc/cpuinfo: %w", err)
	}
	detected, model = ParseCPUInfoForEPYC(contents)
	return detected, model, nil
}

func ParseCPUInfoForEPYC(cpuinfo string) (detected bool, model string) {
	isAMD := false
	modelName := ""
	scanner := bufio.NewScanner(strings.NewReader(cpuinfo))
	for scanner.Scan() {
		key, value, ok := strings.Cut(scanner.Text(), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "vendor_id":
			isAMD = strings.TrimSpace(value) == "AuthenticAMD"
		case "model name":
			if modelName == "" {
				modelName = strings.TrimSpace(value)
			}
		}
		if isAMD && modelName != "" {
			break
		}
	}

	// EPYC in the model name is an unambiguous AMD signal. Some hypervisors
	// hide vendor_id, so accept an explicit AMD EPYC model in that case.
	modelUpper := strings.ToUpper(modelName)
	if strings.Contains(modelUpper, "EPYC") &&
		(isAMD || strings.Contains(modelUpper, "AMD")) {
		return true, modelName
	}
	return false, ""
}
