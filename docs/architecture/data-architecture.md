# Data Architecture

*Part of [ACS Next Architecture](./)*

---

At the single-cluster level, ACS Next has **no custom persistent API**. All
data is served by existing infrastructure — no per-cluster database is required.

## Data Sources by Use Case

| Use Case | Data Source | Query Interface | Auth Model |
|---|---|---|---|
| View active violations | PolicyViolation CRs (namespace-scoped) | kubectl / OCP Console | K8s RBAC |
| View image scan summary | ImageScanSummary CRs (projected from broker) | kubectl / OCP Console | K8s RBAC |
| Drill into CVEs for a specific image | Scanner API (needs design) | Console plugin calls Scanner | Service account |
| Manage security policies | SecurityPolicy CRDs | kubectl / Console / GitOps | K8s RBAC |
| Request/approve vulnerability exceptions | VulnException CRDs (with status subresource) | kubectl / Console | K8s RBAC |
| View violation/CVE trends | Prometheus metrics | OCP Console dashboards | OCP monitoring RBAC |
| Investigate "what happened" (process events, network events) | Customer's SIEM via Notifiers | Splunk / ELK / Loki | Customer's system |
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
Notifiers push them to Splunk, ELK, Syslog — whatever the customer
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

## Notifiers as Event History

The Notifiers component (broker subscriber) pushes security events
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
Notifiers (subscriber)
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
capability. The Notifiers component can push structured events
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

## Summary: Data Source Mapping

| Use Case | Data Source | Notes |
|---|---|---|
| Vulnerability trends | Prometheus metrics + OCP dashboards | Already built into OCP, no custom API needed |
| Event history | Notifiers to customer SIEM | Leverages existing observability infrastructure |
| CVE drill-down (single cluster) | Scanner per-image API (needs design) | Scanner has the data; API needs to be designed |
| CVE queries (fleet) | Vuln Management Service | Purpose-built, scoped, with clear RBAC model |
| Exception workflow | CRDs with status subresource | Pure K8s RBAC, no custom auth |
| RBAC | Cluster-scoped at fleet, namespace-scoped per cluster | Two clean models, no cross-product |
| Scheduled reporting | Vuln Management Service + ReportConfiguration CRDs | Same data, same service — clean internal boundary |
| Compliance reports | Scheduled reports + `roxctl` + Prometheus | Point-in-time artifacts, not live queries |

### Design Choices

**No per-cluster persistent API required.** The architecture enables single-cluster
deployments without a custom database:

* **Vulnerability trends** → Prometheus metrics (already built into OCP)
* **Event history** → Notifiers push to customer's existing SIEM
* **CVE drill-down** → Scanner per-image API (needs design)
* **Active violations** → Summary-level CRs (hundreds to thousands, not millions)

**Vuln Management Service for fleet queries.** When fleet-wide CVE queries are
needed, the Vuln Management Service provides a scoped query API with simple
cluster-level RBAC. It can run on the hub for fleet deployments, or per-cluster
for single-cluster deployments that need historical queries.

**CRD scaling strategy.** Summary-level CRs only — PolicyViolation, ImageScanSummary.
Full CVE-level data stays in Scanner, queried on demand via drill-down.

## Stateful vs Stateless Components

**Principle: only components whose primary function is data persistence get PVCs. Everything else is stateless and replays from the broker on restart.**

Stateful components (have PVCs):

| Component | What it persists | Why |
|---|---|---|
| Broker | Event streams (JetStream) | Recovery source for all consumers |
| Scanner | Vulnerability database | Core function — indexing and matching |
| Vuln Management Service | Fleet-wide scan results | Core function — fleet queries and reporting |

Stateless components (no PVCs):
* CRD Projector, Notifiers, Runtime Evaluator, and any future consumers. On pod restart, they resume from their last acknowledged message position in the broker's durable consumer.

## Avoiding the Distributed Join Anti-Pattern

**ACS Next's guard rails:**

