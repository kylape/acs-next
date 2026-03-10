# ACS Next: Data Architecture & API Simplification

*Status: Draft | Date: 2026-03-09*

---

## Purpose

This document revises the ACS Next data architecture based on a critical
examination of whether the originally proposed Persistence Service actually
simplifies the API surface or just relocates Central's complexity. The
conclusion: **the Persistence Service as originally conceived is unnecessary.**

This document replaces the Persistence Service design in
[ACS_NEXT_ARCHITECTURE.md](ACS_NEXT_ARCHITECTURE.md) with a simpler model
that eliminates the need for a custom REST API at the single-cluster level
entirely.

---

## Key Departures from Original Architecture

| Original Architecture | Revised Architecture |
|---|---|
| Persistence Service (PostgreSQL + REST API) subscribes to all broker feeds, stores all security data, exposes query API | **Eliminated at single-cluster level.** No custom persistent API per cluster. |
| CRD Projector writes all security data as CRs (100k+ vulnerability CRs) | CRD Projector writes **summary-level** CRs only (violations, scan summaries, exceptions). Full CVE data stays in Scanner. |
| Unclear RBAC model on Persistence Service ("K8s RBAC-based, coarse-grained") | **No RBAC problem at single-cluster level.** CRDs use K8s RBAC natively. Scanner handles image-level drill-down. |
| Vuln Management is implicit across components | **Explicit Vuln Management Service at hub level only** — the multi-cluster addon's core value proposition. |

---

## Single-Cluster Data Architecture

At the single-cluster level, ACS Next has **no custom persistent API**. All
data is served by existing infrastructure.

### Data Sources by Use Case

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

### Why No Persistence Service Per Cluster

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
below).

### CRD Scaling Strategy

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

### Scanner's Role (Unchanged from Current)

Scanner remains a **compute service** — it indexes images, matches against
the vulnerability database, and returns results. It does not persist match
results or become a query authority. Its existing API already supports
"give me the vulnerability report for image X," which is sufficient for
single-cluster Console drill-down.

At the fleet level, the Vuln Management Service takes responsibility for
persisting and querying match results across clusters (see below).

---

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

---

## Prometheus Metrics Strategy

Components expose metrics that Prometheus scrapes. This replaces the
Persistence Service's trend/analytics role.

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

---

## External Notifiers as Event History

The External Notifiers component (broker subscriber) pushes security events
to external systems. This replaces the Persistence Service's event history
role.

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

---

## Multi-Cluster: Vuln Management Service

The Vuln Management Service is the **multi-cluster addon's core value
proposition**. It runs on the ACM hub, not per-cluster.

### Why It Exists

At the fleet level, a security admin needs queries that span clusters:

* "Which images across my fleet are affected by CVE-2024-1234?"
* "What's the fleet-wide vulnerability posture by severity?"
* "Show me all clusters running images with critical unfixed CVEs."

These are relational queries across cluster boundaries. CRDs can't answer
them (scaling limits + no cross-cluster joins). Prometheus can answer
aggregate counts but not "which specific images." ACM Search can answer
some of these if CRs are indexed, but struggles with CVE-level granularity.

The Vuln Management Service is purpose-built for this.

### Architecture

```
Cluster A Broker              Cluster B Broker
    |                              |
    | NATS leaf node (mTLS)        | NATS leaf node (mTLS)
    |                              |
    v                              v
+------------------------------------------------------+
|              Vuln Management Service (hub)            |
|                                                      |
|  Subscribes to: image-scans, vulnerabilities         |
|  Watches: VulnException CRs (hub)                    |
|                                                      |
|  +------------------------------------------------+  |
|  |  SQLite (small fleets)                         |  |
|  |  -- or --                                      |  |
|  |  PostgreSQL (BYODB, large fleets)              |  |
|  +------------------------------------------------+  |
|                                                      |
|  Query API:                                          |
|  * GET /images?cve=X          (which images have X)  |
|  * GET /images/{id}/vulns     (vulns for image)      |
|  * GET /summary               (fleet posture)        |
|  * GET /export                (compliance report)    |
+------------------------------------------------------+
         |
         v
    OCP Console multi-cluster perspective
    roxctl fleet-level queries
```

