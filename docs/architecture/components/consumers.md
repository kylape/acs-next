# Consumers

*Part of [ACS Next Architecture](../)*

---

Consumers subscribe to broker feeds and perform actions. Deploy the consumers that fit your needs — a minimal deployment might use only Notifiers, while a full deployment runs all of them.

## Vuln Management Service

* **What it does**: Fleet-wide vulnerability authority — aggregates scan results across clusters, provides query API
* **Subscribes to**: `image-scans`, `vulnerabilities` via NATS leaf nodes
* **Outputs**: Fleet-wide query API, scheduled reports, OCP Console multi-cluster views
* **Deployment**: Typically on ACM hub for fleet-wide queries; can also run per-cluster
* **Notes**: See [Vuln Management Service](vuln-management.md) for full design

## CRD Projector

* **What it does**: Projects **summary-level** security data into Kubernetes CRs
* **Subscribes to**: `policy-violations`, `image-scans`
* **Outputs**: `PolicyViolation`, `ImageScanSummary` CRs (summary-level only)
* **Use case**: Local OCP Console visibility, K8s RBAC for security data
* **Key design**: OCP Console is powered by these CRs — no DB required for basic visibility

**Important:** The CRD Projector writes summary CRs only — not raw vulnerability
data. Full CVE-level data does **not** become CRs. When a user drills into a
specific image's vulnerabilities in the Console, the Console plugin calls the
Scanner directly for the full vulnerability report. This keeps CR counts
manageable (hundreds to low thousands) while still providing drill-down
capability.

**Example CR:**

```yaml
apiVersion: security.openshift.io/v1
kind: PolicyViolation
metadata:
  name: deployment-nginx-privileged
  namespace: production
  labels:
    policy: no-privileged-containers
    severity: high
spec:
  policy: no-privileged-containers
  resource:
    kind: Deployment
    name: nginx
    namespace: production
  violation:
    message: "Container 'nginx' is running as privileged"
    timestamp: "2026-02-27T10:30:00Z"
status:
  state: Active
```

## Notifiers

* **What it does**: Sends notifications to external systems
* **Subscribes to**: `policy-violations`, `vulnerabilities` (configurable)
* **Outputs**: AlertManager, Jira tickets, Splunk events, Slack messages, AWS Security Hub, etc.
* **Notes**: AlertManager is one notifier type among many. Also serves as the event
  history mechanism — pushes security events to customer SIEM for incident response
  queries (see [Data Architecture](../data-architecture.md))

## Risk Scorer

* **What it does**: Calculates composite risk scores for workloads
* **Subscribes to**: Broker feeds (`vulnerabilities`, `policy-violations`, `runtime-events`)
* **Outputs**: Risk scores (publishes back to broker for other consumers)
* **Use case**: Prioritization dashboards, configurable risk weighting based on business context
* **Notes**: Designed for configurability — users adjust weights, factor in business context

**Data sources:**

```mermaid
graph TB
    RT["runtime-events"] --> Risk["Risk Scorer"]
    Vulns["vulnerabilities<br/>policy-violations"] --> Risk
    Ext["External context<br/>(business, asset importance)"] --> Risk
    Risk --> Scores["risk-scores topic"]
```

## Scan Orchestrator

* **What it does**: Coordinates vulnerability scanning — receives index data, requests scans, publishes results
* **Subscribes to**: `node-index`, `image-index`
* **Calls**: Scanner (matcher) API to perform vulnerability matching
* **Outputs**: Publishes `vulnerabilities` to broker
* **Use case**: Decouples data collection (indexers) from vulnerability matching (scanner)
* **Notes**: Replaces the coordination role that Central plays today between data sources and Scanner

```mermaid
graph LR
    subgraph Indexers
        NI[Node Indexer]
        II[Image Indexer]
    end

    NI -->|node-index| Broker
    II -->|image-index| Broker
    Broker --> Orch[Scan Orchestrator]
    Orch -->|request scan| Scanner
    Scanner -->|results| Orch
    Orch -->|vulnerabilities| Broker
```

## Baselines

* **What it does**: Learns normal behavior patterns, detects anomalies
* **Subscribes to**: `runtime-events`, `network-flows`, `process-events`
* **Outputs**: Baseline CRs, anomaly alerts (to broker)
* **Use case**: Process baseline violations, network anomaly detection, policy refinement
