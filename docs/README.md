# Cluster-Bloom Documentation

Welcome to the comprehensive documentation for Cluster-Bloom, an enterprise-ready AI/ML cluster deployment platform built on RKE2 and Kubernetes.

## Documentation Overview

This documentation provides complete guidance for deploying, configuring, and managing Cluster-Bloom environments. Each document covers specific aspects of the platform, from initial sizing to advanced configuration.

## Documentation Index

### Getting Started
- [**Cluster Sizing and Configurations**](cluster-sizing-configurations.md) - Hardware requirements, sizing guidelines, and deployment planning
- [**Product Requirements**](PRD.md) - Product scope, capabilities, and validated GPU driver policy
- [**Installation Guide**](installation-guide.md) - Step-by-step installation procedures and operational commands (replaces the removed manual-steps quick reference)

### Core Deployment
- [**RKE2 Deployment**](rke2-deployment.md) - Kubernetes cluster foundation setup and configuration
- [**AMD GPU Driver and Container ROCm Support**](rocm-support.md) - Host-driver policy and Kubernetes integration for containerized ROCm workloads
- [**GPU Driver Support**](gpu-driver-support.md) - Driver detection, installation, validation, standalone AMD-SMI, and recovery behavior
- [**GPU Driver Installation Quick Reference**](../GPU_AND_ROCM_INSTALLATION.md) - Driver policy, configuration, and host verification commands
- [**Storage Management**](storage-management.md) - Longhorn distributed storage configuration and management
- [**Longhorn Drive Setup and Recovery**](longhorn-drive-setup-and-recovery.md) - Detailed drive recovery, RAID handling, and storage troubleshooting

### Design and Evaluation
- [**Longhorn V2 Data Engine Evaluation**](longhorn-v2-data-engine-evaluation.md) — V2 migration design, cleanup behaviour, and implementation notes

> **Note on `manual-steps-quick-reference.md`:** That file was removed in commit `eefe525` as obsolete (739 lines duplicating installation-guide and other docs, with stale numbered paths like `01-rke2-deployment.md`). It was not restored — substituting [Installation Guide](installation-guide.md) avoids maintaining a second command cheat sheet. Restore only if you need a single-page ops crib sheet after an accuracy audit against current bloom behaviour.

### Infrastructure Configuration  
- [**Network Configuration**](network-configuration.md) - Networking setup, load balancing, and connectivity
- [**Certificate Management**](certificate-management.md) - TLS/SSL certificate handling and automation
- [**TLS SAN Configuration**](tls-san-configuration.md) - Additional domain names for API server certificates
- [**Terminal UI**](terminal-ui.md) - Interactive command-line interface and user experience
- [**Technical Architecture**](technical-architecture.md) - System design, component interactions, and architectural decisions

### Operations and Maintenance
- [**Installation Guide**](installation-guide.md) - Complete step-by-step installation procedures
- [**Configuration Reference**](configuration-reference.md) - Comprehensive configuration options and parameters
- [**OIDC Authentication**](oidc-authentication.md) - Single sign-on integration and identity management

## Quick Navigation

### For New Users
1. Start with [Cluster Sizing and Configurations](cluster-sizing-configurations.md) to plan your deployment
2. Follow the [Installation Guide](installation-guide.md) for step-by-step setup and common operational commands
3. Reference the [Configuration Reference](configuration-reference.md) for supported options

### For System Administrators
- [Technical Architecture](technical-architecture.md) - Understand system design
- [Storage Management](storage-management.md) + [Longhorn Drive Setup and Recovery](longhorn-drive-setup-and-recovery.md) - Complete storage configuration
- [Configuration Reference](configuration-reference.md) - Detailed parameter documentation

### For DevOps Engineers
- [RKE2 Deployment](rke2-deployment.md) - Kubernetes foundation
- [Network Configuration](network-configuration.md) - Infrastructure networking
- [Certificate Management](certificate-management.md) - Security configuration

### Troubleshooting and Recovery
- [Longhorn Drive Setup and Recovery](longhorn-drive-setup-and-recovery.md) - Storage troubleshooting and RAID handling
- [Installation Guide](installation-guide.md) - Deployment verification and operational checks

## Documentation Standards

- **Comprehensive Coverage**: Each document provides complete information for its topic area
- **Practical Examples**: Real-world configurations and command examples
- **Cross-References**: Links between related topics for easy navigation
- **Version Compatibility**: All procedures tested with current platform versions

## Contributing

This documentation is maintained as part of the Cluster-Bloom project. For updates, corrections, or additions:

1. Follow the established documentation patterns
2. Include practical examples and command snippets  
3. Test all procedures before documentation
4. Maintain cross-references between related topics

## Support

For questions about the documentation or Cluster-Bloom platform:
- Reference the [Configuration Reference](configuration-reference.md) for parameter details
- Check [Technical Architecture](technical-architecture.md) for design questions
- Use the [Installation Guide](installation-guide.md) for operational procedures

---

*This is the way to build enterprise-grade AI infrastructure that eliminates impurities.*
