# ACS Next: Rationale

*Status: Draft | Date: 2026-03-12*

This document provides the evidence and analysis behind the
[Business Case Brief](brief.md). Read the brief first. Read this if you
need to understand the tradeoffs, see the alternatives we considered, or
want the detailed case for why the current architecture can't get us where
we need to go.

For technical design details, see [Architecture](architecture.md).

---

## Why the Current Architecture Constrains Us

### RBAC Convergence: Six Solutions, One Constraint

Portfolio integration requires RBAC convergence — users should not need
separate ACS roles in addition to their K8s/OCP roles. For months, teams
have attempted this. The
[RBAC convergence design document](https://docs.google.com/document/d/144jpKqZ17MtkzkJKJUty6vyHWxmlPrE9Y2aUPN0G3iI)
explores six solutions:

| Solution | Challenge |
|----------|------------|
| Naive K8s role resolution | Performance — many RBAC queries across clusters |
| Per-query data filtering | Performance — retrieves entire database per request |
| External role broker | Doesn't exist yet |
| OCP dynamic plugin | No cross-cluster aggregated views |
| Change API granularity | Requires full API rework |
| CR-based declarative config | Still requires separate ACS RBAC configuration |

Each solution runs into significant trade-offs. The challenge isn't a lack
of effort or ideas — it's that **every approach is constrained by the same
architectural assumption: Central remains the aggregator and authority.**
Every solution attempts to answer "how do we make K8s RBAC work with
Central?" The difficulty in finding a clean answer is symptomatic of a
deeper issue: the current architecture was not designed for platform
integration.

This suggests the right question isn't "which RBAC convergence approach
should we pick?" but "should we change the architectural assumptions that
make RBAC convergence so difficult?"

### The Multi-Cluster Identity Problem

The RBAC design doc identifies a core challenge:

> "The RHACS access control engine brings a multi-cluster dimension that
> does not exist in standard Kubernetes clusters. A naive solution relying
> on Kubernetes roles to perform access control within RHACS would have to
> map the RHACS user to a Kubernetes user on the cluster where central is
> running, resolve the identity of that Kubernetes user, identify users on
> the monitored clusters that have the same identity..."

This is Central trying to solve a problem that **ACM has already solved**.
Rather than building RBAC convergence into Central, delegate multi-cluster
identity to the platform component designed for it.

### Why Incremental Approaches Have Hit a Ceiling

Progress has been made incrementally, but each step reveals the limits of
the current architecture:

1. **OCP Console Plugin**: Built and shipped — but limited to
   single-cluster, read-only, incomplete data
2. **Declarative Config via CRDs**: Implemented — but doesn't solve RBAC
   convergence
3. **External Role Broker**: Discussed in multiple design rounds — but the
   scope of building one within Central's model is daunting
4. **ACM coordination**: Ongoing architect conversations — but the
   integration surface remains unclear

Each step delivers value, but none addresses the root constraint: Central's
hub-and-spoke model doesn't naturally compose with the K8s/OCP platform
model.

---

## Organizational Friction: Conway's Law in Action

> "Organizations which design systems are constrained to produce designs
> which are copies of the communication structures of these organizations."
> — Melvin Conway, 1967

The current ACS architecture may be creating organizational friction.
Central's monolithic design emerged from a centralized team structure, but
as teams have grown and specialized, the architecture no longer reflects
how we work — and this mismatch imposes a tax on engineering velocity.

**The Hypothesized Friction**: Features naturally cross component
boundaries (Central, Sensor, shared packages), but team ownership doesn't
align with those boundaries:

```
Current: Architecture crosses team boundaries
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Central   │◄──►│  Shared Pkgs│◄──►│   Sensor    │
│   (Team A)  │    │  (Team ???) │    │   (Team B)  │
└─────────────┘    └─────────────┘    └─────────────┘
        ▲                 ▲                  ▲
        └─────── Feature X spans all three ───┘
```

**Possible Result**: Non-trivial features become cross-team coordination
exercises. Bug fixes bounce between teams. Engineers avoid certain areas
if ownership and blast radius are unclear.

**ACS Next aligns architecture with ownership:**

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

Components have well-defined interfaces (pub/sub topics, CRD schemas),
making ownership explicit. External teams can contribute by subscribing to
the broker or publishing CRDs — no need to understand Central internals.

*This hypothesis should be validated with engineering teams.*

### Policy Engine: A Case Study in Architectural Friction

| Observation | Evidence | Hypothesized Impact |
|-------------|----------|---------------------|
| **Monolithic field registry** | 966-line `initializeFieldMetadata()` function; all policy criteria in one place | Could make adding new detection criteria slower than necessary |
| **6+ files to add one policy field** | Changes required: fieldnames, querybuilders, field_metadata, violationmessages, validate, tests | May increase regression risk; engineers might avoid policy changes |
| **Central/Sensor split ownership** | `pkg/booleanpolicy` (97 files) shared by both; 299 files depend on it | Could create unclear team ownership; bug fixes may require cross-team coordination |
| **No API versioning** | Central and Sensor compile policies independently with shared code | Version skew during upgrades could cause detection issues |
| **20+ dependencies in policy service** | `service_impl.go` constructor injects cluster, deployment, network, notifier, image, MITRE datastores... | May be harder to test and reason about |

#### Hypothesized Velocity Impact (Requires Validation)

**These are not validated metrics.** Before using these claims to justify
investment, they should be validated through engineering team surveys,
ticket/PR data analysis, and comparison with similar architectural changes.

| Metric | Hypothesized Current State | Hypothesized With Clear Ownership |
|--------|---------------------------|-----------------------------------|
| Time to add new policy criterion | ~3 sprints? | ~1-2 sprints? |
| Policy-related bugs per release | ~5-8? | ~2-3? |
| Customer escalation resolution | ~2-3 weeks? | ~1 week? |
| Engineer onboarding to policy code | ~4-6 weeks? | ~2-3 weeks? |

### The Broader Pattern

Policy engine may not be unique. Similar friction appears wherever
Central's monolithic nature requires cross-cutting changes:

* **Adding a new data type**: May touch Central storage, API, UI, Sensor
  collection, gRPC protocol
* **Console plugin features**: Limited by what Central exposes; can't query
  K8s directly
* **Scanner integration**: Tightly coupled to Central's lifecycle
* **Any multi-cluster feature**: Must reason about Sensor connections, data
  propagation, consistency

**Clarification on release coordination**: ACS Next doesn't eliminate
coordination — it moves it to well-defined boundaries. Upstream, teams
develop and release operators independently. Downstream/OPP, the product
release still coordinates which operator versions ship together — but the
boundaries are cleaner (versioned operators with CRD APIs) rather than
entangled (shared packages compiled into different binaries).

---

## Concrete Example: Remediation Guidance

To make the architectural difference tangible, consider a hypothetical
feature: **remediation guidance** — "what critical CVEs will be fixed if I
upgrade OCP?" and "how do I fix this policy violation?"

**In the current architecture**, this feature lives in Central. You'd add
a new service (or extend VulnMgmtService), ingest OCP release manifest
data into PostgreSQL, add new API endpoints, build new UI pages, and wire
it through the SAC engine for RBAC. The data joins are easy (it's all in
one database), but the blast radius is the entire monolith — new tables,
new service dependencies, new API surface, all coupled to Central's
release cycle. A separate team can't own it without deep Central context.

**In ACS Next**, this is two things:

* **Policy violation remediation** requires no new component at all — the
  policy engine enriches PolicyViolation CRs with a `remediation` field
  ("set `privileged: false`", "add memory limits"). It's a richer output
  from existing components.
* **Vulnerability remediation** is a new broker consumer. It subscribes to
  scan results (existing feed), cross-references OCP release data (new
  data source), queries Scanner for fix versions, and produces
  `RemediationAdvice` CRs. These appear in OCP Console alongside
  violations. At fleet level, the Vuln Management Service aggregates
  remediation impact across clusters.

The pattern is **subscribe to existing feeds, produce new CRs**. The
component doesn't modify Scanner, Collector, or the policy engine. A
separate team can build and ship it independently. It's naturally an
OPP-only feature (optional component). And edge clusters benefit for
free — remediation computed on the hub applies everywhere.

The honest trade-off: Central's co-located SQL makes the data joins
simpler. The ACS Next component has to correlate across broker feeds and
Scanner APIs. But the architectural properties — contained blast radius,
independent ownership, natural product tiering — are what enable the
feature to ship faster and evolve independently over time.

| Dimension | Current Architecture | ACS Next |
|---|---|---|
| Where does the logic live? | New service inside Central | New consumer or extension of existing component |
| Blast radius | Touches Central API, storage, UI, RBAC | Self-contained with clear inputs/outputs |
| Can a separate team own it? | Requires deep Central context | Subscribes to broker, produces CRDs |
| Product tiering | Hard — Central is all-or-nothing | Natural — optional component |
| OCP Console integration | Requires proxying through Central | CRs appear natively |

---

## Alternative Technical Paths

### Path A: ACM Addon Querying Central (Minimal Integration)

Build an ACM addon that deploys SecuredCluster and provides a Console view
by querying existing Central API.

**What You Get**:
* Multi-cluster Console view via ACM
* Faster time to value (3-6 months)
* Lower engineering investment
* Works with existing Central architecture

**What You Don't Get**:
* RBAC convergence (still separate ACS RBAC + K8s RBAC)
* K8s-native APIs (no CRDs for security data)
* Low-friction adoption (Central required for any value)
* Engineering velocity improvements (monolithic architecture remains)
* Composability (all-or-nothing deployment)

**Best For**: Organizations prioritizing near-term ACM integration over
long-term architectural transformation.

### Path B: Incremental Decoupling Within Current Architecture

Progressively extract components from Central (Scanner, Policy Engine) as
independent services while maintaining Central as the hub.

**What You Get**:
* Some engineering velocity improvement (cleaner component boundaries)
* Gradual transition (lower risk than full rewrite)
* Maintain existing customer deployments
* Some composability (deploy Scanner independently)

**What You Don't Get**:
* Full RBAC convergence (Central still required)
* K8s-native data model (APIs remain Central-centric)
* ACM as orchestrator (Central remains the hub)
* Low-friction adoption (Central dependency remains)

**Best For**: Organizations wanting architectural improvements but
unwilling to commit to ACM as the orchestration layer.

**Risks**: "Big ball of mud" problem — incremental decoupling often results
in more interfaces without cleaner boundaries. May end up with worst of
both worlds.

### Path C: ACS Next (Full Architectural Shift)

Single-cluster ACS operators with ACM providing multi-cluster
orchestration. CRD-first data model. K8s RBAC replaces ACS RBAC.

**What You Get**:
* RBAC convergence (K8s RBAC is the engine)
* K8s-native APIs (security data as CRDs)
* Low-friction adoption (feels like part of OCP, natural upgrade to OPP)
* Engineering velocity (clear ownership, independent operators)
* Composability (deploy only what you need)
* ACM leverage (reuse ACM Search, Governance, Identity)

**What You Don't Get**:
* Near-term delivery (12-18 months to parity)
* Standalone multi-cluster (ACM required)
* Continuity with current architecture (parallel tracks during transition)

**Best For**: Organizations committed to OPP portfolio integration and
willing to invest in architectural transformation.

### Comparison Matrix

| Capability | Path A (Addon) | Path B (Incremental) | Path C (ACS Next) |
|------------|----------------|----------------------|-------------------|
| RBAC convergence | No | Partial | Yes |
| K8s-native APIs | No | Partial | Yes |
| Low-friction adoption | No | No | Yes |
| Engineering velocity | No improvement | Some improvement? | Potential improvement* |
| Time to value | 3-6 months | 6-12 months | 12-18 months |
| Engineering investment | Low | Medium | High |
| Risk level | Low | Medium (scope creep) | Medium (new architecture) |
| Standalone multi-cluster | Yes (Central) | Yes (Central) | Requires ACM |

*Engineering velocity improvements are hypothesized, not validated.

### Recommendation

**Path C (ACS Next)** if PM/leadership are committed to OPP portfolio
integration and single-pane-of-glass, reducing adoption friction is a
business priority, and RBAC convergence is required (which it is, as a
downstream requirement of portfolio integration).

**Path A (ACM Addon)** if near-term OCP Console integration is the
priority, existing ACS architecture is acceptable, and investment capacity
is limited.

**Avoid Path B** unless there's a specific forcing function. Incremental
decoupling without a clear target architecture tends to create complexity
without solving the underlying problems.

---

## Costs and Risks

### Engineering Costs

| Cost | Severity | Notes |
|------|----------|-------|
| Build single-cluster ACS variant | High | New operator, decoupled components, local persistence |
| CRD data model design | Medium-High | Vulnerabilities, alerts, compliance as CRs |
| Scanner architecture decision | Medium | Local vs. shared vs. hybrid |
| Maintain two architectures | High | During 5.2 LTS transition period |
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
| Customer disruption | 5.2 LTS provides stability; ACS Next is opt-in |
| ACM dependency | Clear OCP vs OPP positioning; ACM is already required for OPP value |
| Engineering effort | Phased approach; MVP first; parallel tracks |
| Feature regression | Explicit feature mapping; gap analysis before migration |
| Microservice proliferation | Consolidate related functions; resist splitting unless clear operational benefit |

### Investment vs. Ongoing Cost

**The question is not "can we afford ACS Next?"**
**The question is "can we afford to keep paying the current architecture
tax?"**

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

### Should Decoupling Apply to ACS Legacy?

If decoupling is valuable for ACS Next, should we backport it to 5.x?

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| **A: ACS Next Only** | Decoupling applies only to new architecture | Lower risk; cleaner separation; 5.2 LTS remains stable | Two architectures diverge |
| **B: Backport to Legacy** | Extract components from Central in 5.x line | Shared components; some velocity benefit | Higher risk to LTS stability |
| **C: Selective Extraction** | Extract specific components (e.g., Scanner) that can be shared | Middle ground; some code sharing | Partial benefit |

**Recommendation: Start with Option A.** 5.2 LTS customers chose LTS for
stability, not for architectural improvements. Evaluate backporting
specific improvements (Option C) after ACS Next architecture is validated
in production (6-12 months post-launch).

---

## ACSCS Strategic Considerations

ACSCS has had limited traction relative to investment. ACS Next changes
the value proposition of a hosted Central model:

| Option | Description | Considerations |
|--------|-------------|----------------|
| **Sunset with 5.2** | End ACSCS when 5.2 LTS begins | Simplifies portfolio; frees investment for ACS Next |
| **Transform to ACM-based** | ACSCS becomes hosted ACM + ACS operators | Maintains cloud offering; different architecture |
| **Maintain parallel** | ACSCS continues alongside ACS Next | Higher investment; two architectures to maintain |

**This is a PM/business decision, not a technical one.** ACS Next enables
ACSCS sunset but does not require it. What sunsetting would enable:

* Removes architectural constraints from ACS Next design (no need to
  support hosted Central model)
* Simplifies the product portfolio
* Redirects investment toward strategic direction
* Reduces ongoing operational burden

---

## Common Questions

**"What about customers who don't have ACM?"**
Single-cluster ACS works standalone. Multi-cluster requires ACM — which is
already the OPP value proposition.

**"What's the migration path for existing customers?"**
5.2 LTS provides 5 years of support. Migration is opt-in when ACS Next
matures. Tooling provided for policy export/import.

**"What about the footprint increase?"**
Full single-cluster config is larger (~4x memory). But: composability lets
you deploy less; Central footprint moves to hub where ACM already lives.

**"How is this different from what we discussed before?"**
5.2 LTS timing is new. ACSCS sunset simplifies. RBAC convergence impasse
is explicit evidence of architectural constraint.

**"What's the minimal version that proves this works?"**
Single-cluster scanner + policy engine with ACM Search aggregating
vulnerability summaries across clusters.

**"How do disconnected customers use ACM?"**
Disconnected (air-gapped) ACM deployments exist and are supported. ACM hub
and managed clusters operate within a closed network. ACS follows the same
pattern: vulnerability DB updates loaded via mirrored content, local
scanner option for fully disconnected clusters, single-cluster ACS
operates independently with no connectivity requirements.

**"What about AI?"**
The event-driven architecture is AI-ready without AI features in scope:

* **Training.** Event streams feed ML pipelines; user actions (exceptions,
  escalations) generate labels organically.
* **Inference.** Models deploy as broker consumers — same pattern as any
  other component.
* **Tooling.** Query APIs (Vuln Management Service, broker subjects) can be
  exposed as MCP tools for AI agents investigating security posture.
* **Development.** The consumer pattern has explicit boundaries — ideal for
  AI-assisted implementation.
* **Securing AI workloads.** AI-specific detection (model exfil, training
  data access) fits the existing consumer model.

ACS Next removes architectural barriers. Whether to build AI features is a
product decision enabled by the architecture, not constrained by it.

---

## Prior Work

This rationale draws on prior analysis from:

* `scratchpad/next-gen-security-platform/unified-architecture.md` —
  Comprehensive composable architecture
* `scratchpad/next-gen-security-platform/two-tier-strategy.md` — ACS
  Advanced + ACS Essentials model
* `scratchpad/stackrox-lite-proposal/PROPOSAL.md` — Single-cluster
  refactor proposal
* `scratchpad/roadmap/STACKROX_ACS_VISION_ROADMAP.md` — Three-phase
  transformation roadmap
* `scratchpad/acm-acs-addon/design.md` — ACM addon technical design
* `scratchpad/acm-acs-addon/k8s-resources-discussion.md` — K8s API
  dependency and security analysis
* `scratchpad/all-the-crds-design/README.md` — Comprehensive CRD design
* `scratchpad/namespace-scoped-acs/CR_FIRST_JUSTIFICATION.md` — Strategic
  justification for CRD-first approach
* [RBAC Convergence Design Doc](https://docs.google.com/document/d/144jpKqZ17MtkzkJKJUty6vyHWxmlPrE9Y2aUPN0G3iI)
* [ACM AddOn Framework](https://github.com/open-cluster-management-io/addon-framework)
