# ACS Next: Executive Summary

*Status: Draft | Date: 2026-02-24*

---

## The Proposal

Transform ACS from a monolithic Central-based architecture into a **single-cluster security platform** with **ACM providing multi-cluster orchestration**.

**Key outcomes:**

* Enables deeper OPP portfolio integration and single-pane-of-glass experiences
* Aligns with Red Hat's platform strategy where ACM is the multi-cluster orchestrator
* Satisfies downstream requirements (like RBAC convergence) that the current architecture cannot
* Enables low-friction adoption by making security feel K8s-native
* Could increase engineering velocity by reducing coordination overhead *(hypothesis - requires validation)*

---

## The Problem

### The Strategic Direction: Deeper Portfolio Integration

PM and UX are driving toward **deeper OPP portfolio integration** and **single-pane-of-glass experiences**. This means unified platform identity, single console experience (OCP Console with multi-cluster perspective), and security that feels like part of OCP rather than a separate product.

This strategic direction creates downstream technical requirements that the current architecture cannot satisfy.

### Downstream Requirement: RBAC Convergence

One critical downstream requirement is **RBAC convergence**—users should not need separate ACS roles. For portfolio integration to work, ACS must honor the same identity and access model as the rest of the platform.

Teams have explored six solutions. No solution stands out as a clear path forward because **Central remains the aggregator and authority**. The implicit constraint—"how do we make K8s RBAC work with Central?"—has no clear answer within the current architecture.

### Engineering Velocity May Be Suffering

The current architecture may create organizational friction *(these are hypotheses that should be validated with engineering teams)*:

* Features cross component boundaries (Central, Sensor, shared packages), but ownership may not align well
* Adding a new policy criterion might take longer than necessary
* Bug fixes could bounce between teams
* The policy engine alone spans 299 files with 20+ dependencies

### We're Duplicating ACM Capabilities

ACS maintains its own multi-cluster management, identity federation, policy distribution, fleet aggregation, and cross-cluster RBAC. ACM already provides all of these.

---

## The Strategic Opportunity

### 5.2 LTS Creates a Transition Window

ACS 5.2 locks in the current architecture for 5 years of support. This enables:

* No customer disruption (existing customers stay on LTS)
* Parallel development runway for new architecture
* Clear investment signal (new features go to ACS Next)

### Alignment with Red Hat's Platform Strategy

Red Hat's multi-cluster strategy: **ACM is the orchestrator**. ACS Next aligns with this rather than competing.

### Low-Friction Adoption

Red Hat sells subscriptions (support/SLAs), not technical feature gates. The adoption challenge is perception:

* **Current**: "Want security? Evaluate this separate product called ACS, with its own UI, RBAC, concepts."
* **ACS Next**: "Security data visible in OCP Console. Uses familiar tools (kubectl, GitOps), familiar RBAC. Want fleet visibility? That's OPP."

---

## The Architecture

### Core Principle

```
┌─────────────────────────────────────────────────────────────┐
│                         ACM Hub                             │
│  • ACM Search for aggregation                               │
│  • ACM Governance for policy distribution                   │
│  • OCP Console multi-cluster perspective for fleet visibility│
└─────────────────────────────────────────────────────────────┘
         │              │              │              │
         ▼              ▼              ▼              ▼
┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│  Cluster A  │  │  Cluster B  │  │  Cluster C  │  │  Cluster D  │
│  ACS Next   │  │  ACS Next   │  │  ACS Next   │  │  ACS Next   │
│  (local)    │  │  (local)    │  │  (local)    │  │  (local)    │
└─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘
```

### Managed Cluster Components

**Security Controller** (1 deployment, 3 containers):
* **Build Container**: roxctl endpoint, scanner client, build policies
* **Deploy Container**: Admission webhook, deploy policies
* **Runtime Container**: Collector subscriber, audit logs, runtime policies
* **Shared**: Policy engine as library (not service), SQLite persistence

**Scanner Operator** (1 deployment): Image indexing, vulnerability DB, matcher service

**Collector** (DaemonSet): eBPF data collection, pub/sub API for extensibility

**Optional OPP components**:
* Baseline Operator: Process/network anomaly detection
* Historical Data Operator: Time-series trends

**Total: 3-5 deployments per cluster** (comparable to today)

### Hub Components

* **ACM** (existing): Search, Governance, Console
* **ACS Addon** (new): Deploys and configures components
* **Hub Scanner** (optional): Shared scanner via Maestro for minimal managed cluster footprint

### Key Patterns

**CRD-first data model**: Security data as CRs enables K8s RBAC, ACM Search aggregation, OCP Console display

**Policy engine as library**: Compiled into each container; no inter-service calls

**No discrete API server**: K8s API is the API; PolicyViolation and ImageVulnerability CRs are the persistence layer

