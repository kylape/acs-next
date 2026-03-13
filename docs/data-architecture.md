# Data Architecture

*Part of [ACS Next Architecture](architecture.md)*

---

At the single-cluster level, ACS Next has **no custom persistent API**. All
data is served by existing infrastructure. The originally proposed Persistence
Service has been eliminated entirely at the per-cluster level.

## Data Sources by Use Case

| Use Case | Data Source | Query Interface | Auth Model |
|---|---|---|---|
| View active violations | PolicyViolation CRs (namespace-scoped) | kubectl / OCP Console | K8s RBAC |
| View image scan summary | ImageScanSummary CRs (projected from broker) | kubectl / OCP Console | K8s RBAC |
| Drill into CVEs for a specific image | Scanner API (already returns vuln reports per image) | Console plugin calls Scanner | Service account |
| Manage security policies | SecurityPolicy CRDs | kubectl / Console / GitOps | K8s RBAC |
| Request/approve vulnerability exceptions | VulnException CRDs (with status subresource) | kubectl / Console | K8s RBAC |
| View violation/CVE trends | Prometheus metrics | OCP Console dashboards | OCP monitoring RBAC |
| Investigate "what happened" (process events, network events) | Customer's SIEM via External Notifiers | Splunk / ELK / Loki | Customer's system |
| CI/CD image checks | `roxctl` talks to Scanner | CLI | Service account / kubeconfig |
| Compliance report export | `roxctl` generates report from Scanner + CRD snapshot | CLI | Service account |

## Why No Persistence Service Per Cluster

The original architecture proposed a Persistence Service for four use cases.
Each has a simpler alternative:

**1. Vulnerability trends ("how are we trending quarter-over-quarter?")**

Components expose Prometheus metrics — violation counts by severity/namespace/policy,
CVE counts by severity/fixability, exception counts by state. OCP's built-in
Prometheus scrapes them. Security leads get dashboards in OCP Console for free.
Longer retention via Thanos/remote write for quarter-over-quarter (OCP already
supports this).

**2. Event history for incident response ("what processes ran in this pod?")**

Process events, network events, and violations flow through the broker.
External Notifiers push them to Splunk, ELK, Syslog — whatever the customer
already runs. SREs query their existing observability stack. Customers without
a SIEM can use OpenShift Logging (Loki/Elasticsearch).

**3. Cross-namespace aggregation ("which teams have the most violations?")**

Prometheus metrics handle aggregate counts. For listing specific violations
across namespaces, kubectl/Console queries over PolicyViolation CRs work —
the volume of *active violations* is manageable (hundreds to low thousands,
not hundreds of thousands).

**4. "Which images are affected by CVE X?"**

This is the one genuinely relational query. It requires joining CVEs to
affected packages to images. This data already exists in the Scanner's
matcher database — it has the vulnerability DB, image indexes, and performs
the matching. At single-cluster level, the Scanner can answer this query
directly. At fleet level, this is the Vuln Management Service's job (see
[Multi-Cluster documentation](multi-cluster.md)).

## CRD Scaling Strategy

etcd has practical limits — 100k CRs of a single type is not realistic.
The strategy is to keep CRs at summary level:

| CR Type | Expected Volume | Content |
|---|---|---|
| PolicyViolation | Hundreds (active violations only) | Violation details, scoped to namespace |
| ImageScanSummary | Thousands (one per unique image) | Critical/high/medium/low counts, top CVEs |
| SecurityPolicy | Tens to hundreds | Policy configuration |
| VulnException | Tens to hundreds | Exception requests with approval status |
| SignatureVerifier | Tens | Signature verification configuration |

**Full CVE-level data does not become CRs.** When a user drills into a
specific image's vulnerabilities in the Console, the Console plugin calls
the Scanner directly for the full vulnerability report. This keeps CR
counts manageable while still providing drill-down capability.

## Prometheus Metrics Strategy

Components expose metrics that Prometheus scrapes. This replaces any need
for a custom trend/analytics database.

### Metrics by Component

**Admission Controller:**
* `acs_admission_violations_total{policy, severity, namespace}` — counter
* `acs_admission_requests_total{action, namespace}` — counter (allowed/denied)

**Runtime Evaluator:**
* `acs_runtime_violations_total{policy, severity, namespace}` — counter
* `acs_runtime_events_total{type, namespace}` — counter (process/network/file)

**Scanner:**
* `acs_image_vulnerabilities{severity, fixable}` — gauge per image (top-level counts)
* `acs_images_scanned_total` — counter
* `acs_scan_duration_seconds` — histogram

