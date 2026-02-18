//go:build linux

package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/silogen/cluster-bloom/pkg/ssh"
	"golang.org/x/sys/unix"
)

func RunContainer(rootfs, playbookDir, playbook string, extraArgs []string, dryRun bool, tags string, outputMode OutputMode) int {
	// Detect the actual user (not root if using sudo)
	actualUser := os.Getenv("SUDO_USER")
	if actualUser == "" {
		actualUser = os.Getenv("USER")
	}
	if actualUser == "" {
		actualUser = "ubuntu"
	}

	// Get current working directory to pass to child for log file
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to get working directory: %v\n", err)
		cwd = ""
	}

	// Setup ephemeral SSH key on HOST before starting container
	fmt.Printf("🔑 Setting up ephemeral SSH key...\n")
	sshManager, err := ssh.NewEphemeralSSHManager(cwd, actualUser)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create SSH manager: %v\n", err)
		return 1
	}
	if err := sshManager.Setup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup ephemeral SSH on host: %v\n", err)
		return 1
	}

	// Setup host-based signal handling for SSH cleanup
	setupHostSSHSignalHandling(sshManager)

	// Ensure SSH cleanup happens when function exits
	defer func() {
		if err := sshManager.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "Error during host SSH cleanup: %v\n", err)
			// Don't exit with error on cleanup failure during defer
		} else {
			fmt.Printf("✅ Host SSH cleanup completed successfully - original authorized_keys restored!\n")
		}
	}()

	childArgs := []string{"__child__", rootfs, playbookDir, playbook, actualUser, cwd, string(outputMode)}
	if dryRun {
		childArgs = append(childArgs, "--dry-run")
	}
	if tags != "" {
		childArgs = append(childArgs, "--tags", tags)
	}
	childArgs = append(childArgs, extraArgs...)

	cmd := exec.Command("/proc/self/exe", childArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID | syscall.CLONE_NEWNS,
		Unshareflags: syscall.CLONE_NEWNS,
	}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode()
		}
		fmt.Fprintf(os.Stderr, "Container error: %v\n", err)
		return 1
	}
	return 0
}

func RunChild() {
	rootfs := os.Args[2]
	playbookDir := os.Args[3]
	playbook := os.Args[4]
	username := os.Args[5]
	workDir := os.Args[6]
	outputMode := OutputMode(os.Args[7])

	// Check if --dry-run and --tags flags are present
	dryRun := false
	tags := ""
	extraArgs := []string{}
	for i := 8; i < len(os.Args); i++ {
		if os.Args[i] == "--dry-run" {
			dryRun = true
		} else if os.Args[i] == "--tags" && i+1 < len(os.Args) {
			tags = os.Args[i+1]
			i++ // Skip next arg
		} else {
			extraArgs = append(extraArgs, os.Args[i])
		}
	}

	syscall.Sethostname([]byte("bloom"))

	containerPlaybooks := filepath.Join(rootfs, "playbooks")
	os.MkdirAll(containerPlaybooks, 0755)
	if err := syscall.Mount(playbookDir, containerPlaybooks, "", syscall.MS_BIND, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mount playbooks: %v\n", err)
		os.Exit(1)
	}

	hostMount := filepath.Join(rootfs, "host")
	os.MkdirAll(hostMount, 0755)
	if err := syscall.Mount("/", hostMount, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mount host: %v\n", err)
		os.Exit(1)
	}

	// Note: SSH key setup and cleanup is now handled on the host, not in container
	// The ephemeral SSH keys should already be available via bind mount

	// Mount ephemeral SSH directory for container
	ephemeralSSHDir := filepath.Join(workDir, "ssh")
	containerSSHDir := filepath.Join(rootfs, "root", ".ssh")

	// Verify ephemeral SSH directory exists
	if _, err := os.Stat(ephemeralSSHDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Ephemeral SSH directory does not exist: %s\n", ephemeralSSHDir)
		os.Exit(1)
	}

	// Verify private key exists
	privKeyPath := filepath.Join(ephemeralSSHDir, "id_ephemeral")
	if _, err := os.Stat(privKeyPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Ephemeral private key does not exist: %s\n", privKeyPath)
		os.Exit(1)
	}

	os.MkdirAll(containerSSHDir, 0700)
	if err := syscall.Mount(ephemeralSSHDir, containerSSHDir, "", syscall.MS_BIND, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to mount ephemeral SSH directory: %v\n", err)
		os.Exit(1)
	}

	if err := pivotRoot(rootfs); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to pivot root: %v\n", err)
		os.Exit(1)
	}

	os.MkdirAll("/proc", 0755)
	syscall.Mount("proc", "/proc", "proc", 0, "")

	os.MkdirAll("/sys", 0755)
	syscall.Mount("sysfs", "/sys", "sysfs", 0, "")

	os.MkdirAll("/dev", 0755)
	syscall.Mount("tmpfs", "/dev", "tmpfs", syscall.MS_NOSUID|syscall.MS_STRICTATIME, "mode=755")

	os.MkdirAll("/dev/pts", 0755)
	syscall.Mount("devpts", "/dev/pts", "devpts", 0, "")

	os.MkdirAll("/dev/shm", 1777)
	syscall.Mount("tmpfs", "/dev/shm", "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "mode=1777")

	unix.Mknod("/dev/null", syscall.S_IFCHR|0666, int(unix.Mkdev(1, 3)))
	unix.Mknod("/dev/zero", syscall.S_IFCHR|0666, int(unix.Mkdev(1, 5)))
	unix.Mknod("/dev/random", syscall.S_IFCHR|0666, int(unix.Mkdev(1, 8)))
	unix.Mknod("/dev/urandom", syscall.S_IFCHR|0666, int(unix.Mkdev(1, 9)))
	unix.Mknod("/dev/tty", syscall.S_IFCHR|0666, int(unix.Mkdev(5, 0)))

	if resolvConf, err := os.ReadFile("/host/run/systemd/resolve/resolv.conf"); err == nil {
		os.WriteFile("/etc/resolv.conf", resolvConf, 0644)
	} else if resolvConf, err := os.ReadFile("/host/etc/resolv.conf"); err == nil {
		os.WriteFile("/etc/resolv.conf", resolvConf, 0644)
	}

	ansibleArgs := []string{
		"--connection=ssh",
		"--inventory=127.0.0.1,",
		"--user=" + username,
		"--become",
		"--ssh-extra-args=-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o IdentitiesOnly=yes -i /root/.ssh/id_ephemeral",
		"-v",
	}
	if tags != "" {
		ansibleArgs = append(ansibleArgs, "--tags", tags)
	}
	ansibleArgs = append(ansibleArgs, filepath.Join("/playbooks", playbook))
	if dryRun {
		ansibleArgs = append(ansibleArgs, "--check")
	}
	ansibleArgs = append(ansibleArgs, extraArgs...)

	// Open log file on host (via /host mount)
	var logFile *os.File
	if workDir != "" {
		logPath := "/host" + workDir + "/bloom.log"
		var err error
		logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to open bloom.log at %s: %v\n", logPath, err)
			logFile = nil
		}
	}
	defer func() {
		if logFile != nil {
			logFile.Close()
		}
	}()

	// Parse config values from extraArgs for post-deployment messaging
	configMap := parseConfigFromExtraArgs(extraArgs)

	// Create output processor
	processor := NewOutputProcessor(outputMode, logFile, configMap)

	cmd := exec.Command("ansible-playbook", ansibleArgs...)
	cmd.Stdin = os.Stdin

	// Use pipes to capture and process output
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create stdout pipe: %v\n", err)
		os.Exit(1)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create stderr pipe: %v\n", err)
		os.Exit(1)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start cluster deployment: %v\n", err)
		os.Exit(1)
	}

	// Process output streams
	go processor.ProcessStream(stdoutPipe, os.Stdout)
	go processor.ProcessStream(stderrPipe, os.Stderr)

	cmd.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
		"USER=" + username,
		"ANSIBLE_LOCALHOST_WARNING=False",
		"ANSIBLE_PYTHON_INTERPRETER=/usr/bin/python3",
	}

	// Wait for command to complete
	if err := cmd.Wait(); err != nil {
		// Print summary before exiting (if clean mode)
		processor.PrintSummary()

		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}

	// Print summary on success (if clean mode)
	processor.PrintSummary()
}

