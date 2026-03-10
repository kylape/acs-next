# ACS Next: A Strategic Architectural Shift

* **Status**: Draft  —  For Discussion
* **Date**: 2026-03-10
* **Author**: [Your Name]

---

## Executive Summary

Red Hat Advanced Cluster Security (ACS) should evolve from a monolithic
Central-based architecture into a **single-cluster security platform** with
**ACM providing multi-cluster orchestration**. This architectural shift:

* **Enables deeper OPP portfolio integration**  —  security becomes a platform
  capability, not a separate product
* **Delivers single-pane-of-glass experiences**  —  OCP Console's multi-cluster
  perspective provides unified fleet visibility
* **Aligns with Red Hat's platform strategy** where ACM is the multi-cluster
  orchestrator
* **Enables natural adoption paths** by making security K8s-native, reducing
  evaluation friction
* **Could increase engineering velocity** by reducing cross-component
  coordination overhead *(hypothesis  —  requires validation)*
* **May reduce codebase by 25-30%** by removing redundant multi-cluster
  infrastructure

**The strategic driver is portfolio integration.** PM and UX are shifting
toward deeper OPP integration and single-pane-of-glass experiences. This
direction has downstream technical requirements — including RBAC
convergence — that the current architecture cannot satisfy. ACS Next is
the architectural response to this strategic direction.

The ACS 5.2 LTS release provides the **transition window**: maintain
current architecture for existing customers while focusing new feature
development on ACS Next (targeting 5.3+ or 6.0).

---

## Part I: The Problem

### The Strategic Direction: Deeper Portfolio Integration

PM and UX are driving toward **deeper OPP portfolio integration** and
**single-pane-of-glass experiences**. This isn't a vague aspiration — it's the
direction the product organization is moving. The goal:

* **Unified platform identity**: Users authenticate once and have consistent
  access across OPP components
* **Single console experience**: OCP Console (with ACM's multi-cluster
  perspective) as the fleet management interface, not separate product UIs
* **Consistent operational model**: Security managed like other platform
  capabilities (observability, compliance, etc.)
* **Reduced adoption friction**: Security feels like part of OCP, not a
  separate product to evaluate

This strategic direction creates **downstream technical requirements** that
the current ACS architecture cannot satisfy.

### Downstream Requirement: RBAC Convergence

One critical downstream requirement is **RBAC convergence** — users should not
need separate ACS roles in addition to their K8s/OCP roles. For portfolio
integration and single-pane-of-glass to work, ACS must honor the same identity
and access model as the rest of the platform.

