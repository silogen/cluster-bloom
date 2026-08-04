//go:build linux

package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	longhornNamespace = "longhorn"
	longhornVersion   = "v1.12.0"
)

// ConfigBool reads a boolean bloom config value with a default when absent or invalid.
func ConfigBool(cfg map[string]any, key string, defaultVal bool) bool {
	raw, ok := cfg[key]
	if !ok || raw == nil {
		return defaultVal
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return defaultVal
}

// TryCleanupLonghornViaAPI attempts the supported Longhorn uninstall sequence while
// the Kubernetes API is reachable. Returns usedAPI=true when the uninstall job ran.
// Failures are non-fatal; callers should fall back to host-level teardown.
func TryCleanupLonghornViaAPI(kubeconfig string) (usedAPI bool, err error) {
	if kubeconfig == "" {
		kubeconfig = "/etc/rancher/rke2/rke2.yaml"
	}
	if _, statErr := os.Stat(kubeconfig); statErr != nil {
		fmt.Println("   ℹ️  kubeconfig not found — skipping Longhorn API cleanup")
		return false, nil
	}
	if !isKubeAPIReachable() {
		fmt.Println("   ℹ️  Kubernetes API unreachable — skipping Longhorn API cleanup")
		return false, nil
	}

	fmt.Println("   🌐 Attempting Longhorn API cleanup...")
	nsCheck, _ := exec.Command("kubectl", "--kubeconfig", kubeconfig,
		"get", "namespace", longhornNamespace, "-o", "name").CombinedOutput()
	if strings.TrimSpace(string(nsCheck)) == "" {
		fmt.Printf("      ℹ️  Namespace %q not found — skipping Longhorn API cleanup\n", longhornNamespace)
		return false, nil
	}

	if out, patchErr := exec.Command("kubectl", "--kubeconfig", kubeconfig,
		"-n", longhornNamespace, "patch", "settings.longhorn.io", "deleting-confirmation-flag",
		"--type=merge", "-p", `{"value":"true"}`).CombinedOutput(); patchErr != nil {
		fmt.Printf("      ⚠️  Warning: could not set deleting-confirmation-flag: %v\n%s\n", patchErr, out)
	} else {
		fmt.Println("      ✓ Set deleting-confirmation-flag=true")
	}

	uninstallManifest, removeManifest, manifestErr := writeEmbeddedLonghornUninstallManifest()
	if manifestErr != nil {
		fmt.Printf("      ⚠️  Warning: could not load bundled uninstall manifest: %v\n", manifestErr)
		return false, nil
	}
	defer removeManifest()

	createOut, createErr := exec.Command("kubectl", "--kubeconfig", kubeconfig,
		"create", "-f", uninstallManifest).CombinedOutput()
	if createErr != nil {
		if !strings.Contains(string(createOut), "AlreadyExists") {
			fmt.Printf("      ⚠️  Warning: could not create Longhorn uninstall job: %v\n%s\n", createErr, createOut)
			return false, nil
		}
		fmt.Println("      ℹ️  Longhorn uninstall job already exists — waiting for completion")
	} else {
		fmt.Println("      ✓ Created Longhorn uninstall job")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	fmt.Println("      ⏳ Waiting for longhorn-uninstall job (up to 5m)...")
	waitCmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfig,
		"-n", longhornNamespace, "wait", "--for=condition=complete",
		"job/longhorn-uninstall", "--timeout=300s")
	if waitOut, waitErr := waitCmd.CombinedOutput(); waitErr != nil {
		fmt.Printf("      ⚠️  Warning: Longhorn uninstall job did not complete: %v\n%s\n", waitErr, waitOut)
		return true, waitErr
	}
	fmt.Println("      ✓ Longhorn uninstall job completed")
	return true, nil
}

// resolveStableDiskPath prefers a /dev/disk/by-id symlink for the given device.
func resolveStableDiskPath(device string) string {
	device = strings.TrimSpace(device)
	if device == "" {
		return device
	}
	base := filepath.Base(device)
	linkDir := "/dev/disk/by-id"
	entries, err := os.ReadDir(linkDir)
	if err != nil {
		return device
	}
	for _, entry := range entries {
		linkPath := filepath.Join(linkDir, entry.Name())
		target, err := filepath.EvalSymlinks(linkPath)
		if err != nil {
			continue
		}
		if target == device || target == "/dev/"+base {
			return linkPath
		}
	}
	return device
}

// waitForBlockDevice polls until device appears in /sys/block or timeout elapses.
func waitForBlockDevice(device string, timeout time.Duration) {
	base := strings.TrimPrefix(filepath.Base(strings.TrimSpace(device)), "")
	if base == "" {
		return
	}
	sysPath := "/sys/block/" + base
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sysPath); err == nil {
			fmt.Printf("      ✓ Device /dev/%s is present\n", base)
			return
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Printf("      ⚠️  Device /dev/%s not visible after %s (may reappear after reboot)\n", base, timeout)
}

// CleanupBloomBlockDisks wipes CLUSTER_DISKS devices for Longhorn V2 block-type usage.
// Block disks are never mounted or formatted by bloom; only filesystem signatures are removed.
func CleanupBloomBlockDisks(clusterDisks string) error {
	fmt.Println("💽 Cleaning Longhorn V2 block disks (wipefs only)...")
	EnterCriticalSection("V2 block disk cleanup")
	defer ExitCriticalSection()

	for device := range strings.SplitSeq(clusterDisks, ",") {
		device = strings.TrimSpace(device)
		if device == "" {
			continue
		}
		device = resolveStableDiskPath(device)
		fmt.Printf("   🧹 Wiping filesystem signatures on %s...\n", device)
		if _, err := exec.Command("wipefs", "-a", device).CombinedOutput(); err != nil {
			fmt.Printf("      ⚠️  Warning: wipefs failed on %s: %v\n", device, err)
		} else {
			fmt.Printf("      ✓ Wiped %s\n", device)
		}
		waitForBlockDevice(device, 45*time.Second)
	}
	fmt.Println("   ✅ V2 block disk cleanup completed")
	return nil
}
