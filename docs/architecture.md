# ACS Next: Detailed Architecture

*Status: Draft | Date: 2026-03-10*

---

## Overview

ACS Next is a single-cluster security platform built on an **event-driven architecture**. At its core is an **Event Hub** — an embedded pub/sub broker that aggregates all security data streams and allows consumers to subscribe to feeds of interest.

This design enables:
* **Decoupled components**: Producers and consumers evolve independently
* **Flexible deployment**: Users choose which consumers to run based on their needs
* **Minimal footprint option**: CRD-only deployment without any custom persistent API
* **Extensibility**: New consumers can be added without modifying core components

---

## Core Architecture

```
┌───────────────────────────────────────────────────────────────────────────────────────┐
│                               ACS Next (per cluster)                                  │
│                                                                                       │
│  SOURCES (raw data + embedded policy engine)                                          │
│  ┌───────────────┐ ┌───────────────┐ ┌───────────────┐ ┌───────────────┐              │
│  │   Collector   │ │   Admission   │ │  Audit Logs   │ │    Scanner    │              │
│  │    (eBPF)     │ │    Control    │ │               │ │  (+roxctl EP) │              │
│  │ runtime phase │ │ deploy phase  │ │               │ │  build phase  │              │
│  └───────┬───────┘ └───────┬───────┘ └───────┬───────┘ └───────┬───────┘              │
│          │                 │                 │                 │                      │
│          ▼                 ▼                 ▼                 ▼                      │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐  │
│  │                       ACS BROKER (embedded NATS)                                │  │
│  │                   (NATS protocol / mTLS for external)                           │  │
│  │                                                                                 │  │
│  │  Feeds:  acs.*.runtime-events | acs.*.process-events | acs.*.network-flows     │  │
│  │          acs.*.admission-events | acs.*.audit-events | acs.*.image-scans       │  │
│  │          acs.*.vulnerabilities | acs.*.policy-violations | acs.*.node-index    │  │
│  └─────────────────────────────────────────────────────────────────────────────────┘  │
│          │                 │                 │                 │                │     │
│          │ internal subscribers              │                 │                │     │
│          ▼                 ▼                 ▼                 ▼                │     │
│  ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌─────────────┐ ┌───────────┐  │     │
│  │  Alerting   │ │  External   │ │    Risk     │ │ Baselines   │ │    CRD    │  │     │
│  │   Service   │ │  Notifiers  │ │   Scorer    │ │             │ │ Projector │  │     │
│  │(AlertMgr)   │ │(Jira,Splunk)│ │             │ │             │ │(summaries)│  │     │
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘ └───────────┘  │     │
│                                                                                 │     │
└─────────────────────────────────────────────────────────────────────────────────┼─────┘
                                                                                  │
                                                           mTLS (external subscription)
                                                                                  │
                                                                                  ▼
┌───────────────────────────────────────────────────────────────────────────────────────┐
│                          OPP Portfolio (currently ACM)                                │
│                                                                                       │
│  ┌─────────────────────────────────────────────────────────────────────────────────┐  │
│  │                    Vuln Management Service (hub)                                 │  │
│  │   • Subscribes directly to Broker feeds from all managed clusters              │  │
│  │   • Aggregates vulnerability data fleet-wide                                   │  │
│  │   • Provides fleet-level query API (cluster-scoped RBAC)                       │  │
│  │   • Feeds OCP Console multi-cluster perspective                                │  │
│  └─────────────────────────────────────────────────────────────────────────────────┘  │
│                                                                                       │
│  • ACM Governance distributes policy CRDs to clusters                                │
│  • OCP Console provides multi-cluster security views                                 │
└───────────────────────────────────────────────────────────────────────────────────────┘
```

---

## Components

### Broker (Event Hub)

The central hub connecting all components. All raw data sources publish to the broker; all consumers subscribe from the broker. This avoids direct component-to-component connections (e.g., multiple Collectors per cluster would require complex fan-out otherwise).

