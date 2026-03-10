# ACS Next: An Architecture for Product Flexibility

*Status: Draft | Date: 2026-03-02*

---

## Purpose

This document presents ACS Next—a proposed architectural shift that gives product management flexibility to position ACS however the market demands.

The current ACS architecture was designed for a different era: standalone deployment, Central as the hub, custom RBAC, proprietary protocols. It serves existing customers well. But it constrains how we can evolve the product.

ACS Next is an investment in architectural freedom. It doesn't dictate a specific product vision—it enables multiple visions.

---

## The Core Idea: Flexibility Through Architecture

**ACS Next gives PM room to maneuver.**

The current architecture makes certain product decisions for us. ACS Next returns those decisions to PM.

| Product Question | Current Architecture | ACS Next |
|------------------|---------------------|----------|
| "Can we offer a freemium tier?" | No — Central is all-or-nothing | **Yes** — CRDs free, advanced features in OPP |
| "Can we have different feature sets per deployment?" | No — Central is monolithic | **Yes** — optional components (Persistence, Risk Scorer, etc.) |
| "Can advanced search be OPP-only?" | No — search is baked into Central | **Yes** — Persistence Service is optional, can live on hub only |
| "Can we reduce footprint for edge?" | Limited — Central + Sensor is minimum | **Yes** — Broker + Collector only, hub provides the rest |
| "Can we align with K8s RBAC?" | Difficult — SAC is deeply embedded | **Yes** — K8s RBAC is native, no custom auth layer |
| "Can security feel like part of OCP?" | Difficult — Central is a separate product | **Yes** — CRDs, OCP Console, familiar tools |
| "Can other teams contribute security features?" | Difficult — requires Central context | **Yes** — subscribe to broker, publish CRDs |
| "Can ACS be a security platform for OCP, not a separate product?" | No — Central is architecturally separate | **Yes** — CRDs and broker enable platform model |
| "Can we support multi-tenant / vendor platform scenarios?" | Difficult — Central RBAC doesn't map to namespace tenancy | **Yes** — K8s RBAC + CRDs scope naturally to namespaces |

**These become PM decisions, not engineering constraints.**

