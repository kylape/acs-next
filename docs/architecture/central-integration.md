# Central Integration: Transitional Architecture

*Status: Draft | Date: 2026-03-18*

---

## Overview

This document describes how ACS-Next integrates with Central during the transition period. Rather than building a separate Vuln Management Service from scratch, Central gains a NATS adapter to consume events from ACS-Next clusters, positioning Central to eventually evolve into the Vuln Management Service.

**Key principles:**

* ACS-Next fully replaces secured cluster components (Sensor, Collector, legacy Scanner)
* Central remains the multi-cluster aggregation layer
* Data flows up via NATS (spoke to hub)
* Configuration flows down via GitOps/ACM (not via NATS)
* Central evolves toward Vuln Management Service by shedding unnecessary components

---

## Architecture

```mermaid
graph TB
    subgraph central["Central / Vuln Management Service"]
        adapter["NATS Adapter<br/>(ingest)"]
        scanner["Scanner<br/>(Matcher + Vuln DB)"]
        storage["PostgreSQL<br/>(storage)"]
        query["Fleet Query API"]
        aggregation["Aggregation Engine"]

        adapter --> storage
        adapter --> aggregation
        scanner --> storage
        aggregation --> query
    end

    subgraph clusterA["Cluster A (ACS-Next)"]
        brokerA["Broker"]
        collectorA["Collector"]
        admissionA["Admission Controller"]
        runtimeA["Runtime Evaluator"]
        coordA["Scanner Coordinator"]
        riskA["Risk Scorer"]

        collectorA --> brokerA
        admissionA --> brokerA
        runtimeA --> brokerA
        coordA --> brokerA
        riskA --> brokerA
    end

    subgraph clusterB["Cluster B (ACS-Next)"]
        brokerB["Broker"]
        componentsB["ACS-Next Components"]
        componentsB --> brokerB
    end

    brokerA -->|NATS leaf node| adapter
    brokerB -->|NATS leaf node| adapter
    coordA -->|scan requests| scanner

    gitops["GitOps / ACM"]
    gitops -->|policies, config| clusterA
    gitops -->|policies, config| clusterB
```

---

## Data Flow Principles

### Unidirectional NATS: Data Up Only

NATS connections are unidirectional for events and telemetry. Central receives data from clusters but does not push configuration via NATS.

| Direction | Transport | Content |
|-----------|-----------|---------|
| **Spoke to Hub** | NATS | Events, scan results, risk scores, violations |
| **Hub to Spoke** | GitOps/ACM | Policies, config, exceptions |

**Why unidirectional for config?**

* Encourages GitOps adoption (ArgoCD, Flux)
* Leverages ACM Governance for policy distribution
* No proprietary sync protocol to maintain
* Clear separation: real-time data vs declarative config

