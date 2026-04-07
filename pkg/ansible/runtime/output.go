package runtime

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// OutputMode defines how Ansible output should be displayed
type OutputMode string

const (
	OutputVerbose OutputMode = "verbose" // Full Ansible output (current behavior)
	OutputClean   OutputMode = "clean"   // Emoji-based summary per task
	OutputJSON    OutputMode = "json"    // Machine-readable JSON output
)

// OutputProcessor handles Ansible output processing and formatting
type OutputProcessor struct {
	mode         OutputMode
	logFile      *os.File
	stats        *PlaybookStats
	currentTask  string
	startTime    time.Time
	taskSeen     bool
	suppressNext bool
	pendingTask  bool
	config       map[string]string // Configuration values (e.g., CLUSTERFORGE_RELEASE, DOMAIN)
}

// NewOutputProcessor creates a new output processor
func NewOutputProcessor(mode OutputMode, logFile *os.File, config map[string]string) *OutputProcessor {
	return &OutputProcessor{
		mode:      mode,
		logFile:   logFile,
		stats:     &PlaybookStats{},
		config:    config,
		startTime: time.Now(),
	}
}

// ProcessStream reads from input and writes processed output to stdout
func (p *OutputProcessor) ProcessStream(input io.Reader, output io.Writer) error {
	scanner := bufio.NewScanner(input)

	for scanner.Scan() {
		line := scanner.Text()

		// Always write to log file
		if p.logFile != nil {
			p.logFile.WriteString(line + "\n")
		}

		// Process and write to output based on mode
		processedLine := p.processLine(line)
		if processedLine != "" {
			if p.pendingTask && !strings.HasPrefix(processedLine, "⏳") {
				// Erase the ⏳ pending line before printing the result
				fmt.Fprint(output, "\033[2K\r")
				p.pendingTask = false
			}
			if strings.HasPrefix(processedLine, "⏳") {
				// Print without newline so it can be overwritten
				fmt.Fprint(output, processedLine)
				p.pendingTask = true
			} else {
				fmt.Fprintln(output, processedLine)
			}
		}
	}

	return scanner.Err()
}

// processLine processes a single line of Ansible output
func (p *OutputProcessor) processLine(line string) string {
	// Verbose mode: passthrough everything
	if p.mode == OutputVerbose {
		return line
	}

	// Clean mode: parse and format
	if p.mode == OutputClean {
		return p.processCleanMode(line)
	}

	// JSON mode: return raw line (would need more sophisticated handling)
	return line
}

// processCleanMode handles clean output formatting
func (p *OutputProcessor) processCleanMode(line string) string {
	// Check for task header
	if taskName, ok := ParseTaskHeader(line); ok {
		p.currentTask = taskName
		p.taskSeen = false
		return "⏳ " + taskName
	}

	// Check for task result
	if taskInfo, ok := ParseTaskResult(line); ok {
		if !p.taskSeen && p.currentTask != "" {
			p.taskSeen = true

			// Check if error should be ignored
			if taskInfo.Status == TaskStatusFailed && IsIgnoredError(line) {
				taskInfo.Status = TaskStatusIgnored
			}

			// Record stats
			p.stats.Record(taskInfo.Status)

			// Format and return task result
			emoji := p.getEmoji(taskInfo.Status)
			output := fmt.Sprintf("%s %s", emoji, p.currentTask)

			// Add message if available and not too verbose
			if taskInfo.Message != "" && !strings.Contains(taskInfo.Message, "{") {
				output += fmt.Sprintf(" (%s)", taskInfo.Message)
			}

			return output
		}
	}

	// Suppress most other output in clean mode
	// Only show critical errors or important messages
	if strings.Contains(line, "ERROR") ||
		strings.Contains(line, "PLAY RECAP") ||
		strings.Contains(line, "PLAY [") {
		return "" // Suppress even these for cleaner output
	}

	return ""
}

// getEmoji returns the emoji for a given task status
func (p *OutputProcessor) getEmoji(status TaskStatus) string {
	switch status {
	case TaskStatusOK:
		return "✅ (ok)"
	case TaskStatusChanged:
		return "🔄 (changed)"
	case TaskStatusFailed:
		return "❌ (failed)"
	case TaskStatusSkipped:
		return "⏭️ (skipped)"
	case TaskStatusUnreachable:
		return "⛔ (unreachable)"
	case TaskStatusIgnored:
		return "🙈 (ignored)"
	default:
		return "•"
	}
}

