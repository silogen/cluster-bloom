//go:build !linux

package runtime

import (
	"fmt"
	"os"
)

func RunContainer(rootfs, playbookDir, playbook string, extraArgs []string, dryRun bool, tags string, outputMode OutputMode) int {
	fmt.Fprintln(os.Stderr, "Error: Cluster deployment is only supported on Linux")
	return 1
}

func RunChild() {
	fmt.Fprintln(os.Stderr, "Error: Cluster deployment is only supported on Linux")
	os.Exit(1)
}
