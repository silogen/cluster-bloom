# Cloud Platform Compatibility

## Overview

ClusterBloom is designed for on-premises bare-metal Kubernetes deployments. This document outlines infrastructure dependencies, potential conflicts, and configuration considerations for applications that may need to run on both ClusterBloom (bare-metal) and managed Kubernetes services (EKS, AKS, GKE).

## Core Infrastructure Differences

### Kubernetes Distribution

**ClusterBloom**: RKE2 with Cilium CNI  
**Cloud**: Managed control planes (EKS, AKS, GKE) with cloud-native CNI

### Storage

**ClusterBloom**: Longhorn distributed storage (`mlstorage` storage class)  
**Cloud**: Platform-specific CSI drivers (EBS, Azure Disk, GCP Persistent Disk)

### Load Balancing

**ClusterBloom**: MetalLB with Layer 2 advertisement  
**Cloud**: Native cloud load balancers (NLB/CLB, Azure LB, GCP LB)

### Certificate Management

**ClusterBloom**: Optional cert-manager with Let's Encrypt  
**Cloud**: Platform-specific certificate services (ACM, Azure Key Vault, Google-managed certs)

### GPU Support

**ClusterBloom**: AMD ROCm drivers with manual installation  
**Cloud**: Platform-specific GPU node pools with pre-configured drivers

## Application Portability Strategy

### Storage Abstraction

Use environment variables to specify storage classes:

```yaml
STORAGE_CLASS_NAME: "mlstorage"  # ClusterBloom
# Cloud alternatives:
# - "gp3" (EKS)
# - "managed-csi" (AKS)  
# - "standard-rwo" (GKE)
```

### Service Abstraction

Apply platform-specific annotations conditionally:

```yaml
apiVersion: v1
kind: Service
metadata:
  annotations:
    # MetalLB for ClusterBloom
    metallb.universe.tf/address-pool: "cluster-bloom-ip-pool"
    # Cloud-specific annotations added via templating
```

### Ingress Abstraction

Use configurable ingress classes:

```yaml
spec:
  ingressClassName: ${INGRESS_CLASS_NAME:-nginx}
```

## Migration Checklist

### Storage Migration
- [ ] Replace hardcoded `mlstorage` references with environment variables
- [ ] Test volume expansion behavior on target platform
- [ ] Migrate backup/snapshot mechanisms
- [ ] Validate RWX volume behavior

### Networking Migration  
- [ ] Update LoadBalancer service annotations
- [ ] Modify ingress controller class and annotations
- [ ] Configure DNS integration
- [ ] Test SSL/TLS termination

### GPU Workload Migration
- [ ] Update GPU resource requests (amd.com/gpu → platform-specific)
- [ ] Modify node selectors for cloud GPU node pools
- [ ] Update pod security policies
- [ ] Verify GPU scheduling behavior

## Platform-Specific Considerations

### Amazon EKS
- Use AWS EBS CSI driver for storage
- Add AWS Load Balancer Controller annotations
- Configure IAM roles for service accounts (IRSA)
- Adjust network policies for VPC CNI

### Azure AKS
- Use Azure Disk CSI driver for storage
- Configure Azure Application Gateway (if needed)
- Integrate with Azure Key Vault for secrets
- Adjust network policies for Azure CNI

### Google GKE
- Use Google Persistent Disk CSI driver
- Configure Workload Identity
- Use Google-managed certificates (optional)
- Adjust network policies for GKE CNI

## Best Practices

### 1. Use Configuration Templating
Leverage Helm or Kustomize for platform-specific values:

```yaml
# values-clusterbloom.yaml
storageClass: mlstorage
loadBalancerType: metallb

# values-eks.yaml
storageClass: gp3
loadBalancerType: aws-nlb
```

### 2. Abstract Platform Dependencies
Create abstraction layers for:
- Storage provisioners
- Load balancers
- Certificate managers
- GPU scheduling

### 3. Document Platform Requirements
Maintain clear documentation of:
- Required annotations per platform
- Storage class mappings
- Network policy differences
- GPU resource naming

### 4. Test on Multiple Platforms
Establish CI/CD pipelines for:
- Bare-metal (ClusterBloom) testing
- Cloud platform testing (EKS, AKS, GKE)
- Multi-cluster service mesh validation

## Trade-offs

### ClusterBloom Advantages
- Full infrastructure control
- Predictable costs
- No cloud vendor lock-in
- Custom AMD GPU optimization
- Data sovereignty

### Cloud Platform Advantages
- Managed control plane
- Native cloud service integration
- Automatic scaling and updates
- Global availability zones
- Enterprise support options

See [PRD.md](../../PRD.md) for product overview and [06-technical-architecture.md](06-technical-architecture.md) for technical details.
