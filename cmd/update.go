package cmd

import (
	"fmt"

	"github.com/silogen/cluster-bloom/pkg/ansible/runtime"
	"github.com/spf13/cobra"
)

var (
	newDomain    string
	certOption   string
	certPath     string
	keyPath      string
	checkDNSOnly bool
	dryRunUpdate bool
	skipRKE2     bool
)

func updateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update domain and/or TLS certificates for an existing cluster-forge installation",
		Long: `Update the domain name and/or TLS certificates for an existing cluster-forge installation.

You can update:
  - Both domain and TLS certificates (provide --new-domain)
  - Just TLS certificates (omit --new-domain)

When updating domain, this command updates:

Application Layer:
  - TLS certificates (cluster-tls secret)
  - cluster-domain ConfigMap
  - OpenBao domain secret
  - cluster-values repository (GitOps source)
  - Keycloak client redirect URIs
  - AIRM cluster records
  - ArgoCD Application (triggers sync)

RKE2 Infrastructure Layer (by default):
  - API server certificate (regenerated with new domain SAN)
  - OIDC authentication configuration
  - rke2-server service restart (~1-2 min downtime)

Use --skip-rke2 to update only the application layer.

When updating only TLS, this command updates:
  - TLS certificates (cluster-tls secret)

Certificate options:
  - generate: Generate new self-signed certificates
  - provide:  Use user-provided certificate files (requires --cert-path and --key-path)
  - cert-manager: Use cert-manager to issue new certificates

DNS updates must be performed manually. The command will display required DNS changes.

Examples:
  # Update domain with generated certificates
  bloom update --new-domain new.example.com --cert-option generate

  # Update domain with provided certificates
  bloom update --new-domain new.example.com \
    --cert-option provide \
    --cert-path /path/to/cert.pem \
    --key-path /path/to/key.pem

  # Update only TLS certificates (no domain change)
  bloom update --cert-option generate

  # Update only TLS with provided certificate
  bloom update --cert-option provide \
    --cert-path /path/to/cert.pem \
    --key-path /path/to/key.pem

  # Update domain without RKE2 layer (no API server downtime)
  bloom update --new-domain new.example.com \
    --cert-option generate \
    --skip-rke2

  # Check DNS configuration for a domain
  bloom update --check-dns new.example.com`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validation
			if checkDNSOnly {
				if newDomain == "" {
					return fmt.Errorf("--new-domain is required with --check-dns")
				}
				return nil
			}

			// cert-option is always required
			if certOption == "" {
				return fmt.Errorf("--cert-option is required (generate|provide|cert-manager)")
			}

			validCertOptions := map[string]bool{
				"generate":     true,
				"provide":      true,
				"cert-manager": true,
			}
			if !validCertOptions[certOption] {
				return fmt.Errorf("invalid --cert-option: %s (must be generate|provide|cert-manager)", certOption)
			}

			if certOption == "provide" {
				if certPath == "" || keyPath == "" {
					return fmt.Errorf("--cert-path and --key-path are required when using --cert-option=provide")
				}
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Determine if we're updating domain or just TLS
			tlsOnly := newDomain == ""

			// Build config map for Ansible
			cfg := map[string]any{
				"NEW_DOMAIN":     newDomain,
				"CERT_OPTION":    certOption,
				"CERT_PATH":      certPath,
				"KEY_PATH":       keyPath,
				"DRY_RUN":        dryRunUpdate,
				"CHECK_DNS_ONLY": checkDNSOnly,
				"TLS_ONLY":       tlsOnly,
				"SKIP_RKE2":      skipRKE2,
			}

			// Run the Ansible playbook
			exitCode, err := runtime.RunPlaybook(
				cfg,
				"update.yaml",
				dryRunUpdate,
				"",
				runtime.OutputClean,
				Version,
			)

			if exitCode != 0 {
				if tlsOnly {
					return fmt.Errorf("TLS update failed with exit code %d", exitCode)
				}
				return fmt.Errorf("domain update failed with exit code %d", exitCode)
			}

			return err
		},
	}

	cmd.Flags().StringVar(&newDomain, "new-domain", "", "New domain name for the cluster (optional - omit to update only TLS)")
	cmd.Flags().StringVar(&certOption, "cert-option", "", "Certificate option: generate|provide|cert-manager (required)")
	cmd.Flags().StringVar(&certPath, "cert-path", "", "Path to certificate file (required with --cert-option=provide)")
	cmd.Flags().StringVar(&keyPath, "key-path", "", "Path to private key file (required with --cert-option=provide)")
	cmd.Flags().BoolVar(&checkDNSOnly, "check-dns", false, "Only check DNS configuration for the specified domain")
	cmd.Flags().BoolVar(&dryRunUpdate, "dry-run", false, "Show what would be changed without applying updates")
	cmd.Flags().BoolVar(&skipRKE2, "skip-rke2", false, "Skip RKE2 layer updates (API server certificate, config files)")

	return cmd
}
