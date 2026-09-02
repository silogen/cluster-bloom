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
	Fail struct {
		Msg string `yaml:"msg"`
	} `yaml:"fail"`
	Copy struct {
		Content string `yaml:"content"`
		Dest    string `yaml:"dest"`
	} `yaml:"copy"`
	SetFact map[string]string `yaml:"set_fact"`
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

// An operator whose own RKE2_EXTRA_CONFIG already sets node-taint collides
// with the Cilium taint above, so bloom skips adding it. Leaving that as a
// debug-only warning meant an operator who didn't know to add the Cilium
// taint themselves got a log line, not a failure, while the datapath race
// this playbook exists to close stayed open on that node.
func TestRKE2ExtraConfigTaintCollisionFailsLoudly(t *testing.T) {
	tasks := loadTasks(t, "tasks/deploy_cluster/prepare_rke2.yaml")

	var collision *playbookTask
	for i := range tasks {
		if strings.Contains(tasks[i].Fail.Msg, "node-taint") && strings.Contains(tasks[i].Fail.Msg, "RKE2_EXTRA_CONFIG") {
			collision = &tasks[i]
			break
		}
	}
	if collision == nil {
		t.Fatal("prepare_rke2.yaml no longer fails when RKE2_EXTRA_CONFIG suppresses the Cilium taint")
	}

	when := collision.whenText()
	if !strings.Contains(when, "not (FIRST_NODE") {
		t.Errorf("the collision check must never apply to the first node, got when: %s", when)
	}
	if !strings.Contains(when, "RKE2_EXTRA_CONFIG") {
		t.Errorf("the collision check must be gated on RKE2_EXTRA_CONFIG, got when: %s", when)
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

// setFact returns the expression a task assigns to name, or "".
func setFact(tasks []playbookTask, name string) string {
	for _, task := range tasks {
		if expr, ok := task.SetFact[name]; ok {
			return expr
		}
	}
	return ""
}

// cilium_config.yaml used to be gated to CLUSTER_SIZE in [small, medium], so a
// large cluster could not receive Cilium tuning at all. Operators worked around
// it by hand-applying a HelmChartConfig of the same name, which silently
// replaced bloom's own values. The include must gate on FIRST_NODE only: a
// HelmChartConfig is cluster-scoped, so writing it from several joining control
// -plane nodes would race, but every size must be able to reach it.
func TestCiliumConfigReachesEveryClusterSize(t *testing.T) {
	tasks := loadTasks(t, "tasks/deploy_cluster/main.yaml")

	i := indexOfInclude(tasks, "cilium_config.yaml")
	if i < 0 {
		t.Fatal("deploy_cluster/main.yaml no longer includes cilium_config.yaml")
	}

	when := tasks[i].whenText()
	if !strings.Contains(when, "FIRST_NODE") {
		t.Errorf("cilium_config.yaml must be gated to the first node, got when: %s", when)
	}
	if strings.Contains(when, "CLUSTER_SIZE") {
		t.Errorf("cilium_config.yaml must not be gated on CLUSTER_SIZE - large clusters need Cilium tuning too, got when: %s", when)
	}
}

// cluster_ready.yaml waits for "at least one cilium-operator Ready" precisely
// because large runs 2 replicas with hard anti-affinity and one is Pending by
// design on a single node (see TestOperatorGateToleratesAPendingReplica).
// Giving large a replicas value here is the one way to break that gate.
func TestCiliumConfigNeverSetsOperatorReplicasOnLarge(t *testing.T) {
	tasks := loadTasks(t, "tasks/deploy_cluster/cilium_config.yaml")

	expr := setFact(tasks, "cilium_default_values")
	if expr == "" {
		t.Fatal("cilium_config.yaml no longer sets cilium_default_values")
	}
	if !strings.Contains(expr, "'large'") && !strings.Contains(expr, `"large"`) {
		t.Errorf("cilium_default_values must special-case large, got: %s", expr)
	}
	// The empty branch has to be the large one. `{} if X != 'large' else {...}`
	// parses fine and inverts the whole thing.
	if !strings.Contains(expr, "{} if") {
		t.Errorf("large must be the branch that contributes no default values, got: %s", expr)
	}
}

// combine() lets the right-hand operand win. Reversed, bloom's own default
// silently beats the operator's override and CILIUM_HELM_VALUES looks like it
// does nothing for any key bloom also sets.
func TestCiliumUserValuesWinOverBloomDefaults(t *testing.T) {
	tasks := loadTasks(t, "tasks/deploy_cluster/cilium_config.yaml")

	expr := setFact(tasks, "cilium_helm_values")
	if expr == "" {
		t.Fatal("cilium_config.yaml no longer sets cilium_helm_values")
	}
	if !strings.Contains(expr, "cilium_default_values | combine(cilium_user_values") {
		t.Errorf("user values must be the right-hand combine() operand, got: %s", expr)
	}
	// Without recursive=True, {'hubble': ...} replaces the whole default dict
	// rather than merging into it, dropping operator.replicas.
	if !strings.Contains(expr, "recursive=True") {
		t.Errorf("the merge must be recursive, got: %s", expr)
	}
}

// Both filter arguments are load-bearing, not style. to_nice_yaml defaults to
// indent=4, which is not byte-identical to the manifest every existing
// small/medium cluster already has on disk: `copy` would report changed,
// helm-controller would run a Cilium upgrade, and every agent would restart for
// a whitespace diff. indent() defaults to first=False, which leaves the first
// line at column 0 inside the block scalar - not valid YAML at all.
func TestCiliumValuesContentIndentationIsPinned(t *testing.T) {
	tasks := loadTasks(t, "tasks/deploy_cluster/cilium_config.yaml")

	var manifest *playbookTask
	for i := range tasks {
		if strings.Contains(tasks[i].Copy.Content, "kind: HelmChartConfig") {
			manifest = &tasks[i]
			break
		}
	}
	if manifest == nil {
		t.Fatal("cilium_config.yaml no longer writes a HelmChartConfig")
	}

	if got := manifest.Copy.Dest; got != "/var/lib/rancher/rke2/server/manifests/rke2-cilium-config.yaml" {
		t.Errorf("the manifest must land where RKE2 auto-deploys it, got %q", got)
	}

	content := manifest.Copy.Content
	for _, want := range []string{
		"to_nice_yaml(indent=2)",
		"indent(4, first=True)",
		"| trim |",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("rendering must use %s, got:\n%s", want, content)
		}
	}

	// The template expression has to sit at the block scalar's own indent so
	// that it starts at column 0 once dedented; indent(4) then supplies the
	// four spaces valuesContent needs. Any other column shifts every line.
	for _, line := range strings.Split(content, "\n") {
		if !strings.Contains(line, "to_nice_yaml") {
			continue
		}
		if strings.HasPrefix(line, " ") {
			t.Errorf("the values expression must start at column 0 of the dedented block, got %q", line)
		}
	}

	// Deleting a hand-placed HelmChartConfig on a rerun is far worse than
	// leaving a stale one; operators hand-apply this exact file today.
	if strings.Contains(fmt.Sprint(manifest.When), "absent") {
		t.Error("cilium_config.yaml must never remove an existing HelmChartConfig")
	}
}