// PrintSummary prints the final playbook summary
func (p *OutputProcessor) PrintSummary() {
	if p.mode != OutputClean {
		return
	}

	duration := time.Since(p.startTime)

	fmt.Println()
	fmt.Printf("Playbook complete: %s\n", p.stats.Summary())
	fmt.Printf("Total time: %s\n", formatDuration(duration))

	// Print credential information if CLUSTERFORGE_RELEASE is configured
	if p.config != nil {
		clusterforgeRelease := p.config["CLUSTERFORGE_RELEASE"]
		domain := p.config["DOMAIN"]

		if clusterforgeRelease != "" && clusterforgeRelease != "none" && domain != "" {
			fmt.Println()
			fmt.Println("🚀 ClusterForge Deployment:")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("⏳ Services are starting up. Endpoints will be available once kgateway is ready.")
			fmt.Println()
			fmt.Println("Run this command to wait for services to be ready (Ctrl+C to exit early):")
			fmt.Println()
			fmt.Println("  kubectl wait --for=condition=ready pod -l app=kgateway -n kgateway-system --timeout=600s && \\")
			fmt.Println("  kubectl wait --for=condition=complete job -l app=cluster-auth -n cluster-auth --timeout=600s && \\")
			fmt.Println("  echo '✅ Services are ready!'")
			fmt.Println()
			fmt.Println("Once ready, access these endpoints:")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println()
			fmt.Println("📋 Credential Information:")
			fmt.Println()
			fmt.Printf("🔐 AI Resource Manager - DevUser:\n")
			fmt.Printf("   URL:      https://airmui.%s\n", domain)
			fmt.Printf("   Username: devuser@%s\n", domain)
			fmt.Printf("   Password: kubectl -n keycloak get secret airm-realm-credentials -o jsonpath='{.data.KEYCLOAK_INITIAL_DEVUSER_PASSWORD}' | base64 --decode && echo\n")
			fmt.Println()
			fmt.Printf("💼 AI Workbench - DevUser:\n")
			fmt.Printf("   URL:      https://aiwbui.%s\n", domain)
			fmt.Printf("   Username: devuser@%s\n", domain)
			fmt.Printf("   Password: kubectl -n keycloak get secret airm-realm-credentials -o jsonpath='{.data.KEYCLOAK_INITIAL_DEVUSER_PASSWORD}' | base64 --decode && echo\n")
			fmt.Println()
			fmt.Printf("📦 ArgoCD - Admin:\n")
			fmt.Printf("   URL:      https://argocd.%s\n", domain)
			fmt.Printf("   Username: admin\n")
			fmt.Printf("   Password: kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 --decode && echo\n")
			fmt.Println()
			fmt.Printf("🔧 Gitea - Admin:\n")
			fmt.Printf("   URL:      https://gitea.%s\n", domain)
			fmt.Printf("   Username: gitea_admin\n")
			fmt.Printf("   Password: kubectl -n gitea get secret gitea-admin-credentials -o jsonpath='{.data.password}' | base64 --decode && echo\n")
			fmt.Println()
			fmt.Printf("🔐 OpenBao - Root Token:\n")
			fmt.Printf("   URL:      https://openbao.%s\n", domain)
			fmt.Printf("   Token:    kubectl -n openbao get secret openbao-root-token -o jsonpath='{.data.token}' | base64 --decode && echo\n")
			fmt.Println()
			fmt.Printf("🔑 Keycloak - Admin:\n")
			fmt.Printf("   URL:      https://kc.%s\n", domain)
			fmt.Printf("   Username: silogen-admin\n")
			fmt.Printf("   Password: kubectl -n keycloak get secret keycloak-credentials -o jsonpath='{.data.KEYCLOAK_INITIAL_ADMIN_PASSWORD}' | base64 --decode && echo\n")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		}
	}
}

// formatDuration formats a duration into a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}

	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60

	if minutes < 60 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}

	hours := minutes / 60
	minutes = minutes % 60

	return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
}
