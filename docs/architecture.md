# ACS Next: Detailed Architecture

*Status: Draft | Date: 2026-02-27*

---

## Overview

ACS Next is a single-cluster security platform built on an **event-driven architecture**. At its core is an **Event Hub**—an embedded pub/sub broker that aggregates all security data streams and allows consumers to subscribe to feeds of interest.

This design enables:
* **Decoupled components**: Producers and consumers evolve independently
* **Flexible deployment**: Users choose which consumers to run based on their needs
* **Minimal footprint option**: CRD-only deployment without PostgreSQL
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
│  │ Persistence │ │  Alerting   │ │  External   │ │    Risk     │ │ Baselines │  │     │
│  │   Service   │ │   Service   │ │  Notifiers  │ │   Scorer    │ │           │  │     │
│  │(PostgreSQL) │ │(AlertMgr)   │ │(Jira,Splunk)│ │             │ │           │  │     │
│  └─────────────┘ └─────────────┘ └─────────────┘ └─────────────┘ └───────────┘  │     │
│                                                                                 │     │
│  ┌─────────────────────────────────────────────────────────────────────────┐    │     │
│  │  CRD Projector (optional) — powers OCP Console without DB dependency   │    │     │
│  └─────────────────────────────────────────────────────────────────────────┘    │     │
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
│  │                               ACS Addon                                         │  │
│  │   • Subscribes directly to Broker feeds from all managed clusters              │  │
│  │   • Aggregates security data fleet-wide (vulnerabilities, violations, risk)    │  │
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

This mirrors current ACS where Sensor evaluates policies locally with deployment context + runtime events. ACS Next is the same pattern—policies from CRDs instead of Central gRPC, but same local evaluation model.

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

**Key constraint:** Admission webhooks are synchronous. The policy engine for deploy-time MUST be in the admission path. A separate Policy Evaluator service would add latency and a hard dependency—if it's down, nothing deploys.

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

Admission Controller is critical path—keeping it lean and isolated from unpredictable runtime workloads is safer.

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

Image signature verification (Cosign, Sigstore) is primarily a **deploy-time** concern—blocking unsigned images before they run.

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

The Broker is the central nervous system—a pub/sub message broker that:

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

**Decision:** The ACS Broker is a custom Go binary that embeds the NATS server as a library. NATS is not deployed as a separate operator—it's an implementation detail inside our broker process.

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
│  │                      ACS Addon                                 │  │
│  │  • Connects as NATS leaf subscriber to all managed clusters   │  │
│  │  • Subscribes to: acs.*.policy-violations, acs.*.vulnerabilities│  │
│  │  • Aggregates security data fleet-wide                        │  │
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

#### ACM Addon (External)

