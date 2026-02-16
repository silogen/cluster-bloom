# Cluster-Bloom Documentation

Welcome to the comprehensive documentation for Cluster-Bloom, an enterprise-ready AI/ML cluster deployment platform built on RKE2 and Kubernetes.

## Documentation Overview

This documentation provides complete guidance for deploying, configuring, and managing Cluster-Bloom environments. Each document covers specific aspects of the platform, from initial sizing to advanced configuration.

## Documentation Index

### Getting Started
- [**Cluster Sizing and Configurations**](cluster-sizing-configurations.md) - Hardware requirements, sizing guidelines, and deployment planning
- [**Manual Steps Quick Reference**](manual-steps-quick-reference.md) - Essential commands and procedures for cluster management

### Core Deployment
- [**RKE2 Deployment**](rke2-deployment.md) - Kubernetes cluster foundation setup and configuration
- [**ROCm Support**](rocm-support.md) - AMD GPU support and ROCm integration for AI workloads
- [**Storage Management**](storage-management.md) - Longhorn distributed storage configuration and management
- [**Longhorn Drive Setup and Recovery**](longhorn-drive-setup-and-recovery.md) - Detailed drive recovery, RAID handling, and storage troubleshooting

### Infrastructure Configuration  
- [**Network Configuration**](network-configuration.md) - Networking setup, load balancing, and connectivity
- [**Certificate Management**](certificate-management.md) - TLS/SSL certificate handling and automation
- [**TLS SAN Configuration**](tls-san-configuration.md) - Additional domain names for API server certificates
- [**Terminal UI**](terminal-ui.md) - Interactive command-line interface and user experience
- [**Technical Architecture**](technical-architecture.md) - System design, component interactions, and architectural decisions

### Operations and Maintenance
- [**Installation Guide**](installation-guide.md) - Complete step-by-step installation procedures
- [**Cloud Compatibility**](cloud-compatibility.md) - Multi-cloud deployment strategies and platform-specific considerations
- [**Configuration Reference**](configuration-reference.md) - Comprehensive configuration options and parameters
- [**OIDC Authentication**](oidc-authentication.md) - Single sign-on integration and identity management

## Quick Navigation

### For New Users
1. Start with [Cluster Sizing and Configurations](cluster-sizing-configurations.md) to plan your deployment
2. Follow the [Installation Guide](installation-guide.md) for step-by-step setup
3. Reference [Manual Steps Quick Reference](manual-steps-quick-reference.md) for common operations

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
- [Manual Steps Quick Reference](manual-steps-quick-reference.md) - Emergency procedures and common fixes

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
- Use [Manual Steps Quick Reference](manual-steps-quick-reference.md) for operational procedures

---

*This is the way to build enterprise-grade AI infrastructure that eliminates impurities.*