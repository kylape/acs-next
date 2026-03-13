# Deployment Profiles

*Part of [ACS Next Architecture](./)*

---

ACS Next supports flexible deployment profiles. The decoupled architecture allows components to run where they make sense — all on-cluster, split between cluster and hub, or minimal on-cluster with everything else centralized.

## Standalone Cluster

Single cluster deployment without ACM. All components run on-cluster.

```mermaid
graph TB
    subgraph cluster["Secured Cluster"]
        collector["Collector<br/>(DaemonSet)"]
        admission["Admission<br/>Controller"]
        broker["Broker"]
        scanner["Scanner"]
        projector["CRD Projector"]
        notifiers["Notifiers"]

        collector --> broker
        admission --> broker
        scanner --> broker
        broker --> projector
        broker --> notifiers
    end

    projector --> console["OCP Console"]
    notifiers --> external["SIEM / Slack / Jira"]
```

## ACM-Managed Cluster

Cluster managed by ACM hub. Core components on-cluster, fleet queries on hub.

```mermaid
graph TB
    subgraph cluster["Managed Cluster"]
        collector["Collector<br/>(DaemonSet)"]
        admission["Admission<br/>Controller"]
        broker["Broker"]
        scanner["Scanner"]
        projector["CRD Projector<br/>(optional)"]

        collector --> broker
        admission --> broker
        scanner --> broker
        broker --> projector
    end

    subgraph hub["ACM Hub"]
        vulnmgmt["Vuln Management<br/>Service"]
        notifiers["Notifiers"]
    end

    broker -.->|NATS leaf| vulnmgmt
    vulnmgmt --> notifiers
```

## Edge Cluster (Minimal)

Resource-constrained edge cluster. Only data collection on-cluster; processing on hub.

```mermaid
graph TB
    subgraph edge["Edge Cluster (minimal)"]
        collector["Collector<br/>(DaemonSet)"]
        broker["Broker"]

        collector --> broker
    end

    subgraph hub["Hub Cluster"]
        scanner["Scanner"]
        baselines["Baselines"]
        risk["Risk Scorer"]
        vulnmgmt["Vuln Management<br/>Service"]
        notifiers["Notifiers"]
    end

    broker -.->|NATS leaf| scanner
    broker -.->|NATS leaf| baselines
    broker -.->|NATS leaf| risk
    broker -.->|NATS leaf| vulnmgmt
    vulnmgmt --> notifiers
```

## Component Placement

| Component | On Secured Cluster | On Hub | Notes |
|-----------|-------------------|--------|-------|
| Collector | Required | - | Must run where workloads run |
| Admission Control | Required | - | Must intercept local API calls |
| Broker | Required | - | Aggregates local events |
| Scanner (indexer) | ✓ | ✓ | Can split indexer/matcher |
| Scanner (matcher) | ✓ | ✓ | Heavy; often better on hub |
| Risk Scorer | ✓ | ✓ | Can run either place |
| Baselines | ✓ | ✓ | Can run either place |
| CRD Projector | ✓ | - | Enables local OCP Console visibility |
| Vuln Management Service | ✓ | ✓ | Fleet queries; typically on hub |