### Data Model

Every record includes a `cluster_id` dimension:

* Image scan results: image manifest, component inventory, matched CVEs,
  cluster where the image is running
* Aggregates: CVE impact across clusters, images affected per CVE

### Default Database

| Fleet Size | Default DB | Rationale |
|---|---|---|
| Small (< 20 clusters) | SQLite on PVC | No operational overhead, single file |
| Large (20+ clusters) | PostgreSQL (BYODB) | Customer provides their own DB for scale |

SQLite handles reads well and write volume is modest — scan results arrive
in batches, not streaming. For larger fleets, customers can point the
service at their own PostgreSQL instance.

**Potential alternative: per-cluster SQLite sharding.** Instead of one
SQLite file for all clusters, use one SQLite file per managed cluster.
This has several appealing properties:

* **Single-cluster mode becomes trivial** — same binary, same code, one
  shard. The Vuln Management Service can run per-cluster with zero
  changes, making it available outside the multi-cluster addon.
* **Cluster isolation is physical, not logical** — no `WHERE cluster_id = ?`
  on every query, no risk of cross-cluster data leaks from query bugs.
* **Adding/removing a cluster** is creating/deleting a file.
* **Fleet-wide queries** open all relevant shards, query each, merge
  results. SQLite handles concurrent readers well.
* **BYODB migration** — when a customer outgrows sharded SQLite, switch
  to PostgreSQL with `cluster_id` as a column. The abstraction layer
  changes from "query N files, merge" to "query one DB with filter."

The trade-off is fleet-wide queries at large scale — querying 200 SQLite
files and merging is more work than a single indexed PostgreSQL query.
But at that fleet size, the customer should be on BYODB anyway.

### Scheduled Reporting

Current ACS has a full report lifecycle management feature — configure,
schedule, generate, deliver, track history. ACS Next preserves this
capability as an internal component of the Vuln Management Service rather
than a separate microservice.

#### Current ACS Reporting (for reference)

* **Report types**: Vulnerability reports (CVE data across deployments/images)
* **Scheduling**: Cron-based — daily, weekly, monthly. Plus on-demand execution.
* **Output**: Zipped CSV with columns for cluster, namespace, deployment,
  image, component, CVE, severity, CVSS, EPSS, advisory info
* **Delivery**: Email (zipped CSV attachment, customizable templates,
  multiple recipients, retry logic) or download via HTTP
* **Scoping**: Filtered by resource collections (cluster, namespace,
  deployment), severity, fixability, time window ("since last report")
* **History**: Full snapshot tracking — who requested, when it ran, status

#### ACS Next Reporting Design

Reporting lives inside the Vuln Management Service as a separate internal
package, not a separate microservice:

```
Vuln Management Service
├── Ingester         (broker subscriber, persists scan results)
├── Query API        (GET /images?cve=X, etc.)
├── Report Scheduler (cron-based, runs queries, formats output)
└── Report Delivery  (publishes to broker for External Notifiers)
```

**Why not a separate service?** Three practical tests:

* *Would these be owned by different teams?* Unlikely — same domain,
  same data, same team.
* *Would you scale them independently?* Possibly — query load scales
  with data size, reporting scales with report frequency. But report
  frequency is low in practice.
* *Would you deploy one without the other?* Unlikely, though an
  "advanced reporting in OPP" scenario is imaginable.

The organizational reality is that the team has valid resistance to
microservice proliferation. One service with clean internal boundaries
is the right starting point.

**Report configuration as CRDs:**

