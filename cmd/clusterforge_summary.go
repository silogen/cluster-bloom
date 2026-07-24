package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/silogen/cluster-bloom/pkg/config"
)

// clusterForgeAppNamespaces are the namespaces where cluster-forge's end-user
// application workloads run. The presence of pods in any of them is treated as
// evidence that cluster-forge is actually deployed on this cluster, as opposed
// to merely being configured in bloom.yaml.
var clusterForgeAppNamespaces = []string{"aiwb", "airm", "aim-system", "blueprints"}

const (
	rke2Kubeconfig = "/etc/rancher/rke2/rke2.yaml"
	rke2Kubectl    = "/var/lib/rancher/rke2/bin/kubectl"
)

// printClusterForgeSummary prints the post-run ClusterForge section. It runs in
// the top-level bloom process on the host (NOT the namespaced ansible child,
// which pivot-roots into a bundled rootfs), so it can query the cluster via
// kubectl.
//
// Unlike the old config-only banner, it looks for actual evidence of a
// cluster-forge deployment (pods in the app namespaces) before printing the
// endpoints/credentials. If cluster-forge was deployed in THIS invocation (a
// full run, or --tags deploy_clusterforge), the banner is shown even before
// pods schedule, since that is the "services are starting up" case. Otherwise,
// with no evidence, it prints how to deploy cluster-forge instead of a
// misleading deployment banner (e.g. after a --tags prepare_node run).
//
// Gated to first nodes, since cluster-forge is only ever deployed from the first
// node (see tasks/deploy_clusterforge/main.yaml).
//
// exitCode is the playbook's exit code: a non-zero code means the run failed
// (e.g. node validation), in which case we don't print a "deploy next" banner
// (there's nothing to deploy onto yet) and instead surface targeted remediation
// for known failures.
func printClusterForgeSummary(cfg config.Config, configFile, tags string, exitCode int) {
	if !cfgBool(cfg, "FIRST_NODE", true) {
		return
	}

	// The run failed. Don't advertise a deploy path onto a node that isn't ready;
	// print actionable remediation for known failures (e.g. undersized
	// /var/lib/rancher) instead. Other failures are already self-describing in
	// the consolidated validation summary above.
	if exitCode != 0 {
		printNodeValidationRemediation(cfg)
		return
	}

	cfRelease := cfgString(cfg, "CLUSTERFORGE_RELEASE")
	domain := cfgString(cfg, "DOMAIN")
	cfConfigured := cfRelease != "" && cfRelease != "none"

	// Did cluster-forge deploy in this invocation? A full run (no --tags) or an
	// explicit --tags deploy_clusterforge deploys it; in that case show the
	// banner even if pods have not scheduled yet (they take a while).
	deployRan := cfConfigured && (tags == "" || strings.Contains(tags, "deploy_clusterforge"))

	if detectClusterForge() || deployRan {
		if domain == "" {
			return
		}
		printClusterForgeCredentials(domain, cfgBool(cfg, "AIWB_ONLY", false))
		return
	}

	// No cluster-forge deployment. What the user should do next depends on
	// whether a cluster exists at all:
	//   - cluster reachable (RKE2 up, e.g. after a prepare-only or full run that
	//     didn't include cluster-forge): guide them to deploy cluster-forge onto
	//     the running cluster.
	//   - no cluster (e.g. a standalone --tags validate_node run on a pristine
	//     node that passed validation): guide them to bloom the node first with
	//     a full run.
	if clusterReachable() {
		printClusterForgeNotDetected(configFile)
		return
	}
	printNodeNotBloomed(configFile)
}

// printNodeNotBloomed guides the user to run a full bloom on a validated but
// not-yet-provisioned node (no cluster reachable). Only reached on a successful
// run, so we know the node passed all validation checks.
func printNodeNotBloomed(configFile string) {
	fmt.Println()
	fmt.Println("✅ Node validation passed, but no cluster is running on this node yet.")
	fmt.Println("   To provision the cluster, run a full bloom:")
	fmt.Println()
	fmt.Printf("     sudo %s cli %s\n", os.Args[0], configFile)
	fmt.Println()
}