Examples of positioning that ACS Next enables (but doesn't require):

* **Freemium model**: Basic security via CRDs (free with OCP), advanced search/trends via OPP subscription
* **Edge-optimized deployments**: Minimal on-cluster footprint, hub provides heavy lifting
* **Per-cluster vs. fleet features**: Different capabilities at different scopes
* **Portfolio-native experience**: Security that feels like part of ACM, not a bolt-on
* **Vendor platforms**: ACS customers with their own customers get namespace-level tenancy via K8s RBAC (support may be incremental—violations first, then policies, then reports)

The architecture accommodates multiple positioning choices. PM decides which ones to pursue.

---

## What is ACS Next?

ACS Next shifts from a Central-based model to a **single-cluster security platform** with **the OPP portfolio providing multi-cluster orchestration**.

**Current architecture:**
```
Central (hub) ──────► Sensor (cluster A)
      │
      ├──────────────► Sensor (cluster B)
      │
      └──────────────► Sensor (cluster C)

Central owns: aggregation, RBAC, UI, API, policy authority
```

**ACS Next architecture:**
```
OPP Multi-Cluster Orchestration (currently ACM)
  │  (Search, Governance, OCP Console multi-cluster perspective)
  │
  ├──► Cluster A: Collector + Scanner + Broker + K8s RBAC
  │
  ├──► Cluster B: Collector + Scanner + Broker + K8s RBAC
  │
  └──► Cluster C: Collector + Scanner + Broker + K8s RBAC

Each cluster owns: its own data, K8s RBAC, CRD-based configuration
Portfolio owns: fleet aggregation, policy distribution, multi-cluster views
```

*See [ACS_NEXT_ARCHITECTURE.md](ACS_NEXT_ARCHITECTURE.md) for detailed component design.*

**Key differences:**

| Aspect | Current | ACS Next |
|--------|---------|----------|
| Multi-cluster aggregation | Central | Portfolio (currently ACM) |
| Policy distribution | Central → Sensor gRPC | Portfolio (currently ACM) |
| RBAC | Central's SAC engine | K8s RBAC (native) |
| Fleet UI | Central UI | OCP Console multi-cluster perspective |
| Data storage | Central PostgreSQL | CRDs for real-time, optional PostgreSQL for history |
| Identity | Central auth providers | K8s + portfolio identity federation |
| Configuration | Central database | CRDs (GitOps-friendly) |

**Per-cluster components:**

* Collector (eBPF runtime data), Scanner, Admission Control
* Event Broker (pub/sub for security data streams)
* CRDs for configuration, policies, and security data projections
* Optional: PostgreSQL for extended query functionality

The Persistence Service enables historical queries, trend analysis, and advanced search—capabilities that require data beyond real-time CRDs.

**Core capabilities:**

* Vulnerability scanning and matching
* Runtime data collection (eBPF)
* Policy evaluation
* Third-party integrations (Jira, Splunk, Slack, etc.)

*Note: Implementation details may differ significantly. Code sharing between current ACS and ACS Next is possible but not guaranteed.*

---

## Why This Architecture?

### 1. Composability

ACS Next is built from discrete, optional components:

* **Required**: Collector, Scanner, Admission Control
* **Optional**: Persistence Service, Risk Scorer, Baselines, Historical Data

This composability is what enables different deployment profiles and product tiers. The current architecture's monolithic Central doesn't offer this flexibility.

### 2. K8s-Native Data Model

Security data as CRDs makes these straightforward:

* **K8s RBAC**: Standard access control, no custom auth layer
* **GitOps**: Policies in git, deployed via standard tooling
* **Portfolio aggregation**: ACM Search indexes CRs natively
* **OCP Console**: Security data appears alongside other workload data

### 3. Portfolio Alignment

Red Hat's multi-cluster strategy centers on ACM as the orchestrator. ACS Next aligns with this:

* ACM Governance distributes security policies (they're just CRDs)
* ACM Search aggregates security data (they're just CRs)
* OCP Console's multi-cluster perspective provides fleet visibility

We stop building parallel infrastructure for things the portfolio already provides.

This also means ACS engineers can focus on security—vulnerability detection, policy evaluation, runtime protection—rather than maintaining fleet management, cross-cluster coordination, and identity federation. Those are hard problems, but they're not security problems. Let the portfolio teams solve them.

### 4. Event-Driven Architecture

Pub/sub data flow (via embedded NATS) enables:

* **Extensibility**: New consumers subscribe to existing feeds
* **Decoupling**: Components evolve independently
* **Cross-cluster**: ACM addon subscribes as a NATS leaf node

### 5. Organizational Flexibility

Conway's Law works both ways. The current architecture's tight coupling encourages a monolithic team structure—features cross component boundaries, ownership is unclear, and contributions require deep context.

ACS Next's event-driven, CRD-based architecture enables:

* **Flexible component ownership**: If a component (e.g., Risk Scorer) makes sense under different ownership, it can transfer without architectural surgery
* **External teams can contribute**: Any team can build security features by subscribing to the broker or publishing CRDs—no need to understand Central internals
* **Clear ownership boundaries**: Components have well-defined interfaces (pub/sub topics, CRD schemas), making ownership explicit
* **ACS engineers focus on security**: Fleet management, cross-cluster coordination, and identity federation become the portfolio's responsibility—ACS engineers can focus on vulnerability detection, policy evaluation, and runtime protection

This isn't just about code—it's about enabling Red Hat to organize security capabilities however makes sense organizationally, without fighting the architecture.

**Product boundaries become optional, too.** The current architecture enforces a hard line: Central is ACS, everything else is not-ACS. But this boundary is somewhat arbitrary—why is vulnerability scanning "ACS" while compliance scanning is "compliance-operator"? From a customer perspective, it's all "OCP security."

ACS Next dissolves this boundary. Security capabilities are CRDs and broker subscriptions. The "ACS" brand matters less than the reality that OCP has excellent security built in. Given that the vast majority of ACS revenue comes from OCP/OPP customers, this alignment makes business sense: ACS becomes a security platform for OCP, not a separate product bolted on.

### 6. What We Stop Building

The value of ACS Next isn't in what we add—it's in what we stop maintaining.

| Capability | Current Owner | ACS Next Owner |
|------------|---------------|----------------|
| Multi-cluster aggregation | Central | ACM Search (ACS maintains adapter) |
| Cross-cluster identity | Central | K8s + ACM identity federation |
| Policy distribution | Central → Sensor gRPC | ACM Governance (just CRDs) |
| Custom RBAC engine | Central (SAC) | K8s RBAC |
| Fleet UI | Central UI | OCP Console multi-cluster perspective (ACS maintains plugin) |
| Auth provider integrations | Central | K8s + portfolio |
| Central-Sensor sync protocol | 90+ files, bidirectional gRPC, deduper state | Not needed—single-cluster |

**The sync machinery deserves special mention.** Current ACS maintains an elaborate synchronization protocol between Central and Sensor: bidirectional gRPC streaming with 50+ message types, dual-sided hash deduplication (371 lines on Central side alone), chunked state transfer on reconnect, 7-layer stream wrappers, strict sync sequencing, and reconciliation logic for handling disconnects. This solves real problems—network partitions, duplicate processing, event ordering—but it's complexity we only need because Central is remote from Sensor.

In ACS Next, components run in the same cluster. There's no cross-cluster network partition to recover from. Local pub/sub (NATS with JetStream) handles event distribution and provides recovery via durable consumers—if a component crashes, it replays from its last acknowledged position. This is a fundamentally simpler model: local replay from a persistent log vs. bidirectional state reconciliation across network boundaries.

*Note: Local recovery still requires resource planning—JetStream retention windows affect PVC sizing (~150-300 MB estimated per cluster). See architecture doc for details.*

These are hard problems. They're also solved problems—solved by K8s, ACM, and OCP. Central re-solves them because the architecture requires it. ACS Next lets us stop reinventing them.

The broker, the CRDs, the event-driven architecture—these aren't innovations for their own sake. They're the mechanism that lets us hand off complexity we shouldn't own.

---

## What It Enables

ACS Next doesn't just unblock specific features—it changes what kinds of features are possible.

### RBAC That Just Works

**The opportunity:** Users authenticate once, access ACS data via standard K8s RBAC.

Single-cluster ACS uses local K8s RBAC directly. No cross-cluster identity resolution needed—that's the portfolio's job. The complexity disappears because we're no longer trying to solve multi-cluster identity in a component that runs on a single cluster.

### Native Policy Distribution

**The opportunity:** Security policies flow through ACM Governance like any other policy.

Policies are CRDs. ACM distributes them. No special integration needed—it's just Kubernetes resources. Security policy becomes part of the organization's standard GitOps workflow.

### Portfolio-Native Experience

**The opportunity:** ACS feels like part of ACM, not a separate product with an addon.

Security data lives in CRDs that ACM Search aggregates. Security views appear in OCP Console's multi-cluster perspective. There's no separate ACS UI for fleet operations—the capability is native to the platform.

*Note: The exact integration model—whether ACM Search indexes CRs directly or the ACM addon subscribes to broker topics—requires further design exploration. The architecture supports both patterns.*

### Discrete, Configurable Components

**The opportunity:** Security capabilities as composable building blocks.

Components like Risk Scorer, Baselines, and Historical Data become separate, optional pieces with clear ownership. Configurability is a design goal from the start, not something bolted onto existing architecture. Organizations can adopt what they need.

---

## Context: The Current Architecture

The current ACS architecture was designed in a different context:

* **Before portfolio integration was the strategic direction** — ACS needed its own multi-cluster capabilities
* **Before K8s became the standard** — Custom protocols and proprietary data models were common
* **Before ACM matured** — The primitives for fleet management weren't available

Central evolved to serve these needs well. It provides robust multi-cluster security for customers who need it today.

But Central's design assumptions create constraints:

* Central is the hub; Sensors are spokes
* Central owns data aggregation across clusters
* Central owns identity and access control
* Central owns the UI and API

These assumptions conflict with the portfolio integration model:

* The portfolio provides multi-cluster orchestration
* Each cluster owns its own data; the portfolio aggregates
* K8s and the portfolio own identity and access control
* OCP Console is the UI; K8s API is the API

This isn't a criticism of the current architecture—it was designed for different requirements. But adapting it to portfolio integration would require changing foundational assumptions, which is why incremental approaches have hit walls:

* RBAC convergence: Six solutions evaluated, none acceptable within current model
* ACM integration: Years of architect conversations with limited progress
* OCP Console Plugin: Works, but runs into RBAC/authz issues for cross-cluster

### Implications for Roadmap

If ACS Next is not pursued, capabilities that require architectural change should be removed from near-term roadmap discussions:

* RBAC convergence
* Policy placement via ACM Governance
* Deep ACM integration / single-pane-of-glass
* Configurable risk scoring (beyond incremental improvements)

This isn't punitive—it's practical. Continued design work on these capabilities within the current architecture has not produced viable solutions, and future attempts are unlikely to change that. Engineering effort is better directed toward achievable improvements.

The current architecture remains capable of serving existing customers and supporting incremental feature development. These roadmap adjustments only affect capabilities that depend on assumptions the current architecture doesn't make.

---

## Architectural Progress

Design work has moved ACS Next from concept to concrete architecture. See [ACS_NEXT_ARCHITECTURE.md](ACS_NEXT_ARCHITECTURE.md) for detailed technical design and [ACS_NEXT_GAP_ANALYSIS.md](ACS_NEXT_GAP_ANALYSIS.md) for feature gap analysis. Prototype implementation is the next validation step.

---

## Investment Considerations

### What ACS Next Requires

* **Multi-quarter engineering investment** — This is a significant undertaking
* **Parallel maintenance** — Current ACS continues serving existing customers (5.2 LTS provides 5-year runway)
* **Migration tooling** — Customers moving to ACS Next need policy export/import, configuration migration, historical data migration
* **Organizational focus** — Dedicated team capacity, clear ownership

### What We Have Today

* Concrete architecture with key decisions made
* Gap analysis identifying remaining work
* 5.2 LTS providing transition runway
* ACM infrastructure ready (Search, Governance, AddOn framework)

### What We Don't Have Yet

* Prototype validating the architecture end-to-end
* Allocated engineering capacity
* Organizational alignment on priorities

---

## Recommended Approach

### Phase 0: Validation

Build a minimal prototype demonstrating the core architecture:

* Event broker + CRD Projector + basic policy violation flow
* ACM Search indexing PolicyViolation CRs
* OCP Console displaying violations

This validates the architecture before committing to full implementation.

### Phase 1: Core Platform

Build single-cluster ACS with essential capabilities:

* Collector, Scanner, Admission Control
* Policy engine, vulnerability matching
* CRD-based configuration and data model

### Phase 2: Portfolio Integration

* ACM addon for deployment and configuration
* Hub-level Persistence Service for historical data
* Cross-cluster subscription via NATS leaf nodes

### Phase 3: Migration Support

* Policy and configuration export from current ACS
* Import tooling for ACS Next
* Documentation and customer guidance

---

## Summary

ACS Next is an investment in architectural flexibility.

The current architecture serves existing customers well. But it constrains how we can evolve the product—freemium tiers, edge deployments, portfolio integration, K8s-native experiences are all limited or blocked by Central's foundational assumptions.

ACS Next doesn't dictate a specific product vision. It enables multiple visions. PM decides the positioning; the architecture accommodates it. And by delegating fleet management to the portfolio, ACS engineers can focus on what matters: security.

Given that the vast majority of ACS revenue comes from OCP/OPP, this alignment makes business sense. ACS Next positions ACS as a security platform for OCP—not a separate product bolted on, but foundational infrastructure that blurs the line between "ACS features" and "OCP security."

The question for leadership: **Do we want the flexibility to position ACS differently for different markets?**

If yes, ACS Next is worth the investment.

If not, the current architecture continues to serve existing customers well—but capabilities like RBAC convergence and deep ACM integration should come off the roadmap. Not as punishment, but because pursuing them within the current architecture hasn't worked and continued attempts won't change the underlying constraints. Engineering effort is better spent on what the architecture can deliver.

---

## References

* **[ACS_NEXT_ARCHITECTURE.md](ACS_NEXT_ARCHITECTURE.md)** — Detailed technical architecture
* **[ACS_NEXT_GAP_ANALYSIS.md](ACS_NEXT_GAP_ANALYSIS.md)** — Feature gap analysis
* **[ACS_NEXT_SUMMARY.md](ACS_NEXT_SUMMARY.md)** — Executive summary

---

*This document reflects analysis as of March 2026. The architectural design has progressed through multiple iterations based on gap analysis and technical evaluation.*