```yaml
apiVersion: acs.openshift.io/v1
kind: ReportConfiguration
metadata:
  name: weekly-critical-cves
spec:
  schedule:
    intervalType: WEEKLY
    hour: 8
    minute: 0
    daysOfWeek: [1]  # Monday
  filters:
    severity: [CRITICAL, IMPORTANT]
    fixable: FIXABLE
    sinceLastReport: true
  scope:
    # Resource collection reference or label selectors
    namespaceSelector:
      matchLabels:
        env: production
  delivery:
    notifiers:
      - notifierRef: email-security-team
        recipients: ["security@example.com"]
        customSubject: "Weekly CVE Report — Production"
status:
  lastRun:
    timestamp: "2026-03-10T08:00:00Z"
    state: DELIVERED
  nextRun: "2026-03-17T08:00:00Z"
```

Using CRDs for report configuration has two advantages:

1. **K8s RBAC controls who can create/modify report schedules** — no
   custom authorization layer needed.
2. **Future-proofs for separation** — if reporting ever needs to become
   a separate service (e.g., "advanced reporting" as an OPP feature),
   it just watches the same CRDs independently. Nothing changes for
   the user.

**Report delivery via broker:**

The reporting component publishes completed reports (zipped CSV) to a
broker topic (e.g., `acs.reports.ready`). External Notifiers subscribe
and handle email delivery. This avoids embedding email/Slack logic in
the Vuln Management Service and uses the same delivery infrastructure
as violation notifications.

**Fleet-level reporting:**

At the hub, the Vuln Management Service has fleet-wide vulnerability
data. Reports can span clusters — "all critical fixable CVEs across
production clusters" — using the same query layer that powers the
Console. Report scoping respects the same cluster-level RBAC as
interactive queries.

### What It Doesn't Do

* **No user management** — no custom auth providers, no API tokens
* **No exception CRUD** — exceptions are CRDs, managed via kubectl/Console,
  distributed via ACM Governance
* **No policy management** — policies are CRDs
* **No direct notification delivery** — publishes to broker; External
  Notifiers handle email/Slack/SIEM delivery
* **No event history** — that's the customer's SIEM

---

## Fleet-Level RBAC

### Design Principle

**Fleet level: cluster-scoped RBAC. Cluster level: namespace-scoped RBAC.**

Two clean models at two levels. No cross-product. No identity mapping.
No custom RBAC engine.

### How It Works

The Vuln Management Service filters query results by the user's cluster
access:

```
User queries Vuln Management Service
    |
    +-- Service checks: "Which ManagedClusters (or ManagedClusterSets)
    |   does this user have access to?"
    |   (SubjectAccessReview or ACM RBAC API -- needs validation)
    |
    +-- Filters results to only those clusters
    |
    +-- Returns cluster-scoped results
```

| Persona | Fleet view scope | Mechanism |
|---|---|---|
| Security Lead / Fleet Admin | All clusters | Full ManagedCluster access |
| Team Lead | Their team's clusters | ManagedClusterSet binding |
| Developer | **Does not use fleet view** | Uses single-cluster Console with K8s RBAC |

### Why Not Namespace-Level RBAC at Fleet Level

Namespace-level filtering at the fleet level — "user sees namespace A from
cluster 1 and namespace B from cluster 2" — reintroduces the RBAC
complexity that makes the current architecture unmaintainable:

* The hub would need to know every user's namespace-level permissions on
  every managed cluster
* This requires either syncing all RoleBindings to the hub (Central's SAC
  engine) or making SubjectAccessReview calls to remote clusters per query
  (latency and availability issues)
* ACM does not model namespace-level permissions on managed clusters —
  ACM RBAC operates at the ManagedCluster / ManagedClusterSet level

**The right UX model:** Fleet personas (security leads, team leads) operate
at cluster granularity. Namespace personas (developers) use the
single-cluster Console where K8s RBAC scopes naturally. These are different
tools for different jobs. Forcing the fleet view to also be a namespace
view creates complexity without proportional user value.