See [Event Hub section](#event-hub--embedded-policy-engine) for details on messaging protocol and implementation.

---

### Sources of Raw Data

These components generate security data and publish to the broker.

#### Collector

* **What it does**: eBPF-based runtime data collection + node vulnerability indexing
* **Publishes to**: Broker (`runtime-events`, `process-events`, `network-flows`, `node-index`)
* **Deployment**: DaemonSet (one per node)
* **Embeds**: Policy engine (runtime phase)
* **Notes**:
  * Evaluates runtime policies locally; publishes violations to broker
  * Scans host filesystem for installed packages (node indexing)
  * Node index sent to Scanner matcher for vulnerability matching

#### Admission Control

* **What it does**: Validates workloads at deploy time via admission webhook
* **Publishes to**: Broker (`admission-events`, `policy-violations`)
* **Deployment**: Deployment (HA recommended)
* **Embeds**: Policy engine (deploy phase)
* **Notes**: Blocks or alerts on policy violations during deployment

#### Audit Logs

* **What it does**: Collects audit logs from control plane pods/nodes
* **Publishes to**: Broker (`audit-events`)
* **Deployment**: DaemonSet or sidecar pattern
* **Notes**: Source for compliance and anomaly detection

#### Scanner

* **What it does**: Image vulnerability scanning, SBOM generation
* **Publishes to**: Broker (`image-scans`, `vulnerabilities`, `sbom-updates`)
* **Deployment**: Flexible (see below)
* **Embeds**: Policy engine (build phase)
* **Exposes**: roxctl endpoint for CI pipeline integration

Scanner is a **compute service** — it indexes images, matches against the
vulnerability database, and returns results. It does **not** persist match
results or become a query authority. Its existing API already supports
"give me the vulnerability report for image X," which is sufficient for
single-cluster Console drill-down.

At the fleet level, the Vuln Management Service takes responsibility for
persisting and querying match results across clusters (see
[Multi-Cluster: Vuln Management Service](#multi-cluster-vuln-management-service)).

**Deployment options:**
* **Local**: Full scanner on secured cluster (higher resource cost)
* **Hub-based matcher**: Indexer on secured cluster, matcher on hub via ACM transports (lower secured cluster footprint)
* **Full hub**: Delegate all scanning to hub scanner (minimal secured cluster footprint)

---

### Components with Embedded Policy Engine

Multiple components embed the policy engine library. Each reads the same set of policy CRDs and filters based on lifecycle phase:

| Component | Phase | Example Policies |
|-----------|-------|------------------|
| Scanner | Build | Image CVE thresholds, required labels, base image restrictions |
| Admission Control | Deploy | Privileged containers, resource limits, namespace restrictions |
| Collector | Runtime | Process execution, network connections, file access |

**How it works:**
* Policy CRDs are the source of truth (distributed via ACM Governance)
* Each component watches policy CRDs and loads relevant policies
* Policy engine is a Go library compiled into each component
* Violations published to broker `policy-violations` feed

**Multi-phase policies:**

A single policy can span multiple phases:

```yaml
apiVersion: acs.openshift.io/v1
kind: SecurityPolicy
spec:
  name: no-privileged-containers
  lifecycleStages:
    - DEPLOY
    - RUNTIME
```

Each component filters policies by phase and evaluates those containing its phase.

**Cross-phase criteria:**

Policies can combine deploy-time and runtime criteria:

```
"Alert if image from untrusted registry AND container executes shell"
```

This works because:
* Runtime evaluation has full deployment context (image, registry, labels)
* Collector has local access to K8s API for pod/deployment specs
* Policy engine evaluates against all available context at evaluation time

This mirrors current ACS where Sensor evaluates policies locally with deployment context + runtime events. ACS Next is the same pattern — policies from CRDs instead of Central gRPC, but same local evaluation model.

---

### Policy Engine Architecture Options

The policy engine can be deployed in different ways depending on organizational and operational goals. This section lays out the options.

#### Constraints by Phase

| Phase | Sync Required? | Can Decouple from Source? | Notes |
|-------|----------------|---------------------------|-------|
| Build | Yes (CI waits) | Yes, with latency cost | Scanner has image context |
| Deploy | Yes (webhook) | **No** — must be in webhook path | Admission latency critical |
| Runtime (alert) | No | Yes | Async evaluation acceptable |
| Runtime (enforce) | Fast preferred | Yes, with latency cost | Kill pod, scale to zero |

**Key constraint:** Admission webhooks are synchronous. The policy engine for deploy-time MUST be in the admission path. A separate Policy Evaluator service would add latency and a hard dependency — if it's down, nothing deploys.

#### Option A: Embedded in Each Source (Current Proposal)

```
Scanner (embeds policy engine) ─────────────────► violations
Admission Control (embeds policy engine) ────────► violations
Collector (embeds policy engine) ────────────────► violations
```

* **Pros:** Simple deployment, no network dependencies, low latency
* **Cons:** Collector becomes more complex (needs K8s API access for deployment context)

#### Option B: Separate Runtime Evaluator (Collector Independent)

```
Scanner (embeds policy engine) ─────────────────────────► violations
Admission Control (embeds policy engine) ───────────────► violations
Collector (raw events only) ──► Broker ──► Runtime Evaluator ──► violations
```

* **Pros:**
  * Collector stays simple (just eBPF collection)
  * Conway's Law: Collector can be independent operator consumed by ACS
  * Runtime processing isolated from admission failure domain
* **Cons:**
  * Additional component (Runtime Evaluator)
  * Latency for runtime enforcement actions

**Why isolate runtime from admission?**

| Scenario | Separate Evaluator | Combined with Admission |
|----------|-------------------|------------------------|
| Runtime bug crashes evaluator | Deploys still work | **Deploys blocked** |
| Runtime event flood | Evaluator falls behind | **Admission slows** |
| Runtime memory leak | Evaluator OOMs | **Admission OOMs** |

Admission Controller is critical path — keeping it lean and isolated from unpredictable runtime workloads is safer.

#### Option C: Unified Policy Evaluator Service

```
Scanner ──────────► Policy Evaluator ◄──────── Admission Control
                         ▲                    (gRPC call)
                         │
Collector ──► Broker ────┘ (subscribes)
```

* **Pros:** Single policy logic location
* **Cons:**
  * Admission Controller has network dependency in critical path
  * If Policy Evaluator down → cluster can't deploy
  * Combines failure domains

**Not recommended** due to admission reliability concerns.

#### Recommendation

**Option B (Separate Runtime Evaluator)** with shared policy engine library:

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Policy Engine (shared Go library)                 │
└─────────────────────────────────────────────────────────────────────┘
        │                       │                        │
        ▼                       ▼                        ▼
┌───────────────┐    ┌───────────────────┐    ┌───────────────────────┐
│    Scanner    │    │ Admission Control │    │  Runtime Evaluator    │
│  (BUILD)      │    │ (DEPLOY)          │    │  (RUNTIME)            │
│               │    │                   │    │                       │
│ - CI endpoint │    │ - Webhook         │    │ - Broker subscriber   │
│ - Embeds lib  │    │ - Embeds lib      │    │ - Embeds lib          │
└───────────────┘    └───────────────────┘    └───────────────────────┘
                                                        ▲
                                                        │
                                              Collector ─┴─► Broker
                                              (raw events only)
```

**Benefits:**
* Collector is independent (can be separate operator/team)
* Admission isolated from runtime workloads
* Same policy engine code, different binaries
* Each component scales independently

---

### Signature Verification

Image signature verification (Cosign, Sigstore) is primarily a **deploy-time** concern — blocking unsigned images before they run.

#### Where Signature Verification Belongs

| Phase | Use Case | Priority |
|-------|----------|----------|
| Build | "Fail CI if image isn't signed" | Optional |
| **Deploy** | "Block unsigned images from cluster" | **Primary** |
| Runtime | Image already running | N/A |

**Primary enforcement: Admission Controller**

Admission Controller should verify signatures during admission. This matches how other tools work (Connaisseur, Kyverno, Gatekeeper).

```
┌─────────────────────────────────────────────────────────────┐
│              Admission Controller                            │
│                                                              │
│  ┌─────────────────┐  ┌─────────────────────────────────┐   │
│  │ Policy Engine   │  │ Signature Verifier              │   │
│  │ (deploy checks) │  │ (Cosign SDK, Sigstore client)   │   │
│  └─────────────────┘  └─────────────────────────────────┘   │
│           │                        │                         │
│           └───────── AND ──────────┘                        │
│                      │                                       │
│              Allow / Deny                                    │
└─────────────────────────────────────────────────────────────┘
```

**Configuration:**

```yaml
apiVersion: acs.openshift.io/v1
kind: SignatureVerifier
metadata:
  name: prod-signing-keys
spec:
  type: cosign
  keys:
    - secretRef:
        name: cosign-public-key
        key: cosign.pub
  # OR keyless (Fulcio/Rekor)
  keyless:
    issuer: https://accounts.google.com
    subject: builder@example.com
```

**Scanner's role (optional):**

Scanner could also verify signatures for build-time ("fail CI if unsigned"), but this is secondary to admission enforcement. Both would use the same signature verification library.

---

### Broker (ACS Broker with Embedded NATS)

The Broker is the central nervous system — a pub/sub message broker that:

* Receives events from all producers (Collector, Scanner, Admission Control, etc.)
* Organizes events into typed feeds (NATS subjects)
* Allows consumers to subscribe to feeds with filtering (NATS wildcards)
* Provides delivery guarantees (at-least-once via JetStream)
* Handles backpressure
* Streams data across cluster boundaries via NATS leaf nodes (secured cluster → hub)

**Key design decision:** The Broker does **not** embed the policy engine. This enables:

* Cleaner separation of concerns
* Policy engine embedded in sources (Collector, Scanner, Admission Control) where evaluation happens
* Broker remains a thin messaging layer

#### Implementation: Embedded NATS

**Decision:** The ACS Broker is a custom Go binary that embeds the NATS server as a library. NATS is not deployed as a separate operator — it's an implementation detail inside our broker process.

**Why NATS:**

| Requirement | NATS Fit |
|-------------|----------|
| Lightweight footprint | ~20-50MB embedded; no JVM, no external deps |
| K8s/cloud-native ecosystem | CNCF project; used in K8s ecosystem |
| Protobuf compatibility | Payload-agnostic; protobuf bytes work natively |
| Pub/sub with durability | JetStream provides at-least-once, replay, persistence |
| Cross-cluster streaming | Leaf nodes for ACM addon subscription |
| Go-native | Official client, embeddable server |

**Why embedded (not operator):**

* **Single deployment** — One pod, one binary, no operator dependency
* **NATS is invisible** — Customers see "ACS Broker", not "NATS"
* **Version control** — We control NATS version via go.mod
* **Simpler ops** — No CRDs for NATS, no operator reconciliation loops

**Architecture:**

```
┌─────────────────────────────────────────────────────────────┐
│                    ACS Broker Pod                            │
│  ┌───────────────────────────────────────────────────────┐  │
│  │  acs-broker binary (single Go binary)                  │  │
│  │                                                         │  │
│  │  ├── Embedded NATS server (library)                    │  │
│  │  │   └── JetStream (persistence to PVC)                │  │
│  │  │                                                      │  │
│  │  ├── Stream manager (creates/manages feeds)            │  │
│  │  ├── Leaf node listener (mTLS, port 7422)              │  │
│  │  └── Health/metrics endpoints                          │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                              │
│  Memory: ~50-100MB                                           │
│  Ports: 4222 (NATS internal), 7422 (leaf/mTLS), 9090        │
└─────────────────────────────────────────────────────────────┘
```

**Example implementation:**

```go
import (
    "github.com/nats-io/nats-server/v2/server"
    "github.com/nats-io/nats.go"
)

func main() {
    // Embedded NATS server
    opts := &server.Options{
        ServerName: "acs-broker",
        Host:       "0.0.0.0",
        Port:       4222,
        JetStream:  true,
        StoreDir:   "/data/jetstream",

        // Leaf node for external subscribers (ACM addon)
        LeafNode: server.LeafNodeOpts{
            Host:      "0.0.0.0",
            Port:      7422,
            TLSConfig: loadMTLSConfig(),
        },
    }

    ns, _ := server.NewServer(opts)
    go ns.Start()
    ns.ReadyForConnections(10 * time.Second)

    // In-process client (zero network hop)
    nc, _ := nats.Connect(ns.ClientURL())
    js, _ := nc.JetStream()

    // Create streams for ACS feeds
    js.AddStream(&nats.StreamConfig{
        Name:     "RUNTIME_EVENTS",
        Subjects: []string{"acs.*.runtime-events"},
    })
    js.AddStream(&nats.StreamConfig{
        Name:     "POLICY_VIOLATIONS",
        Subjects: []string{"acs.*.policy-violations"},
    })

    select {}  // Block forever
}
```

#### Subject Hierarchy

NATS uses dot-separated subjects with wildcard support:

| Feed | Subject Pattern | Example |
|------|-----------------|---------|
| Runtime events | `acs.<cluster>.runtime-events` | `acs.cluster-a.runtime-events` |
| Process events | `acs.<cluster>.process-events` | `acs.cluster-a.process-events` |
| Network flows | `acs.<cluster>.network-flows` | `acs.cluster-a.network-flows` |
| Policy violations | `acs.<cluster>.policy-violations` | `acs.cluster-a.policy-violations` |
| Vulnerabilities | `acs.<cluster>.vulnerabilities` | `acs.cluster-a.vulnerabilities` |
| Image scans | `acs.<cluster>.image-scans` | `acs.cluster-a.image-scans` |
| Node index | `acs.<cluster>.node-index` | `acs.cluster-a.node-index` |

**Wildcards:**

* `acs.*.policy-violations` — All clusters' violations (single-level wildcard)
* `acs.cluster-a.>` — All feeds from cluster-a (multi-level wildcard)

#### JetStream Streams

JetStream provides durability and replay:

| Stream | Subjects | Retention | Notes |
|--------|----------|-----------|-------|
| `RUNTIME_EVENTS` | `acs.*.runtime-events` | Limits (size/age) | High-volume, recent events |
| `POLICY_VIOLATIONS` | `acs.*.policy-violations` | Interest-based | Must not lose violations |
| `VULNERABILITIES` | `acs.*.vulnerabilities` | Limits | Scan results |
| `IMAGE_SCANS` | `acs.*.image-scans` | Limits | Full scan data |
| `NODE_INDEX` | `acs.*.node-index` | Limits | Host package inventory |

#### Consumer Recovery and Failure Modes

**The problem ACS Next solves differently:** Current ACS handles Central-Sensor disconnects with elaborate sync machinery (90+ files). ACS Next eliminates cross-cluster sync but must still handle local component failures. JetStream provides the recovery mechanism, but this has resource implications.

**Failure scenarios and recovery:**

| Component | On Crash | Recovery Mechanism | Retention Needed |
|-----------|----------|-------------------|------------------|
| Policy Engine (in Collector) | Misses runtime events | Durable consumer replays from last ack | Minutes (catch-up window) |
| Alerting Service | Violations queue in broker | Durable consumer replays missed violations | Hours (must not lose) |
| CRD Projector | Events queue, CRs stale | Durable consumer replays; may need catch-up mode | Minutes |
| Baselines | Misses learning data | Acceptable loss; baselines are statistical | None (ephemeral OK) |
| Risk Scorer | Misses inputs | Acceptable; risk recalculates periodically | None (ephemeral OK) |
| Broker (acs-broker) | All consumers stall | JetStream replays from PVC on restart | Full retention window |

**Durable vs. ephemeral consumers:**

* **Durable consumers** (track position, survive restarts): Alerting Service, CRD Projector, ACM addon
* **Ephemeral consumers** (start from "now"): Baselines, Risk Scorer, optional analytics

**Storage implications:**

Durable consumers with replay require JetStream to retain messages. Rough sizing:

| Stream | Event Rate | Retention | Storage (estimate) |
|--------|------------|-----------|-------------------|
| `RUNTIME_EVENTS` | ~1000/min (busy cluster) | 15 min | ~50-100 MB |
| `POLICY_VIOLATIONS` | ~10/min (typical) | 24 hours | ~5-10 MB |
| `VULNERABILITIES` | ~100/scan | Until consumed | ~10-20 MB |
| `IMAGE_SCANS` | Variable | 1 hour | ~50-100 MB |
| **Total per cluster** | | | **~150-300 MB** |

*These are rough estimates. Actual sizing depends on cluster activity, event payload sizes, and retention policy decisions.*

**What's acceptable to lose:**

| Data Type | Acceptable to Lose? | Rationale |
|-----------|---------------------|-----------|
| Policy violations | **No** | Security-critical; must reach alerting |
| Runtime events (for policy) | **Yes** (bounded) | If 15 min of events lost, re-evaluation on next event catches up |
| Baseline learning events | **Yes** | Statistical; gaps smooth out over time |
| Scan results | **No** | Would require re-scan; expensive |
| Risk score inputs | **Yes** | Risk recalculates; eventual consistency OK |

**Open decisions:**

1. **Retention window sizes** — Affects PVC sizing; 15 min vs 1 hour vs 24 hours
2. **Catch-up performance** — If consumer falls behind, how fast must it catch up?
3. **Backpressure handling** — If broker fills, drop oldest events or block publishers?

#### Feed Schema

Events are typed with protobuf schemas. Each feed has a well-defined event type:

```protobuf
message RuntimeEvent {
  string cluster_id = 1;
  string namespace = 2;
  string pod = 3;
  google.protobuf.Timestamp timestamp = 4;
  oneof event {
    ProcessEvent process = 5;
    NetworkEvent network = 6;
    FileEvent file = 7;
  }
}
```

---

### External Subscribers (ACM Addon)

The ACS Broker exposes a NATS leaf node listener (mTLS) for external subscribers. This enables **ACM addon to subscribe directly to broker feeds**, bypassing the K8s API entirely.

**Why this matters:**

| Approach | Scalability | Security |
|----------|-------------|----------|
| **CRD-based** (write CRs, ACM Search indexes) | Limited by CR count; 1000 images × 50 CVEs = 50k CRs | Security data traverses K8s API |
| **Direct subscription** (ACM addon subscribes to feeds) | No CR limit; addon aggregates in-memory | mTLS between Event Hub and addon; bypasses K8s API |

**Architecture with NATS leaf nodes:**

```
┌─────────────────────────────┐         ┌─────────────────────────────┐
│      Cluster A              │         │      Cluster B              │
│  ┌───────────────────────┐  │         │  ┌───────────────────────┐  │
│  │      ACS Broker       │  │         │  │      ACS Broker       │  │
│  │  (NATS leaf :7422)    │  │         │  │  (NATS leaf :7422)    │  │
│  └───────────┬───────────┘  │         │  └───────────┬───────────┘  │
└──────────────┼──────────────┘         └──────────────┼──────────────┘
               │                                       │
               │ NATS leaf node (mTLS)                 │ NATS leaf node (mTLS)
               │                                       │
               ▼                                       ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         ACM Hub                                      │
│  ┌───────────────────────────────────────────────────────────────┐  │
│  │                  Vuln Management Service                       │  │
│  │  • Connects as NATS leaf subscriber to all managed clusters   │  │
│  │  • Subscribes to: acs.*.image-scans, acs.*.vulnerabilities    │  │
│  │  • Aggregates vulnerability data fleet-wide                   │  │
│  │  • Feeds OCP Console multi-cluster perspective                │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

**Benefits:**
* **No CR cardinality problem**: Vulnerability data streams directly to addon, no 50k CRs per cluster
* **Better security posture**: Security data never touches K8s API; attackers with K8s API access don't see vulnerability feeds
* **Lower latency**: Direct streaming vs CR write → index → query
* **Simpler managed cluster**: No CRD Projector needed if using direct subscription

**Trade-off:** Requires ACM addon to be running. Standalone clusters (no ACM) would still use CRD Projector for local visibility.

---

### Consumers

Consumers subscribe to broker feeds and perform actions. Users choose which consumers to deploy based on their needs.

#### Vuln Management Service (Hub Only)

* **What it does**: Fleet-wide vulnerability authority — aggregates scan results across clusters, provides query API
* **Subscribes to**: `image-scans`, `vulnerabilities` via NATS leaf nodes
* **Outputs**: Fleet-wide query API, scheduled reports, OCP Console multi-cluster views
* **Deployment**: Runs on ACM hub only, not per-cluster
* **Notes**: See [Multi-Cluster: Vuln Management Service](#multi-cluster-vuln-management-service) for full design

#### CRD Projector (Optional)

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

#### Alerting Service (Optional)

* **What it does**: Generates alerts from policy violations
* **Subscribes to**: `policy-violations`
* **Outputs**: Alerts to AlertManager
* **Why separate**: Allows OCP-native alerting; users may have existing alerting infra

#### External Notifiers (Optional)

* **What it does**: Sends notifications to external systems
* **Subscribes to**: `policy-violations`, `vulnerabilities` (configurable)
* **Outputs**: Jira tickets, Splunk events, Slack messages, AWS Security Hub, etc.
* **Notes**: Maintains parity with current ACS notifier integrations. Also serves as
  the event history mechanism — pushes security events to customer SIEM for
  incident response queries (see [External Notifiers as Event History](#external-notifiers-as-event-history))

#### Risk Scorer (Optional)

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

#### Baselines (Optional)

* **What it does**: Learns normal behavior patterns, detects anomalies
* **Subscribes to**: `runtime-events`, `network-flows`, `process-events`
* **Outputs**: Baseline CRs, anomaly alerts (to broker)
* **Use case**: Process baseline violations, network anomaly detection
* **Notes**: Can be used for both alerting and policy refinement

---

## Data Architecture

At the single-cluster level, ACS Next has **no custom persistent API**. All
data is served by existing infrastructure. The originally proposed Persistence
Service has been eliminated entirely at the per-cluster level.

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
[Multi-Cluster: Vuln Management Service](#multi-cluster-vuln-management-service)).

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

### Prometheus Metrics Strategy

Components expose metrics that Prometheus scrapes. This replaces any need
for a custom trend/analytics database.

#### Metrics by Component

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

#### What This Enables

* OCP Console dashboards showing violation and CVE trends over time
* Alerting via AlertManager on metric thresholds (e.g., "critical CVE
  count increased by 20% this week")
* Long-term retention via Thanos for quarter-over-quarter comparisons
* No custom API, no custom database, no RBAC mapping — just Prometheus

### External Notifiers as Event History

The External Notifiers component (broker subscriber) pushes security events
to external systems. This replaces any need for a custom event history
database.

#### Flow

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

#### For Customers Without a SIEM

OpenShift Logging (Loki or Elasticsearch) is available as a platform
capability. The External Notifiers component can push structured events
to the cluster's logging stack, making them queryable via the OCP Console
log viewer.

### Vulnerability Exception Workflow

Vulnerability exceptions use CRDs with the status subresource pattern,
keeping the workflow entirely within K8s RBAC.

#### Single-Cluster Flow

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

#### Fleet-Level Flow

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

#### What This Avoids

* No custom exception API endpoints
* No API token management for exception workflow
* No custom RBAC for "who can approve exceptions" — K8s RBAC on the
  status subresource handles it
* Fleet distribution uses existing ACM Governance — no custom sync

### Summary: What Changed and Why

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

#### What's Eliminated

* **Persistence Service** (per-cluster PostgreSQL + REST API) — replaced by
  Prometheus, External Notifiers, Scanner, and CRDs
* **Custom RBAC on a persistent API** — no per-cluster API means no RBAC
  mapping problem at single-cluster level
* **100k+ vulnerability CRs** — summary CRs only; drill-down via Scanner

#### What's Added

* **Vuln Management Service** (hub only) — fleet-wide vulnerability
  authority with a scoped query API and simple cluster-level RBAC
* **Prometheus metrics** from all components — replaces custom trend/analytics
* **Explicit CRD scaling strategy** — summary-level CRs, not raw data dumps

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

### Fleet-Level RBAC

#### Design Principle

**Fleet level: cluster-scoped RBAC. Cluster level: namespace-scoped RBAC.**

Two clean models at two levels. No cross-product. No identity mapping.
No custom RBAC engine.

#### How It Works

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

#### Why Not Namespace-Level RBAC at Fleet Level

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

#### Open Question: ACM RBAC Validation

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

## Compliance Operator Integration

**Decision**: Compliance operator integration is dissolved in ACS Next.

* Compliance operator management moves directly into OCP Console
* ACS no longer wraps or proxies compliance operator functionality
* Security policies (ACS) and compliance policies (compliance-operator) are separate concerns
* Users configure compliance operator directly; results visible in OCP Console

This simplifies ACS Next scope and avoids duplicating OCP-native compliance tooling.

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
| Alerting Service | Deployment | ~100-200MB | 50-100m | AlertManager integration |
| Risk Scorer | Deployment | ~200-500MB | 100-500m | Depends on cluster size |
| Baselines | Deployment | ~200-500MB | 100-500m | ML/statistical models |

### Profile: Single Cluster — Standalone (no ACM)

```
Components: Collector + Scanner + Admission Controller + Broker
            + Runtime Evaluator + CRD Projector
Optional:   External Notifiers, Alerting Service
Storage:    Broker PVC (~150-300 MB), Scanner DB (existing)
Custom API: None
```

### Profile: Single Cluster — ACM-Managed

```
Components: Collector + Scanner + Admission Controller + Broker
            + Runtime Evaluator
Optional:   CRD Projector (for local Console visibility)
            External Notifiers, Alerting Service
Storage:    Broker PVC
Custom API: None (hub provides fleet queries)
```

### Profile: Hub (Multi-Cluster Addon)

```
Components: Vuln Management Service
            + fleet-level External Notifiers (optional)
Storage:    SQLite on PVC (small fleets)
            or PostgreSQL BYODB (large fleets)
Custom API: Vuln Management Service query API (cluster-scoped RBAC)
```

### Profile: Edge (minimal on-cluster)

```
Components: Broker + Collector only (everything else on hub)
Footprint:  ~600-950MB cluster-wide + ~500-750MB per node
Storage:    None
Use case:   Resource-constrained edge clusters
Notes:      Scanner, Risk, Baselines all on hub cluster
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
| Scanner (indexer) | Optional | Optional | Can split indexer/matcher |
| Scanner (matcher) | Optional | Preferred | Heavy; often better on hub |
| Risk Scorer | Optional | Optional | Can run either place |
| Baselines | Optional | Optional | Can run either place |
| CRD Projector | Optional | - | Only for local OCP Console |

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

---

## Data Flow Examples

### Example 1: Image Scan → Summary CR + Console Drill-Down

```
1. Scanner scans image "nginx:1.21"
2. Scanner publishes to "image-scans" feed:
   { image: "nginx:1.21", vulns: [...], sbom: {...} }
3. CRD Projector subscribes to "image-scans"
4. CRD Projector creates ImageScanSummary CR (counts by severity, top CVEs)
5. User sees summary in OCP Console
6. User clicks "View full vulnerabilities" in Console
7. Console plugin calls Scanner API for full vulnerability report
8. Scanner returns detailed CVE list for the specific image
```

### Example 2: Runtime Event → Policy Violation → Alert

```
1. Collector detects privileged container start
2. Collector publishes to Event Hub "runtime-events" feed:
   { pod: "nginx", container: "nginx", privileged: true, ... }
3. Runtime Evaluator (broker subscriber) receives event
4. Runtime Evaluator evaluates "no-privileged-containers" policy
5. Runtime Evaluator publishes to "policy-violations" feed:
   { policy: "no-privileged-containers", resource: {...}, ... }
6. CRD Projector creates PolicyViolation CR (if deployed)
7. Vuln Management Service receives violation via leaf node (if deployed)
8. Alerting Service sends alert to AlertManager
9. External Notifiers creates Jira ticket (if configured)
```

### Example 3: Risk Calculation

```
1. Risk Scorer subscribes to multiple sources:
   - Event Hub "vulnerabilities" feed (base vulnerability score)
   - Event Hub "policy-violations" feed (policy risk factor)
   - Collector direct API (raw runtime events for exposure analysis)
2. Risk Scorer applies configurable weights and business context
3. Risk Scorer calculates composite risk for workload
4. Risk Scorer outputs risk scores to subscribers:
   { workload: "deployment/nginx", score: 8.5, factors: [...] }
5. Vuln Management Service aggregates risk across fleet (if deployed)
```

---

## Key Design Decisions

### Why Event Hub instead of direct storage?

| Direct Storage | Event Hub |
|----------------|-----------|
| Producers coupled to persistence | Producers publish, don't care who consumes |
| Single consumer (database) | Multiple consumers with different purposes |
| Hard to add new consumers | New consumers subscribe to existing feeds |
| All-or-nothing persistence | Choose your persistence strategy |

### Why embedded broker instead of external?

* **Footprint**: No additional infrastructure to deploy
* **Operational simplicity**: One less thing to manage
* **Latency**: In-process communication is faster
* **Trade-off**: Limited to single-cluster scale (which is the ACS Next model)

### Why CRD Projector is now optional?

With ACM addon direct subscription, CRs are no longer the only path to portfolio integration:

| Scenario | CRD Projector needed? |
|----------|----------------------|
| ACM deployed, addon subscribes directly | No — addon aggregates via Event Hub |
| Standalone cluster, local OCP Console visibility | Yes — CRs provide local UI |
| GitOps workflows for security policies | No — policies are CRDs regardless |
| K8s RBAC for security data access | Depends — direct subscription uses mTLS auth instead |

CRD Projector remains valuable for standalone clusters and local visibility, but is not required when ACM addon provides fleet aggregation.

### Why was the Persistence Service eliminated?

The original architecture proposed an optional Persistence Service
(per-cluster PostgreSQL + REST API) for historical queries, vulnerability
trends, and event history. Analysis showed that every use case it served
has a simpler alternative that leverages existing infrastructure:

* **Vulnerability trends** → Prometheus metrics + OCP dashboards
* **Event history** → External Notifiers to customer SIEM
* **CVE drill-down** → Scanner's existing per-image API
* **Cross-namespace aggregation** → Prometheus + PolicyViolation CRs
* **Fleet-level queries** → Vuln Management Service on ACM hub

Eliminating the Persistence Service removes the need for per-cluster
PostgreSQL, a custom REST API, and the RBAC mapping problem on that API.
See the [Data Architecture](#data-architecture) section for details.

---

## CRD Design

ACS Next is CRD-first. Configuration, credentials, policies, and security data are all represented as Kubernetes Custom Resources.

### Design Principles

1. **Separate CRDs for shared concerns** — Registries, notifiers, and other shared configurations are standalone CRDs, not inline in component specs
2. **Credentials via K8s Secrets** — ACS Next references Secrets; credential lifecycle is external (ESO, Sealed Secrets, Vault, Workload Identity)
3. **Label-based selection** — Components discover configuration via label selectors, not explicit references
4. **Status subresource for workflows** — Approval workflows use `/status` subresource with separate RBAC
5. **Summary-level output CRs** — Output CRs contain summary data only; full CVE-level detail is served by Scanner on demand

### Credential Management

ACS Next does **not** implement credential storage or rotation. Credentials are K8s Secrets, managed by user's existing tooling:

| Approach | Use Case |
|----------|----------|
| Manual `kubectl create secret` | Simple, low-scale |
| External Secrets Operator | Vault, AWS Secrets Manager, Azure Key Vault, GCP Secret Manager |
| Sealed Secrets | GitOps with encrypted secrets in repo |
| CSI Secret Store Driver | Mount secrets without K8s Secret objects |
| IRSA / Workload Identity | Cloud-native (no static credentials for cloud services) |

### Configuration CRDs

#### ImageRegistry

Defines a container registry that Scanner can pull from:

```yaml
apiVersion: acs.openshift.io/v1
kind: ImageRegistry
metadata:
  name: ecr-prod
  labels:
    env: production
spec:
  type: ecr  # ecr, gcr, acr, quay, docker, generic
  endpoint: 123456789.dkr.ecr.us-east-1.amazonaws.com
  credentialsRef:
    name: ecr-creds
    key: config.json
  # Alternative: use workload identity (no credentials)
  # useWorkloadIdentity: true
```

Scanner discovers registries via label selector:

```yaml
apiVersion: acs.openshift.io/v1
kind: Scanner
spec:
  registrySelector:
    matchLabels:
      env: production
```

#### Notifier

Defines a notification target:

```yaml
apiVersion: acs.openshift.io/v1
kind: Notifier
metadata:
  name: slack-security
spec:
  type: slack  # slack, teams, pagerduty, jira, splunk, webhook, etc.
  slack:
    webhookRef:
      name: slack-webhook
      key: url
    channel: "#security-alerts"
---
apiVersion: acs.openshift.io/v1
kind: Notifier
metadata:
  name: jira-security
spec:
  type: jira
  jira:
    url: https://company.atlassian.net
    project: SEC
    credentialsRef:
      name: jira-creds
```

Policies reference notifiers directly:

```yaml
apiVersion: acs.openshift.io/v1
kind: SecurityPolicy
spec:
  name: no-privileged-containers
  severity: High
  notifierRefs:
    - slack-security
    - jira-security
```

#### VulnerabilityException

Supports approval workflow via status subresource:

```yaml
apiVersion: acs.openshift.io/v1
kind: VulnerabilityException
metadata:
  name: cve-2024-1234-nginx
spec:
  # Requester sets spec (requires vulnerabilityexceptions create/update)
  cves:
    - CVE-2024-1234
  scope:
    images:
      - "registry.example.com/nginx:*"
  expiration:
    type: time
    until: "2026-06-01"
  reason: "Mitigated by network policy; patch not yet available"
status:
  # Approver sets status (requires vulnerabilityexceptions/status update)
  approval:
    state: Approved  # Pending, Approved, Denied
    approvedBy: security-team@example.com
    timestamp: "2026-02-27T10:00:00Z"
    comment: "Reviewed; network isolation confirmed"
```

**RBAC:**
* `vulnerabilityexceptions` (create/update) → developers, requesters
* `vulnerabilityexceptions/status` (update) → security approvers only

### Output CRDs (Created by Components)

These CRDs are created by ACS components to expose security data. They
contain **summary-level data only** — full vulnerability details are
served by Scanner on demand.

#### PolicyViolation

Created by CRD Projector when policies are violated:

```yaml
apiVersion: acs.openshift.io/v1
kind: PolicyViolation
metadata:
  name: deploy-nginx-privileged-abc123
  namespace: production
  labels:
    policy: no-privileged-containers
    severity: high
    resource-kind: Deployment
    resource-name: nginx
spec:
  policy: no-privileged-containers
  resource:
    kind: Deployment
    name: nginx
    namespace: production
  violations:
    - message: "Container 'nginx' is running as privileged"
      container: nginx
status:
  state: Active
  firstSeen: "2026-02-27T10:30:00Z"
  lastSeen: "2026-02-27T10:30:00Z"
```

#### ImageScanSummary

Created by CRD Projector with summary-level vulnerability counts:

```yaml
apiVersion: acs.openshift.io/v1
kind: ImageScanSummary
metadata:
  name: sha256-abc123
  labels:
    image: registry.example.com/nginx
spec:
  image: registry.example.com/nginx@sha256:abc123
  lastScanned: "2026-03-10T10:00:00Z"
  summary:
    critical: 2
    high: 5
    medium: 12
    low: 23
  topCVEs:
    - cve: CVE-2024-1234
      severity: Critical
      component: openssl
      fixedIn: "1.1.1t"
    - cve: CVE-2024-5678
      severity: Critical
      component: curl
      fixedIn: "8.5.0"
status:
  affectedDeployments:
    - namespace: production
      name: nginx
    - namespace: staging
      name: nginx
```

**Note:** Full CVE-level detail is not stored as CRs. The Console plugin
calls Scanner directly for drill-down into the complete vulnerability list.

#### NodeVulnerability

Created by CRD Projector for node/host vulnerability data:

```yaml
apiVersion: acs.openshift.io/v1
kind: NodeVulnerability
metadata:
  name: node1-cve-2024-5678
  labels:
    node: node1
    severity: high
spec:
  node: node1
  cve: CVE-2024-5678
  severity: High
  cvss: 7.5
  component: openssl
  version: "1.1.1k"
  fixedIn: "1.1.1t"
status:
  nodeInfo:
    os: "Red Hat Enterprise Linux CoreOS"
    kernelVersion: "5.14.0-284.30.1.el9_2.x86_64"
```

**Node scanning flow:**
1. Collector (DaemonSet) scans host filesystem for installed packages
2. Collector publishes `node-index` to broker
3. Scanner matcher receives node index, matches against vulnerability DB
4. Scanner publishes node vulnerabilities to `vulnerabilities` feed
5. CRD Projector creates `NodeVulnerability` CRs

### CRD Inventory

| CRD | Category | Owner/Controller | Purpose |
|-----|----------|------------------|---------|
| **Installation** | | | |
| `ACSSecuredCluster` | Config | Top-level operator | Installation profile, component selection |
| `Scanner` | Config | Scanner operator | Scanner deployment configuration |
| `Collector` | Config | Collector operator | Collector deployment configuration |
| `Broker` | Config | Broker operator | Event hub configuration |
| **Configuration** | | | |
| `ImageRegistry` | Config | Scanner (watches) | Container registry credentials |
| `Notifier` | Config | External Notifiers (watches) | Notification target credentials |
| `SignatureVerifier` | Config | Admission Control (watches) | Cosign/Sigstore public keys |
| `ReportConfiguration` | Config | Vuln Management Service (watches) | Scheduled report configuration |
| **Policies** | | | |
| `SecurityPolicy` | Policy | Policy engine (embedded) | Security policy definitions |
| `VulnerabilityException` | Policy | Scanner, CRD Projector | Exception with approval workflow |
| `NetworkBaseline` | Policy | Baselines | Learned network patterns |
| `ProcessBaseline` | Policy | Baselines | Learned process patterns |
| **Output** | | | |
| `PolicyViolation` | Output | CRD Projector (creates) | Active policy violations |
| `ImageScanSummary` | Output | CRD Projector (creates) | Image vulnerability summary (counts + top CVEs) |
| `NodeVulnerability` | Output | CRD Projector (creates) | Node/host vulnerability records |
| `WorkloadRisk` | Output | Risk Scorer (creates) | Risk scores per workload |
| `BaselineAnomaly` | Output | Baselines (creates) | Detected anomalies |

### Fleet Distribution

ACM Governance distributes configuration CRDs fleet-wide:

```yaml
apiVersion: policy.open-cluster-management.io/v1
kind: Policy
metadata:
  name: security-registries
spec:
  remediationAction: enforce
  policy-templates:
    - objectDefinition:
        apiVersion: acs.openshift.io/v1
        kind: ImageRegistry
        metadata:
          name: corporate-registry
        spec:
          type: quay
          endpoint: quay.example.com
          credentialsRef:
            name: quay-creds
```

This enables:
* Consistent registry configuration across fleet
* Centralized notifier management
* Policy distribution (SecurityPolicy CRDs)
* Exception distribution (for fleet-wide exceptions)

---

## Open Questions

1. **JetStream retention and recovery**:
   * What retention window per stream? (15 min vs 1 hour vs 24 hours)
   * How fast must consumers catch up after restart?
   * Backpressure policy: drop oldest events or block publishers?
   * PVC sizing: Initial estimates suggest ~150-300 MB per cluster, but needs validation

2. **Cross-component protocol**: gRPC for everything, or mix of gRPC and native K8s watches for CRD-watching components?

3. **Scanner placement**: Local scanner per cluster, or hub scanner via ACM transports (Maestro), or both?

4. **Risk Scorer output**: Does Risk Scorer publish back to broker, or directly to consumers? (Affects whether risk is a "feed" or a "query")

5. **ACM RBAC validation**: Confirm ManagedClusterSet RBAC works for filtering Vuln Management Service data (see [Open Question: ACM RBAC Validation](#open-question-acm-rbac-validation))

6. **Scanner drill-down**: Validate Scanner's existing API supports the Console plugin's needs for per-image CVE listing

## Resolved Questions

* **Broker implementation**: Embedded NATS in custom `acs-broker` Go binary. NATS is a library dependency, not a separate operator. JetStream for durability, leaf nodes for cross-cluster subscription.
* **ACM Addon subscription protocol**: NATS leaf nodes over mTLS. Addon connects as leaf subscriber to each managed cluster's broker on port 7422.
* **Policy Engine placement**: Embedded in sources (Collector, Admission Control, Scanner) for low-latency evaluation
* **CR cardinality**: Solved by summary-level CRs (PolicyViolation, ImageScanSummary) + Scanner drill-down for full CVE data. ACM addon direct subscription for fleet aggregation.
* **Credential management**: K8s Secrets referenced by CRDs; lifecycle handled by External Secrets Operator, Sealed Secrets, or Workload Identity
* **Vulnerability exceptions**: CRD with status subresource; K8s RBAC separates requesters from approvers
* **Persistence Service**: Eliminated at per-cluster level. Replaced by Prometheus metrics (trends), External Notifiers (event history), Scanner (CVE drill-down), and CRDs (active violations/summaries). Fleet-level persistence is the Vuln Management Service on the hub.

---

## Comparison to Current Architecture

| Aspect | Current ACS | ACS Next |
|--------|-------------|----------|
| Data aggregation | Central pulls from Sensors | Vuln Management Service subscribes via NATS leaf nodes |
| Multi-cluster | Central is the hub | ACM hub + Vuln Management Service |
| Messaging | Custom gRPC (Sensor → Central) | Embedded NATS with JetStream |
| Storage (per-cluster) | Central PostgreSQL | No custom persistent API — CRDs + Prometheus + Scanner |
| Storage (fleet) | Central PostgreSQL | Vuln Management Service (SQLite or BYODB PostgreSQL) |
| Extensibility | Modify Central | Add new broker subscriber |
| Minimum footprint | Central + Sensor + Scanner | Collector + Scanner + ACS Broker (~50MB) |
| RBAC (per-cluster) | Central SAC | K8s RBAC (native, on CRDs) |
| RBAC (fleet) | Central SAC | Cluster-scoped via ManagedClusterSet |
| Security data path | Via K8s API (Sensor → Central) | NATS leaf nodes (Broker → Vuln Management Service) |

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
   Scanner drill-down
7. **Evaluate scanner options**: Local vs hub vs hybrid
8. **Notifier parity audit**: Which of the 14 current notifier types are P0 for ACS Next?

---

*This document describes the proposed architecture for ACS Next. It is a
starting point for discussion, not a final design. The data architecture
section reflects analysis as of March 2026.*