For months, teams have attempted to converge ACS access control with
Kubernetes/OpenShift RBAC. The
[RBAC convergence design document](https://docs.google.com/document/d/144jpKqZ17MtkzkJKJUty6vyHWxmlPrE9Y2aUPN0G3iI)
explores six solutions:

| Solution | Challenge |
|----------|------------|
| Naive K8s role resolution | Performance  —  many RBAC queries across clusters |
| Per-query data filtering | Performance  —  retrieves entire database per request |
| External role broker | Doesn't exist yet |
| OCP dynamic plugin | No cross-cluster aggregated views |
| Change API granularity | Requires full API rework |
| CR-based declarative config | Still requires separate ACS RBAC configuration |

**No solution stands out as a clear path forward.** Each has significant
trade-offs that are difficult to resolve within the current architecture.

The implicit constraint in all discussions: **Central remains the aggregator
and authority**. Every solution attempts to answer "how do we make K8s RBAC
work with Central?" — and no clear answer has emerged. This may be symptomatic
of a deeper issue: the current architecture was not designed for platform
integration.

### Why Incremental Approaches Have Failed

The pattern repeats:

1. **OCP Console Plugin**: Built, but limited to single-cluster, read-only,
   incomplete data
2. **Declarative Config via CRDs**: Implemented, but doesn't solve RBAC
   convergence
3. **External Role Broker**: Discussed repeatedly, but no one is building it
4. **ACM coordination**: "Let's have architect conversations"  —  overhead
   without progress

Each increment hits the same wall: **Central's architecture fundamentally
doesn't fit the Kubernetes/OpenShift model**. This isn't a criticism of past
decisions — Central was designed before platform integration was the strategic
direction. But now that direction has changed, and the architecture must evolve.

### The Multi-Cluster Identity Problem

The RBAC design doc identifies a core challenge:

> "The RHACS access control engine brings a multi-cluster dimension that does
> not exist in standard Kubernetes clusters. A naive solution relying on
> Kubernetes roles to perform access control within RHACS would have to map
> the RHACS user to a Kubernetes user on the cluster where central is running,
> resolve the identity of that Kubernetes user, identify users on the
> monitored clusters that have the same identity..."

This is Central trying to solve a problem that **ACM has already solved**. The
strategic insight: rather than building RBAC convergence into Central, delegate
multi-cluster identity to the platform component designed for it.

### Organizational Friction: Conway's Law in Action

> "Organizations which design systems are constrained to produce designs which
> are copies of the communication structures of these organizations."
>  —  Melvin Conway, 1967

The current ACS architecture may be an example of Conway's Law creating
organizational friction. Central's monolithic design emerged from a centralized
team structure, but as teams have grown and specialized, the architecture may
no longer reflect how we work — and this mismatch could impose a tax on
engineering velocity.

**The Hypothesized Friction**: Features naturally cross component boundaries
(Central, Sensor, shared packages), but team ownership may not align with
those boundaries. This could create coordination overhead:

```
Current: Architecture crosses team boundaries
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Central   │◄──►│  Shared Pkgs│◄──►│   Sensor    │
│   (Team A)  │    │  (Team ???) │    │   (Team B)  │
└─────────────┘    └─────────────┘    └─────────────┘
        ▲                 ▲                  ▲
        └─────── Feature X spans all three ───┘
```

**Possible Result**: Non-trivial features may become cross-team coordination
exercises. Bug fixes could bounce between teams. Engineers might avoid certain
areas if ownership and blast radius are unclear.

**Why This Matters for ACS Next**: Decoupling into independent operators isn't
just about technical cleanliness — it could help align architecture with team
structure. Each operator could be owned end-to-end by a single team. Features
that currently require multi-team coordination might become single-team
deliverables.

```
ACS Next: Architecture aligns with team ownership
┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐
│ Policy Operator │   │Scanner Operator │   │ Runtime Operator│
│   (Team A)      │   │   (Team B)      │   │   (Team C)      │
└─────────────────┘   └─────────────────┘   └─────────────────┘
        │                     │                     │
   Full ownership        Full ownership        Full ownership
   CRD → Engine → API    Scan → Match → API    Collect → Detect
```

**The strategic hypothesis**: ACS Next is as much an organizational change as
a technical one. By restructuring the architecture, we could restructure how
teams collaborate — potentially reducing coordination overhead and increasing
delivery velocity. *This hypothesis should be validated with engineering teams.*

Conway's Law works both ways. The current architecture's tight coupling
encourages a monolithic team structure — features cross component boundaries,
ownership is unclear, and contributions require deep context.

ACS Next's event-driven, CRD-based architecture enables:

* **Flexible component ownership**: If a component (e.g., Risk Scorer) makes
  sense under different ownership, it can transfer without architectural surgery
* **External teams can contribute**: Any team can build security features by
  subscribing to the broker or publishing CRDs — no need to understand Central
  internals
* **Clear ownership boundaries**: Components have well-defined interfaces
  (pub/sub topics, CRD schemas), making ownership explicit
* **ACS engineers focus on security**: Fleet management, cross-cluster
  coordination, and identity federation become the portfolio's
  responsibility — ACS engineers can focus on vulnerability detection, policy
  evaluation, and runtime protection

This isn't just about code — it's about enabling Red Hat to organize security
capabilities however makes sense organizationally, without fighting the
architecture.

**Product boundaries become optional, too.** The current architecture enforces
a hard line: Central is ACS, everything else is not-ACS. But this boundary is
somewhat arbitrary — why is vulnerability scanning "ACS" while compliance
scanning is "compliance-operator"? From a customer perspective, it's all
"OCP security."

ACS Next dissolves this boundary. Security capabilities are CRDs and broker
subscriptions. The "ACS" brand matters less than the reality that OCP has
excellent security built in. Given that the vast majority of ACS revenue
comes from OCP/OPP customers, this alignment makes business sense: ACS
becomes a security platform for OCP, not a separate product bolted on.

---

### How Current Architecture May Slow Us Down

Beyond the organizational friction described above, the Central-based
architecture may impose engineering friction. The following observations are
based on code structure analysis; the claimed impacts are hypotheses that
should be validated with the teams.

#### Policy Engine: A Case Study in Potential Architectural Friction

| Observation | Evidence | Hypothesized Impact |
|-------------|----------|---------------------|
| **Monolithic field registry** | 966-line `initializeFieldMetadata()` function; all policy criteria in one place | Could make adding new detection criteria slower than necessary |
| **6+ files to add one policy field** | Changes required: fieldnames, querybuilders, field_metadata, violationmessages, validate, tests | May increase regression risk; engineers might avoid policy changes |
| **Central/Sensor split ownership** | `pkg/booleanpolicy` (97 files) shared by both; 299 files depend on it | Could create unclear team ownership; bug fixes may require cross-team coordination |
| **No API versioning** | Central and Sensor compile policies independently with shared code | Version skew during upgrades could cause detection issues |
| **20+ dependencies in policy service** | `service_impl.go` constructor injects cluster, deployment, network, notifier, image, MITRE datastores... | May be harder to test and reason about |

#### Hypothesized Impact (Requires Validation)

The following table presents *hypotheses* about potential improvements.
**These are not validated metrics.** Before using these claims to justify
investment, they should be validated through:

* Engineering team surveys
* Analysis of ticket/PR data
* Comparison with similar architectural changes in other projects

| Metric | Hypothesized Current State | Hypothesized With Clear Ownership |
|--------|---------------------------|-----------------------------------|
| Time to add new policy criterion | ~3 sprints? | ~1-2 sprints? |
| Policy-related bugs per release | ~5-8? | ~2-3? |
| Customer escalation resolution | ~2-3 weeks? | ~1 week? |
| Engineer onboarding to policy code | ~4-6 weeks? | ~2-3 weeks? |

*These are rough estimates based on architectural analysis, not measured data.*

#### The Broader Pattern

Policy engine may not be unique. Similar friction could appear wherever
Central's monolithic nature requires cross-cutting changes:

* **Adding a new data type**: May touch Central storage, API, UI, Sensor
  collection, gRPC protocol
* **Console plugin features**: Limited by what Central exposes; can't query
  K8s directly
* **Scanner integration**: Tightly coupled to Central's lifecycle
* **Any multi-cluster feature**: Must reason about Sensor connections, data
  propagation, consistency

**ACS Next's modular architecture could reduce this friction by establishing
clearer ownership boundaries.** Each operator would be independently developed,
released, and deployed.

**Clarification on release coordination**:

* **Upstream**: Teams develop and release operators independently. A policy
  engine fix doesn't require coordinating with scanner team.
* **Downstream/OPP**: The OPP product release still coordinates which operator
  versions ship together — but the boundaries are cleaner (versioned operators
  with CRD APIs) rather than entangled (shared packages compiled into different
  binaries).

The improvement is not "no coordination" but "coordination at well-defined
boundaries" rather than "coordination through shared source code."

---

## Part II: The Strategic Opportunity

### ACS 5.2 LTS Creates the Transition Window

ACS 5.2 will lock in the current architecture for 5 years of long-term
support. This provides:

| Benefit | Implication |
|---------|-------------|
| **No customer disruption** | 5.0 LTS customers keep what works |
| **Parallel development runway** | Years to prove new architecture |
| **Clear investment signal** | New features go to ACS Next |
| **Reduced technical debt accumulation** | Stop adding complexity to legacy |

**Timeline**:

```
ACS 5.2 (LTS - 5 years)
├── Current Central-based architecture
├── Supported, stable, enterprise customers stay here
└── Maintenance: bug fixes, security patches, minor enhancements

ACS 5.3+ / 6.0 ("ACS Next")
├── Single-cluster + ACM architecture
├── New feature development focus
└── K8s-native, modular, composable
```

### Alignment with Red Hat Platform Strategy

Red Hat's multi-cluster strategy is clear: **ACM is the orchestrator**. ACS
currently competes with this by maintaining its own:

| Capability | ACS Today | Red Hat Platform |
|------------|-----------|------------------|
| Multi-cluster management | Central | ACM |
| Identity federation | Custom auth providers | ACM identity |
| Policy distribution | Central → Sensor | ACM Governance |
| Fleet aggregation | Central database | ACM Search |
| Cross-cluster RBAC | ACS SAC engine | ACM RBAC |

ACS is building and maintaining capabilities that ACM already provides.
This is:

* **Redundant engineering investment**
* **Confusing for OPP customers** (two ways to manage multi-cluster)
* **Friction for platform teams** (separate RBAC, separate console, separate
  concepts)

### Product Flexibility Through Architecture

**ACS Next gives PM room to maneuver.**

The current architecture makes certain product decisions for us. ACS Next
returns those decisions to PM.

| Product Question | Current Architecture | ACS Next |
|------------------|---------------------|----------|
| "Can we offer a freemium tier?" | No  —  Central is all-or-nothing | **Yes**  —  CRDs free, advanced features in OPP |
| "Can we have different feature sets per deployment?" | No  —  Central is monolithic | **Yes**  —  optional components (Vuln Management Service, Risk Scorer, etc.) |
| "Can advanced search be OPP-only?" | No  —  search is baked into Central | **Yes**  —  Vuln Management Service is optional, runs on hub only |
| "Can we reduce footprint for edge?" | Limited  —  Central + Sensor is minimum | **Yes**  —  Broker + Collector only, hub provides the rest |
| "Can we align with K8s RBAC?" | Difficult  —  SAC is deeply embedded | **Yes**  —  K8s RBAC is native, no custom auth layer |
| "Can security feel like part of OCP?" | Difficult  —  Central is a separate product | **Yes**  —  CRDs, OCP Console, familiar tools |
| "Can other teams contribute security features?" | Difficult  —  requires Central context | **Yes**  —  subscribe to broker, publish CRDs |
| "Can ACS be a security platform for OCP, not a separate product?" | No  —  Central is architecturally separate | **Yes**  —  CRDs and broker enable platform model |
| "Can we support multi-tenant / vendor platform scenarios?" | Difficult  —  Central RBAC doesn't map to namespace tenancy | **Yes**  —  K8s RBAC + CRDs scope naturally to namespaces |

**These become PM decisions, not engineering constraints.**

Examples of positioning that ACS Next enables (but doesn't require):

* **Freemium model**: Basic security via CRDs (free with OCP), advanced
  search/trends via OPP subscription
* **Edge-optimized deployments**: Minimal on-cluster footprint, hub provides
  heavy lifting
* **Per-cluster vs. fleet features**: Different capabilities at different scopes
* **Portfolio-native experience**: Security that feels like part of ACM, not
  a bolt-on
* **Vendor platforms**: ACS customers with their own customers get
  namespace-level tenancy via K8s RBAC (support may be
  incremental — violations first, then policies, then reports)

The architecture accommodates multiple positioning choices. PM decides which
ones to pursue.

### Adoption Through Platform Integration

**Important context on Red Hat's business model**: Red Hat sells subscriptions
for support and entitlements, not technical feature gates. Customers can
technically install any component — they pay for certified builds, support, and
SLAs. There is no in-cluster mechanism to determine what subscriptions a
customer has purchased.

This means the adoption strategy isn't about technical enforcement. It's about:

1. **Reducing evaluation friction**  —  Make ACS feel like part of OCP, not a
   separate product
2. **Creating natural upgrade paths**  —  Customers experience value, then
   want more
3. **Perceived continuity**  —  OPP feels like "more of what I have" not "a
   different product"

**The adoption problem today:**

```
Current experience:
1. Customer has OCP
2. "Want security? Evaluate this separate product called ACS"
3. Deploy Central, Sensor, Scanner, learn new UI, new RBAC, new concepts
4. Feels like a separate product → separate evaluation → friction → slower adoption
```

**ACS Next changes the experience:**

```
ACS Next experience:
1. Customer has OCP
2. Security data visible in OCP Console as K8s-native resources
3. Uses familiar tools (kubectl, GitOps), familiar RBAC (K8s RBAC)
4. "Want fleet visibility and advanced policy? That's OPP"
5. Feels like "more of what I have" → natural upgrade → faster adoption
```

**The sales motion changes:**

| Model | Sales Conversation |
|-------|-------------------|
| **Current** | "Here's ACS, a security platform. Let me show you Central's UI, explain our RBAC model..." |
| **ACS Next** | "You're already seeing security data in Console. Want that across your fleet? That's OPP." |

### OCP Security Pillar Integration

ACS Next enables security to feel like part of OCP rather than a separate
product:

| Experience | OCP (what they have) | OPP (subscription upgrade) |
|------------|---------------------|---------------------------|
| **Vulnerability visibility** | Namespace-scoped CVEs in Console | Fleet-wide aggregation via ACM |
| **Policy enforcement** | Basic admission control | Advanced policy engine, runtime detection |
| **Compliance** | K8s best practices | Compliance frameworks, audit reports |
| **Support** | Community/self-service | Red Hat support, certified builds, SLAs |

**Key insight**: The technical capabilities may be similar, but the
*experience* is different. ACS Next makes security feel like a platform
capability. Path A makes it feel like an integrated-but-separate product.

This enables:

* **"OCP includes security visibility"** vs "evaluate this separate security
  product"
* **Natural upgrade path**  —  customers see value, want more, upgrade to OPP
* **Reduced evaluation friction**  —  no new UI to learn, no new RBAC to
  configure

### RHACS Cloud Service: Strategic Considerations

ACSCS has had limited traction relative to investment. This creates a strategic
opportunity, though one that requires careful consideration:

**Current State**:

* ACSCS represents significant ongoing operational investment
* Customer adoption has been slower than projected
* Architecture is tightly coupled to Central-based model

**What ACS Next Enables**:

If ACS becomes a single-cluster platform with ACM providing multi-cluster
orchestration, the value proposition of ACSCS changes fundamentally. ACS
Next **opens the option** to reconsider ACSCS's role:

| Option | Description | Considerations |
|--------|-------------|----------------|
| **Sunset with 5.2** | End ACSCS when 5.2 LTS begins | Simplifies portfolio; frees investment for ACS Next |
| **Transform to ACM-based** | ACSCS becomes hosted ACM + ACS operators | Maintains cloud offering; different architecture |
| **Maintain parallel** | ACSCS continues alongside ACS Next | Higher investment; two architectures to maintain |

**Important**: This is a PM/business decision, not a technical one. ACS
Next **enables** ACSCS sunset but does not require it. This document raises
the strategic consideration; the decision requires PM and business analysis.

**What Sunsetting Would Enable**:

* Removes architectural constraints from ACS Next design (no need to support
  hosted Central model)
* Simplifies the product portfolio
* Redirects investment toward strategic direction
* Reduces ongoing operational burden

---

## Part III: The Architecture

### Core Principle: Single-Cluster ACS + ACM Multi-Cluster

```
┌─────────────────────────────────────────────────────────────┐
│                         ACM Hub                             │
│  (Multi-cluster orchestration, aggregated views, fleet RBAC)│
│  * ACM Search for aggregation                               │
│  * ACM Governance for policy placement                      │
│  * ACM identity federation                                  │
│  * Vuln Management Service (fleet queries, trends)          │
└─────────────────────────────────────────────────────────────┘
         │              │              │              │
         ▼              ▼              ▼              ▼
┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│  Cluster A  │  │  Cluster B  │  │  Cluster C  │  │  Cluster D  │
│ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │  │ ┌─────────┐ │
│ │ACS Next │ │  │ │ACS Next │ │  │ │ACS Next │ │  │ │ACS Next │ │
│ │         │ │  │ │         │ │  │ │         │ │  │ │         │ │
│ │Scanner  │ │  │ │Scanner  │ │  │ │Scanner  │ │  │ │Scanner  │ │
│ │Collector│ │  │ │Collector│ │  │ │Collector│ │  │ │Collector│ │
│ │Policy   │ │  │ │Policy   │ │  │ │Policy   │ │  │ │Policy   │ │
│ │API      │ │  │ │API      │ │  │ │API      │ │  │ │API      │ │
│ └─────────┘ │  │ └─────────┘ │  │ └─────────┘ │  │ └─────────┘ │
│  K8s RBAC   │  │  K8s RBAC   │  │  K8s RBAC   │  │  K8s RBAC   │
└─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘
```

*Note: There is no Persistence Service per cluster. The Vuln Management
Service runs at the hub level only, providing fleet-wide queries and trend
analysis. Scanner operates as a compute service on each cluster  —  it indexes
images and matches vulnerabilities but does not persist match results itself.
CRDs on managed clusters carry summary-level security data (not individual
vulnerability records).*

### How This Enables Portfolio Integration

The architecture directly addresses the downstream requirements of the
strategic direction:

**RBAC Convergence** (previously blocked):

| Design Doc Problem | ACS Next Solution |
|--------------------|-------------------|
| "External Role Broker doesn't exist" | **ACM already is the role broker** |
| "OCP Plugin lacks cross-cluster views" | **ACM Search/Console provides cross-cluster** |
| "Resource mapping to K8s" | **CRDs become the data model**  —  required, not optional |
| "Multi-cluster identity mapping" | **ACM's identity federation handles this** |
| "Performance of role resolution" | **Local K8s RBAC is fast; aggregation is ACM's problem** |

**Single-Pane-of-Glass**: OCP Console's multi-cluster perspective becomes the
fleet security view. No separate ACS UI for multi-cluster.

**Unified Platform Identity**: K8s RBAC + ACM identity federation. No separate
ACS auth configuration.

### Composable Operator Architecture

Each capability is an independent operator:

```
┌─────────────────────────────────────────────────────────────┐
│                    Feature Building Blocks                   │
│                                                              │
│   ┌─────────────┐ ┌─────────────┐ ┌─────────────┐           │
│   │   Scanner   │ │  Collector  │ │   Policy    │           │
│   │  Operator   │ │  (eBPF)     │ │   Engine    │           │
│   └─────────────┘ └─────────────┘ └─────────────┘           │
│                                                              │
│   ┌─────────────┐ ┌─────────────┐ ┌─────────────┐           │
│   │  Baseline   │ │    Risk     │ │  Admission  │           │
│   │  Operator   │ │  Operator   │ │  Control    │           │
│   └─────────────┘ └─────────────┘ └─────────────┘           │
│                                                              │
│   Composable: deploy only what the customer needs.          │
│   Modular architecture enables flexible deployment options. │
└─────────────────────────────────────────────────────────────┘
```

### How This Could Increase Engineering Velocity

The composable architecture is designed to address the friction described
in Part I:

```
ACS Next: Clearer ownership, independent deployment
┌─────────────────────────────────────────────────────────────┐
│              Policy Operator (owned by Policy Team)          │
│  ┌─────────┐  ┌─────────┐  ┌─────────────────┐              │
│  │  CRD    │  │ Engine  │  │ Admission Ctrl  │              │
│  │  API    │  │ (eval)  │  │ (enforcement)   │              │
│  └─────────┘  └─────────┘  └─────────────────┘              │
│                                                              │
│  * One team could own entire policy lifecycle               │
│  * CRD is the API contract                                  │
│  * Could ship without coordinating Central/Sensor releases  │
│  * Could refactor internals without cross-team coordination │
└─────────────────────────────────────────────────────────────┘
```

| Hypothesized Current Problem | ACS Next Solution | Potential Velocity Impact |
|------------------------------|-------------------|---------------------------|
| 6+ files to add policy criterion | Single operator, clean plugin interface | Potentially faster feature delivery |
| Cross-team coordination for bugs | One team owns component end-to-end | Could enable faster escalation resolution |
| Central/Sensor version coupling | Operators are self-contained | Could enable safer, independent upgrades |
| 60+ scattered test files | Focused integration tests per operator | Could improve confidence, reduce flaky CI |
| 4-6 week onboarding to policy code | Clear boundaries, smaller surface | Could speed engineer ramp-up |

*These potential benefits are based on architectural principles, not measured
outcomes. Actual impact would need to be validated after implementation.*

**The hypothesis**: Modular architecture could remove coordination overhead
that may be slowing us down today.

### CRD-First Data Model

For ACM Search to aggregate and K8s RBAC to control access, ACS data must
exist as CRs:

```yaml
apiVersion: security.stackrox.io/v1
kind: ImageVulnerability
metadata:
  name: CVE-2024-1234-nginx
  namespace: my-app
spec:
  image: nginx:1.25
  cve: CVE-2024-1234
  severity: Critical
  fixedIn: 1.25.1
```

This enables:

* **K8s RBAC controls access**  —  who can `get/list` ImageVulnerabilities
  in a namespace
* **ACM Search aggregates**  —  query ImageVulnerabilities across fleet
* **OCP Console displays**  —  using standard K8s APIs
* **No separate ACS RBAC engine needed**  —  K8s RBAC is the engine

### CRD Tiering Strategy

Not all data belongs in etcd. CRDs carry summary-level data only  —  not
100k individual vulnerability CRs:

| Tier | Storage | Examples | Access Pattern |
|------|---------|----------|----------------|
| **Config CRDs** | etcd | SecurityPolicy, RiskProfile, ScannerConfig | User-managed, low volume |
| **Projection CRDs** | etcd | ClusterSecuritySummary, NamespaceRiskStatus | Operator-managed summaries, periodic updates |
| **Backend Data** | Hub Vuln Management Service | VulnerabilityReport details, historical trends, fleet queries | High volume, API access at hub level |

Console pattern:

* Dashboard/summary views -> Projection CRDs (fast, cached)
* Drill-down/detail views -> Hub Vuln Management Service API (fleet queries,
  historical)

### CRD Architecture: Scalability and Security Considerations

A legitimate concern with CRD-first architecture: **Can CRDs scale for
policy evaluations, and does using K8s APIs to secure K8s create circular
dependencies?**

Prior analysis in `scratchpad/acm-acs-addon/k8s-resources-discussion.md`
examined this in depth. Key findings:

#### Trust Boundary Model

**Critical insight**: The security-critical data path already bypasses K8s
API:

| Function | K8s API Dependency | Risk Level | Notes |
|----------|-------------------|------------|-------|
| **Runtime data collection** | Low (eBPF direct) | Highest impact | Kernel -> Collector -> Sensor -> Central |
| **Security event transport** | Low (gRPC) | Highest impact | Direct gRPC channel, no K8s API |
| **Cluster-wide policies** | Low (gRPC) | Highest impact | Delivered via gRPC from Central |
| **K8s inventory** | High (Watch API) | High | Cross-check with Collector's process visibility |
| **SecurityResult CRs** | High (write) | **Low** | Read-only projection; Central is source of truth |
| **Namespace policies** | High (read) | Medium-High | Additive only; cluster-wide baseline maintained |

**Design Principle**:

```
Cluster-wide policies (from Central, via gRPC) = Immutable security baseline
Namespace policies (from K8s API) = Additive restrictions only

If K8s API is compromised:
* Namespace policies can be deleted/weakened
* BUT cluster-wide baseline remains enforced
* Security is degraded but not eliminated
```

#### Addressing the Circular Dependency Concern

The concern "StackRox secures K8s, but uses K8s API to operate" is valid but
manageable:

1. **Collection bypasses K8s API**: eBPF collects process/network data
   directly from kernel
2. **Transport bypasses K8s API**: gRPC channels deliver security events
3. **Enforcement bypasses K8s API for baseline**: Cluster-wide policies
   enforced regardless of K8s API state
4. **CRs are observability layer**: SecurityResult CRs are projections for
   user consumption, not the source of truth

**Mitigation**: Central tracks expected namespace policies and alerts on
drift. If K8s API is compromised and policies are tampered with, Central
detects the drift.

#### Prior Work and Validation

* **`scratchpad/all-the-crds-design/`**: Comprehensive CRD design including
  schema definitions, decision matrix, and implementation plan
* **`stackrox-results-operator`**: Working implementation demonstrating CRD
  patterns at scale
* **`scratchpad/namespace-scoped-acs/CR_FIRST_JUSTIFICATION.md`**: Scalability
  analysis showing CRD aggregation patterns (one CR per namespace with
  truncation) manage etcd load effectively

#### Scalability Recommendation

Following the two-tier CR strategy:

* **`SecurityResult`** (namespace-scoped): Top N violations/CVEs for developer
  workflows
* **`ClusterSecuritySummary`** (cluster-scoped): Pre-aggregated counts for ACM
  dashboards

This keeps CR count manageable (~250 CRs cluster-wide for 250 namespaces)
while providing K8s-native access patterns.

### Scanner Architecture

**PM Sensitivity Note**: Product management is highly sensitive to adding
workloads on managed clusters. Any scanner architecture must minimize managed
cluster footprint while maintaining functionality.

Scanner operates as a **compute service** on each cluster: it indexes images
and matches vulnerabilities but does not persist match results locally.
Vulnerability match results flow to summary-level CRDs on the managed cluster
and to the hub-level Vuln Management Service for fleet queries and historical
trends.

Options for vulnerability scanning architecture:

| Model | Managed Cluster Footprint | Pros | Cons |
|-------|---------------------------|------|------|
| **Local scanner per cluster** | High (Scanner + VulnDB per cluster) | Works disconnected; fast local scans | Resource duplication; PM concern |
| **Hub scanner (DB sync)** | Medium (Local scanner, synced DB) | Centralized DB updates; local evaluation | Still requires scanner on each cluster |
| **Hub scanner via Maestro** | **Minimal** (SBOM collector only) | Minimal footprint; shared scanner | Connectivity required; latency |
| **SaaS matcher** | Minimal | No infrastructure | Disconnected customers can't use |

#### Shared Scanner via ACM Transports (Maestro)

A promising option leverages ACM's multi-cluster transport layer (Maestro)
for shared scanning:

```
┌─────────────────────────────────────────────────────────────┐
│                         ACM Hub                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │              Shared Scanner Service                     │  │
│  │  * Vulnerability DB (single instance)                   │  │
│  │  * Receives SBOMs from managed clusters via Maestro     │  │
│  │  * Returns vulnerability matches                        │  │
│  └────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
         ▲                    │                    ▲
         │ SBOM               │ Results            │ SBOM
         │ (via Maestro)      ▼ (via Maestro)      │
┌─────────────────┐                        ┌─────────────────┐
│   Cluster A     │                        │   Cluster B     │
│ ┌─────────────┐ │                        │ ┌─────────────┐ │
│ │SBOM Collector│ │                        │ │SBOM Collector│ │
│ │(lightweight) │ │                        │ │(lightweight) │ │
│ └─────────────┘ │                        │ └─────────────┘ │
└─────────────────┘                        └─────────────────┘
```

**How it works**:

1. Lightweight SBOM collector on managed cluster extracts image/package
   inventory
2. SBOM sent to hub via Maestro (ACM's existing multi-cluster transport)
3. Shared scanner on hub matches SBOM against vulnerability DB
4. Results returned to managed cluster via Maestro
5. Results stored as summary-level CRs on managed cluster for local access

**Trade-offs**:

| Consideration | Hub Scanner (Maestro) | Local Scanner |
|---------------|----------------------|---------------|
| Managed cluster footprint | **Minimal** (SBOM collector ~50MB) | High (Scanner + DB ~2GB) |
| Disconnected operation | No | Yes |
| Scan latency | Higher (network round-trip) | Lower (local) |
| DB freshness | Centralized, always current | Depends on sync frequency |
| Operational complexity | Lower (one scanner to manage) | Higher (scanner per cluster) |

**Recommendation**: Offer both models:

* **Default**: Hub scanner via Maestro (minimal footprint, connected
  deployments)
* **Option**: Local scanner with DB sync (disconnected/air-gapped
  environments)

This addresses PM sensitivity to managed cluster footprint while providing
flexibility for disconnected customers.

### What Gets Removed from Current Architecture

| Component | Current | ACS Next | Rationale |
|-----------|---------|----------|-----------|
| Central aggregation | Core | Removed | ACM handles this |
| Sensor-Central gRPC | Core | Removed | Same-cluster, not needed |
| Network graph | ~30% UI | Removed | Complexity vs value |
| Custom notifiers | 10+ integrations | Removed | Use AlertManager |
| Compliance framework | Built-in | Delegated | ACM compliance-operator |
| Multi-cluster identity | Custom | Delegated | ACM identity federation |
| Custom RBAC engine | ACS SAC | Delegated | K8s RBAC + ACM |

**Estimated codebase reduction**: 25-30%

### What Replaces Central: Component Architecture

Central currently handles data aggregation, policy management, UI/API,
vulnerability correlation, compliance, risk scoring, and notifications. In
ACS Next, these responsibilities are distributed:

#### Managed Cluster Components

```
┌─────────────────────────────────────────────────────────────────┐
│              Security Controller (single deployment)             │
│                                                                  │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐              │
│  │    Build    │  │   Deploy    │  │   Runtime   │              │
│  │  Container  │  │  Container  │  │  Container  │              │
│  │             │  │             │  │             │              │
│  │ * roxctl    │  │ * Admission │  │ * Collector │              │
│  │   endpoint  │  │   webhook   │  │   sub       │              │
│  │ * Scanner   │  │ * Deploy    │  │ * Audit     │              │
│  │   client    │  │   policies  │  │   logs      │              │
│  │ * Build     │  │             │  │ * Runtime   │              │
│  │   policies  │  │             │  │   policies  │              │
│  └─────────────┘  └─────────────┘  └─────────────┘              │
│                         │                                        │
│         ┌───────────────┴───────────────┐                       │
│         │  Shared: Policy Engine (lib)  │                       │
│         │  Shared: SQLite persistence   │                       │
│         └───────────────────────────────┘                       │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      Scanner Operator                            │
│  * Image indexing (compute service)                              │
│  * Vulnerability DB                                              │
│  * Matcher service                                               │
│  * Emits summary-level CRs (not individual vuln records)        │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                         Collector                                │
│  * eBPF data collection (DaemonSet)                             │
│  * Pub/sub API with subscriber-side filtering                   │
│  * Extensibility point for customers                            │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│              Baseline Operator (OPP-only, optional)              │
│  * Process baseline learning and anomaly detection              │
│  * Network baseline learning and anomaly detection              │
│  * Subscribes to Collector                                       │
│  * Emits BaselineAlert CRs                                      │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│           Historical Data Operator (OPP-only, optional)          │
│  * Watches PolicyViolation CRs                                   │
│  * Aggregates and builds trends over time                       │
│  * Enables historical queries without burdening etcd            │
│  * Local persistence for time-series data                       │
└─────────────────────────────────────────────────────────────────┘
```

**Component summary:**

| Component | Type | OCP/OPP |
|-----------|------|---------|
| Security Controller | 1 Deployment (3 containers) | OCP |
| Scanner Operator | 1 Deployment | OCP |
| Collector | 1 DaemonSet | OCP |
| Baseline Operator | 1 Deployment | OPP (optional) |
| Historical Data Operator | 1 Deployment | OPP (optional) |

**Total: 3-5 deployments per cluster** (comparable to today's footprint)

#### Hub Components

```
┌─────────────────────────────────────────────────────────────────┐
│                          ACM Hub                                 │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                      ACM (existing)                       │   │
│  │  * Search: aggregates CRs from managed clusters          │   │
│  │  * Governance: distributes policies to managed clusters  │   │
│  │  * Console: multi-cluster perspective in OCP Console     │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │                    ACS Addon (new)                        │   │
│  │  * Manages deployment of components to clusters          │   │
│  │  * Configures which operators deploy where               │   │
│  │  * Health monitoring                                      │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │              Hub-Based Scanner (optional)                 │   │
│  │  * Shared vulnerability DB                                │   │
│  │  * Receives SBOMs via Maestro                            │   │
│  │  * Returns matches to managed clusters                    │   │
│  │  * Minimizes managed cluster footprint                    │   │
│  └──────────────────────────────────────────────────────────┘   │
│                                                                  │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │           Vuln Management Service (hub-level)             │   │
│  │  * Fleet-wide vulnerability queries and search           │   │
│  │  * Historical trend analysis across clusters              │   │
│  │  * Replaces per-cluster Persistence Service               │   │
│  │  * Backs drill-down/detail views in Console               │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

#### Key Architectural Patterns

**Policy engine as library, not service:**

* Each container in Security Controller imports the policy evaluation library
* No inter-service calls for policy evaluation
* Policy CRDs (StackroxPolicy, ClusterStackroxPolicy) watched by all
  containers
* Same library compiled into Build, Deploy, and Runtime containers

**Deploy Phase: Admission controller approach:**

* ValidatingWebhookConfiguration intercepts CREATE/UPDATE/DELETE/CONNECT
* Evaluates policies at admission time
* **MVP scope**: Admission-only (no continuous K8s API watch)
* **Optional**: Startup list to evaluate existing resources when operator
  deploys
* **Post-MVP**: Watch for policy changes to trigger re-evaluation of existing
  resources
* This simplifies the architecture; continuous reconciliation adds complexity
  without MVP-critical value

**Runtime Phase: Audit log consumption:**

* Runtime container consumes K8s audit logs in addition to Collector events
* Audit logs catch operations admission webhook cannot:
    * `kubectl exec` into pods
    * `kubectl port-forward`
    * Secrets access patterns
    * API access anomalies
* Enables runtime policy evaluation for API-level activity

**Collector as extensible event source:**

* Collector exposes pub/sub API for runtime events (process exec, network
  connections)
* **Subscriber registration**: Subscribers register with filter specifications
* **Server-side filtering**: Collector applies filters before transmitting
  (efficient)
* Transport mechanism: gRPC streaming or in-cluster message bus (TBD)
* Runtime container and Baseline Operator subscribe independently
* **Customer extensibility**: Documented API enables customers to write
  custom subscribers
    * Build custom detections
    * Integrate with proprietary SIEM
    * Feed into internal security tooling
* This is an explicit differentiator: "OCP security provides raw runtime
  events as an extensibility point"

**No discrete API server (CRDs as primary persistence):**

* PolicyViolation CRs emitted by all phase containers (labeled by phase:
  build/deploy/runtime)
* ImageVulnerability summary CRs emitted by Scanner Operator
* BaselineAlert CRs emitted by Baseline Operator
* SQLite for local state that doesn't fit CRDs (scan details, baseline data)
* **K8s API is the API**: No Central-like REST/gRPC server to build or
  maintain
* **Rationale**: Reduces operational complexity; leverages existing K8s
  infrastructure
* Historical trends handled by Historical Data Operator (watches CRs, builds
  time-series)

**OCP-native notifications:**

* Prometheus metrics -> AlertManager -> Slack, PagerDuty, email, etc.
* Structured logs -> OCP Logging stack -> Splunk, Elastic, etc.
* K8s Events -> OCP Console visibility
* Central's custom notifiers are removed; OCP handles routing
* **Result**: We emit signals; OCP routes them. No notification logic to
  maintain.

**ACM for fleet operations:**

* ACM Search aggregates CRs across managed clusters (no custom aggregation)
* ACM Governance distributes policy CRs to managed clusters (no custom
  distribution)
* OCP Console's multi-cluster perspective provides security dashboard (no
  custom fleet UI)
* **Principle**: Leverage ACM entirely; don't rebuild fleet capabilities

#### Central Function Mapping

| Central Function | ACS Next Replacement |
|------------------|---------------------|
| Data storage (PostgreSQL) | CRDs + SQLite per cluster; Vuln Management Service on hub |
| Multi-cluster aggregation | ACM Search |
| Policy management | Policy CRDs + ACM Governance |
| UI/Dashboard | OCP Console (single-cluster + multi-cluster perspectives) |
| REST/gRPC API | K8s API (CRDs) |
| Vulnerability correlation | Scanner Operator (compute) + hub Vuln Management Service |
| Compliance | Integration with compliance-operator |
| Risk scoring | Computed locally, stored as CRs |
| Notifications | AlertManager + Logging stack |
| Fleet queries / trends | Vuln Management Service (hub-level) |

#### Product Identity

ACS Next components are not branded "ACS" in the user experience. They are
**OCP security capabilities**:

* Security Controller, Scanner Operator, Collector appear as platform
  components
* OCP Console shows security views (not "ACS Dashboard")
* OCP Console's multi-cluster perspective shows fleet security (not "ACS
  Fleet View")
* The brand is invisible; the capability is native

**OPP subscription** provides:

* Support for security components
* Access to advanced operators (Baseline, Historical)
* Certified builds and SLAs

This aligns with Red Hat's subscription model where customers pay for
support, not feature gates.

#### Open Questions

The following decisions are deferred for later design phases:

| Question | Options | Notes |
|----------|---------|-------|
| **Collector pub/sub transport** | gRPC streaming vs in-cluster message bus (NATS, etc.) | Both viable; gRPC is familiar, message bus is more decoupled |
| **Runtime Phase: OCP or OPP?** | Include in OCP vs require OPP subscription | Affects adoption strategy; runtime detection is significant value |
| **Additional OCP notification APIs** | Other notification/observability integrations beyond AlertManager | Need to survey OCP capabilities |
| **Cloud-specific notifiers** | AWS Security Hub, Google Cloud SCC integration | May need separate solution; doesn't fit AlertManager pattern |
| **Scanner service deployment** | Inside Build container vs separate deployment | Affects footprint and separation of concerns |
| **Compliance integration** | Extend compliance-operator vs build ACS compliance operator | Depends on compliance-operator capabilities and roadmap |

*See [architecture.md](architecture.md) for detailed component design and
[gap-analysis.md](gap-analysis.md) for feature gap analysis.*

---

## Part IV: Costs and Risks

### Engineering Costs

| Cost | Severity | Notes |
|------|----------|-------|
| Build single-cluster ACS variant | High | New operator, decoupled components, local persistence |
| CRD data model design | Medium-High | Vulnerabilities, alerts, compliance as CRs |
| Scanner architecture decision | Medium | Local vs. shared vs. hybrid |
| Maintain two architectures | High | During 5.0 LTS transition period |
| ACM addon development | Medium | Integration work with ACM team |

### Business/Customer Costs

| Cost | Severity | Notes |
|------|----------|-------|
| ACM required for multi-cluster | High | What about standalone K8s customers? |
| Migration path complexity | High | Existing Central-based deployments |
| Feature parity during transition | Medium | Some features may lag in new model |
| Customer communication | Medium | Explaining the direction |

### Organizational Costs

| Cost | Severity | Notes |
|------|----------|-------|
| Buy-in across teams | High | ACS, ACM, PM, leadership alignment |
| Potential resistance | Medium | Investment in current architecture |
| Timeline uncertainty | High | Large architectural shifts are hard to estimate |

### Risk Mitigations

| Risk | Mitigation |
|------|------------|
| Customer disruption | 5.0 LTS provides stability; ACS Next is opt-in |
| ACM dependency | Clear OCP vs OPP positioning; ACM is already required for OPP value |
| Engineering effort | Phased approach; MVP first; parallel tracks |
| Feature regression | Explicit feature mapping; gap analysis before migration |
| Microservice proliferation | Consolidate related functions (3 containers in 1 deployment); resist splitting unless clear operational benefit; maintain discipline on component count |

### Investment vs. Ongoing Cost: The Real Comparison

**The question is not "can we afford ACS Next?"**
**The question is "can we afford to keep paying the current architecture tax?"**

| Scenario | One-Time Cost | Ongoing Cost per Quarter |
|----------|---------------|--------------------------|
| **Current Architecture** | $0 | High: coordination overhead, slow features, RBAC impasse, tech debt accumulation |
| **ACS Next** | High: 12-18 month development | Lower: clear ownership, independent shipping, leverage ACM investment |

Consider what the current architecture costs us every quarter:

* Engineering time lost to cross-team coordination
* Features that take 3x longer than they should
* Bugs that bounce between teams
* RBAC convergence meetings that don't converge
* Duplicating ACM capabilities
* Console plugin workarounds

**ACS Next is not a cost — it's an investment that reduces ongoing costs.**

### Should Decoupling Apply to ACS Legacy?

A natural question: if decoupling components is valuable for ACS Next, should
we apply the same decoupling to ACS Legacy (5.x)?

**The Concern**: Building ACS Next with decoupled architecture while
maintaining ACS Legacy with the current monolithic architecture means
maintaining two divergent codebases. Should we backport decoupling?

#### Options

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| **A: ACS Next Only** | Decoupling applies only to new architecture | Lower risk; cleaner separation; 5.2 LTS remains stable | Two architectures diverge; no velocity benefit for Legacy |
| **B: Backport to Legacy** | Extract components from Central in 5.x line | Shared components; some velocity benefit for Legacy | Higher risk to LTS stability; significant 5.x disruption |
| **C: Selective Extraction** | Extract specific components (e.g., Scanner) that can be shared | Middle ground; some code sharing | Partial benefit; still some divergence |

#### Analysis

**Risk-benefit for Option B (Backport)**:

* **Risk**: 5.2 LTS customers expect stability. Major architectural changes
  to LTS are antithetical to LTS purpose.
* **Benefit**: Reduced code divergence between Legacy and Next.
* **Assessment**: Risk outweighs benefit. LTS customers chose LTS for
  stability, not for architectural improvements.

**Case for Option A (ACS Next Only)**:

* 5.2 LTS gets bug fixes and security patches, not architectural changes
* ACS Next can iterate freely without risking LTS stability
* Clear mental model: Legacy = stable, Next = innovative
* After ACS Next validation, teams can evaluate selective backporting with
  lower risk

#### Recommendation

**Start with Option A (ACS Next Only)**. Evaluate backporting specific
improvements (Option C) after ACS Next architecture is validated in production
(6-12 months post-launch).

**Rationale**: The 5.2 LTS commitment means stability for existing customers.
ACS Next is the place for architectural innovation. Attempting to modernize
both simultaneously doubles risk without doubling benefit.

---

## Part V: Why Now?

### The Forcing Functions

1. **PM/UX strategic direction**  —  Product and UX are shifting toward deeper
   OPP portfolio integration and single-pane-of-glass experiences. This is the
   primary driver. Everything else follows from this direction.
2. **RBAC convergence is a downstream requirement**  —  portfolio integration
   requires unified identity; RBAC convergence is stuck because the current
   architecture can't support it
3. **5.2 LTS timing**  —  creates natural transition window without customer
   disruption
4. **ACM maturity**  —  ACM now has the capabilities (Search, Governance,
   AddOn framework) to support this
5. **Competitive pressure**  —  market expects K8s-native security, GitOps
   workflows, low-friction adoption
6. **Engineering velocity may be suffering**  —  cross-team coordination,
   unclear ownership, and monolithic coupling could slow features *(hypothesis
    —  requires validation)*

### The Potential Cost of Inaction

Continuing the incremental approach could mean:

**For Engineering (hypothesized  —  requires validation):**

* New features may continue fighting Central's monolithic architecture
* Policy changes might continue taking longer than necessary
* Cross-team coordination overhead could persist
* Engineers might continue avoiding certain areas (policy engine, multi-cluster
  code)
* CI pipeline could remain slower than desired
* Technical debt may compound

**For Product:**

* More months/years of RBAC convergence discussions without resolution
* Continued duplication of ACM capabilities in ACS
* OCP Console plugin remaining a "limited view"
* No path to low-friction adoption (ACS feels like separate product)
* Continued customer friction with separate RBAC, separate console, separate
  concepts

**For the Business:**

* Growing technical debt in Central
* Harder to hire/retain engineers (legacy architecture)
* Competitive disadvantage against K8s-native security tools
* No path to low-friction adoption (ACS feels like separate product)
* Continued customer friction with separate RBAC, separate console, separate
  concepts

### Implications for Roadmap

If ACS Next is not pursued, capabilities that require architectural change
should be removed from near-term roadmap discussions:

* RBAC convergence
* Policy placement via ACM Governance
* Deep ACM integration / single-pane-of-glass
* Configurable risk scoring (beyond incremental improvements)

This isn't punitive — it's practical. Continued design work on these
capabilities within the current architecture has not produced viable solutions,
and future attempts are unlikely to change that. Engineering effort is better
directed toward achievable improvements.

The current architecture remains capable of serving existing customers and
supporting incremental feature development. These roadmap adjustments only
affect capabilities that depend on assumptions the current architecture
doesn't make.

### The Alternative Paths: A Detailed Comparison

This section presents the strategic alternatives to help stakeholders evaluate
trade-offs explicitly. Each path represents a valid choice with different
trade-offs.

#### Path A: ACM Addon Querying Central (Minimal Integration)

**Description**: Build an ACM addon that deploys SecuredCluster and provides a
Console view by querying existing Central API.

**What You Get**:

* Multi-cluster Console view via ACM
* Faster time to value (3-6 months)
* Lower engineering investment
* Works with existing Central architecture

**What You Don't Get**:

* RBAC convergence (still separate ACS RBAC + K8s RBAC)
* K8s-native APIs (no CRDs for security data)
* Low-friction adoption (Central required for any value = higher evaluation
  barrier)
* Engineering velocity improvements (monolithic architecture remains)
* Composability (all-or-nothing deployment)

**Best For**: Organizations prioritizing near-term ACM integration over
long-term architectural transformation.

---

#### Path B: Incremental Decoupling Within Current Architecture

**Description**: Progressively extract components from Central (Scanner,
Policy Engine) as independent services while maintaining Central as the hub.

**What You Get**:

* Some engineering velocity improvement (cleaner component boundaries)
* Gradual transition (lower risk than full rewrite)
* Maintain existing customer deployments
* Some composability (deploy Scanner independently)

**What You Don't Get**:

* Full RBAC convergence (Central still required)
* K8s-native data model (APIs remain Central-centric)
* ACM as orchestrator (Central remains the hub)
* Low-friction adoption (Central dependency remains = separate product feel)

**Best For**: Organizations wanting architectural improvements but unwilling to
commit to ACM as the orchestration layer.

**Risks**: "Big ball of mud" problem — incremental decoupling often results in
more interfaces without cleaner boundaries. May end up with worst of both
worlds.

---

#### Path C: ACS Next (Full Architectural Shift)

**Description**: Single-cluster ACS operators with ACM providing multi-cluster
orchestration. CRD-first data model. K8s RBAC replaces ACS RBAC.

**What You Get**:

* RBAC convergence (K8s RBAC is the engine)
* K8s-native APIs (security data as CRDs)
* Low-friction adoption (K8s-native = feels like part of OCP, natural upgrade
  to OPP)
* Engineering velocity (clear ownership, independent operators)
* Composability (deploy only what you need)
* ACM leverage (reuse ACM Search, Governance, Identity)

**What You Don't Get**:

* Near-term delivery (12-18 months to parity)
* Standalone multi-cluster (ACM required)
* Continuity with current architecture (parallel tracks during transition)

**Best For**: Organizations committed to OPP portfolio integration and willing
to invest in architectural transformation.

---

#### Comparison Matrix

| Capability | Path A (Addon) | Path B (Incremental) | Path C (ACS Next) |
|------------|----------------|----------------------|-------------------|
| RBAC convergence | No | Partial | Yes |
| K8s-native APIs | No | Partial | Yes |
| Low-friction adoption | No (separate product feel) | No (separate product feel) | Yes (feels like OCP) |
| Engineering velocity | No improvement | Some improvement? | Potential improvement* |
| Time to value | 3-6 months | 6-12 months | 12-18 months |
| Engineering investment | Low | Medium | High |
| Risk level | Low | Medium (scope creep) | Medium (new architecture) |
| Standalone multi-cluster | Yes (Central) | Yes (Central) | Requires ACM |

*Engineering velocity improvements are hypothesized based on architectural
principles, not validated through measurement.

---

#### Recommendation

**Path C (ACS Next)** if:

* PM/leadership are committed to OPP portfolio integration and
  single-pane-of-glass
* Reducing adoption friction is a business priority
* RBAC convergence is required (which it is, as a downstream requirement of
  portfolio integration)
* Potential engineering velocity improvement is valued (though this benefit is
  hypothesized, not proven)

**Path A (ACM Addon)** if:

* Near-term OCP Console integration is the priority
* Reducing adoption friction is not a priority
* Existing ACS architecture is acceptable for foreseeable future
* Investment capacity is limited

**Avoid Path B** unless there's a specific forcing function. Incremental
decoupling without a clear target architecture tends to create complexity
without solving the underlying problems.

---

## Part VI: Next Steps

### Immediate (Socialization)

1. **Identify allies**: PM stakeholders aligned with OCP security pillar
   vision
2. **ACM team engagement**: Gauge appetite for ACS as ACM addon responsibility
3. **Leadership alignment**: Present business case (cost savings, market
   positioning, ARR story)

### Near-term (Validation)

1. **Prototype**: Minimal single-cluster ACS with ACM Search aggregation
2. **Customer signal**: Identify pilot customer for ACS Next
3. **Resource estimation**: Rough sizing with engineering leads

### Medium-term (Execution)

1. **Phase 0**: Validation prototype (1-2 months)
2. **Phase 1**: Core single-cluster ACS (3-4 months)
3. **Phase 2**: ACM integration (2-3 months)
4. **Phase 3**: Migration tooling and polish (2-3 months)

---

## Appendix A: Prior Work

This document consolidates prior analysis from:

### Strategic Architecture

* `scratchpad/next-gen-security-platform/unified-architecture.md`  —
  Comprehensive composable architecture
* `scratchpad/next-gen-security-platform/two-tier-strategy.md`  —  ACS
  Advanced + ACS Essentials model
* `scratchpad/stackrox-lite-proposal/PROPOSAL.md`  —  Single-cluster refactor
  proposal
* `scratchpad/roadmap/STACKROX_ACS_VISION_ROADMAP.md`  —  Three-phase
  transformation roadmap

### ACM Integration

* `scratchpad/acm-acs-addon/design.md`  —  ACM addon technical design
* `scratchpad/acm-acs-addon/k8s-resources-discussion.md`  —  K8s API
  dependency and security analysis; trust boundary model for CRD architecture

### CRD Architecture and Implementation

* `scratchpad/all-the-crds-design/README.md`  —  Comprehensive CRD design
  documentation including schema definitions, decision matrix, and
  implementation plan
* `scratchpad/all-the-crds-design/crd-design-plan.md`  —  Detailed 50+ page
  design plan for security results as CRDs
* `stackrox-results-operator`  —  Working implementation of CRD patterns
  demonstrating feasibility

### Multi-Tenancy and Namespace Scoping

* `scratchpad/namespace-scoped-acs/CR_FIRST_JUSTIFICATION.md`  —  Strategic
  justification for CRD-first approach including scalability analysis
* `scratchpad/namespace-scoped-acs/`  —  Multi-tenancy design work addressing
  namespace-level isolation

### RBAC Convergence

* RBAC convergence design document and meeting notes (2026-02-24)
* [RBAC Convergence Google Doc](https://docs.google.com/document/d/144jpKqZ17MtkzkJKJUty6vyHWxmlPrE9Y2aUPN0G3iI)
   —  Six solution analysis showing architectural constraints

## Appendix B: Key Stakeholder Questions

Questions likely to arise during socialization:

1. **"What about customers who don't have ACM?"**
   Single-cluster ACS works standalone. Multi-cluster requires ACM  —  which
   is already the OPP value proposition.

2. **"What's the migration path for existing customers?"**
   5.0 LTS provides 5 years of support. Migration is opt-in when ACS Next
   matures. Tooling provided for policy export/import.

3. **"What about the footprint increase?"**
   Full single-cluster config is larger (~4x memory). But: composability lets
   you deploy less; Central footprint moves to hub where ACM already lives.

4. **"How is this different from what we discussed before?"**
   5.0 LTS timing is new. ACSCS sunset simplifies. RBAC convergence impasse
   is explicit evidence of architectural constraint.

5. **"What's the minimal version that proves this works?"**
   Single-cluster scanner + policy engine with ACM Search aggregating
   vulnerability summaries across clusters.

6. **"How do disconnected customers use ACM? Wouldn't ACS have the same
   problem?"**
   Disconnected (air-gapped) ACM deployments exist and are supported. In
   disconnected mode:
   * ACM hub and managed clusters operate within a closed network
   * Catalog updates and images are loaded via sneakernet/mirrored registries
   * Multi-cluster management still works within the disconnected environment

   ACS would follow the same pattern:
   * Vulnerability DB updates loaded via mirrored content
   * Local scanner option (not hub-based) for fully disconnected clusters
   * Single-cluster ACS operates independently with no connectivity
     requirements
   * ACM integration works within the disconnected hub/managed-cluster
     topology

## Appendix C: References

* [RBAC Convergence Design Doc](https://docs.google.com/document/d/144jpKqZ17MtkzkJKJUty6vyHWxmlPrE9Y2aUPN0G3iI)
* [ACM AddOn Framework](https://github.com/open-cluster-management-io/addon-framework)
* [OpenShift Console Dynamic Plugins](https://docs.openshift.com/container-platform/latest/web_console/dynamic-plugin/overview-dynamic-plugin.html)
* RFE-5918  —  Customer requests for K8s RBAC integration
* [architecture.md](architecture.md)  —  Detailed technical architecture
* [gap-analysis.md](gap-analysis.md)  —  Feature gap analysis
