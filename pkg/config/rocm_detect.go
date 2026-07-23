package config

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	amdSmiROCmVersionRe = regexp.MustCompile(`ROCm version:\s*([0-9]+\.[0-9]+\.[0-9]+)`)
	rocmVersionRe       = regexp.MustCompile(`([0-9]+\.[0-9]+\.[0-9]+)`)
)

// DetectInstalledROCmVersion best-effort reads the host's installed ROCm version
// (e.g. "7.2.3"), returning ("", false) when none is found. It never errors: any
// failure (no ROCm, no tools, exec/read error) is treated as "not detected", so
// the caller — an informational pre-flight mismatch check — degrades gracefully.
//
// It mirrors the Ansible gpu_rocm_detect discovery order (amd-smi header first,
// then the /opt/rocm*/.info/version marker) so the Go pre-flight prompt and the
// authoritative Ansible gate agree on the same version. Ansible remains the
// source of truth; this is only used to warn the operator earlier.
func DetectInstalledROCmVersion() (version string, found bool) {
	if v := rocmVersionFromAmdSmi(); v != "" {
		return v, true
	}
	if v := rocmVersionFromInfoFiles(); v != "" {
		return v, true
	}
	return "", false
}

func rocmVersionFromAmdSmi() string {
	bin := findROCmTool("amd-smi")
	if bin == "" {
		return ""
	}
	// Bound the call: amd-smi normally prints its header instantly, but never
	// let a wedged GPU tool hang the whole pre-flight.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, _ := exec.CommandContext(ctx, bin).CombinedOutput()
	return parseAmdSmiROCmVersion(string(out))
}

// parseAmdSmiROCmVersion extracts "X.Y.Z" from amd-smi header output, or "".
func parseAmdSmiROCmVersion(output string) string {
	m := amdSmiROCmVersionRe.FindStringSubmatch(output)
	if m == nil {
		return ""
	}
	return m[1]
}

func rocmVersionFromInfoFiles() string {
	candidates := []string{"/opt/rocm/.info/version", "/opt/rocm/core/.info/version"}
	candidates = append(candidates, globSorted("/opt/rocm/core-*/.info/version")...)
	candidates = append(candidates, globSorted("/opt/rocm-*/.info/version")...)
	for _, f := range candidates {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		if v := normalizeROCmVersion(string(data)); v != "" {
			return v
		}
	}
	return ""
}

// normalizeROCmVersion extracts "X.Y.Z" from a .info/version file body such as
// "7.2.3-70203" or "7.13.0-preview\n", or "".
func normalizeROCmVersion(content string) string {
	m := rocmVersionRe.FindStringSubmatch(strings.TrimSpace(content))
	if m == nil {
		return ""
	}
	return m[1]
}

// findROCmTool locates a ROCm CLI (e.g. amd-smi) on PATH or under the usual
// /opt/rocm* trees, returning "" if not found.
func findROCmTool(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	candidates := []string{filepath.Join("/opt/rocm/bin", name)}
	candidates = append(candidates, globSorted("/opt/rocm-*/bin/"+name)...)
	candidates = append(candidates, globSorted("/opt/rocm/core-*/bin/"+name)...)
	for _, c := range candidates {
		if isExecutableFile(c) {
			return c
		}
	}
	return ""
}

func isExecutableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0111 != 0
}

// globSorted returns matches for pattern in reverse-lexical order, so a newer
// versioned tree (e.g. /opt/rocm-7.2.3) is preferred over an older one.
func globSorted(pattern string) []string {
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return nil
	}
	sort.Sort(sort.Reverse(sort.StringSlice(matches)))
	return matches
}