* **What it does**: Aggregates security data across fleet, feeds OCP Console multi-cluster views
* **Subscribes to**: All feeds via mTLS
* **Outputs**: Fleet-wide dashboards, multi-cluster queries
* **Deployment**: Runs on ACM hub, subscribes to Event Hubs on all managed clusters
* **Notes**: Primary aggregation path for ACM-managed deployments; see [External Subscribers](#external-subscribers-acm-addon)

#### CRD Projector (Optional)

* **What it does**: Projects security data into Kubernetes CRs
* **Subscribes to**: `policy-violations`, `vulnerabilities`, `risk-scores`
* **Outputs**: `PolicyViolation`, `ImageVulnerability`, `NodeVulnerability`, `WorkloadRisk` CRs
* **When needed**: Standalone clusters (no ACM), or local OCP Console visibility
* **Key design**: OCP Console is powered by these CRs—no DB required for basic visibility

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

#### Persistence Service (Optional)

* **What it does**: Stores security data in PostgreSQL, exposes REST API
* **Subscribes to**: All feeds (configurable)
* **Outputs**: PostgreSQL tables, REST/GraphQL API
* **Why optional**: OCP Console works with CRDs alone; DB enables extended queries
* **RBAC model**: K8s RBAC-based—service accounts granted coarse-grained access to DB

**When to deploy:**
* Extended query functionality in OCP Console (CVE trends, affected images, historical data)
* Historical data retention beyond CR limits
* Complex risk analytics
* Third-party integrations that expect REST APIs

**Architecture:**

```
┌─────────────────────────────────────────────────────────┐
│                  Persistence Service                     │
│                                                          │
│  ┌──────────────┐    ┌──────────────┐    ┌────────────┐ │
│  │   Ingester   │───►│  PostgreSQL  │◄───│  REST API  │ │
│  │ (subscriber) │    │              │    │            │ │
│  └──────────────┘    └──────────────┘    └────────────┘ │
│         ▲                                      ▲        │
│         │                                      │        │
└─────────┼──────────────────────────────────────┼────────┘
          │ Broker                               │ HTTP
          │                                      │
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
* **Notes**: Maintains parity with current ACS notifier integrations

#### Risk Scorer (Optional)

* **What it does**: Calculates composite risk scores for workloads
* **Subscribes to**: Broker feeds (`vulnerabilities`, `policy-violations`, `runtime-events`)
* **Outputs**: Risk scores (publishes back to broker for other consumers)
* **Why separate**: Allows independent scaling; customers want configurable risk calculation
* **Notes**: Designed for configurability—users adjust weights, factor in business context

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

## Compliance Operator Integration

**Decision**: Compliance operator integration is dissolved in ACS Next.

* Compliance operator management moves directly into OCP Console
* ACS no longer wraps or proxies compliance operator functionality
* Security policies (ACS) and compliance policies (compliance-operator) are separate concerns
* Users configure compliance operator directly; results visible in OCP Console

This simplifies ACS Next scope and avoids duplicating OCP-native compliance tooling.

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
| Persistence Service | Deployment | ~200-500MB + PostgreSQL | 100-500m | PostgreSQL: 2-8GB typical |
| Alerting Service | Deployment | ~100-200MB | 50-100m | AlertManager integration |
| Risk Scorer | Deployment | ~200-500MB | 100-500m | Depends on cluster size |
| Baselines | Deployment | ~200-500MB | 100-500m | ML/statistical models |

### Profile: Minimal (ACM-managed)

```
Components: Broker + Collector + Admission Control + Scanner (hub-matcher mode)
Footprint:  ~1-1.5GB cluster-wide + ~500-750MB per node (Collector)
Storage:    None (stateless)
Use case:   Fleet visibility via ACM addon direct subscription
Notes:      Scanner matcher on hub; ACM addon aggregates data
```

### Profile: Standalone (no ACM)

```
Components: Minimal + CRD Projector + Scanner (local)
Footprint:  ~2-3GB cluster-wide + ~500-750MB per node
Storage:    None (CRs stored in etcd)
Use case:   Single-cluster with local OCP Console visibility
Notes:      Full scanner local; CRs power OCP Console
```

### Profile: Standard

```
Components: Standalone + Persistence Service + Alerting Service
Footprint:  ~2.5-4GB cluster-wide + ~500-750MB per node + PostgreSQL
Storage:    PostgreSQL 2-8GB (depends on retention)
Use case:   Extended queries in OCP Console, historical data, OCP alerting
```

### Profile: Enterprise

```
Components: Standard + External Notifiers + Risk Scorer + Baselines
Footprint:  ~3-5GB cluster-wide + ~500-750MB per node + PostgreSQL
Storage:    PostgreSQL 4-16GB (depends on retention and cluster count)
Use case:   Full feature set with integrations, configurable risk, anomaly detection
```

### Profile: Edge (minimal on-cluster)

```
Components: Broker + Collector only (everything else on hub)
Footprint:  ~600-950MB cluster-wide + ~500-750MB per node
Storage:    None
Use case:   Resource-constrained edge clusters
Notes:      Scanner, Risk, Baselines, Persistence all on hub cluster
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
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌───────────┐  │
│  │ Scanner │ │  Risk   │ │Baselines│ │Alerting │ │Persistence│  │
│  │(matcher)│ │ Scorer  │ │         │ │ Service │ │  Service  │  │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └───────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

**Edge cluster footprint:** ~150-200MB (Collector + Broker only)

### Hub-Based DB Pattern

DB Persistence can run on a separate cluster, with broker streaming data via ACM transport:

```
┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐
│  Secured Cluster │  │  Secured Cluster │  │  Secured Cluster │
│      A           │  │      B           │  │      C           │
│  ┌────────────┐  │  │  ┌────────────┐  │  │  ┌────────────┐  │
│  │   Broker   │  │  │  │   Broker   │  │  │  │   Broker   │  │
│  └─────┬──────┘  │  │  └─────┬──────┘  │  │  └─────┬──────┘  │
└────────┼─────────┘  └────────┼─────────┘  └────────┼─────────┘
         │                     │                     │
         │ ACM transport       │                     │
         ▼                     ▼                     ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Hub Cluster                             │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                   Persistence Service                    │    │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐               │    │
│  │  │  DB: A   │  │  DB: B   │  │  DB: C   │  (built-in    │    │
│  │  │(cluster) │  │(cluster) │  │(cluster) │   sharding)   │    │
│  │  └──────────┘  └──────────┘  └──────────┘               │    │
│  └─────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────┘
```

**Benefits:**
* **Built-in sharding**: Each secured cluster gets its own DB on the hub
* **Reduced secured cluster footprint**: No PostgreSQL per cluster
* **Centralized operations**: Single DB cluster to manage

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
| Persistence Service | Optional | Preferred | Centralized is simpler |
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
  profile: minimal  # or standalone, standard, enterprise
  collector:
    enabled: true
  scanner:
    mode: hub-matcher  # local, hub-matcher, or hub-full
  persistence:
    enabled: false  # use hub-based persistence
```

The operator creates the necessary component CRs and manages their lifecycle.

---

## Data Flow Examples

### Example 1: Image Scan → Vulnerability CR

```
1. Scanner scans image "nginx:1.21"
2. Scanner publishes to "image-scans" feed:
   { image: "nginx:1.21", vulns: [...], sbom: {...} }
3. Scanner publishes to "vulnerabilities" feed:
   { image: "nginx:1.21", cve: "CVE-2024-1234", severity: "High", ... }
4. CRD Projector subscribes to "vulnerabilities"
5. CRD Projector creates/updates ImageVulnerability CR
6. ACM Search indexes the CR
7. User sees vulnerability in OCP Console multi-cluster view
```

### Example 2: Runtime Event → Policy Violation → Alert

```
1. Collector detects privileged container start
2. Collector publishes to Event Hub "runtime-events" feed:
   { pod: "nginx", container: "nginx", privileged: true, ... }
3. Embedded Policy Engine (in Event Hub) receives event
4. Policy Engine evaluates "no-privileged-containers" policy
5. Policy Engine publishes to "policy-violations" feed:
   { policy: "no-privileged-containers", resource: {...}, ... }
6. CRD Projector creates PolicyViolation CR (if deployed)
7. ACM Addon receives violation via direct subscription (if deployed)
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
5. Persistence Service stores for trend analysis (if deployed)
6. ACM Addon aggregates risk across fleet (if deployed)
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

### Why Persistence Service as optional?

Not everyone needs queryable historical data:
* **Small clusters**: CRs are sufficient
* **GitOps-heavy orgs**: Policies are source-controlled, violations are ephemeral
* **Cost-sensitive**: PostgreSQL adds resource overhead

Making it optional reduces minimum footprint and lets users pay for what they need.

---

## CRD Design

ACS Next is CRD-first. Configuration, credentials, policies, and security data are all represented as Kubernetes Custom Resources.

### Design Principles

1. **Separate CRDs for shared concerns** — Registries, notifiers, and other shared configurations are standalone CRDs, not inline in component specs
2. **Credentials via K8s Secrets** — ACS Next references Secrets; credential lifecycle is external (ESO, Sealed Secrets, Vault, Workload Identity)
3. **Label-based selection** — Components discover configuration via label selectors, not explicit references
4. **Status subresource for workflows** — Approval workflows use `/status` subresource with separate RBAC

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

These CRDs are created by ACS components to expose security data:

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

#### ImageVulnerability

Created by CRD Projector for vulnerability data:

```yaml
apiVersion: acs.openshift.io/v1
kind: ImageVulnerability
metadata:
  name: sha256-abc123-cve-2024-1234
  labels:
    image: registry.example.com/nginx
    severity: critical
spec:
  image: registry.example.com/nginx@sha256:abc123
  cve: CVE-2024-1234
  severity: Critical
  cvss: 9.8
  component: openssl
  version: "1.1.1"
  fixedIn: "1.1.1t"
status:
  affectedDeployments:
    - namespace: production
      name: nginx
    - namespace: staging
      name: nginx
```

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
| **Policies** | | | |
| `SecurityPolicy` | Policy | Policy engine (embedded) | Security policy definitions |
| `VulnerabilityException` | Policy | Scanner, CRD Projector | Exception with approval workflow |
| `NetworkBaseline` | Policy | Baselines | Learned network patterns |
| `ProcessBaseline` | Policy | Baselines | Learned process patterns |
| **Output** | | | |
| `PolicyViolation` | Output | CRD Projector (creates) | Active policy violations |
| `ImageVulnerability` | Output | CRD Projector (creates) | Image vulnerability records |
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

## Resolved Questions

* **Broker implementation**: Embedded NATS in custom `acs-broker` Go binary. NATS is a library dependency, not a separate operator. JetStream for durability, leaf nodes for cross-cluster subscription.
* **ACM Addon subscription protocol**: NATS leaf nodes over mTLS. Addon connects as leaf subscriber to each managed cluster's broker on port 7422.
* **Policy Engine placement**: Embedded in sources (Collector, Admission Control, Scanner) for low-latency evaluation
* **CR cardinality**: Solved by ACM addon direct subscription—CRs optional for local use only
* **Credential management**: K8s Secrets referenced by CRDs; lifecycle handled by External Secrets Operator, Sealed Secrets, or Workload Identity
* **Vulnerability exceptions**: CRD with status subresource; K8s RBAC separates requesters from approvers

---

## Comparison to Current Architecture

| Aspect | Current ACS | ACS Next |
|--------|-------------|----------|
| Data aggregation | Central pulls from Sensors | ACM addon subscribes via NATS leaf nodes |
| Multi-cluster | Central is the hub | Portfolio is the hub |
| Messaging | Custom gRPC (Sensor → Central) | Embedded NATS with JetStream |
| Storage | Central PostgreSQL | Optional per-cluster PostgreSQL |
| Extensibility | Modify Central | Add new broker subscriber |
| Minimum footprint | Central + Sensor + Scanner | Collector + Scanner + ACS Broker (~50MB) |
| RBAC | Central SAC | mTLS for NATS, K8s RBAC for CRs |
| Security data path | Via K8s API (Sensor → Central) | NATS leaf nodes (Broker → ACM addon) |

---

## Next Steps

1. **Prototype ACS Broker**: Build `acs-broker` binary with embedded NATS; validate footprint (~50-100MB) and JetStream durability
2. **Validate leaf node subscription**: Test ACM addon connecting as NATS leaf subscriber across cluster boundaries
3. **Finalize CR schemas**: Refine CRD definitions; validate ACM Search indexing
4. **Prototype CRD Projector**: Can we get policy violations into OCP Console via CRs?
5. **Evaluate scanner options**: Local vs hub vs hybrid
6. **Notifier parity audit**: Which of the 14 current notifier types are P0 for ACS Next?

---

*This document describes the proposed architecture for ACS Next. It is a starting point for discussion, not a final design.*
