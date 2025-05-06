package main

import (
    "fmt"
    "os"
    "os/exec"
    "strings"
)

// Log levels
const (
    Debug = iota
    Info
    Warn
    Error
)

// LogMessage prints messages with appropriate prefix
func LogMessage(level int, message string) {
    prefix := ""
    switch level {
    case Debug:
        prefix = "DEBUG: "
    case Info:
        prefix = "INFO: "
    case Warn:
        prefix = "WARNING: "
    case Error:
        prefix = "ERROR: "
    }
    fmt.Println(prefix + message)
}

// OpenPorts adds necessary iptables rules
func OpenPorts() bool {
    ports := []string{
        "22;tcp", "80;tcp", "443;tcp", "2376;tcp", "2379;tcp", "2380;tcp", "6443;tcp",
        "8472;udp", "9099;tcp", "9345;tcp", "10250;tcp", "10254;tcp", "30000:32767;tcp", "30000:32767;udp",
    }

    for _, entry := range ports {
        parts := strings.Split(entry, ";")
        port, protocol := parts[0], parts[1]
        cmd := exec.Command("sudo", "iptables", "-A", "INPUT", "-p", protocol, "-m", "state", 
                           "--state", "NEW", "-m", protocol, "--dport", port, "-j", "ACCEPT")
        if err := cmd.Run(); err != nil {
            LogMessage(Error, fmt.Sprintf("Failed to open port %s/%s: %v", port, protocol, err))
            return false
        }
        LogMessage(Debug, fmt.Sprintf("Opened port %s/%s", port, protocol))
    }
    if err := exec.Command("sudo", "iptables-save").Run(); err != nil {
        LogMessage(Error, fmt.Sprintf("Failed to save iptables rules: %v", err))
        return false
    }

    LogMessage(Debug, "All iptables rules have been added and saved.")
    return true
}

func main() {
    // Check if running as root/sudo
    if os.Geteuid() != 0 {
        fmt.Println("This program must be run as root (sudo)")
        os.Exit(1)
    }

    fmt.Println("Opening ports for Kubernetes cluster...")
    success := OpenPorts()
    
    if success {
        fmt.Println("✅ Ports opened successfully")
        os.Exit(0)
    } else {
        fmt.Println("❌ Failed to open ports")
        os.Exit(1)
    }
}