func pivotRoot(newRoot string) error {
	if err := syscall.Mount(newRoot, newRoot, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return fmt.Errorf("bind mount newRoot: %w", err)
	}

	putOld := filepath.Join(newRoot, ".pivot_old")
	if err := os.MkdirAll(putOld, 0700); err != nil {
		return fmt.Errorf("mkdir pivot_old: %w", err)
	}

	if err := syscall.PivotRoot(newRoot, putOld); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}

	if err := os.Chdir("/"); err != nil {
		return fmt.Errorf("chdir: %w", err)
	}

	putOld = "/.pivot_old"
	if err := syscall.Unmount(putOld, syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("unmount pivot_old: %w", err)
	}

	return os.RemoveAll(putOld)
}

// setupHostSSHSignalHandling sets up signal handlers for host-based SSH cleanup
// This ensures that SSH cleanup happens on the host when signals are received
func setupHostSSHSignalHandling(sshManager *ssh.EphemeralSSHManager) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)

	go func() {
		sig := <-c

		// Perform SSH cleanup directly on host
		if err := sshManager.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "🔥 CRITICAL: Host SSH cleanup failed: %v\n", err)
			os.Exit(1) // Exit with error as requested
		} else {
			fmt.Printf("✅ Host SSH cleanup completed successfully - original authorized_keys restored!\n")
		}

		// Exit with appropriate signal-based exit code
		switch sig {
		case os.Interrupt:
			os.Exit(130) // 128 + SIGINT
		case syscall.SIGTERM:
			os.Exit(143) // 128 + SIGTERM
		case syscall.SIGHUP:
			os.Exit(129) // 128 + SIGHUP
		case syscall.SIGQUIT:
			os.Exit(131) // 128 + SIGQUIT
		default:
			os.Exit(1)
		}
	}()
}

// parseConfigFromExtraArgs extracts configuration values from Ansible extra vars
// Extra vars are in the format: -e {"KEY": "value"}
func parseConfigFromExtraArgs(extraArgs []string) map[string]string {
	config := make(map[string]string)

	for i := 0; i < len(extraArgs); i++ {
		if extraArgs[i] == "-e" && i+1 < len(extraArgs) {
			// Parse JSON extra var
			var varMap map[string]interface{}
			if err := json.Unmarshal([]byte(extraArgs[i+1]), &varMap); err == nil {
				// Extract values we care about
				if val, ok := varMap["CLUSTERFORGE_RELEASE"].(string); ok {
					config["CLUSTERFORGE_RELEASE"] = val
				}
				if val, ok := varMap["DOMAIN"].(string); ok {
					config["DOMAIN"] = val
				}
			}
			i++ // Skip the next argument as we've already processed it
		}
	}

	return config
}