// detectClusterForge reports whether any cluster-forge application pods are
// present on the cluster. Best-effort: a missing kubeconfig/kubectl, an
// unreachable API server, or an absent namespace all count as "not detected"
// rather than an error, since this only drives an informational summary.
func detectClusterForge() bool {
	kubectl := rke2Kubectl
	if _, err := os.Stat(kubectl); err != nil {
		p, lookErr := exec.LookPath("kubectl")
		if lookErr != nil {
			return false
		}
		kubectl = p
	}
	if _, err := os.Stat(rke2Kubeconfig); err != nil {
		return false
	}

	for _, ns := range clusterForgeAppNamespaces {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		out, err := exec.CommandContext(ctx, kubectl,
			"--kubeconfig", rke2Kubeconfig,
			"get", "pods", "-n", ns, "--no-headers").Output()
		cancel()
		if err != nil {
			continue
		}
		if len(strings.TrimSpace(string(out))) > 0 {
			return true
		}
	}
	return false
}

// clusterReachable reports whether an RKE2 cluster is up and answering on this
// node. Best-effort: a missing kubeconfig/kubectl or an unreachable API server
// all count as "not reachable". Used to decide whether the next step is to
// deploy cluster-forge (cluster up) or to bloom the node (no cluster yet).
func clusterReachable() bool {
	if _, err := os.Stat(rke2Kubeconfig); err != nil {
		return false
	}
	kubectl := rke2Kubectl
	if _, err := os.Stat(kubectl); err != nil {
		p, lookErr := exec.LookPath("kubectl")
		if lookErr != nil {
			return false
		}
		kubectl = p
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err := exec.CommandContext(ctx, kubectl,
		"--kubeconfig", rke2Kubeconfig,
		"get", "--raw", "/readyz").Run()
	return err == nil
}

// rancherPartitionMinGB mirrors the hard-fail threshold in the validate_node
// rancher_partition.yaml task. Kept in sync manually; both are fixed (not
// user-configurable) values.
const rancherPartitionMinGB = 100

// printNodeValidationRemediation prints actionable guidance for known
// validation failures. Currently it handles the undersized /var/lib/rancher
// partition case: it re-measures the partition on the host (the same way the
// ansible check does) and, if it is the culprit, tells the user how to fix it —
// either by attaching a dedicated device via RANCHER_DISK (listing candidate
// devices from lsblk) or, failing that, by skipping the check.
//
// It is a no-op when the check was bypassed (SKIP_RANCHER_PARTITION_CHECK), a
// device is already configured (RANCHER_DISK), or the partition is adequately
// sized (in which case some other check failed and is self-describing).
func printNodeValidationRemediation(cfg config.Config) {
	if cfgBool(cfg, "SKIP_RANCHER_PARTITION_CHECK", false) {
		return
	}
	if cfgString(cfg, "RANCHER_DISK") != "" {
		return
	}
	gb, ok := rancherPartitionGB()
	if !ok || gb >= rancherPartitionMinGB {
		return
	}

	fmt.Println()
	fmt.Printf("💡 The /var/lib/rancher partition (%dGB) is below the required %dGB minimum.\n", gb, rancherPartitionMinGB)
	fmt.Println()

	devices := candidateRancherDevices()
	if len(devices) > 0 {
		fmt.Println("   To fix this, point RANCHER_DISK at a dedicated device in your bloom.yaml")
		fmt.Println("   (Bloom will format it and mount it at /var/lib/rancher). Available devices:")
		fmt.Println()
		for _, d := range devices {
			fmt.Printf("     - %s\n", d)
		}
		fmt.Println()
		fmt.Println("   For example:")
		fmt.Println()
		fmt.Printf("     RANCHER_DISK: %s\n", firstDeviceName(devices[0]))
		fmt.Println()
		fmt.Println("   Alternatively, set SKIP_RANCHER_PARTITION_CHECK: true to skip this check.")
	} else {
		fmt.Println("   No spare block devices were found to use for /var/lib/rancher.")
		fmt.Println("   To proceed on this node, set the following in your bloom.yaml:")
		fmt.Println()
		fmt.Println("     SKIP_RANCHER_PARTITION_CHECK: true")
	}
	fmt.Println()
}

// rancherPartitionGB returns the size (in GB) of the filesystem that holds, or
// would hold, /var/lib/rancher. It mirrors the ansible check: walk up to the
// nearest existing ancestor and measure that filesystem, without creating the
// directory. Returns ok=false on any error.
func rancherPartitionGB() (int, bool) {
	p := "/var/lib/rancher"
	for {
		if _, err := os.Stat(p); err == nil {
			break
		}
		parent := filepath.Dir(p)
		if parent == p {
			return 0, false
		}
		p = parent
	}
	out, err := exec.Command("df", "-BG", p).Output()
	if err != nil {
		return 0, false
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0, false
	}
	fields := strings.Fields(lines[len(lines)-1])
	if len(fields) < 2 {
		return 0, false
	}
	gb, err := strconv.Atoi(strings.TrimSuffix(fields[1], "G"))
	if err != nil {
		return 0, false
	}
	return gb, true
}

// candidateRancherDevices lists whole-disk block devices that could host
// /var/lib/rancher, excluding the disk backing the root filesystem. Each entry
// is formatted as "<device> (<size>)". Best-effort: returns nil on any error.
func candidateRancherDevices() []string {
	rootDisk := rootBackingDisk()

	out, err := exec.Command("lsblk", "-dnpo", "NAME,SIZE,TYPE").Output()
	if err != nil {
		return nil
	}

	var devices []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		name, size, typ := fields[0], fields[1], fields[2]
		if typ != "disk" || name == rootDisk {
			continue
		}
		devices = append(devices, fmt.Sprintf("%s (%s)", name, size))
	}
	return devices
}