**CRD Projector:**
* `acs_active_violations{severity, namespace}` — gauge
* `acs_active_exceptions{status}` — gauge (pending/approved/denied)

### What This Enables

* OCP Console dashboards showing violation and CVE trends over time
* Alerting via AlertManager on metric thresholds (e.g., "critical CVE
  count increased by 20% this week")
* Long-term retention via Thanos for quarter-over-quarter comparisons
* No custom API, no custom database, no RBAC mapping — just Prometheus

## External Notifiers as Event History

The External Notifiers component (broker subscriber) pushes security events
to external systems. This replaces any need for a custom event history
database.

### Flow

```
Collector / Admission Controller / Scanner
    |
    v
Broker (NATS JetStream)
    |
    v
External Notifiers (subscriber)
    |
    |-- Splunk (process events, violations)
    |-- ELK / Loki (structured logs)
    |-- Jira (violation tickets)
    |-- Slack / Teams (alerts)
    |-- Syslog (compliance)
    |-- AWS Security Hub / Sentinel (cloud SIEM)
```

### For Customers Without a SIEM

OpenShift Logging (Loki or Elasticsearch) is available as a platform
capability. The External Notifiers component can push structured events
to the cluster's logging stack, making them queryable via the OCP Console
log viewer.

## Vulnerability Exception Workflow

Vulnerability exceptions use CRDs with the status subresource pattern,
keeping the workflow entirely within K8s RBAC.

### Single-Cluster Flow

```
Developer                     Security Lead                  Scanner / Policy Engine
    |                              |                            |
    |-- creates VulnException CR ->|                            |
    |   (K8s RBAC: create)         |                            |
    |                              |                            |
    |                              |-- updates status to        |
    |                              |   "approved"               |
    |                              |   (K8s RBAC: update/status)|
    |                              |                            |
    |                              |         Policy engine  ----|
    |                              |         watches CRs and    |
    |                              |         factors approved   |
    |                              |         exceptions into    |
    |                              |         evaluation         |
```

### Fleet-Level Flow

```
Security Admin (hub)
    |
    |-- creates VulnException CR on hub
    |   (K8s RBAC on hub cluster)
    |
    v
ACM Governance
    |
    |-- distributes VulnException CR to managed clusters
    |   (standard CRD distribution, same as policies)
    |
    v
Per-cluster policy engines factor in the exception
```

### What This Avoids

* No custom exception API endpoints
* No API token management for exception workflow
* No custom RBAC for "who can approve exceptions" — K8s RBAC on the
  status subresource handles it
* Fleet distribution uses existing ACM Governance — no custom sync

## Summary: What Changed and Why

| Concern | Original Design | Revised Design | Why |
|---|---|---|---|
| Vulnerability trends | Persistence Service + REST API | Prometheus metrics + OCP dashboards | Already built into OCP, no custom API needed |
| Event history | Persistence Service | External Notifiers to customer SIEM | Leverage existing observability infrastructure |
| CVE drill-down (single cluster) | Persistence Service or 100k CRs | Scanner's existing per-image API | Scanner already has this data and capability |
| CVE queries (fleet) | Unclear | Vuln Management Service on hub | Purpose-built, scoped, with clear RBAC model |
| Exception workflow | Unclear (Persistence Service?) | CRDs with status subresource | Pure K8s RBAC, no custom auth |
| RBAC | "K8s RBAC-based, coarse-grained" (hand-wavy) | Cluster-scoped at fleet, namespace-scoped per cluster | Two clean models, no cross-product |
| Scheduled reporting | Central's ReportService | Vuln Management Service internal component + ReportConfiguration CRDs | Same data, same service — clean internal boundary, not a separate microservice |
| Compliance reports | Persistence Service | Scheduled reports + `roxctl` + Prometheus trend evidence | Point-in-time artifacts, not live queries |

### What's Eliminated

* **Persistence Service** (per-cluster PostgreSQL + REST API) — replaced by
  Prometheus, External Notifiers, Scanner, and CRDs
* **Custom RBAC on a persistent API** — no per-cluster API means no RBAC
  mapping problem at single-cluster level
* **100k+ vulnerability CRs** — summary CRs only; drill-down via Scanner

### What's Added

* **Vuln Management Service** (hub only) — fleet-wide vulnerability
  authority with a scoped query API and simple cluster-level RBAC
* **Prometheus metrics** from all components — replaces custom trend/analytics
* **Explicit CRD scaling strategy** — summary-level CRs, not raw data dumps
