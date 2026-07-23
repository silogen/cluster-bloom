package config

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
