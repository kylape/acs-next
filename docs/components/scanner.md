# Scanner Architecture

*Part of [ACS Next Architecture](../architecture.md)*

---

The Scanner consists of two distinct components that can be deployed independently:

## Components

**Indexer**
* Pulls container images from registries
* Extracts installed packages (RPM, APK, DEB, language packages)
* Generates SBOM (Software Bill of Materials)
* Outputs: Image index (package inventory)
* **Requires**: Network access to image registries

**Matcher**
* Takes image indexes as input
* Matches packages against the vulnerability database
* Produces vulnerability reports (CVEs, severity, fixability)
* **Requires**: Access to vulnerability database (bundled or fetched)

```
┌─────────────────────────────────────────────────────────────────┐
│                        Scanner Flow                              │
│                                                                  │
│  Image Registry                                                  │
│       │                                                          │
│       │ pull image layers                                        │
│       ▼                                                          │
│  ┌─────────┐    image index    ┌─────────┐    vuln report       │
│  │ Indexer │ ───────────────►  │ Matcher │ ───────────────►     │
│  └─────────┘                   └─────────┘                       │
│                                     │                            │
│                                     │ queries                    │
│                                     ▼                            │
│                              ┌───────────┐                       │
│                              │ Vuln DB   │                       │
│                              └───────────┘                       │
└─────────────────────────────────────────────────────────────────┘
```

## Deployment Topologies

Each component can be deployed on the spoke cluster, the hub, or a combination — depending on customer constraints.

| Topology | Indexer | Matcher | Use Case |
|----------|---------|---------|----------|
| **Local (full)** | Spoke | Spoke | Air-gapped clusters, low latency requirements |
| **Split** | Spoke | Hub | Resource-constrained spokes, centralized vuln DB management |
| **Delegated** | Hub | Hub | Minimal spoke footprint, hub has registry access |

### Topology 1: Local (Full Scanner on Spoke)

```
┌─────────────────────────────────────────┐
│            Spoke Cluster                 │
│                                          │
│  Registry ──► Indexer ──► Matcher        │
│                              │           │
│                           Vuln DB        │
│                              │           │
│                              ▼           │
│                           Broker         │
└─────────────────────────────────────────┘
```

* **When to use**:
  * Air-gapped or disconnected clusters
  * Strict data locality requirements
  * Low-latency scanning needed
* **Trade-offs**:
  * Higher resource consumption on spoke (~2-4 GB RAM for matcher + vuln DB)
  * Vuln DB updates must reach each cluster

### Topology 2: Split (Indexer on Spoke, Matcher on Hub)

```
┌───────────────────────┐         ┌───────────────────────────────┐
│     Spoke Cluster     │         │           ACM Hub              │
│                       │         │                                │
│  Registry ──► Indexer │         │  ┌─────────────────────────┐  │
│                  │    │         │  │   Shared Matcher         │  │
│                  │    │  image  │  │   (serves all spokes)    │  │
│                  └────┼─────────┼─►│                          │  │
│                       │  index  │  │   Vuln DB (centralized)  │  │
│                       │         │  └───────────┬──────────────┘  │
│                       │         │              │                 │
│             Broker ◄──┼─────────┼──────────────┘                │
│                       │  vuln   │         vuln report           │
│                       │  report │                                │
└───────────────────────┘         └───────────────────────────────┘
```

* **When to use**:
  * Spoke clusters have registry access but limited resources
  * Centralized vulnerability database management preferred
  * Hub has good connectivity to spokes
* **Trade-offs**:
  * Lower spoke footprint (~200-500 MB for indexer only)
  * Single vuln DB to update (on hub)
  * Requires spoke-to-hub connectivity for matching
  * Image layers stay on spoke (only index sent to hub)

### Topology 3: Delegated (Full Scanner on Hub)

```
┌───────────────────────┐         ┌───────────────────────────────┐
│     Spoke Cluster     │         │           ACM Hub              │
│                       │         │                                │
│  (no scanner)         │         │  Registry ──► Indexer          │
│                       │         │                  │              │
│                       │  scan   │                  ▼              │
│  Admission ───────────┼─────────┼──────────►   Matcher           │
│  Controller           │  request│                  │              │
│                       │         │               Vuln DB          │
│             Broker ◄──┼─────────┼──────────────────┘             │
│                       │  vuln   │                                │
│                       │  report │                                │
└───────────────────────┘         └───────────────────────────────┘
```

* **When to use**:
  * Spoke clusters cannot reach image registries (hub has access)
  * Minimal spoke footprint required
  * Centralized scanning infrastructure preferred
* **Trade-offs**:
  * Lowest spoke resource usage (no scanner components)
  * Hub must have network access to all image registries
  * Higher hub resource requirements (scales with fleet size)
  * Scan latency depends on hub connectivity

## Deployment Decision Matrix

| Constraint | Recommended Topology |
|------------|---------------------|
| Air-gapped spoke clusters | Local |
| Spoke cannot reach registries, hub can | Delegated |
| Spoke has <4GB RAM available for security | Split or Delegated |
| Strict data locality (images can't leave cluster) | Local |
| Want single vuln DB to manage | Split or Delegated |
| Hub has limited connectivity to spokes | Local |
| Mixed constraints across fleet | Mix topologies per cluster |

## Configuration

Scanner topology is configured per-cluster via the `ScannerConfiguration` CRD:

```yaml
apiVersion: acs.openshift.io/v1
kind: ScannerConfiguration
metadata:
  name: scanner-config
  namespace: acs-next
spec:
  # Topology: "local", "split", or "delegated"
  topology: split

  indexer:
    # Only applies to "local" and "split" topologies
    resources:
      requests:
        memory: "500Mi"
        cpu: "200m"
      limits:
        memory: "2Gi"
        cpu: "2"
    registryAccess:
      # Pull secrets for image registries
      imagePullSecrets:
        - name: registry-credentials

  matcher:
    # Only applies to "local" topology
    # For "split"/"delegated", matcher runs on hub
    resources:
      requests:
        memory: "2Gi"
        cpu: "500m"
      limits:
        memory: "4Gi"
        cpu: "4"
    vulnDatabase:
      # "bundled" (offline) or "online" (fetch updates)
      mode: online
      updateInterval: 4h

  hub:
    # For "split" and "delegated" topologies
    # How to reach the hub's matcher service
    endpoint: "https://scanner.acm-hub.svc:8443"
    # mTLS client cert for hub communication
    tlsSecretRef:
      name: hub-scanner-client-tls
```

## Fleet-Level Scanner Management

For multi-cluster deployments, ACM Governance distributes `ScannerConfiguration` CRDs:

```yaml
apiVersion: policy.open-cluster-management.io/v1
kind: Policy
metadata:
  name: scanner-topology-policy
spec:
  remediationAction: enforce
  policy-templates:
    - objectDefinition:
        apiVersion: acs.openshift.io/v1
        kind: ScannerConfiguration
        metadata:
          name: scanner-config
        spec:
          topology: split  # Default for most clusters
          # ...
```

Clusters can be grouped by topology requirements using placement rules — e.g., air-gapped clusters get `topology: local`, resource-constrained edge clusters get `topology: delegated`.
