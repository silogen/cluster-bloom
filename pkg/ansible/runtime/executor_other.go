//go:build !linux

package runtime

import (
	"fmt"
	"os"
)

func RunContainer(rootfs, playbookDir, playbook string, extraArgs []string, dryRun bool, tags string) int {
	fmt.Fprintln(os.Stderr, "Error: Ansible deployment is only supported on Linux")
	return 1
}

func RunChild() {
	fmt.Fprintln(os.Stderr, "Error: Ansible deployment is only supported on Linux")
	os.Exit(1)
}