// rootBackingDisk returns the whole-disk device path backing the root
// filesystem (e.g. /dev/nvme0n1), so it can be excluded from RANCHER_DISK
// candidates. Returns "" if it cannot be determined.
func rootBackingDisk() string {
	src, err := exec.Command("findmnt", "-no", "SOURCE", "/").Output()
	if err != nil {
		return ""
	}
	dev := strings.TrimSpace(string(src))
	if dev == "" {
		return ""
	}
	// Resolve the parent (whole) disk of the root partition. Empty PKNAME means
	// the source is already a whole disk.
	if pk, err := exec.Command("lsblk", "-no", "PKNAME", dev).Output(); err == nil {
		if parent := strings.TrimSpace(string(pk)); parent != "" {
			return "/dev/" + parent
		}
	}
	return dev
}

// firstDeviceName extracts the device path from a "<device> (<size>)" entry.
func firstDeviceName(entry string) string {
	if i := strings.IndexByte(entry, ' '); i > 0 {
		return entry[:i]
	}
	return entry
}

func printClusterForgeNotDetected(configFile string) {
	invocation := fmt.Sprintf("sudo %s cli %s --tags deploy_clusterforge", os.Args[0], configFile)
	fmt.Println()
	fmt.Println("ℹ️  No cluster-forge deployment detected on this cluster.")
	fmt.Println("   (No application pods found in: " + strings.Join(clusterForgeAppNamespaces, ", ") + ")")
	fmt.Println("   To deploy it, set CLUSTERFORGE_RELEASE: [tag|branch|commit] in your")
	fmt.Println("   bloom.yaml and run:")
	fmt.Println()
	fmt.Printf("     %s\n", invocation)
	fmt.Println()
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

// printClusterForgeCredentials prints the endpoint + credential retrieval block.
// The endpoint table and readiness script are the same ones cluster-forge
// exposes; this variant runs on the host and is gated on real deployment
// evidence by the caller.
func printClusterForgeCredentials(domain string, aiwbOnly bool) {
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

// cfgString reads a string config value, tolerating an absent/nil/non-string
// entry. Local to this file so the fix_prepare_node_tag branch stays
// self-contained (configString lives in an EAI-7530-only file).
func cfgString(cfg config.Config, key string) string {
	v, ok := cfg[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

// cfgBool reads a bool config value, tolerating an absent/nil entry (returns
// def) and a string form as produced when the value comes from an environment
// variable rather than parsed YAML.
func cfgBool(cfg config.Config, key string, def bool) bool {
	v, ok := cfg[key]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "1", "yes":
			return true
		case "false", "0", "no":
			return false
		}
	}
	return def
}
