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
	dryRunDomain bool
)

func newUpdateDomainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-domain",
		Short: "Update the domain for an existing cluster-forge installation",
		Long: `Update the domain name for an existing cluster-forge installation.

This command updates:
  - TLS certificates (cluster-tls secret)
  - cluster-domain ConfigMap
  - OpenBao domain secret
  - ArgoCD Application global.domain parameter
  - All dependent HTTPRoutes and TLSRoutes (via ArgoCD sync)

Certificate options:
  - generate: Generate new self-signed certificates
  - provide:  Use user-provided certificate files (requires --cert-path and --key-path)
  - cert-manager: Use cert-manager to issue new certificates

DNS updates must be performed manually. The command will display required DNS changes.

Examples:
  # Update domain with generated certificates
  bloom update-domain --new-domain new.example.com --cert-option generate

  # Update domain with provided certificates
  bloom update-domain --new-domain new.example.com \
    --cert-option provide \
    --cert-path /path/to/cert.pem \
    --key-path /path/to/key.pem

  # Check DNS configuration for a domain
  bloom update-domain --check-dns new.example.com`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Validation
			if checkDNSOnly {
				if newDomain == "" {
					return fmt.Errorf("--new-domain is required with --check-dns")
				}
				return nil
			}

			if newDomain == "" {
				return fmt.Errorf("--new-domain is required")
			}

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
			// Build config map for Ansible
			cfg := map[string]any{
				"NEW_DOMAIN":     newDomain,
				"CERT_OPTION":    certOption,
				"CERT_PATH":      certPath,
				"KEY_PATH":       keyPath,
				"DRY_RUN":        dryRunDomain,
				"CHECK_DNS_ONLY": checkDNSOnly,
			}

			// Run the Ansible playbook
			exitCode, err := runtime.RunPlaybook(
				cfg,
				"update-domain.yaml",
				dryRunDomain,
				"",
				runtime.OutputClean,
				Version,
			)

			if exitCode != 0 {
				return fmt.Errorf("domain update failed with exit code %d", exitCode)
			}

			return err
		},
	}

	cmd.Flags().StringVar(&newDomain, "new-domain", "", "New domain name for the cluster (required)")
	cmd.Flags().StringVar(&certOption, "cert-option", "", "Certificate option: generate|provide|cert-manager (required)")
	cmd.Flags().StringVar(&certPath, "cert-path", "", "Path to certificate file (required with --cert-option=provide)")
	cmd.Flags().StringVar(&keyPath, "key-path", "", "Path to private key file (required with --cert-option=provide)")
	cmd.Flags().BoolVar(&checkDNSOnly, "check-dns", false, "Only check DNS configuration for the specified domain")
	cmd.Flags().BoolVar(&dryRunDomain, "dry-run", false, "Show what would be changed without applying updates")

	return cmd
}
