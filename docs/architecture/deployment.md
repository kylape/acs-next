# Deployment

*Part of [ACS Next Architecture](./)*

---

## Deployment Profiles

Different users have different needs. ACS Next supports multiple deployment profiles.

*Note: Resource estimates are preliminary and require validation via prototyping.*

### Component Resource Estimates

*Note: Estimates based on current ACS resource requests. Actual values require validation.*

| Component | Type | Memory (est.) | CPU (est.) | Notes |
|-----------|------|---------------|------------|-------|
| Broker | Deployment | ~100-200MB | 100-500m | Depends on message volume |
| Collector | DaemonSet | ~500-750MB **per node** | 100-500m | eBPF, current ACS: 700Mi |
| Scanner (full) | Deployment | ~1-2GB | 500m-2 | Image analysis; current ACS: 1.5Gi |
| Scanner (indexer only) | Deployment | ~500MB-1GB | 250m-1 | Reduced if matcher on hub |
| Admission Control | Deployment | ~100-200MB | 100-250m | Webhook, HA recommended |
| Audit Logs | DaemonSet | ~100MB **per node** | 50-100m | Log shipping |
| CRD Projector | Deployment | ~100-200MB | 50-100m | CR transformations |
| Notifiers | Deployment | ~100-200MB | 50-100m | AlertManager, Jira, Slack, SIEM |
| Risk Scorer | Deployment | ~200-500MB | 100-500m | Depends on cluster size |
| Baselines | Deployment | ~200-500MB | 100-500m | ML/statistical models |

### Profile: Single Cluster — Standalone (no ACM)

```
Core:       Collector + Scanner + Admission Controller + Broker
            + Runtime Evaluator + CRD Projector
Add-ons:    Notifiers, Risk Scorer, Baselines
Storage:    Broker PVC (~150-300 MB), Scanner DB (existing)
Custom API: None
```

### Profile: Single Cluster — ACM-Managed

```
Core:       Collector + Scanner + Admission Controller + Broker
            + Runtime Evaluator
Add-ons:    CRD Projector (for local Console visibility), Notifiers,
            Risk Scorer, Baselines
Storage:    Broker PVC
Custom API: None (hub provides fleet queries)
```

### Profile: Hub (Multi-Cluster Addon)

```
Core:       Vuln Management Service
Add-ons:    Notifiers, Risk Scorer (fleet-level)
Storage:    SQLite on PVC (small fleets)
            or PostgreSQL BYODB (large fleets)
Custom API: Vuln Management Service query API (cluster-scoped RBAC)
```

### Profile: Edge (minimal on-cluster)

```
Core:       Broker + Collector only
On hub:     Scanner, Risk Scorer, Baselines, Notifiers
Footprint:  ~600-950MB cluster-wide + ~500-750MB per node
Storage:    None
Use case:   Resource-constrained edge clusters
```

---

## Flexible Deployment Topologies

The decoupled architecture enables components to run across cluster boundaries. This is especially valuable for edge clusters with constrained resources.

### Edge Cluster Pattern

For resource-constrained edge clusters, only the minimum data collection runs on-cluster:

```
┌─────────────────────────────────┐
│     Edge Cluster (minimal)      │
│  ┌───────────┐  ┌───────────┐   │
│  │ Collector │  │  Broker   │   │
│  │  (eBPF)   │  │ (streams) │   │
│  └───────────┘  └─────┬─────┘   │
└───────────────────────┼─────────┘
                        │ ACM transport
                        ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Hub Cluster (full stack)                     │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐               │
│  │ Scanner │ │  Risk   │ │Baselines│ │Alerting │               │
│  │(matcher)│ │ Scorer  │ │         │ │ Service │               │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘               │
└─────────────────────────────────────────────────────────────────┘
```

**Edge cluster footprint:** ~150-200MB (Collector + Broker only)

### Component Placement Options

| Component | On Secured Cluster | On Hub | Notes |
|-----------|-------------------|--------|-------|
| Collector | Required | - | Must run where workloads run |
| Admission Control | Required | - | Must intercept local API calls |
| Broker | Required | - | Aggregates local events |
| Scanner (indexer) | ✓ | ✓ | Can split indexer/matcher |
| Scanner (matcher) | ✓ | ✓ (preferred) | Heavy; often better on hub |
| Risk Scorer | ✓ | ✓ | Can run either place |
| Baselines | ✓ | ✓ | Can run either place |
| CRD Projector | ✓ | - | Enables local OCP Console visibility |

---

## Installation and Operator

A single operator manages installation and configuration of all ACS Next components.

**Approach:**
* Top-level CR (e.g., `ACSSecuredCluster`) defines desired state
* Operator creates component-specific CRs as needed (e.g., `Collector`, `Scanner`, `Broker`)
* Component-specific CRs allow fine-grained configuration
* Operator handles upgrades, scaling, and lifecycle

**Example:**

```yaml
apiVersion: acs.openshift.io/v1
kind: ACSSecuredCluster
metadata:
  name: secured-cluster
spec:
  profile: standalone  # or acm-managed, hub, edge
  collector:
    enabled: true
  scanner:
    mode: hub-matcher  # local, hub-matcher, or hub-full
```

The operator creates the necessary component CRs and manages their lifecycle.
