package runtime

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
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
	joinInfo     string            // Captured join information from Display join information task
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

			// Add message if available and not too verbose. Flatten to a single
			// line (collapsing newlines/whitespace) so multi-line fail messages
			// don't dump blank lines and box-art on screen; the full text is
			// still written verbatim to bloom.log. Skip entirely when the task
			// name already directs the user to the log (e.g. "... see 'tail
			// bloom.log' for full details"), so the summary isn't duplicated.
			selfDescribing := strings.Contains(strings.ToLower(p.currentTask), "for full details")
			// Ansible's shell/command modules stamp this generic message on any
			// task with failed_when: false that happens to exit non-zero (e.g.
			// detection probes for something that may not exist yet). It's an
			// implementation detail, not information the user needs alongside
			// an already-successful/skipped task, so don't surface it.
			genericMsg := strings.EqualFold(strings.TrimSpace(taskInfo.Message), "non-zero return code")
			if !selfDescribing && !genericMsg && taskInfo.Message != "" && !strings.Contains(taskInfo.Message, "{") {
				if msg := flattenMessage(taskInfo.Message); msg != "" {
					output += fmt.Sprintf(" (%s)", msg)
				}
			}

			return output
		}
	}

	// Capture join information from "Display join information" task
	// Use case-insensitive matching and check for partial matches to be more robust
	if strings.Contains(strings.ToLower(p.currentTask), "display join") {
		// Trim spaces and check for msg and cluster content (handles indented JSON)
		trimmedLine := strings.TrimSpace(line)
		if strings.Contains(trimmedLine, "\"msg\":") && strings.Contains(trimmedLine, "Cluster setup complete!") {
			// Extract the join information message from the JSON output
			p.joinInfo = p.extractJoinInfoMessage(trimmedLine)
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

var whitespaceRunRegex = regexp.MustCompile(`\s+`)

// flattenMessage collapses a (possibly multi-line) task message into a single
// tidy line for clean-mode display: newlines and repeated whitespace become a
// single space and the result is rune-safe truncated. The full, unmodified
// message is still written to bloom.log by ProcessStream.
func flattenMessage(msg string) string {
	msg = whitespaceRunRegex.ReplaceAllString(msg, " ")
	msg = strings.TrimSpace(msg)

	const maxLen = 240
	if runes := []rune(msg); len(runes) > maxLen {
		msg = string(runes[:maxLen-3]) + "..."
	}
	return msg
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

	// Print join information if available
	if p.joinInfo != "" {
		fmt.Println()
		fmt.Print(p.joinInfo)
		fmt.Println()
	}

	// Print credential information if CLUSTERFORGE_RELEASE is configured
	if p.config != nil {
		clusterforgeRelease := p.config["CLUSTERFORGE_RELEASE"]
		domain := p.config["DOMAIN"]

		if clusterforgeRelease != "" && clusterforgeRelease != "none" && domain != "" {
			aiwbOnly := strings.EqualFold(p.config["AIWB_ONLY"], "true")
			endpoints := clusterForgeEndpoints(aiwbOnly)

			fmt.Println()
			fmt.Println("🚀 ClusterForge Deployment:")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println("⏳ Services are starting up. Endpoints will be available once everything below is ready.")
			fmt.Println()
			fmt.Println("Run this command to wait for services to be ready (Ctrl+C to exit early):")
			fmt.Println()
			printReadinessScript(aiwbOnly)
			fmt.Println()
			fmt.Println("Once ready, access these endpoints:")
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
			fmt.Println()
			fmt.Println("📋 Credential Information:")
			fmt.Println()
			for _, ep := range endpoints {
				fmt.Printf("%s %s:\n", ep.Emoji, ep.Label)
				fmt.Printf("   URL:      https://%s.%s\n", ep.Subdomain, domain)
				if ep.Username != "" {
					fmt.Printf("   Username: %s\n", ep.formatUsername(domain))
					fmt.Printf("   Password: kubectl -n %s get secret %s -o jsonpath='{%s}' | base64 --decode && echo\n", ep.SecretNamespace, ep.SecretName, ep.SecretJSONPath)
				} else {
					fmt.Printf("   Token:    kubectl -n %s get secret %s -o jsonpath='{%s}' | base64 --decode && echo\n", ep.SecretNamespace, ep.SecretName, ep.SecretJSONPath)
				}
				fmt.Println()
			}
			fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		}
	}
}

// clusterForgeEndpoint describes one ClusterForge UI/service endpoint whose
// credential lives in a Kubernetes secret.
type clusterForgeEndpoint struct {
	Emoji           string
	Label           string
	Subdomain       string
	Username        string // literal username, "devuser" to expand to devuser@<domain>, or "" for a token-only credential (e.g. OpenBao)
	SecretNamespace string
	SecretName      string
	SecretJSONPath  string
}

func (e clusterForgeEndpoint) formatUsername(domain string) string {
	if e.Username == "devuser" {
		return fmt.Sprintf("devuser@%s", domain)
	}
	return e.Username
}

// clusterForgeEndpoints is the single source of truth for the endpoints listed
// in the credential reference block. The AI Resource Manager endpoint only
// exists on a full (non-AIWB_ONLY) install, so it's omitted for AIWB-only
// installs where the airm namespace/app is never deployed.
func clusterForgeEndpoints(aiwbOnly bool) []clusterForgeEndpoint {
	all := []clusterForgeEndpoint{
		{"🔐", "AI Resource Manager - DevUser", "airmui", "devuser", "keycloak", "airm-realm-credentials", ".data.KEYCLOAK_INITIAL_DEVUSER_PASSWORD"},
		{"💼", "AI Workbench - DevUser", "aiwbui", "devuser", "keycloak", "airm-realm-credentials", ".data.KEYCLOAK_INITIAL_DEVUSER_PASSWORD"},
		{"📦", "ArgoCD - Admin", "argocd", "admin", "argocd", "argocd-initial-admin-secret", ".data.password"},
		{"🔧", "Gitea - Admin", "gitea", "silogen-admin", "cf-gitea", "gitea-admin-credentials", ".data.password"},
		{"🔐", "OpenBao - Root Token", "openbao", "", "cf-openbao", "openbao-keys", ".data.root_token"},
		{"🔑", "Keycloak - Admin", "kc", "silogen-admin", "keycloak", "keycloak-credentials", ".data.KEYCLOAK_INITIAL_ADMIN_PASSWORD"},
	}
	if !aiwbOnly {
		return all
	}
	endpoints := make([]clusterForgeEndpoint, 0, len(all)-1)
	for _, ep := range all {
		if ep.Subdomain == "airmui" {
			continue
		}
		endpoints = append(endpoints, ep)
	}
	return endpoints
}

// printReadinessScript prints a single copy-pasteable chain of `kubectl wait`
// commands covering every namespace that needs to be up before the endpoints
// below are reachable. The airm wait is only included on a full install,
// since AIWB_ONLY disables the airm app entirely (see DISABLED_APPS in
// deploy_clusterforge/main.yaml).
func printReadinessScript(aiwbOnly bool) {
	fmt.Println("  # Wait for envoy-gateway pods to be ready")
	fmt.Println("  kubectl wait --for=condition=ready pod --all -n envoy-gateway-system --timeout=600s && \\")
	fmt.Println("  # Wait for cluster-auth job to complete (creates initial auth configuration)")
	fmt.Println("  kubectl wait --for=condition=complete job --all -n cluster-auth --timeout=600s && \\")
	fmt.Println("  # Wait for Keycloak pods to be ready (auth/identity provider)")
	fmt.Println("  kubectl wait --for=condition=ready pod --all -n keycloak --timeout=600s && \\")
	fmt.Println("  # Wait for AI Workbench pods to be ready")
	fmt.Println("  kubectl wait --for=condition=ready pod --all -n aiwb --timeout=600s && \\")
	if !aiwbOnly {
		fmt.Println("  # Wait for AI Resource Manager pods to be ready")
		fmt.Println("  kubectl wait --for=condition=ready pod --all -n airm --timeout=600s && \\")
	}
	fmt.Println("  echo ''")
	fmt.Println("  echo '✅ Services are ready! Endpoints are now accessible.'")
}

// extractJoinInfoMessage extracts join information from Ansible debug output
func (p *OutputProcessor) extractJoinInfoMessage(line string) string {
	// The line from your log appears to start directly with: "msg": "content..."
	// Let's use a regex to properly extract the message content
	msgPattern := `"msg":\s*"(.*)"`
	re := regexp.MustCompile(msgPattern)
	matches := re.FindStringSubmatch(line)

	if len(matches) < 2 {
		return ""
	}

	// Extract the message content
	msg := matches[1]

	// Replace escaped newlines with actual newlines
	msg = strings.ReplaceAll(msg, "\\n", "\n")
	// Replace escaped quotes
	msg = strings.ReplaceAll(msg, "\\\"", "\"")

	return msg
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
