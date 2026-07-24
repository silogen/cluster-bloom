package config

import (
	"fmt"
	"os"
	"strings"
)

// SupportedOS describes an operating system cluster-bloom officially supports,
// including how to install/start an SSH server on it.
type SupportedOS struct {
	// Name is the human-readable distro name shown in guidance (e.g. "Ubuntu").
	Name string
	// OSReleaseID is the /etc/os-release ID (e.g. "ubuntu").
	OSReleaseID string
	// Versions are the supported VERSION_IDs (e.g. "22.04").
	Versions []string
	// SSHInstallCmd installs and starts an SSH server on this OS.
	SSHInstallCmd string
}

// SupportedOSes is the SINGLE SOURCE OF TRUTH for the operating systems
// cluster-bloom officially supports. Everything else derives from it:
//   - the Ansible `supported_ubuntu_versions` var is populated from here
//     (injected in cmd before export/run), so the playbook's OS check and the
//     Go side never drift (a test enforces this);
//   - the sshd pre-flight guidance (pkg/ansible/runtime) is rendered from here.
//
// To change what bloom supports, edit this list only.
var SupportedOSes = []SupportedOS{
	{
		Name:          "Ubuntu",
		OSReleaseID:   "ubuntu",
		Versions:      []string{"20.04", "22.04", "24.04"},
		SSHInstallCmd: "sudo apt-get install -y openssh-server && sudo systemctl enable --now ssh",
	},
}

// SupportedUbuntuVersions returns the flat list of supported OS version strings.
// It is injected as the Ansible `supported_ubuntu_versions` variable so the
// playbook OS check and the Go side share one definition.
func SupportedUbuntuVersions() []string {
	versions := make([]string, 0)
	for _, os := range SupportedOSes {
		versions = append(versions, os.Versions...)
	}
	return versions
}

// SupportedOSSummary renders a human-readable, one-per-line summary of the
// officially supported OSes and versions, e.g. "Ubuntu 20.04 / 22.04 / 24.04".
func SupportedOSSummary() string {
	parts := make([]string, 0, len(SupportedOSes))
	for _, o := range SupportedOSes {
		parts = append(parts, fmt.Sprintf("%s %s", o.Name, strings.Join(o.Versions, " / ")))
	}
	return strings.Join(parts, "\n     ")
}

// HostOSInfo is the local machine's OS identity, parsed from os-release.
type HostOSInfo struct {
	ID         string // os-release ID, e.g. "ubuntu", "opensuse-tumbleweed"
	VersionID  string // os-release VERSION_ID, e.g. "22.04"
	PrettyName string // os-release PRETTY_NAME, e.g. "openSUSE Tumbleweed"
}

// DetectHostOS reads /etc/os-release (falling back to /usr/lib/os-release) and
// returns the local OS identity.
func DetectHostOS() (HostOSInfo, error) {
	for _, p := range []string{"/etc/os-release", "/usr/lib/os-release"} {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		return parseOSRelease(string(data)), nil
	}
	return HostOSInfo{}, fmt.Errorf("no os-release file found")
}

func parseOSRelease(content string) HostOSInfo {
	m := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		m[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return HostOSInfo{ID: m["ID"], VersionID: m["VERSION_ID"], PrettyName: m["PRETTY_NAME"]}
}

// IsSupported reports whether this host's OS and version are officially
// supported (matched against SupportedOSes by os-release ID + version).
func (h HostOSInfo) IsSupported() bool {
	for _, o := range SupportedOSes {
		if !strings.EqualFold(o.OSReleaseID, h.ID) {
			continue
		}
		for _, v := range o.Versions {
			if v == h.VersionID {
				return true
			}
		}
	}
	return false
}

// DisplayName returns the best available human-readable OS name.
func (h HostOSInfo) DisplayName() string {
	switch {
	case h.PrettyName != "":
		return h.PrettyName
	case h.ID != "" && h.VersionID != "":
		return h.ID + " " + h.VersionID
	case h.ID != "":
		return h.ID
	default:
		return "unknown OS"
	}
}