**Exception:** Scan request/response is bidirectional (see [Hub Scanner](#hub-scanner) below). This is data flow, not configuration.

### Subjects Flowing to Central

| Subject | Content | Retention |
|---------|---------|-----------|
| `acs.<cluster-id>.process-events` | Process start/stop, exec | Short (aggregation) |
| `acs.<cluster-id>.network-flows` | Connection events | Short (aggregation) |
| `acs.<cluster-id>.policy-violations` | Violations from all sources | Long (audit) |
| `acs.<cluster-id>.risk-scores` | Per-workload risk scores | Short (latest only) |
| `acs.<cluster-id>.node-index` | Host package inventory | Short (latest only) |
| `acs.hub.scan-requests` | Scan requests to hub | Until processed |
| `acs.hub.scan-responses.<cluster-id>` | Scan results from hub | Until consumed |

---

## NATS Adapter in Central

Central gains a new subsystem that accepts NATS leaf node connections from ACS-Next clusters.

### Connection Model

Clusters initiate connections to Central (matching existing Sensor model):

```mermaid
graph LR
    subgraph cluster["Cluster"]
        broker["Broker<br/>(:7422 leaf)"]
    end

    subgraph central["Central"]
        listener["NATS Leaf Listener<br/>(:7422)"]
        adapter["NATS Adapter"]
        listener --> adapter
    end

    broker -->|mTLS| listener
```

* Cluster init bundle includes Central's NATS endpoint and credentials
* mTLS authentication between cluster broker and Central
* Leaf node configuration controls which subjects cross the boundary

### Leaf Node Configuration

```yaml
# Cluster broker leafnode config
leafnodes:
  remotes:
    - url: "nats://central.example.com:7422"
      credentials: "/etc/nats/cluster-creds.creds"

      # Subjects sent to hub
      publish:
        - "acs.hub.>"              # Hub-bound requests
        - "acs.<cluster-id>.>"     # Cluster events

      # Subjects received from hub
      subscribe:
        - "acs.hub.scan-responses.<cluster-id>"
```

### Adapter Responsibilities

| Function | Description |
|----------|-------------|
| **Accept connections** | Leaf node listener for cluster brokers |
| **Subscribe to subjects** | `acs.*.process-events`, `acs.*.policy-violations`, etc. |
| **Translate events** | ACS-Next protobuf to Central internal model |
| **Route to handlers** | Feed existing detection/storage pipeline |
| **Cluster lifecycle** | Handle cluster add/remove, reconnects |
| **Metrics** | Connection status, message rates, lag |

### Translation Layer

The adapter translates ACS-Next event schemas to Central's internal types:

```mermaid
graph LR
    nats["NATS Message<br/>(ACS-Next protobuf)"]
    translate["Translation Layer"]
    internal["Central Internal Type"]
    pipeline["Detection/Storage Pipeline"]

    nats --> translate --> internal --> pipeline
```

This is the only place where ACS-Next and Central schemas couple. Changes to either require updating the translation layer.

---

## Hub Scanner

Scanner runs on the hub (Central) to reduce per-cluster footprint and centralize vulnerability database management.

### Scan Request/Response Flow

```mermaid
sequenceDiagram
    participant AC as Admission Controller
    participant Broker as Broker (per-cluster)
    participant Coord as Scanner Coordinator
    participant Leaf as NATS Leaf Node
    participant Adapter as Central NATS Adapter
    participant Scanner as Scanner (Hub)

    AC->>Broker: publish scan-requests
    Broker->>Coord: subscribe
    Coord->>Coord: check cache

    alt Cache Hit
        Coord->>Broker: publish scan-responses
        Broker->>AC: cached result
    else Cache Miss
        Coord->>Broker: publish hub.scan-requests
        Broker->>Leaf: forward to hub
        Leaf->>Adapter: receive request
        Adapter->>Scanner: index + match
        Scanner->>Adapter: scan result
        Adapter->>Leaf: publish hub.scan-responses.cluster-id
        Leaf->>Broker: forward to cluster
        Broker->>Coord: receive response
        Coord->>Coord: update cache
        Coord->>Broker: publish scan-responses
        Broker->>AC: scan result
    end
```

### Subject Design

| Subject | Direction | Purpose |
|---------|-----------|---------|
| `acs.scan-requests` | Local | Components request scans |
| `acs.hub.scan-requests` | Spoke to Hub | Coordinator forwards to Central |
| `acs.hub.scan-responses.<cluster-id>` | Hub to Spoke | Central returns results |
| `acs.scan-responses` | Local | Coordinator forwards to requesters |

The `hub.` prefix distinguishes subjects that cross the leaf node boundary.

### Scanner Coordinator

The Scanner Coordinator is a per-cluster component that mediates between local components and the hub Scanner.

```mermaid
graph TB
    subgraph cluster["Per-Cluster"]
        ac["Admission Controller"]
        re["Runtime Evaluator"]
        coord["Scanner Coordinator"]
        cache["Local Cache"]
        broker["Broker"]

        ac -->|scan request| broker
        re -->|scan request| broker
        broker --> coord
        coord --> cache
        coord -->|hub request| broker
        broker -->|response| coord
        coord -->|response| broker
        broker --> ac
        broker --> re
    end

    subgraph hub["Hub"]
        scanner["Scanner"]
    end

    broker <-->|NATS leaf| hub
```

**Coordinator responsibilities:**

| Function | Benefit |
|----------|---------|
| **Deduplication** | Same image requested by multiple components becomes one hub request |
| **Caching** | Recently scanned images served locally |
| **Batching** | Aggregate requests before sending to hub |
| **Retry/circuit breaker** | Handle hub unavailability gracefully |
| **Request correlation** | Match responses to original requesters |
| **Metrics** | Scan latency, cache hit rate, hub availability |

### Latency Mitigation

Hub scanning adds network latency to admission decisions. Mitigation strategies:

| Strategy | Description |
|----------|-------------|
| **Aggressive caching** | Cache results for hours/days (vulns change slowly) |
| **Pre-scanning** | Scan on image pull, before admission |
| **Fleet-wide cache** | Central caches results; if Cluster A scanned image, Cluster B gets cached result |
| **Optimistic allow** | Allow admission, scan async, alert on violations |
| **Local fallback** | If hub unavailable, use stale cache or skip CVE check |

**Pre-scanning flow:**

```mermaid
sequenceDiagram
    participant Kubelet
    participant Collector
    participant Coord as Scanner Coordinator
    participant Hub as Hub Scanner

    Note over Kubelet,Hub: Image Pull (before admission)
    Kubelet->>Collector: pull image
    Collector->>Coord: scan request (async)
    Coord->>Hub: forward to hub
    Hub->>Coord: scan result
    Coord->>Coord: cache result

    Note over Kubelet,Hub: Admission (later)
    Kubelet->>Coord: scan request
    Coord->>Coord: cache hit
    Coord->>Kubelet: immediate response
```

### Fleet-Wide Caching

Central can cache scan results across all clusters:

```mermaid
graph TB
    subgraph clusterA["Cluster A"]
        coordA["Coordinator"]
    end

    subgraph clusterB["Cluster B"]
        coordB["Coordinator"]
    end

    subgraph central["Central"]
        cache["Fleet Cache"]
        scanner["Scanner"]
    end

    coordA -->|"scan nginx:1.25"| central
    scanner -->|result| cache
    cache -->|result| coordA

    coordB -->|"scan nginx:1.25"| central
    cache -->|"cache hit"| coordB
```

Common base images (nginx, redis, postgres, ubi) get scanned once for the entire fleet.

---

## Per-Cluster Components

ACS-Next fully replaces the secured cluster stack:

| Component | Responsibility |
|-----------|----------------|
| **Broker** | Event hub, NATS leaf node to Central |
| **Collector** | eBPF runtime events, node indexing |
| **Admission Controller** | Deploy-time policy enforcement |
| **Runtime Evaluator** | Runtime policy evaluation |
| **Scanner Coordinator** | Scan request routing, caching |
| **Risk Scorer** | Composite risk calculation |
| **CRD Projector** | Summary CRs for Console visibility |
| **Notifiers** | Per-cluster alerting |

### Risk Scorer

Risk Scorer is a new per-cluster component that computes composite risk scores:

```mermaid
graph LR
    vulns["vulnerabilities"]
    violations["policy-violations"]
    runtime["runtime-events"]

    scorer["Risk Scorer"]

    vulns --> scorer
    violations --> scorer
    runtime --> scorer

    scorer --> scores["risk-scores"]
    scores --> projector["CRD Projector"]
    scores --> central["Central (aggregation)"]
```

* Subscribes to relevant subjects
* Computes risk per workload using configurable weights
* Publishes to `acs.risk-scores`
* CRD Projector can annotate Deployments
* Central aggregates for fleet-level risk views

---

## Central Evolution Path

The NATS adapter enables Central to evolve toward Vuln Management Service:

```mermaid
graph TB
    subgraph phase1["Phase 1: Add NATS Adapter"]
        central1["Central"]
        sensor1["Sensor Handler (gRPC)"]
        nats1["NATS Adapter"]
        central1 --- sensor1
        central1 --- nats1
    end

    subgraph phase2["Phase 2: Migrate Clusters"]
        central2["Central"]
        sensor2["Sensor Handler<br/>(decreasing traffic)"]
        nats2["NATS Adapter<br/>(increasing traffic)"]
        central2 --- sensor2
        central2 --- nats2
    end

    subgraph phase3["Phase 3: Remove Sensor Path"]
        central3["Central"]
        nats3["NATS Adapter<br/>(only path)"]
        central3 --- nats3
    end

    subgraph phase4["Phase 4: Strip Down"]
        vms["Vuln Management Service"]
        nats4["NATS Adapter"]
        storage4["PostgreSQL"]
        query4["Fleet Query API"]
        vms --- nats4
        vms --- storage4
        vms --- query4
    end

    phase1 --> phase2 --> phase3 --> phase4
```

### Components Removed During Evolution

| Component | Disposition | Rationale |
|-----------|-------------|-----------|
| Sensor gRPC handler | Remove | Replaced by NATS adapter |
| Policy engine | Remove | ACS-Next Runtime Evaluator handles per-cluster |
| Admission coordination | Remove | ACS-Next Admission Controller handles per-cluster |
| Compliance framework | Remove | compliance-operator handles this |
| Network graph builder | Remove | Per-cluster concern |
| Image scan coordination | Remove | ACS-Next Scanner Coordinator handles per-cluster |

### Components Retained (Become VMS)

| Component | Purpose in VMS |
|-----------|----------------|
| NATS Adapter | Ingest from all clusters |
| Scanner (Matcher) | CVE matching, vuln DB |
| PostgreSQL | Historical storage |
| Aggregation Engine | Fleet-wide rollups |
| Query API | Fleet CVE queries |
| Reporting | Scheduled reports, exports |

---

## Configuration Distribution

Configuration flows via GitOps and ACM, not NATS:

```mermaid
graph TB
    subgraph hub["Hub"]
        git["Git Repository"]
        acm["ACM Governance"]
        argo["ArgoCD"]
    end

    subgraph clusterA["Cluster A"]
        crdsA["CRDs"]
    end

    subgraph clusterB["Cluster B"]
        crdsB["CRDs"]
    end

    git --> argo
    git --> acm
    argo -->|sync| crdsA
    acm -->|distribute| crdsA
    argo -->|sync| crdsB
    acm -->|distribute| crdsB
```

**Configuration CRDs:**

| CRD | Purpose |
|-----|---------|
| `StackroxPolicy` | Security policies |
| `VulnException` | Vulnerability exceptions |
| `Notifier` | Alerting integrations |
| `ImageRegistry` | Registry credentials |
| `SignatureVerifier` | Signature verification config |

This approach:

* Leverages existing GitOps tooling
* Uses ACM Governance for policy distribution
* Provides audit trail via git history
* Enables PR-based review for policy changes
* Requires no proprietary sync protocol

---

## Failure Modes

| Failure | Impact | Handling |
|---------|--------|----------|
| **Hub unreachable** | Scan requests queue locally | Circuit breaker, serve from cache |
| **Cluster broker crash** | Events buffered in JetStream | Replay on recovery |
| **NATS Adapter overloaded** | Backpressure to clusters | Horizontal scaling, rate limiting |
| **Scanner overloaded** | Scan latency increases | Queue depth monitoring, scaling |
| **Stale cache** | Old vuln data used | TTL on cache, background refresh |

---

## Open Questions

1. **Scan result granularity:** Full CVE list vs summary vs policy-relevant subset?

2. **Cache TTL:** How long are scan results valid? Hours? Days?

3. **Risk score aggregation:** Does Central compute fleet-level risk, or just store per-workload scores?

4. **Compliance rollup:** Who owns multi-cluster compliance UI? OCP Console? ACM?

5. **Legacy cluster support:** How long do we maintain the Sensor gRPC path alongside NATS?

---

## Related Documents

* [Architecture Overview](README.md)
* [Broker](components/broker.md)
* [Scanner](components/scanner.md)
* [Multi-Cluster](multi-cluster.md)
* [Migration Guide](../migration.md)

---

*This document describes the transitional architecture for Central integration. It will evolve as implementation progresses.*