**Compared to current ACS:** Current ACS has fine-grained multi-cluster
RBAC (SAC engine with cluster x namespace x resource type x access level).
It's complex to configure, hard to reason about, and most customers end
up with a handful of broad roles anyway. ACS Next replaces this with two
familiar models that require zero custom RBAC configuration.

### Open Question: ACM RBAC Validation

**Needs validation with the ACM team:**

1. Does ACM Search filter results by ManagedClusterSet RBAC? If a user
   only has access to ManagedClusterSet `prod-east`, does ACM Search
   only return CRs from clusters in that set?

2. Does this extend to custom resources indexed from managed clusters?

3. Is there an API the Vuln Management Service can call — "given this
   user, which clusters can they see?" — so we can filter query results
   without reimplementing ACM's RBAC logic?

4. How do ManagedClusterSetBindings interact with addon data visibility?

---

## Compliance and Vulnerability Reporting

### Scheduled Reports

The Vuln Management Service's built-in reporting capability (see
[Scheduled Reporting](#scheduled-reporting) above) is the primary
mechanism for recurring vulnerability reports. Security leads configure
ReportConfiguration CRDs to automatically generate and deliver scoped
CVE reports on a schedule.

At single-cluster level (without the Vuln Management Service), `roxctl`
can generate on-demand reports directly from the Scanner:

```bash
# On-demand vulnerability report from Scanner
roxctl report generate --format csv --severity CRITICAL,IMPORTANT

# Export SBOM for an image
roxctl image sbom registry.example.com/app:v1.2
```

### Compliance Evidence

Compliance standards (PCI-DSS, NIST 800-53, FedRAMP) typically require:

1. **Proof of scanning** — scheduled vulnerability reports satisfy this.
   The ReportConfiguration CRD's status tracks run history, providing
   an audit trail of scan cadence.
2. **Proof of remediation** — Prometheus metrics with Thanos long-term
   retention provide "remediation over time" evidence. Grafana dashboards
   showing CVE counts trending downward satisfy auditor requirements.
3. **SBOM** — Scanner produces SBOMs on demand via `roxctl`.

The compliance auditor persona does not interact with ACS directly — a
customer employee (security lead or platform engineer) generates reports
via scheduled delivery or `roxctl` and provides them to the auditor.

---

## Deployment Profiles (Revised)

### Single Cluster — Standalone (no ACM)

```
Components: Collector + Scanner + Admission Controller + Broker
            + Runtime Evaluator + CRD Projector
Optional:   External Notifiers, Alerting Service
Storage:    Broker PVC (~150-300 MB), Scanner DB (existing)
Custom API: None
```

### Single Cluster — ACM-Managed

```
Components: Collector + Scanner + Admission Controller + Broker
            + Runtime Evaluator
Optional:   CRD Projector (for local Console visibility)
            External Notifiers, Alerting Service
Storage:    Broker PVC
Custom API: None (hub provides fleet queries)
```

### Hub (Multi-Cluster Addon)

```
Components: Vuln Management Service
            + fleet-level External Notifiers (optional)
Storage:    SQLite on PVC (small fleets)
            or PostgreSQL BYODB (large fleets)
Custom API: Vuln Management Service query API (cluster-scoped RBAC)
```

---

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

---

## Next Steps

1. **Validate ACM RBAC model** — confirm ManagedClusterSet RBAC works
   for filtering addon data (ACM architect meeting)
2. **Define CRD schemas** — PolicyViolation, ImageScanSummary,
   VulnException, SecurityPolicy, ReportConfiguration
3. **Define Prometheus metrics** — what each component exports
4. **Design Vuln Management Service** — data model, query API, SQLite
   schema, BYODB abstraction
5. **Validate Scanner drill-down** — confirm Scanner's existing API
   supports the Console plugin's needs for per-image CVE listing
6. **Prototype Console plugin** — single-cluster experience with CRDs +
   Scanner drill-down, no Persistence Service

---

*This document reflects analysis as of March 2026. It should be updated
based on the ACM RBAC validation and CRD schema design work.*
