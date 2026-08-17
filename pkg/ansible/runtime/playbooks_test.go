package runtime

import (
	"fmt"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type playbookTask struct {
	Name         string `yaml:"name"`
	IncludeTasks string `yaml:"include_tasks"`
	When         any    `yaml:"when"`
	Shell        string `yaml:"shell"`
	Command      string `yaml:"command"`
	FailedWhen   any    `yaml:"failed_when"`
	Blockinfile  struct {
		Path  string `yaml:"path"`
		Block string `yaml:"block"`
	} `yaml:"blockinfile"`
}

func (t playbookTask) whenText() string {
	return fmt.Sprint(t.When)
}

func loadTasks(t *testing.T, path string) []playbookTask {
	t.Helper()

	raw, err := embeddedPlaybooks.ReadFile("playbooks/" + path)
	if err != nil {
		t.Fatalf("read embedded %s: %v", path, err)
	}

	var tasks []playbookTask
	if err := yaml.Unmarshal(raw, &tasks); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return tasks
}

func indexOfInclude(tasks []playbookTask, include string) int {
	for i, task := range tasks {
		if task.IncludeTasks == include {
			return i
		}
	}
	return -1
}

// The gate is what keeps node_annotator's Job and the storage provisioners from
// creating pods while Cilium is still initializing. If it ever drifts below
// them the reordering is silent — everything still deploys, just racily.
func TestReadinessGatePrecedesPodCreatingIncludes(t *testing.T) {
	tasks := loadTasks(t, "tasks/deploy_k8s_apps/main.yaml")

	gate := indexOfInclude(tasks, "../deploy_cluster/cluster_ready.yaml")
	if gate < 0 {
		t.Fatal("deploy_k8s_apps/main.yaml does not include the cluster readiness gate")
	}

	for _, include := range []string{"node_annotator.yaml", "metallb.yaml", "local_path.yaml", "longhorn.yaml"} {
		i := indexOfInclude(tasks, include)
		if i < 0 {
			t.Errorf("%s is no longer included", include)
			continue
		}
		if i < gate {
			t.Errorf("%s runs before the cluster readiness gate", include)
		}
	}
}

func TestJoiningNodesWaitForTheirCiliumAgent(t *testing.T) {
	tasks := loadTasks(t, "tasks/deploy_cluster/main.yaml")

	i := indexOfInclude(tasks, "cilium_agent_ready.yaml")
	if i < 0 {
		t.Fatal("deploy_cluster/main.yaml does not include cilium_agent_ready.yaml")
	}

	if when := tasks[i].whenText(); !strings.Contains(when, "not (FIRST_NODE") {
		t.Errorf("cilium_agent_ready.yaml must be gated to joining nodes, got when: %s", when)
	}
}

// Tainting the first node deadlocks a fresh cluster: helm-install-rke2-cilium
// does not tolerate node.cilium.io/agent-not-ready, so the Cilium install that
// clears the taint can never be scheduled.
func TestCiliumTaintSkipsTheFirstNode(t *testing.T) {
	tasks := loadTasks(t, "tasks/deploy_cluster/prepare_rke2.yaml")

	var taint *playbookTask
	for i := range tasks {
		if strings.Contains(tasks[i].Blockinfile.Block, "node.cilium.io/agent-not-ready") {
			taint = &tasks[i]
			break
		}
	}
	if taint == nil {
		t.Fatal("prepare_rke2.yaml no longer registers joining nodes with the Cilium taint")
	}

	if !strings.Contains(taint.Blockinfile.Block, "node.cilium.io/agent-not-ready=true:NoExecute") {
		t.Errorf("unexpected taint spec:\n%s", taint.Blockinfile.Block)
	}

	when := taint.whenText()
	if !strings.Contains(when, "not (FIRST_NODE") {
		t.Errorf("the Cilium taint must never apply to the first node, got when: %s", when)
	}
	if !strings.Contains(when, "RKE2_EXTRA_CONFIG") {
		t.Errorf("the Cilium taint must yield to an operator-supplied node-taint, got when: %s", when)
	}

	if got := taint.Blockinfile.Path; got != "/etc/rancher/rke2/config.yaml" {
		t.Errorf("the taint must land in the RKE2 config kubelet reads at registration, got %q", got)
	}
}

// The gate only works if it is reachable. `when: false`, or dropping the
// FIRST_NODE condition so it never matches, would leave every other assertion
// in this file passing against a gate that never runs.
func TestReadinessGateIsReachable(t *testing.T) {
	for path, include := range map[string]string{
		"tasks/deploy_k8s_apps/main.yaml": "../deploy_cluster/cluster_ready.yaml",
		"tasks/deploy_cluster/main.yaml":  "cluster_ready.yaml",
	} {
		tasks := loadTasks(t, path)

		i := indexOfInclude(tasks, include)
		if i < 0 {
			t.Errorf("%s no longer includes %s", path, include)
			continue
		}

		when := tasks[i].whenText()
		if !strings.Contains(when, "FIRST_NODE") {
			t.Errorf("%s: gate must be gated on FIRST_NODE, got when: %s", path, when)
		}
		if strings.Contains(when, "false") {
			t.Errorf("%s: gate is disabled by its own condition: %s", path, when)
		}
	}
}

// CLUSTER_SIZE large runs 2 cilium-operator replicas with hard pod
// anti-affinity, so on the single-node first-node run one replica is Pending by
// design. Both of the "obvious" rewrites below block on *all* replicas and hang
// for their full timeout in exactly the scenario the gate exists to fix. This
// is the load-bearing detail of the whole change, and it is invisible to a
// reviewer who does not know the anti-affinity story — so pin it.
func TestOperatorGateToleratesAPendingReplica(t *testing.T) {
	tasks := loadTasks(t, "tasks/deploy_cluster/cluster_ready.yaml")

	var operator *playbookTask
	for i := range tasks {
		body := tasks[i].Shell + tasks[i].Command
		if strings.Contains(body, "name=cilium-operator") {
			operator = &tasks[i]
			break
		}
	}
	if operator == nil {
		t.Fatal("cluster_ready.yaml no longer waits for a cilium-operator")
	}

	body := operator.Shell + operator.Command
	for _, banned := range []string{
		"rollout status deploy",
		"wait --for=condition=Ready pod",
	} {
		if strings.Contains(body, banned) {
			t.Errorf("%q blocks on every operator replica and hangs when one is Pending by design", banned)
		}
	}

	// A soft failure here is worse than no gate: the run goes green while pods
	// are created into the race the gate was added to prevent.
	if operator.FailedWhen != nil {
		t.Errorf("the operator gate must be fatal, got failed_when: %v", operator.FailedWhen)
	}
}
