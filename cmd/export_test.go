package cmd

import (
	"strings"
	"testing"
)

func TestTweakRootPlaybookForExportContentPreservesTaskKeyOrder(t *testing.T) {
	input := []byte(`---
- name: Example play
  hosts: all
  vars:
    BLOOM_DIR: "/tmp/bloom"
  pre_tasks:
    - name: Check passwordless sudo is configured
      ansible.builtin.command:
        cmd: /bin/true
      become: true
      changed_when: false
      register: sudo_check
      ignore_errors: true
`)

	out, err := tweakRootPlaybookForExportContent(input)
	if err != nil {
		t.Fatalf("tweakRootPlaybookForExportContent: %v", err)
	}

	s := string(out)
	if !strings.Contains(s, "  hosts: localhost\n") {
		t.Fatalf("expected hosts: localhost, got:\n%s", s)
	}
	if !strings.Contains(s, "  vars_files:\n    - bloom-vars.yaml\n") {
		t.Fatalf("expected vars_files block, got:\n%s", s)
	}
	if !strings.Contains(s, `BLOOM_DIR: "{{ ansible_env.PWD | default(playbook_dir) }}"`) {
		t.Fatalf("expected export BLOOM_DIR, got:\n%s", s)
	}

	taskIdx := strings.Index(s, "- name: Check passwordless sudo is configured")
	if taskIdx == -1 {
		t.Fatal("expected sudo check task name first in task block")
	}
	becomeIdx := strings.Index(s[taskIdx:], "become: true")
	commandIdx := strings.Index(s[taskIdx:], "ansible.builtin.command:")
	if becomeIdx == -1 || commandIdx == -1 {
		t.Fatalf("expected become and command in sudo task, got:\n%s", s[taskIdx:])
	}
	if becomeIdx < commandIdx {
		t.Fatalf("expected module key before become in task, got:\n%s", s[taskIdx:taskIdx+200])
	}
}
