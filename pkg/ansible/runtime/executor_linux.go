//go:build linux

package runtime

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/silogen/cluster-bloom/pkg/ssh"
	"golang.org/x/sys/unix"
)

func RunContainer(rootfs, playbookDir, playbook string, extraArgs []string, dryRun bool, tags string) int {
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

	childArgs := []string{"__child__", rootfs, playbookDir, playbook, actualUser, cwd}
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

	// Check if --dry-run and --tags flags are present
	dryRun := false
	tags := ""
	extraArgs := []string{}
	for i := 7; i < len(os.Args); i++ {
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

	// Setup ephemeral SSH key for authentication
	sshManager := ssh.NewEphemeralSSHManager(workDir, username)
	if err := sshManager.Setup(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to setup ephemeral SSH: %v\n", err)
		os.Exit(1)
	}

	// Ensure cleanup happens even if process is interrupted
	defer func() {
		if err := sshManager.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: SSH cleanup failed: %v\n", err)
		}
	}()

	// Mount ephemeral SSH directory for container
	ephemeralSSHDir := filepath.Join(workDir, "ssh")
	containerSSHDir := filepath.Join(rootfs, "root", ".ssh")
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
		"--ssh-extra-args=-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null",
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

	cmd := exec.Command("ansible-playbook", ansibleArgs...)
	cmd.Stdin = os.Stdin

	// Tee output to both stdout and log file
	if logFile != nil {
		cmd.Stdout = io.MultiWriter(os.Stdout, logFile)
		cmd.Stderr = io.MultiWriter(os.Stderr, logFile)
	} else {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}

	cmd.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/root",
		"USER=" + username,
		"ANSIBLE_LOCALHOST_WARNING=False",
		"ANSIBLE_PYTHON_INTERPRETER=/usr/bin/python3",
	}

	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
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
