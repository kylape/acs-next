# Consumers

*Part of [ACS Next Architecture](../)*

---

Consumers subscribe to broker feeds and perform actions. Users choose which consumers to deploy based on their needs.

## Vuln Management Service (Hub Only)

* **What it does**: Fleet-wide vulnerability authority — aggregates scan results across clusters, provides query API
* **Subscribes to**: `image-scans`, `vulnerabilities` via NATS leaf nodes
* **Outputs**: Fleet-wide query API, scheduled reports, OCP Console multi-cluster views
* **Deployment**: Runs on ACM hub only, not per-cluster
* **Notes**: See [Multi-Cluster documentation](../multi-cluster.md) for full design

## CRD Projector (Optional)

* **What it does**: Projects **summary-level** security data into Kubernetes CRs
* **Subscribes to**: `policy-violations`, `image-scans`
* **Outputs**: `PolicyViolation`, `ImageScanSummary` CRs (summary-level only)
* **When needed**: Standalone clusters (no ACM), or local OCP Console visibility
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

## Alerting Service (Optional)

* **What it does**: Generates alerts from policy violations
* **Subscribes to**: `policy-violations`
* **Outputs**: Alerts to AlertManager
* **Why separate**: Allows OCP-native alerting; users may have existing alerting infra

## External Notifiers (Optional)

* **What it does**: Sends notifications to external systems
* **Subscribes to**: `policy-violations`, `vulnerabilities` (configurable)
* **Outputs**: Jira tickets, Splunk events, Slack messages, AWS Security Hub, etc.
* **Notes**: Maintains parity with current ACS notifier integrations. Also serves as
  the event history mechanism — pushes security events to customer SIEM for
  incident response queries (see [Data Architecture](../data-architecture.md))

## Risk Scorer (Optional)

* **What it does**: Calculates composite risk scores for workloads
* **Subscribes to**: Broker feeds (`vulnerabilities`, `policy-violations`, `runtime-events`)
* **Outputs**: Risk scores (publishes back to broker for other consumers)
* **Why separate**: Allows independent scaling; customers want configurable risk calculation
* **Notes**: Designed for configurability — users adjust weights, factor in business context

**Data sources:**
```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Broker    │     │   Broker    │     │  External   │
│   feeds     │     │   feeds     │     │  context    │
└──────┬──────┘     └──────┬──────┘     └──────┬──────┘
       │                   │                   │
       │ runtime-events    │ vulnerabilities   │ business context
       │                   │ policy-violations │ asset importance
       │                   │                   │
       ▼                   ▼                   ▼
┌─────────────────────────────────────────────────────┐
│                    Risk Scorer                       │
│  • Consumes multiple signals                         │
│  • Applies configurable weights                      │
│  • Calculates composite risk per workload            │
│  • Outputs risk scores to subscribers                │
└─────────────────────────────────────────────────────┘
```

## Baselines (Optional)

* **What it does**: Learns normal behavior patterns, detects anomalies
* **Subscribes to**: `runtime-events`, `network-flows`, `process-events`
* **Outputs**: Baseline CRs, anomaly alerts (to broker)
* **Use case**: Process baseline violations, network anomaly detection
* **Notes**: Can be used for both alerting and policy refinement
