package runtime

import (
	"regexp"
	"strings"
)

// TaskInfo represents parsed information from an Ansible task
type TaskInfo struct {
	Name    string
	Status  TaskStatus
	Message string
}

var (
	// Match "TASK [task name] ****"
	taskHeaderRegex = regexp.MustCompile(`^TASK \[(.*?)\]`)

	// Match result lines like "ok: [127.0.0.1]", "changed: [host]", etc.
	resultRegex = regexp.MustCompile(`^(ok|changed|failed|skipping|unreachable|ignoring):\s*\[(.*?)\](.*)`)

	// Match fatal errors
	fatalRegex = regexp.MustCompile(`^fatal:\s*\[(.*?)\]:(.*)`)
)

// ParseTaskHeader checks if a line is a task header and extracts the task name
func ParseTaskHeader(line string) (string, bool) {
	matches := taskHeaderRegex.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1], true
	}
	return "", false
}

// ParseTaskResult checks if a line is a task result and extracts status info
func ParseTaskResult(line string) (*TaskInfo, bool) {
	// Try result regex first
	matches := resultRegex.FindStringSubmatch(line)
	if len(matches) > 1 {
		status := normalizeStatus(matches[1])
		message := strings.TrimSpace(matches[3])

		// Try to extract changed message from JSON-like output
		if strings.Contains(message, "=>") {
			// Extract message after =>
			parts := strings.SplitN(message, "=>", 2)
			if len(parts) == 2 {
				message = extractBriefMessage(parts[1])
			}
		}

		return &TaskInfo{
			Status:  status,
			Message: message,
		}, true
	}

	// Try fatal regex
	matches = fatalRegex.FindStringSubmatch(line)
	if len(matches) > 1 {
		message := strings.TrimSpace(matches[2])
		message = extractBriefMessage(message)

		return &TaskInfo{
			Status:  TaskStatusFailed,
			Message: message,
		}, true
	}

	return nil, false
}

// normalizeStatus converts Ansible status strings to TaskStatus
func normalizeStatus(status string) TaskStatus {
	switch strings.ToLower(status) {
	case "ok":
		return TaskStatusOK
	case "changed":
		return TaskStatusChanged
	case "failed":
		return TaskStatusFailed
	case "skipping":
		return TaskStatusSkipped
	case "unreachable":
		return TaskStatusUnreachable
	case "ignoring":
		return TaskStatusIgnored
	default:
		return TaskStatusOK
	}
}

// extractBriefMessage extracts a brief human-readable message from Ansible output
func extractBriefMessage(fullMsg string) string {
	// Try to extract msg field from JSON
	if strings.Contains(fullMsg, `"msg":`) {
		msgRegex := regexp.MustCompile(`"msg":\s*"([^"]+)"`)
		if matches := msgRegex.FindStringSubmatch(fullMsg); len(matches) > 1 {
			return matches[1]
		}
	}

	// Try to extract stderr or stdout
	if strings.Contains(fullMsg, `"stderr":`) {
		stderrRegex := regexp.MustCompile(`"stderr":\s*"([^"]+)"`)
		if matches := stderrRegex.FindStringSubmatch(fullMsg); len(matches) > 1 {
			msg := matches[1]
			if len(msg) > 100 {
				msg = msg[:97] + "..."
			}
			return msg
		}
	}

	// Try to extract changed info
	if strings.Contains(fullMsg, "changed=true") || strings.Contains(fullMsg, `"changed": true`) {
		return "configuration updated"
	}

	// Return truncated version if too long
	if len(fullMsg) > 100 {
		return fullMsg[:97] + "..."
	}

	return fullMsg
}

// IsIgnoredError checks if a task failure should be treated as ignored
func IsIgnoredError(line string) bool {
	return strings.Contains(line, "...ignoring") ||
		strings.Contains(line, "ignore_errors=True")
}
