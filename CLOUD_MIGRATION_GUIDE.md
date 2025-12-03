# ClusterBloom Cloud Migration Guide

## Table of Contents
1. [Overview](#overview)
2. [Complete Feature Matrix](#complete-feature-matrix)
3. [Conflict Resolution Guide](#conflict-resolution-guide)
4. [Detailed Migration Checklist](#detailed-migration-checklist)
5. [Platform-Specific Configurations](#platform-specific-configurations)
6. [Common Migration Pitfalls](#common-migration-pitfalls)

## Overview

This guide provides comprehensive instructions for migrating applications built on ClusterBloom's bare-metal infrastructure to managed Kubernetes platforms (EKS, AKS, GKE). It addresses every conflict identified in the PRD and manual installation guide.

## Complete Feature Matrix

### Infrastructure Components

| Component | ClusterBloom | EKS | AKS | GKE | Compatibility | Migration Effort |
|-----------|--------------|-----|-----|-----|---------------|------------------|
| **Kubernetes Distribution** | RKE2 v1.28+ | EKS 1.28+ | AKS 1.28+ | GKE 1.28+ | ✅ High | Low |
| **Control Plane** | Self-managed (RKE2) | AWS-managed | Azure-managed | Google-managed | ⚠️ Partial | High |
| **etcd** | Embedded in RKE2 | AWS-managed | Azure-managed | Google-managed | ✅ Compatible | None |
| **Kubelet** | RKE2-provided | EKS-managed | AKS-managed | GKE-managed | ✅ Compatible | None |

### Networking

| Component | ClusterBloom | EKS | AKS | GKE | Compatibility | Migration Effort |
|-----------|--------------|-----|-----|-----|---------------|------------------|
| **CNI** | Cilium (VXLAN) | AWS VPC CNI | Azure CNI/Kubenet | GKE CNI | ⚠️ Different | Medium |
| **Cluster CIDR** | 10.242.0.0/16 | Configurable | Configurable | Configurable | ⚠️ Must not overlap | Low |
| **Service CIDR** | 10.243.0.0/16 | Configurable | Configurable | Configurable | ⚠️ Must not overlap | Low |
| **NetworkPolicy** | Cilium | Calico add-on | Azure NPM/Calico | GKE Network Policy | ⚠️ Feature parity varies | Medium |
| **Pod Security** | Cilium encryption | VPC security groups | NSGs | VPC firewall | ❌ Different model | High |
| **Service Mesh** | Cilium (optional) | App Mesh/Istio | Istio | Anthos Service Mesh | ⚠️ Different | High |
| **Network Observability** | Hubble | VPC Flow Logs | NSG Flow Logs | VPC Flow Logs | ❌ Different tools | High |

### Storage

| Component | ClusterBloom | EKS | AKS | GKE | Compatibility | Migration Effort |
|-----------|--------------|-----|-----|-----|---------------|------------------|
| **Block Storage** | Longhorn | EBS CSI | Azure Disk CSI | PD CSI | ⚠️ Different | High |
| **Storage Class** | mlstorage | gp3/gp2/io1/io2 | managed-csi/premium | standard-rwo/ssd | ❌ Must change | Medium |
| **RWO Support** | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Compatible | Low |
| **RWX Support** | ✅ Yes (built-in) | ❌ Need EFS | ❌ Need Azure Files | ❌ Need Filestore | ❌ Different service | Very High |
| **Snapshots** | Longhorn built-in | CSI snapshots | CSI snapshots | CSI snapshots | ⚠️ Different API | Medium |
| **Backups** | S3/NFS target | AWS Backup | Azure Backup | Cloud Backup | ❌ Different service | High |
| **Replication** | 3 replicas (configurable) | AZ-level (EBS) | Zone-level | Zone-level | ⚠️ Different model | High |
| **Data Locality** | Configurable | Zone-bound | Zone-bound | Zone/region-bound | ⚠️ Different | Medium |
| **Performance** | NVMe-backed | gp3: 3000-16000 IOPS | Premium: 120-20000 IOPS | pd-ssd: varies | ⚠️ Must retune | High |

### Load Balancing

| Component | ClusterBloom | EKS | AKS | GKE | Compatibility | Migration Effort |
|-----------|--------------|-----|-----|-----|---------------|------------------|
| **Load Balancer** | MetalLB L2 | NLB/CLB | Azure Load Balancer | Cloud Load Balancer | ❌ Completely different | Very High |
| **IP Assignment** | Node IP (/32) | Public or private | Public or private | Public or private | ❌ Different | High |
| **L2 Mode** | ARP-based | N/A | N/A | N/A | ❌ Not applicable | N/A |
| **BGP Mode** | Optional | N/A | N/A | N/A | ❌ Not applicable | N/A |
| **Cross-Zone** | Single node (L2) | Multi-AZ default | Multi-zone default | Multi-zone default | ❌ Different | Medium |
| **SSL Termination** | At ingress | At LB or ingress | At LB or ingress | At LB or ingress | ⚠️ Different location | Medium |
| **Health Checks** | TCP/HTTP at service | LB health probes | LB health probes | LB health probes | ⚠️ Different config | Medium |
| **Annotations** | metallb.universe.tf/* | service.beta.kubernetes.io/aws-* | service.beta.kubernetes.io/azure-* | cloud.google.com/* | ❌ All different | High |

### Ingress

| Component | ClusterBloom | EKS | AKS | GKE | Compatibility | Migration Effort |
|-----------|--------------|-----|-----|-----|---------------|------------------|
| **Controller** | User-provided (NGINX suggested) | AWS ALB Controller | App Gateway Ingress | GCE Ingress | ⚠️ Different | High |
| **Ingress Class** | nginx/custom | alb | azure-application-gateway | gce | ❌ Must change | Medium |
| **Path Routing** | NGINX rules | ALB rules | App Gateway rules | URL map | ⚠️ Syntax differs | Medium |
| **SSL/TLS** | cert-manager or manual | ACM integration | Key Vault integration | Google-managed certs | ⚠️ Different | High |
| **Annotations** | kubernetes.io/ingress.class | alb.ingress.kubernetes.io/* | appgw.ingress.kubernetes.io/* | ingress.gcp.kubernetes.io/* | ❌ All different | High |
| **Backend Protocol** | HTTP/HTTPS | HTTP/HTTPS/gRPC | HTTP/HTTPS | HTTP/HTTPS | ✅ Compatible | Low |

### Certificates

| Component | ClusterBloom | EKS | AKS | GKE | Compatibility | Migration Effort |
|-----------|--------------|-----|-----|-----|---------------|------------------|
| **Provider** | cert-manager + Let's Encrypt | ACM (recommended) | Key Vault | Google-managed | ⚠️ Different | High |
| **Issuance** | ACME (HTTP-01/DNS-01) | AWS Certificate Manager | Azure Key Vault | ManagedCertificate CRD | ❌ Different method | High |
| **Storage** | Kubernetes Secret | ACM (referenced by ARN) | Key Vault (synced to Secret) | Google-managed | ⚠️ Different | Medium |
| **Renewal** | cert-manager auto-renew | AWS auto-renew | Azure auto-renew | Google auto-renew | ✅ Both auto | Low |
| **Wildcard Support** | ✅ Yes (DNS-01) | ✅ Yes | ✅ Yes | ✅ Yes | ✅ Compatible | Low |
| **Custom CA** | ✅ Yes (ClusterIssuer) | ❌ No (ACM only) | ⚠️ Via Key Vault | ⚠️ Limited | ⚠️ Limited | High |

### GPU Support

| Component | ClusterBloom | EKS | AKS | GKE | Compatibility | Migration Effort |
|-----------|--------------|-----|-----|-----|---------------|------------------|
| **AMD ROCm** | v6.3.2 (bare-metal install) | GPU instance types | GPU VMs | GPU VMs | ⚠️ Pre-installed on cloud | Medium |
| **Driver Installation** | Manual via amdgpu-install | AMI with drivers | VM image with drivers | Image with drivers | ⚠️ Different | Medium |
| **Device Plugin** | Manual deployment | Pre-deployed | Pre-deployed | Pre-deployed | ⚠️ Different | Low |
| **Resource Name** | amd.com/gpu | amd.com/gpu | amd.com/gpu | amd.com/gpu | ✅ Compatible | None |
| **Node Labels** | gpu=true, amd.com/gpu=true | instance-type label | accelerator label | gke-accelerator label | ⚠️ Different | Low |
| **Instance Types** | Any with AMD GPU | g4ad.*, g5.* | NC-series | a2-highgpu-* | ❌ Cloud-specific | Medium |
| **udev Rules** | Manual (/dev/kfd, /dev/dri) | Pre-configured | Pre-configured | Pre-configured | ⚠️ Not needed | None |

### System Services

| Component | ClusterBloom | EKS | AKS | GKE | Compatibility | Migration Effort |
|-----------|--------------|-----|-----|-----|---------------|------------------|
| **Time Sync** | Chrony (NTP pools or master) | Amazon Time Sync | Azure NTP | Google NTP | ⚠️ Different source | Low |
| **Log Management** | rsyslog + logrotate | CloudWatch Logs | Azure Monitor Logs | Cloud Logging | ❌ Different system | High |
| **Audit Logging** | K8s audit policy file | CloudWatch Logs | Azure Monitor | Cloud Audit Logs | ⚠️ Different destination | Medium |
| **System Updates** | Manual (apt) | EKS-managed | AKS-managed | GKE-managed | ⚠️ Different | Low |
| **Node OS** | Ubuntu 20.04/22.04/24.04 | Amazon Linux 2/Bottlerocket | Ubuntu/Azure Linux | Container-Optimized OS | ⚠️ Different | Low |

## Conflict Resolution Guide

### 1. Storage Class Name Conflicts

**Problem**: Applications with hardcoded `storageClassName: mlstorage` will fail on cloud platforms.

**Resolution**:
```yaml
# BAD (ClusterBloom-only):
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
spec:
  storageClassName: mlstorage
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi

# GOOD (Platform-agnostic):
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-pvc
spec:
  storageClassName: {{ .Values.storage.defaultStorageClass }}
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

**Helm values.yaml**:
```yaml
# For ClusterBloom
storage:
  defaultStorageClass: "mlstorage"

# For EKS
storage:
  defaultStorageClass: "gp3"

# For AKS
storage:
  defaultStorageClass: "managed-csi"

# For GKE
storage:
  defaultStorageClass: "standard-rwo"
```

### 2. ReadWriteMany (RWX) Volume Conflicts

**Problem**: Longhorn supports RWX natively; cloud block storage (EBS, Azure Disk, PD) does NOT support RWX.

**Resolution**:

**Option A**: Use ReadWriteOnce (RWO) if possible
```yaml
# Change application to use RWO with single pod or StatefulSet
accessModes:
  - ReadWriteOnce
```

**Option B**: Use cloud file storage for RWX
```yaml
# ClusterBloom (Longhorn)
storageClassName: mlstorage
accessModes:
  - ReadWriteMany

# EKS (EFS)
storageClassName: efs-sc
accessModes:
  - ReadWriteMany
# Requires: EFS filesystem pre-created, EFS CSI driver installed

# AKS (Azure Files)
storageClassName: azurefile-csi
accessModes:
  - ReadWriteMany
# Note: Different performance characteristics (SMB-based)

# GKE (Filestore)
storageClassName: filestore-csi
accessModes:
  - ReadWriteMany
# Requires: Filestore instance created (min 1TB capacity)
```

**Migration Steps for RWX**:
1. Identify all PVCs with ReadWriteMany
2. Determine if truly needed (can it be RWO?)
3. If RWX required:
   - EKS: Create EFS filesystem, install EFS CSI driver, create storage class
   - AKS: Enable Azure Files CSI driver, create storage class
   - GKE: Create Filestore instance, install Filestore CSI driver
4. Test performance (file storage is typically slower than block)
5. Migrate data from Longhorn to cloud file storage

### 3. MetalLB Service Annotation Conflicts

**Problem**: Services with MetalLB annotations will not work on cloud platforms.

**Resolution**:
```yaml
# BAD (ClusterBloom-only):
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    metallb.universe.tf/address-pool: cluster-bloom-ip-pool
spec:
  type: LoadBalancer
  ports:
    - port: 80
      targetPort: 8080

# GOOD (Platform-agnostic with Helm):
apiVersion: v1
kind: Service
metadata:
  name: my-service
  annotations:
    {{- if eq .Values.platform.type "clusterbloom" }}
    metallb.universe.tf/address-pool: {{ .Values.loadBalancer.metallb.ipAddressPools[0].name }}
    {{- else if eq .Values.platform.type "eks" }}
    service.beta.kubernetes.io/aws-load-balancer-type: {{ .Values.loadBalancer.aws.serviceAnnotations.loadBalancerType }}
    service.beta.kubernetes.io/aws-load-balancer-scheme: {{ .Values.loadBalancer.aws.serviceAnnotations.scheme }}
    {{- else if eq .Values.platform.type "aks" }}
    service.beta.kubernetes.io/azure-load-balancer-internal: {{ .Values.loadBalancer.azure.serviceAnnotations.internal | quote }}
    {{- else if eq .Values.platform.type "gke" }}
    cloud.google.com/load-balancer-type: {{ .Values.loadBalancer.gcp.serviceAnnotations.loadBalancerType }}
    {{- end }}
spec:
  type: LoadBalancer
  ports:
    - port: 80
      targetPort: 8080
```

### 4. Ingress Class and Annotation Conflicts

**Problem**: Ingress resources with platform-specific classes and annotations won't work cross-platform.

**Resolution**:
```yaml
# BAD (Assumes NGINX on ClusterBloom):
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
spec:
  ingressClassName: nginx
  tls:
    - hosts:
        - example.com
      secretName: example-tls
  rules:
    - host: example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 80

# GOOD (Platform-agnostic):
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-ingress
  annotations:
    {{- if eq .Values.platform.type "clusterbloom" }}
    cert-manager.io/cluster-issuer: {{ .Values.certificates.certManager.defaultIssuer }}
    nginx.ingress.kubernetes.io/ssl-redirect: "true"
    {{- else if eq .Values.platform.type "eks" }}
    alb.ingress.kubernetes.io/scheme: {{ .Values.ingress.awsAlb.annotations.scheme }}
    alb.ingress.kubernetes.io/target-type: {{ .Values.ingress.awsAlb.annotations.targetType }}
    alb.ingress.kubernetes.io/certificate-arn: {{ .Values.ingress.awsAlb.annotations.certificateArn }}
    {{- else if eq .Values.platform.type "aks" }}
    appgw.ingress.kubernetes.io/ssl-redirect: "true"
    {{- else if eq .Values.platform.type "gke" }}
    ingress.gcp.kubernetes.io/pre-shared-cert: {{ .Values.ingress.gce.annotations.preSharedCertificate }}
    {{- end }}
spec:
  ingressClassName: {{ .Values.ingress.className }}
  {{- if eq .Values.platform.type "clusterbloom" }}
  tls:
    - hosts:
        - {{ .Values.domain.name }}
      secretName: {{ .Values.domain.tlsSecretName }}
  {{- end }}
  rules:
    - host: {{ .Values.domain.name }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-service
                port:
                  number: 80
```

### 5. Certificate Management Conflicts

**Problem**: cert-manager resources won't work on cloud platforms that use native certificate services.

**Resolution**:

**Option A**: Continue using cert-manager (works on all platforms)
```yaml
# Works on ClusterBloom, EKS, AKS, GKE
# Install cert-manager on the cluster
# Use HTTP-01 or DNS-01 challenge
```

**Option B**: Use cloud-native certificate management
```yaml
# EKS with ACM:
# 1. Create certificate in ACM
# 2. Reference by ARN in ALB Ingress annotation
alb.ingress.kubernetes.io/certificate-arn: arn:aws:acm:region:account:certificate/id

# AKS with Key Vault:
# 1. Store certificate in Azure Key Vault
# 2. Use akv2k8s or CSI driver to sync to secret
# 3. Reference secret in Ingress

# GKE with Google-managed certificates:
# 1. Create ManagedCertificate resource
# 2. Reference in Ingress annotation
apiVersion: networking.gke.io/v1
kind: ManagedCertificate
metadata:
  name: my-cert
spec:
  domains:
    - example.com
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    networking.gke.io/managed-certificates: my-cert
```

### 6. Network CIDR Conflicts

**Problem**: ClusterBloom uses 10.242.0.0/16 for pods and 10.243.0.0/16 for services, which may overlap with cloud VPC ranges.

**Resolution**:
1. Check cloud VPC CIDR before deployment
2. Ensure pod and service CIDRs don't overlap with VPC
3. Common cloud VPC ranges to avoid:
   - AWS VPCs: Often 10.0.0.0/16, 172.31.0.0/16, 192.168.0.0/16
   - Azure VNets: Often 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
   - GCP VPCs: Often 10.128.0.0/9

**Safe configuration**:
```yaml
# ClusterBloom (default, safe if not using 10.242.x or 10.243.x for VPC)
kubernetes:
  network:
    clusterCIDR: "10.242.0.0/16"
    serviceCIDR: "10.243.0.0/16"

# EKS (if VPC is 10.0.0.0/16)
kubernetes:
  network:
    clusterCIDR: "100.64.0.0/16"   # Non-overlapping
    serviceCIDR: "172.20.0.0/16"   # Non-overlapping

# AKS (if VNet is 10.0.0.0/16)
kubernetes:
  network:
    clusterCIDR: "10.244.0.0/16"   # Default AKS, non-overlapping
    serviceCIDR: "10.96.0.0/16"    # Default AKS, non-overlapping

# GKE (VPC-native)
kubernetes:
  network:
    clusterCIDR: "10.4.0.0/14"     # Secondary range in VPC
    serviceCIDR: "10.8.0.0/20"     # Secondary range in VPC
```

### 7. GPU Node Selection Conflicts

**Problem**: GPU node labels and resource names differ between bare-metal and cloud.

**Resolution**:
```yaml
# BAD (ClusterBloom-only):
apiVersion: v1
kind: Pod
metadata:
  name: gpu-pod
spec:
  nodeSelector:
    gpu: "true"
    amd.com/gpu: "true"
  containers:
    - name: gpu-container
      image: rocm/pytorch:latest
      resources:
        limits:
          amd.com/gpu: 1

# GOOD (Platform-agnostic):
apiVersion: v1
kind: Pod
metadata:
  name: gpu-pod
spec:
  nodeSelector:
    {{- if eq .Values.platform.type "clusterbloom" }}
    gpu: "true"
    {{- else if eq .Values.platform.type "eks" }}
    node.kubernetes.io/instance-type: {{ .Values.gpu.nodeSelector["node.kubernetes.io/instance-type"] }}
    {{- else if eq .Values.platform.type "aks" }}
    accelerator: {{ .Values.gpu.nodeSelector.accelerator }}
    {{- else if eq .Values.platform.type "gke" }}
    cloud.google.com/gke-accelerator: {{ .Values.gpu.nodeSelector["cloud.google.com/gke-accelerator"] }}
    {{- end }}
  {{- with .Values.gpu.tolerations }}
  tolerations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  containers:
    - name: gpu-container
      image: {{ .Values.gpu.containerImage }}
      resources:
        limits:
          {{ .Values.gpu.resourceName }}: 1
```

**Values configuration**:
```yaml
# ClusterBloom
gpu:
  enabled: true
  vendor: "amd"
  resourceName: "amd.com/gpu"
  containerImage: "rocm/pytorch:latest"
  nodeSelector:
    gpu: "true"
  tolerations: []

# EKS
gpu:
  enabled: true
  vendor: "amd"
  resourceName: "amd.com/gpu"
  containerImage: "rocm/pytorch:latest"
  nodeSelector:
    node.kubernetes.io/instance-type: "g4ad.xlarge"
  tolerations: []

# AKS
gpu:
  enabled: true
  vendor: "amd"
  resourceName: "amd.com/gpu"
  containerImage: "rocm/pytorch:latest"
  nodeSelector:
    accelerator: "amd-gpu"
  tolerations: []

# GKE
gpu:
  enabled: true
  vendor: "amd"
  resourceName: "amd.com/gpu"
  containerImage: "rocm/pytorch:latest"
  nodeSelector:
    cloud.google.com/gke-accelerator: "amd-gpu"
  tolerations: []
```

## Detailed Migration Checklist

### Pre-Migration Assessment

- [ ] **Inventory all PersistentVolumeClaims**
  - [ ] List all storage classes used
  - [ ] Identify RWX volumes (requires different storage in cloud)
  - [ ] Document volume sizes and access patterns
  - [ ] Calculate storage costs in cloud vs. Longhorn

- [ ] **Inventory all Services with type: LoadBalancer**
  - [ ] List all MetalLB annotations
  - [ ] Document required IP addresses
  - [ ] Identify internal vs. external services
  - [ ] Plan DNS updates

- [ ] **Inventory all Ingress resources**
  - [ ] List ingress controllers used
  - [ ] Document TLS certificates
  - [ ] Identify cert-manager usage
  - [ ] Map annotations to cloud equivalents

- [ ] **Identify GPU workloads**
  - [ ] List GPU resource requests
  - [ ] Document ROCm version requirements
  - [ ] Identify compatible cloud instance types
  - [ ] Plan driver compatibility testing

- [ ] **Document network dependencies**
  - [ ] List NetworkPolicy resources
  - [ ] Document Cilium-specific features
  - [ ] Identify service mesh usage
  - [ ] Plan network policy migration

### Storage Migration

#### ClusterBloom to EKS

- [ ] **Create EBS storage classes**
  ```bash
  kubectl apply -f - <<EOF
  apiVersion: storage.k8s.io/v1
  kind: StorageClass
  metadata:
    name: gp3
  provisioner: ebs.csi.aws.com
  parameters:
    type: gp3
    iops: "3000"
    throughput: "125"
    encrypted: "true"
  volumeBindingMode: WaitForFirstConsumer
  allowVolumeExpansion: true
  reclaimPolicy: Delete
  EOF
  ```

- [ ] **For RWX volumes: Set up EFS**
  ```bash
  # 1. Create EFS filesystem
  aws efs create-file-system \
    --region us-west-2 \
    --performance-mode generalPurpose \
    --tags Key=Name,Value=eks-efs
  
  # 2. Install EFS CSI driver
  kubectl apply -k "github.com/kubernetes-sigs/aws-efs-csi-driver/deploy/kubernetes/overlays/stable/?ref=release-1.7"
  
  # 3. Create storage class
  kubectl apply -f - <<EOF
  apiVersion: storage.k8s.io/v1
  kind: StorageClass
  metadata:
    name: efs-sc
  provisioner: efs.csi.aws.com
  parameters:
    provisioningMode: efs-ap
    fileSystemId: fs-xxxxxxxxx  # Replace with your EFS ID
    directoryPerms: "700"
  EOF
  ```

- [ ] **Migrate data from Longhorn to EBS**
  1. Create snapshot of Longhorn volume
  2. Create temporary pod with both volumes attached
  3. Copy data: `kubectl exec -it migration-pod -- rsync -av /old-mount/ /new-mount/`
  4. Verify data integrity
  5. Update application to use new PVC
  6. Delete old Longhorn volume

#### ClusterBloom to AKS

- [ ] **Create Azure Disk storage classes**
  ```bash
  kubectl apply -f - <<EOF
  apiVersion: storage.k8s.io/v1
  kind: StorageClass
  metadata:
    name: managed-csi
  provisioner: disk.csi.azure.com
  parameters:
    storageaccounttype: Premium_LRS
    kind: Managed
    cachingMode: ReadOnly
  volumeBindingMode: WaitForFirstConsumer
  allowVolumeExpansion: true
  reclaimPolicy: Delete
  EOF
  ```

- [ ] **For RWX volumes: Enable Azure Files CSI driver**
  ```bash
  # Azure Files CSI driver is pre-installed on AKS
  kubectl apply -f - <<EOF
  apiVersion: storage.k8s.io/v1
  kind: StorageClass
  metadata:
    name: azurefile-csi
  provisioner: file.csi.azure.com
  parameters:
    skuName: Standard_LRS
  volumeBindingMode: Immediate
  allowVolumeExpansion: true
  reclaimPolicy: Delete
  EOF
  ```

#### ClusterBloom to GKE

- [ ] **Create Persistent Disk storage classes**
  ```bash
  kubectl apply -f - <<EOF
  apiVersion: storage.k8s.io/v1
  kind: StorageClass
  metadata:
    name: standard-rwo
  provisioner: pd.csi.storage.gke.io
  parameters:
    type: pd-standard
    replication-type: none
  volumeBindingMode: WaitForFirstConsumer
  allowVolumeExpansion: true
  reclaimPolicy: Delete
  EOF
  ```

- [ ] **For RWX volumes: Set up Filestore**
  ```bash
  # 1. Create Filestore instance (via Console or gcloud)
  gcloud filestore instances create my-filestore \
    --zone=us-central1-a \
    --tier=BASIC_HDD \
    --file-share=name=vol1,capacity=1TB \
    --network=name=default
  
  # 2. Install Filestore CSI driver (if not pre-installed)
  kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gcp-filestore-csi-driver/master/deploy/kubernetes/overlays/stable/deployment.yaml
  
  # 3. Create storage class
  kubectl apply -f - <<EOF
  apiVersion: storage.k8s.io/v1
  kind: StorageClass
  metadata:
    name: filestore-csi
  provisioner: filestore.csi.storage.gke.io
  parameters:
    tier: BASIC_HDD
    network: default
  volumeBindingMode: Immediate
  allowVolumeExpansion: true
  EOF
  ```

### Load Balancer Migration

#### ClusterBloom to EKS

- [ ] **Install AWS Load Balancer Controller**
  ```bash
  # 1. Create IAM policy
  curl -o iam_policy.json https://raw.githubusercontent.com/kubernetes-sigs/aws-load-balancer-controller/v2.6.0/docs/install/iam_policy.json
  aws iam create-policy \
    --policy-name AWSLoadBalancerControllerIAMPolicy \
    --policy-document file://iam_policy.json
  
  # 2. Create service account
  eksctl create iamserviceaccount \
    --cluster=my-cluster \
    --namespace=kube-system \
    --name=aws-load-balancer-controller \
    --attach-policy-arn=arn:aws:iam::<ACCOUNT_ID>:policy/AWSLoadBalancerControllerIAMPolicy \
    --approve
  
  # 3. Install controller via Helm
  helm repo add eks https://aws.github.io/eks-charts
  helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
    -n kube-system \
    --set clusterName=my-cluster \
    --set serviceAccount.create=false \
    --set serviceAccount.name=aws-load-balancer-controller
  ```

- [ ] **Update Service annotations**
  ```yaml
  # Remove MetalLB annotations
  # metallb.universe.tf/address-pool: cluster-bloom-ip-pool
  
  # Add AWS annotations
  service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
  service.beta.kubernetes.io/aws-load-balancer-scheme: "internet-facing"
  service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled: "true"
  ```

- [ ] **Update DNS records**
  - Note: AWS LBs get DNS names (not IPs)
  - Create CNAME records pointing to LB DNS
  - Or use Route 53 Alias records

#### ClusterBloom to AKS

- [ ] **Update Service annotations**
  ```yaml
  # Remove MetalLB annotations
  # metallb.universe.tf/address-pool: cluster-bloom-ip-pool
  
  # Add Azure annotations (if needed)
  service.beta.kubernetes.io/azure-load-balancer-internal: "false"
  # Optional: Specify static IP
  # service.beta.kubernetes.io/azure-load-balancer-ipv4: "x.x.x.x"
  ```

- [ ] **Reserve static public IP (optional)**
  ```bash
  az network public-ip create \
    --resource-group MC_myResourceGroup_myAKSCluster_eastus \
    --name myPublicIP \
    --sku Standard \
    --allocation-method Static
  ```

#### ClusterBloom to GKE

- [ ] **Update Service annotations**
  ```yaml
  # Remove MetalLB annotations
  # metallb.universe.tf/address-pool: cluster-bloom-ip-pool
  
  # Add GCP annotations (if needed)
  cloud.google.com/load-balancer-type: "External"
  cloud.google.com/neg: '{"ingress": true}'
  ```

- [ ] **Reserve static external IP (optional)**
  ```bash
  gcloud compute addresses create my-static-ip --region=us-central1
  ```

### Ingress Migration

#### ClusterBloom to EKS (ALB Ingress)

- [ ] **Install AWS Load Balancer Controller** (same as above)

- [ ] **Update Ingress resources**
  ```yaml
  apiVersion: networking.k8s.io/v1
  kind: Ingress
  metadata:
    name: my-ingress
    annotations:
      # Change class
      # kubernetes.io/ingress.class: nginx
      alb.ingress.kubernetes.io/scheme: internet-facing
      alb.ingress.kubernetes.io/target-type: ip
      # Certificate via ACM
      alb.ingress.kubernetes.io/certificate-arn: arn:aws:acm:region:account:certificate/id
      alb.ingress.kubernetes.io/listen-ports: '[{"HTTPS":443}]'
      alb.ingress.kubernetes.io/ssl-redirect: '443'
  spec:
    ingressClassName: alb
    # Remove TLS section (handled by ALB)
    rules:
      - host: example.com
        http:
          paths:
            - path: /
              pathType: Prefix
              backend:
                service:
                  name: my-service
                  port:
                    number: 80
  ```

- [ ] **Create ACM certificate**
  ```bash
  aws acm request-certificate \
    --domain-name example.com \
    --validation-method DNS \
    --region us-west-2
  ```

#### ClusterBloom to AKS (Application Gateway)

- [ ] **Enable Application Gateway Ingress Controller**
  ```bash
  az aks enable-addons \
    --resource-group myResourceGroup \
    --name myAKSCluster \
    --addon ingress-appgw \
    --appgw-name myApplicationGateway
  ```

- [ ] **Update Ingress resources**
  ```yaml
  apiVersion: networking.k8s.io/v1
  kind: Ingress
  metadata:
    name: my-ingress
    annotations:
      appgw.ingress.kubernetes.io/ssl-redirect: "true"
      # Certificate from Key Vault
      appgw.ingress.kubernetes.io/appgw-ssl-certificate: "my-cert"
  spec:
    ingressClassName: azure-application-gateway
    tls:
      - hosts:
          - example.com
        secretName: example-tls  # Synced from Key Vault
    rules:
      - host: example.com
        http:
          paths:
            - path: /
              pathType: Prefix
              backend:
                service:
                  name: my-service
                  port:
                    number: 80
  ```

#### ClusterBloom to GKE (GCE Ingress)

- [ ] **Create ManagedCertificate resource**
  ```yaml
  apiVersion: networking.gke.io/v1
  kind: ManagedCertificate
  metadata:
    name: my-cert
  spec:
    domains:
      - example.com
  ```

- [ ] **Update Ingress resources**
  ```yaml
  apiVersion: networking.k8s.io/v1
  kind: Ingress
  metadata:
    name: my-ingress
    annotations:
      networking.gke.io/managed-certificates: my-cert
      kubernetes.io/ingress.global-static-ip-name: my-static-ip
  spec:
    ingressClassName: gce
    rules:
      - host: example.com
        http:
          paths:
            - path: /
              pathType: Prefix
              backend:
                service:
                  name: my-service
                  port:
                    number: 80
  ```

### Certificate Migration

#### Option 1: Continue using cert-manager (Recommended for portability)

- [ ] **Install cert-manager on cloud cluster**
  ```bash
  kubectl apply -f https://github.com/cert-manager/cert-manager/releases/download/v1.13.0/cert-manager.yaml
  ```

- [ ] **Create ClusterIssuer**
  ```yaml
  apiVersion: cert-manager.io/v1
  kind: ClusterIssuer
  metadata:
    name: letsencrypt-prod
  spec:
    acme:
      server: https://acme-v02.api.letsencrypt.org/directory
      email: admin@example.com
      privateKeySecretRef:
        name: letsencrypt-prod
      solvers:
        - http01:
            ingress:
              class: nginx  # or alb, azure-application-gateway, gce
  ```

- [ ] **No changes needed to Certificate resources**

#### Option 2: Migrate to cloud-native certificates

**For EKS (ACM)**:
- [ ] Create certificate in ACM
- [ ] Validate domain ownership (DNS or email)
- [ ] Reference in ALB Ingress annotation
- [ ] Remove cert-manager Certificate resources

**For AKS (Key Vault)**:
- [ ] Upload certificate to Azure Key Vault
- [ ] Install akv2k8s or use CSI driver to sync
- [ ] Reference secret in Ingress
- [ ] Remove cert-manager Certificate resources

**For GKE (Google-managed)**:
- [ ] Create ManagedCertificate resource
- [ ] Reference in Ingress annotation
- [ ] Remove cert-manager Certificate resources

### GPU Migration

#### ClusterBloom to EKS

- [ ] **Choose GPU instance type**
  - g4ad.xlarge (1x AMD Radeon Pro V520)
  - g4ad.2xlarge (1x AMD Radeon Pro V520)
  - g4ad.4xlarge (1x AMD Radeon Pro V520)
  - g4ad.8xlarge (2x AMD Radeon Pro V520)
  - g4ad.16xlarge (4x AMD Radeon Pro V520)

- [ ] **Create GPU node group**
  ```bash
  eksctl create nodegroup \
    --cluster my-cluster \
    --name gpu-nodes \
    --node-type g4ad.xlarge \
    --nodes 2 \
    --nodes-min 1 \
    --nodes-max 4 \
    --node-ami-family Ubuntu2004 \
    --node-labels gpu=true
  ```

- [ ] **Install AMD GPU device plugin**
  ```bash
  kubectl apply -f https://raw.githubusercontent.com/RadeonOpenCompute/k8s-device-plugin/master/k8s-ds-amdgpu-dp.yaml
  ```

- [ ] **Update pod specifications**
  ```yaml
  # Change node selector
  nodeSelector:
    # gpu: "true"  # ClusterBloom
    node.kubernetes.io/instance-type: g4ad.xlarge  # EKS
  
  # Resource name stays the same
  resources:
    limits:
      amd.com/gpu: 1
  ```

#### ClusterBloom to AKS

- [ ] **Choose GPU VM size**
  - Standard_NC4as_T4_v3 (1x AMD MI25)
  - Standard_NC8as_T4_v3 (1x AMD MI25)
  - Standard_NC16as_T4_v3 (1x AMD MI25)
  - Standard_NC64as_T4_v3 (4x AMD MI25)

- [ ] **Create GPU node pool**
  ```bash
  az aks nodepool add \
    --resource-group myResourceGroup \
    --cluster-name myAKSCluster \
    --name gpunodepool \
    --node-count 2 \
    --node-vm-size Standard_NC4as_T4_v3 \
    --node-taints sku=gpu:NoSchedule \
    --labels gpu=true
  ```

- [ ] **GPU drivers and device plugin are pre-installed on AKS**

- [ ] **Update pod specifications**
  ```yaml
  nodeSelector:
    # gpu: "true"  # ClusterBloom
    accelerator: amd-gpu  # AKS
  
  tolerations:
    - key: sku
      value: gpu
      effect: NoSchedule
  
  resources:
    limits:
      amd.com/gpu: 1
  ```

#### ClusterBloom to GKE

- [ ] **Choose GPU machine type**
  - a2-highgpu-1g (1x AMD MI250X)
  - a2-highgpu-2g (2x AMD MI250X)
  - a2-highgpu-4g (4x AMD MI250X)
  - a2-highgpu-8g (8x AMD MI250X)

- [ ] **Create GPU node pool**
  ```bash
  gcloud container node-pools create gpu-pool \
    --cluster=my-cluster \
    --zone=us-central1-a \
    --machine-type=a2-highgpu-1g \
    --num-nodes=2 \
    --accelerator=type=amd-gpu,count=1 \
    --node-labels=gpu=true
  ```

- [ ] **Install AMD GPU driver DaemonSet**
  ```bash
  kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/container-engine-accelerators/master/nvidia-driver-installer/cos/daemonset-amd.yaml
  ```

- [ ] **Update pod specifications**
  ```yaml
  nodeSelector:
    # gpu: "true"  # ClusterBloom
    cloud.google.com/gke-accelerator: amd-gpu  # GKE
  
  resources:
    limits:
      amd.com/gpu: 1
  ```

### Network Policy Migration

#### ClusterBloom (Cilium) to EKS

- [ ] **Install Calico for NetworkPolicy support**
  ```bash
  kubectl apply -f https://raw.githubusercontent.com/aws/amazon-vpc-cni-k8s/v1.15.0/config/master/calico-operator.yaml
  kubectl apply -f https://raw.githubusercontent.com/aws/amazon-vpc-cni-k8s/v1.15.0/config/master/calico-crs.yaml
  ```

- [ ] **Test NetworkPolicy resources**
  - Most standard NetworkPolicy resources should work
  - Cilium-specific CRDs (CiliumNetworkPolicy) won't work; convert to standard NetworkPolicy

#### ClusterBloom (Cilium) to AKS

- [ ] **Enable Azure Network Policy Manager or Calico**
  ```bash
  # During cluster creation:
  az aks create \
    --network-policy azure  # or calico
  
  # Or enable on existing cluster:
  az aks update \
    --resource-group myResourceGroup \
    --name myAKSCluster \
    --network-policy azure
  ```

- [ ] **Test NetworkPolicy resources**

#### ClusterBloom (Cilium) to GKE

- [ ] **Enable Network Policy enforcement** (enabled by default on GKE Autopilot)
  ```bash
  gcloud container clusters update my-cluster \
    --enable-network-policy \
    --zone=us-central1-a
  ```

- [ ] **Test NetworkPolicy resources**

### Monitoring and Observability Migration

#### Cilium Hubble to Cloud Native

**ClusterBloom (Hubble)**:
- Network flow visualization
- Service dependency map
- L7 metrics

**Migration to**:

**EKS**: VPC Flow Logs + AWS X-Ray
```bash
# Enable VPC Flow Logs
aws ec2 create-flow-logs \
  --resource-type VPC \
  --resource-ids vpc-xxxxx \
  --traffic-type ALL \
  --log-destination-type s3 \
  --log-destination arn:aws:s3:::my-flow-logs-bucket

# Install X-Ray daemon
kubectl apply -f https://eksworkshop.com/intermediate/245_x-ray/daemonset.files/xray-k8s-daemonset.yaml
```

**AKS**: NSG Flow Logs + Azure Monitor
```bash
# Enable NSG Flow Logs via Azure Portal or CLI
az network watcher flow-log create \
  --name MyFlowLog \
  --nsg MyNetworkSecurityGroup \
  --storage-account mystorageaccount \
  --resource-group myResourceGroup \
  --location eastus

# Enable Azure Monitor for containers
az aks enable-addons \
  --resource-group myResourceGroup \
  --name myAKSCluster \
  --addons monitoring
```

**GKE**: VPC Flow Logs + Cloud Trace
```bash
# Enable VPC Flow Logs (via Console or gcloud)
gcloud compute networks subnets update my-subnet \
  --region=us-central1 \
  --enable-flow-logs

# Cloud Trace is integrated with GKE by default
```

## Platform-Specific Configurations

### Complete EKS Configuration

```yaml
# cloud-platform-values.yaml for EKS
platform:
  type: "eks"
  cloud:
    provider: "aws"
    region: "us-west-2"

kubernetes:
  distribution: "eks"
  network:
    clusterCIDR: "10.0.0.0/16"  # Ensure no VPC overlap
    serviceCIDR: "172.20.0.0/16"
    cni:
      provider: "aws-vpc-cni"
      awsVpcCni:
        enabled: true
        version: "v1.15.0"
        enablePodEni: false
        securityGroupsForPods: false
  networkPolicy:
    enabled: true
    provider: "calico"  # Requires Calico add-on

storage:
  defaultStorageClass: "gp3"
  provisioner: "ebs.csi.aws.com"
  classes:
    - name: "gp3"
      provisioner: "ebs.csi.aws.com"
      parameters:
        type: "gp3"
        iops: "3000"
        throughput: "125"
        encrypted: "true"
    - name: "efs-sc"  # For RWX
      provisioner: "efs.csi.aws.com"
      parameters:
        provisioningMode: "efs-ap"
        fileSystemId: "fs-xxxxx"  # Replace

loadBalancer:
  type: "aws-nlb"
  metallb:
    enabled: false
  aws:
    enabled: true
    serviceAnnotations:
      loadBalancerType: "nlb"
      scheme: "internet-facing"
      crossZoneLoadBalancing: "true"

ingress:
  controller: "aws-alb"
  className: "alb"
  nginx:
    enabled: false
  awsAlb:
    enabled: true
    annotations:
      scheme: "internet-facing"
      targetType: "ip"
      certificateArn: "arn:aws:acm:us-west-2:xxxxx:certificate/xxxxx"

certificates:
  provider: "aws-acm"
  certManager:
    enabled: false
  awsAcm:
    enabled: true
    certificateArn: "arn:aws:acm:us-west-2:xxxxx:certificate/xxxxx"

gpu:
  enabled: true
  vendor: "amd"
  resourceName: "amd.com/gpu"
  nodeSelector:
    node.kubernetes.io/instance-type: "g4ad.xlarge"
```

### Complete AKS Configuration

```yaml
# cloud-platform-values.yaml for AKS
platform:
  type: "aks"
  cloud:
    provider: "azure"
    region: "eastus"

kubernetes:
  distribution: "aks"
  network:
    clusterCIDR: "10.244.0.0/16"
    serviceCIDR: "10.96.0.0/16"
    cni:
      provider: "azure-cni"
      azureCni:
        enabled: true
        networkPlugin: "azure"
        networkPolicy: "azure"

storage:
  defaultStorageClass: "managed-csi"
  provisioner: "disk.csi.azure.com"
  classes:
    - name: "managed-csi"
      provisioner: "disk.csi.azure.com"
      parameters:
        storageaccounttype: "Premium_LRS"
        kind: "Managed"
    - name: "azurefile-csi"  # For RWX
      provisioner: "file.csi.azure.com"
      parameters:
        skuName: "Standard_LRS"

loadBalancer:
  type: "azure-lb"
  metallb:
    enabled: false
  azure:
    enabled: true
    serviceAnnotations:
      internal: "false"

ingress:
  controller: "azure-appgw"
  className: "azure-application-gateway"
  nginx:
    enabled: false
  azureAppGw:
    enabled: true
    annotations:
      applicationGatewayName: "myAppGateway"
      resourceGroup: "myResourceGroup"

certificates:
  provider: "azure-keyvault"
  certManager:
    enabled: false
  azureKeyVault:
    enabled: true
    name: "myKeyVault"
    tenantId: "xxxxx"
    certificateName: "my-cert"

gpu:
  enabled: true
  vendor: "amd"
  resourceName: "amd.com/gpu"
  nodeSelector:
    accelerator: "amd-gpu"
  tolerations:
    - key: "sku"
      value: "gpu"
      effect: "NoSchedule"
```

### Complete GKE Configuration

```yaml
# cloud-platform-values.yaml for GKE
platform:
  type: "gke"
  cloud:
    provider: "gcp"
    region: "us-central1"

kubernetes:
  distribution: "gke"
  network:
    clusterCIDR: "10.4.0.0/14"
    serviceCIDR: "10.8.0.0/20"
    cni:
      provider: "gke-cni"
      gkeCni:
        enabled: true
  networkPolicy:
    enabled: true
    provider: "gke"  # GKE native

storage:
  defaultStorageClass: "standard-rwo"
  provisioner: "pd.csi.storage.gke.io"
  classes:
    - name: "standard-rwo"
      provisioner: "pd.csi.storage.gke.io"
      parameters:
        type: "pd-standard"
    - name: "filestore-csi"  # For RWX
      provisioner: "filestore.csi.storage.gke.io"
      parameters:
        tier: "BASIC_HDD"
        network: "default"

loadBalancer:
  type: "gce-lb"
  metallb:
    enabled: false
  gcp:
    enabled: true
    serviceAnnotations:
      loadBalancerType: "External"
      networkTier: "PREMIUM"

ingress:
  controller: "gce"
  className: "gce"
  nginx:
    enabled: false
  gce:
    enabled: true
    annotations:
      staticIpName: "my-static-ip"

certificates:
  provider: "gcp-managed"
  certManager:
    enabled: false
  gcpManaged:
    enabled: true
    certificateName: "my-cert"
    domains:
      - "example.com"

gpu:
  enabled: true
  vendor: "amd"
  resourceName: "amd.com/gpu"
  nodeSelector:
    cloud.google.com/gke-accelerator: "amd-gpu"
```

## Common Migration Pitfalls

### 1. Forgetting to Update Storage Class Names

**Problem**: Application fails to create PVCs because "mlstorage" doesn't exist.

**Solution**: Use Helm values or environment variables for storage class names.

### 2. RWX Volumes Not Working

**Problem**: PVC stuck in Pending state because cloud block storage doesn't support RWX.

**Solution**: 
- Change to RWO if possible
- Use cloud file storage (EFS, Azure Files, Filestore) for RWX

### 3. Load Balancer Gets External IP Instead of Expected IP

**Problem**: MetalLB gave specific node IP; cloud LB gives random public IP.

**Solution**:
- Reserve static IP in cloud
- Update DNS to point to cloud LB
- Use cloud DNS integration (Route 53, Azure DNS, Cloud DNS)

### 4. Ingress SSL/TLS Not Working

**Problem**: cert-manager Certificate not creating secret, or ALB not using certificate.

**Solution**:
- For cert-manager: Verify HTTP-01 or DNS-01 challenge is working
- For cloud certificates: Verify certificate is created and ARN/reference is correct
- Check ingress annotations match controller type

### 5. GPU Pods Not Scheduling

**Problem**: Pods stuck in Pending with "Insufficient amd.com/gpu" message.

**Solution**:
- Verify GPU nodes exist and are labeled correctly
- Check GPU device plugin is running
- Verify tolerations if GPU nodes are tainted
- Check node selector matches cloud node labels

### 6. Network Policies Not Enforced

**Problem**: NetworkPolicy resources exist but not enforced.

**Solution**:
- Verify network policy controller is enabled (Calico for EKS, Azure NPM for AKS, etc.)
- Convert Cilium-specific policies to standard NetworkPolicy
- Test with simple deny-all policy first

### 7. Service Stuck in Pending (LoadBalancer)

**Problem**: Service type LoadBalancer never gets an external IP.

**Solution**:
- Verify load balancer controller is installed (AWS LB Controller for EKS)
- Check service annotations are correct for platform
- Verify cloud account has permissions to create load balancers
- Check cloud quotas (IP addresses, load balancers)

### 8. High Storage Costs

**Problem**: Cloud storage is much more expensive than expected.

**Solution**:
- Right-size storage requests (don't over-provision)
- Use appropriate storage tier (Standard vs Premium)
- Consider block storage cost vs file storage cost
- Set up automated cleanup of unused PVs
- Use volume snapshots instead of keeping multiple copies

### 9. Cross-AZ Data Transfer Costs

**Problem**: Unexpected network charges from cross-AZ traffic.

**Solution**:
- Use pod topology spread constraints to keep pods in same AZ
- Consider volumeBindingMode: WaitForFirstConsumer to create volumes in pod's AZ
- For EKS: Use regional EBS if cross-AZ replication is needed
- Monitor data transfer metrics

### 10. Incompatible Kubernetes Versions

**Problem**: Application uses features not available in cloud K8s version.

**Solution**:
- Check cloud provider's supported K8s versions
- Test application compatibility before migration
- Update deprecated API usage
- Plan for version upgrades

---

## Summary

This guide provides comprehensive coverage of:
- ✅ All infrastructure components and their cloud equivalents
- ✅ All conflicts between ClusterBloom and cloud platforms
- ✅ Detailed migration checklist for each component
- ✅ Platform-specific configurations for EKS, AKS, and GKE
- ✅ Common pitfalls and their solutions

Use this guide in conjunction with `cloud-platform-values.yaml` for successful migration from ClusterBloom to managed Kubernetes platforms.
