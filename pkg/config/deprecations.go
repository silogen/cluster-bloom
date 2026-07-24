package config

import (
	"fmt"
	"sort"
)

// deprecation describes a config key that older bloom releases accepted but the
// current schema no longer does. Kept separate from the schema so a stale key
// in an old bloom.yaml produces a helpful, actionable warning instead of the
// generic "Unknown configuration key" hard-fail (which cannot tell a removed
// key apart from a typo).
type deprecation struct {
	// Successor names the current key(s) that replace this one, or "" when the
	// setting was removed with no direct replacement.
	Successor string
	// Detail explains what happened and what to do instead.
	Detail string
	// RemovedIn is the first release that no longer accepted the key,
	// reconstructed from the schema/args history across git tags.
	RemovedIn string
}

// deprecatedKeys was reconstructed by diffing the set of valid config keys at
// every major.minor tag (v0.1.0 → v2.2.1) against the current schema — the
// authoritative record of removed keys, since no changelog tracked them. Each
// entry is a key that existed in a released bloom.yaml schema and has since
// been dropped.
var deprecatedKeys = map[string]deprecation{
	"OIDC_URL": {
		RemovedIn: "v1.3.0",
		Detail:    "the OIDC issuer is now derived from DOMAIN (kc.<DOMAIN>); set DOMAIN instead, and use ADDITIONAL_OIDC_PROVIDERS for extra issuers.",
	},
	"SELECTED_DISKS": {
		Successor: "CLUSTER_DISKS",
		RemovedIn: "v0.3.0",
		Detail:    "the disk configuration was reworked; use CLUSTER_DISKS (plus CLUSTER_PREMOUNTED_DISKS / NO_DISKS_FOR_CLUSTER) instead.",
	},
	"LONGHORN_DISKS": {
		Successor: "CLUSTER_DISKS",
		RemovedIn: "v0.3.0",
		Detail:    "the disk configuration was reworked; use CLUSTER_DISKS (plus CLUSTER_PREMOUNTED_DISKS / NO_DISKS_FOR_CLUSTER) instead.",
	},
	"SKIP_DISK_CHECK": {
		Successor: "SKIP_RANCHER_PARTITION_CHECK",
		RemovedIn: "v0.3.0",
		Detail:    "use SKIP_RANCHER_PARTITION_CHECK (and SKIP_DATA_SAFETY) instead.",
	},
	"DNSMASQ": {
		Successor: "FIX_DNS",
		RemovedIn: "v2.1.0",
		Detail:    "local DNS handling was reworked; use FIX_DNS (with DNS_SERVERS) instead.",
	},
	"RANCHER_PARTITION_MIN_GB": {
		Successor: "SKIP_RANCHER_PARTITION_CHECK",
		Detail:    "the /var/lib/rancher size thresholds are no longer configurable (fixed at 100GB min / 500GB recommended); use SKIP_RANCHER_PARTITION_CHECK to bypass the check, or RANCHER_DISK to attach a dedicated device.",
	},
	"RANCHER_PARTITION_RECOMMENDED_GB": {
		Successor: "SKIP_RANCHER_PARTITION_CHECK",
		Detail:    "the /var/lib/rancher size thresholds are no longer configurable (fixed at 100GB min / 500GB recommended); use SKIP_RANCHER_PARTITION_CHECK to bypass the check, or RANCHER_DISK to attach a dedicated device.",
	},
	"INSTALL_ARGOCD": {
		RemovedIn: "v2.1.0",
		Detail:    "ArgoCD is now deployed by ClusterForge, not bloom; remove this key.",
	},
	"ARGOCD_VERSION": {
		RemovedIn: "v2.1.0",
		Detail:    "ArgoCD is now deployed by ClusterForge, not bloom; remove this key.",
	},
}

// ApplyDeprecations strips any deprecated keys from cfg (so they don't trip the
// unknown-key check in Validate) and returns a human-readable warning for each
// one found. Deprecated keys are ignored rather than fatal: an old bloom.yaml
// keeps working, while a genuinely unknown key (typo) still hard-fails in
// Validate. Warnings are returned sorted for deterministic output.
func ApplyDeprecations(cfg Config) []string {
	var warnings []string
	for key, dep := range deprecatedKeys {
		if _, present := cfg[key]; !present {
			continue
		}
		delete(cfg, key)

		msg := fmt.Sprintf("%q is deprecated and ignored", key)
		if dep.RemovedIn != "" {
			msg += fmt.Sprintf(" (removed in %s)", dep.RemovedIn)
		}
		if dep.Detail != "" {
			msg += ": " + dep.Detail
		}
		warnings = append(warnings, msg)
	}
	sort.Strings(warnings)
	return warnings
}