1. **The broker is the integration layer, not APIs.** Components don't call each other. They publish events and subscribe to events.

2. **Components that need historical data ingest via broker and materialize their own view.** The Vuln Management Service subscribes to scan result events and builds its own database.

3. **Single-cluster level has almost no cross-component joins.** PolicyViolation CRs are self-contained. ImageScanSummary CRs are self-contained. Scanner answers per-image queries. Prometheus answers aggregates.

4. **Keep the number of stateful components small.** Three stateful components (Broker, Scanner, Vuln Management Service) is manageable.

## Consumer Recovery and State Reconstruction

Event-driven architectures face a fundamental challenge: **how does a consumer
reconstruct its state after restart?** This is especially critical for
consumers that derive state from event streams.

### The Problem: Process Baselines Example

Process Baselines needs to know the current set of processes running in each
container. Collector publishes process start/stop events to a topic. If
Baselines restarts:

* It can't replay from the beginning — retention limits and time constraints
* Even with infinite retention, replaying days of events is impractical
* The "current state" isn't directly stored anywhere

This pattern applies to any consumer that materializes a view from events:
network baselines (current connections), risk scores (current risk per
workload), etc.

### Recovery Strategies

| Strategy | How it works | Trade-offs |
|----------|--------------|------------|
| **Replay from topic** | Replay events from retention window; rebuild state | Simple; requires idempotent handling; limited by retention |
| **Snapshot topics** | Periodic snapshots published to separate topic | Enables longer recovery; adds publish complexity |
| **Consumer PVC** | Consumer persists its own state to disk | Full history; adds stateful component |
| **Graceful degradation** | Accept gaps; rebuild state over time | Simplest; acceptable when gaps are tolerable |

**Anti-pattern: Source re-query.** Having consumers query sources directly for
current state (e.g., Baselines asking Collector "what's running now?") violates
the event-driven architecture. It couples consumers to sources, requires sources
to maintain queryable APIs, and defeats the purpose of the broker as the
integration layer. Avoid unless there's a strong justification.

### Recommended Approach by Consumer

| Consumer | Recovery Strategy | Rationale |
|----------|-------------------|-----------|
| **CRD Projector** | Replay from topic | CRs are idempotent; replaying violations/scans is safe |
| **Notifiers** | Replay from topic | May re-send some notifications; acceptable with dedup |
| **Risk Scorer** | Replay from topic | Scores are derived; can recompute from recent events |
| **Process Baselines** | Snapshot topic | Must know current process set; raw events insufficient |
| **Network Baselines** | Snapshot topic | Must know current connections; raw events insufficient |

### Snapshot Topic Pattern

For consumers like Baselines that need point-in-time state:

```
Collector publishes:
  acs.<cluster>.process-events     (individual start/stop events)
  acs.<cluster>.process-snapshots  (periodic full snapshot per container)

Baselines recovery:
  1. Read latest snapshot from process-snapshots
  2. Subscribe to process-events from snapshot timestamp forward
  3. Apply events to snapshot to reach current state
```

**Open questions:**

* Snapshot frequency vs. storage cost (every 5 min? 15 min? 1 hour?)
* Who publishes snapshots — Collector, or a separate aggregator?
* JetStream retention policy for snapshot topics (keep only latest per key?)

### Broker Retention Configuration

Retention windows affect recovery time and storage:

| Stream | Retention | Rationale |
|--------|-----------|-----------|
| `process-events` | 15-60 min | High volume; consumers should keep up |
| `network-flows` | 15-60 min | High volume |
| `policy-violations` | 24 hours | Lower volume; important to not miss |
| `image-scans` | 24 hours | Lower volume; batch arrivals |
| `*-snapshots` | Keep latest N | Only need recent snapshots for recovery |

**Storage estimate:** ~150-300 MB per cluster (needs validation via load testing)

**Backpressure policy:** Drop oldest events when full. Blocking publishers would
cascade failures through the system.