**OCP-native notifications**: AlertManager + Logging stack replace Central's custom notifiers

### What Gets Removed

| Removed | Replacement |
|---------|-------------|
| Central aggregation | ACM Search |
| Sensor-Central gRPC | Same-cluster (not needed) |
| Custom notifiers | AlertManager |
| Compliance framework | compliance-operator |
| Custom RBAC engine | K8s RBAC + ACM |

**Estimated codebase reduction: 25-30%**

---

## How This Enables Portfolio Integration

**RBAC Convergence** (previously blocked):

| Current Problem | ACS Next Solution |
|-----------------|-------------------|
| "External Role Broker doesn't exist" | ACM already is the role broker |
| "OCP Plugin lacks cross-cluster views" | ACM Search/Console provides cross-cluster |
| "Multi-cluster identity mapping" | ACM's identity federation handles this |
| "Performance of role resolution" | Local K8s RBAC is fast; aggregation is ACM's problem |

**Single-Pane-of-Glass**: OCP Console's multi-cluster perspective becomes the fleet security view.

**Unified Platform Identity**: K8s RBAC + ACM identity federation. No separate ACS auth.

---

## Costs and Risks

### Engineering Costs

* Build single-cluster ACS variant (High)
* CRD data model design (Medium-High)
* Maintain two architectures during transition (High)

### Mitigations

| Risk | Mitigation |
|------|------------|
| Customer disruption | 5.2 LTS provides stability; ACS Next is opt-in |
| ACM dependency | ACM already required for OPP value |
| Feature regression | Explicit feature mapping; gap analysis before migration |
| Microservice proliferation | Consolidate functions (3 containers in 1 deployment) |

---

## The Alternative Paths

### Path A: ACM Addon Querying Central

* **What you get**: Multi-cluster Console view; faster time to value (3-6 months)
* **What you don't get**: Portfolio integration; RBAC convergence; K8s-native APIs; low-friction adoption

### Path B: Incremental Decoupling

* **What you get**: Some potential velocity improvement; gradual transition
* **What you don't get**: Portfolio integration; full RBAC convergence; K8s-native model
* **Risk**: "Big ball of mud"—more interfaces without cleaner boundaries

### Path C: ACS Next (Full Shift)

* **What you get**: Portfolio integration; single-pane-of-glass; RBAC convergence; K8s-native; low-friction adoption
* **What you don't get**: Near-term delivery (12-18 months); standalone multi-cluster (ACM required)

### Comparison

| Capability | Path A | Path B | Path C (ACS Next) |
|------------|--------|--------|-------------------|
| Portfolio integration | No | No | Yes |
| Single-pane-of-glass | Partial | No | Yes |
| RBAC convergence | No | Partial | Yes |
| K8s-native APIs | No | Partial | Yes |
| Time to value | 3-6 months | 6-12 months | 12-18 months |

**Recommendation**: Path C if portfolio integration and single-pane-of-glass are the strategic direction. Path A only if near-term OCP Console integration is the sole priority.

---

## Why Now

### Forcing Functions

1. **PM/UX strategic direction** (portfolio integration, single-pane-of-glass) — this is the primary driver
2. RBAC convergence is a downstream requirement that's stuck with current architecture
3. 5.2 LTS timing (natural transition window)
4. ACM maturity (Search, Governance, AddOn framework ready)
5. Engineering velocity may be suffering *(hypothesis - requires validation)*

### The Cost of Inaction

* Portfolio integration and single-pane-of-glass remain blocked
* RBAC convergence continues to have no viable path
* ACS continues to feel like a separate product, not part of OCP
* Competitive disadvantage against K8s-native security tools

---

## Next Steps

**Immediate (Socialization)**:
* Identify PM allies aligned with OCP security pillar vision
* Engage ACM team on addon responsibility
* Present business case to leadership

**Near-term (Validation)**:
* Prototype: Minimal single-cluster ACS with ACM Search aggregation
* Identify pilot customer

**Medium-term (Execution)**:
* Phase 0: Validation prototype (1-2 months)
* Phase 1: Core single-cluster ACS (3-4 months)
* Phase 2: ACM integration (2-3 months)
* Phase 3: Migration tooling (2-3 months)

---

## Key Questions Answered

**"What about customers without ACM?"**
Single-cluster ACS works standalone. Multi-cluster requires ACM—which is the OPP value proposition.

**"Migration path for existing customers?"**
5.2 LTS provides 5 years of support. Migration is opt-in. Tooling provided for policy export/import.

**"How do disconnected customers use this?"**
Same as disconnected ACM: operates within closed network, catalog updates via mirrored registries, local scanner option available.

---

*Full document: [ACS_NEXT_POSITION_PAPER.md](ACS_NEXT_POSITION_PAPER.md)*
