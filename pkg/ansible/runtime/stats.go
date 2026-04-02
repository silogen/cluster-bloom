package runtime

import "fmt"

// PlaybookStats tracks the outcome statistics for an Ansible playbook run
type PlaybookStats struct {
	OK          int
	Changed     int
	Failed      int
	Skipped     int
	Unreachable int
	Ignored     int
}

// TaskStatus represents the outcome of an Ansible task
type TaskStatus string

const (
	TaskStatusOK          TaskStatus = "ok"
	TaskStatusChanged     TaskStatus = "changed"
	TaskStatusFailed      TaskStatus = "failed"
	TaskStatusSkipped     TaskStatus = "skipped"
	TaskStatusUnreachable TaskStatus = "unreachable"
	TaskStatusIgnored     TaskStatus = "ignored"
)

// Record increments the counter for the given status
func (s *PlaybookStats) Record(status TaskStatus) {
	switch status {
	case TaskStatusOK:
		s.OK++
	case TaskStatusChanged:
		s.Changed++
	case TaskStatusFailed:
		s.Failed++
	case TaskStatusSkipped:
		s.Skipped++
	case TaskStatusUnreachable:
		s.Unreachable++
	case TaskStatusIgnored:
		s.Ignored++
	}
}

// Total returns the total number of tasks
func (s *PlaybookStats) Total() int {
	return s.OK + s.Changed + s.Failed + s.Skipped + s.Unreachable + s.Ignored
}

// Summary returns a formatted summary string
func (s *PlaybookStats) Summary() string {
	return fmt.Sprintf("%d ok, %d changed, %d failed, %d skipped, %d unreachable, %d ignored",
		s.OK, s.Changed, s.Failed, s.Skipped, s.Unreachable, s.Ignored)
}
