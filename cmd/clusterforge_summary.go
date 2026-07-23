package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
// credentials/endpoints. If cluster-forge was deployed in THIS invocation (a
// full run, or --tags deploy_clusterforge), the banner is shown even before
// pods schedule, since that is the "services are starting up" case. Otherwise,
// with no evidence, it prints how to deploy cluster-forge instead of a
// misleading deployment banner (e.g. after a --tags prepare_node run).
//
// Gated to first nodes, since cluster-forge is only ever deployed from the first
// node (see tasks/deploy_clusterforge/main.yaml).
func printClusterForgeSummary(cfg config.Config, configFile, tags string) {
	if !cfgBool(cfg, "FIRST_NODE", true) {
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
		printClusterForgeCredentials(domain)
		return
	}

	printClusterForgeNotDetected(configFile)
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

// printClusterForgeCredentials prints the endpoint + credential retrieval block.
// Relocated verbatim from the ansible OutputProcessor summary so it can be gated
// on real deployment evidence from the host.
func printClusterForgeCredentials(domain string) {
	fmt.Println()
	fmt.Println("🚀 ClusterForge Deployment:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println("⏳ Services are starting up. Endpoints will be available once envoy-gateway is ready.")
	fmt.Println()
	fmt.Println("Run this command to wait for services to be ready (Ctrl+C to exit early):")
	fmt.Println()
	fmt.Println("  # Wait for envoy-gateway pods to be ready")
	fmt.Println("  kubectl wait --for=condition=ready pod --all -n envoy-gateway-system --timeout=600s && \\")
	fmt.Println("  # Wait for cluster-auth job to complete (creates initial auth configuration)")
	fmt.Println("  kubectl wait --for=condition=complete job --all -n cluster-auth --timeout=600s && \\")
	fmt.Println("  echo ''")
	fmt.Println("  echo '✅ Services are ready! Endpoints are now accessible.'")
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
	fmt.Printf("   Username: silogen-admin\n")
	fmt.Printf("   Password: kubectl -n cf-gitea get secret gitea-admin-credentials -o jsonpath='{.data.password}' | base64 --decode && echo\n")
	fmt.Println()
	fmt.Printf("🔐 OpenBao - Root Token:\n")
	fmt.Printf("   URL:      https://openbao.%s\n", domain)
	fmt.Printf("   Token:    kubectl -n cf-openbao get secret openbao-keys -o jsonpath='{.data.root_token}' | base64 --decode && echo\n")
	fmt.Println()
	fmt.Printf("🔑 Keycloak - Admin:\n")
	fmt.Printf("   URL:      https://kc.%s\n", domain)
	fmt.Printf("   Username: silogen-admin\n")
	fmt.Printf("   Password: kubectl -n keycloak get secret keycloak-credentials -o jsonpath='{.data.KEYCLOAK_INITIAL_ADMIN_PASSWORD}' | base64 --decode && echo\n")
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
