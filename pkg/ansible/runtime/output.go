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
			if !selfDescribing && taskInfo.Message != "" && !strings.Contains(taskInfo.Message, "{") {
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

	// Collapse the per-status counts into a single, unambiguous verdict. Without
	// this, a run whose last printed task is a "❌ (failed)" (e.g. the
	// validate_node consolidated failure) reads as if it broke midway and later
	// steps never ran — even though the run reached its natural end. The counts
	// above already show the full picture; this states the bottom line.
	fmt.Println()
	switch {
	case p.stats.Failed > 0 || p.stats.Unreachable > 0:
		fmt.Printf("❌ Overall status: FAILED — %d task(s) did not pass. All steps ran; review the failures above (full log: bloom.log).\n",
			p.stats.Failed+p.stats.Unreachable)
	case p.stats.Ignored > 0:
		fmt.Printf("⚠️  Overall status: COMPLETED WITH WARNINGS — %d issue(s) were tolerated and did not stop the run.\n",
			p.stats.Ignored)
	default:
		fmt.Println("✅ Overall status: SUCCESS — all steps completed.")
	}

	// Print join information if available
	if p.joinInfo != "" {
		fmt.Println()
		fmt.Print(p.joinInfo)
		fmt.Println()
	}

	// The ClusterForge deployment banner + credentials block used to print here
	// based on config alone (CLUSTERFORGE_RELEASE + DOMAIN set), regardless of
	// whether cluster-forge was actually deployed. It now lives in the top-level
	// bloom process (cmd.printClusterForgeSummary), which runs on the host and
	// can check for real deployment evidence via kubectl before printing — this
	// ansible child pivot-roots into a bundled rootfs and cannot reliably query
	// the cluster.
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
