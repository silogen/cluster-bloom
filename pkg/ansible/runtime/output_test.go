package runtime

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout runs fn and returns everything it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("failed to read pipe: %v", err)
	}
	return buf.String()
}

func newTestProcessor(config map[string]string) *OutputProcessor {
	return NewOutputProcessor(OutputClean, nil, config)
}

func TestHasFailures(t *testing.T) {
	tests := []struct {
		name  string
		stats PlaybookStats
		want  bool
	}{
		{"clean run", PlaybookStats{OK: 5, Changed: 2}, false},
		{"only ignored", PlaybookStats{OK: 3, Ignored: 1}, false},
		{"failed task", PlaybookStats{OK: 3, Failed: 1}, true},
		{"unreachable task", PlaybookStats{OK: 3, Unreachable: 1}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.stats.HasFailures(); got != tc.want {
				t.Errorf("HasFailures() = %v, want %v", got, tc.want)
			}
		})
	}
}

// On a mid-run failure the ClusterForge Deployment title/credentials block must
// NOT be printed, even when CLUSTERFORGE_RELEASE and DOMAIN are configured.
func TestPrintSummary_SuppressesClusterForgeOnFailure(t *testing.T) {
	p := newTestProcessor(map[string]string{
		"CLUSTERFORGE_RELEASE": "v2.2.1",
		"DOMAIN":               "example.com",
	})
	p.stats.Record(TaskStatusOK)
	p.stats.Record(TaskStatusFailed)

	out := captureStdout(t, p.PrintSummary)

	if strings.Contains(out, "ClusterForge Deployment") {
		t.Errorf("expected ClusterForge Deployment block to be suppressed on failure, got:\n%s", out)
	}
	if strings.Contains(out, "Credential Information") {
		t.Errorf("expected credential block to be suppressed on failure, got:\n%s", out)
	}
}

// On a clean, successful run the ClusterForge Deployment block SHOULD be printed
// when CLUSTERFORGE_RELEASE and DOMAIN are configured.
func TestPrintSummary_PrintsClusterForgeOnSuccess(t *testing.T) {
	p := newTestProcessor(map[string]string{
		"CLUSTERFORGE_RELEASE": "v2.2.1",
		"DOMAIN":               "example.com",
	})
	p.stats.Record(TaskStatusOK)
	p.stats.Record(TaskStatusChanged)

	out := captureStdout(t, p.PrintSummary)

	if !strings.Contains(out, "ClusterForge Deployment") {
		t.Errorf("expected ClusterForge Deployment block on success, got:\n%s", out)
	}
	if !strings.Contains(out, "example.com") {
		t.Errorf("expected credential endpoints for the configured domain, got:\n%s", out)
	}
}

// When CLUSTERFORGE_RELEASE is "none" the block is never printed, regardless of
// outcome (pre-existing behavior; guarded here to prevent regressions).
func TestPrintSummary_NoneReleaseNeverPrints(t *testing.T) {
	p := newTestProcessor(map[string]string{
		"CLUSTERFORGE_RELEASE": "none",
		"DOMAIN":               "example.com",
	})
	p.stats.Record(TaskStatusOK)

	out := captureStdout(t, p.PrintSummary)

	if strings.Contains(out, "ClusterForge Deployment") {
		t.Errorf("expected no ClusterForge block when release is 'none', got:\n%s", out)
	}
}
