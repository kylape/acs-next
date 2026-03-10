# ACS Next: The Business Case

*Status: Draft | Date: 2026-03-10*

*For the full strategic and technical analysis, see
[proposal.md](proposal.md).*

---

## The Opportunity

ACS has an adoption problem. It's a powerful product, but it requires a
separate evaluation, separate deployment, separate UI and RBAC model,
and separate budget justification. Every one of those "separates" is
friction that limits how many OCP customers become ACS customers.

ACS Next eliminates this friction. Security becomes a built-in platform
capability — something customers *already have* with OCP, not something
they need to go buy. Fleet-level capabilities become a reason to
subscribe to OPP. And resource-constrained clusters that can't run ACS
today get security for the first time.

The vast majority of ACS revenue already comes from OCP/OPP customers.
ACS Next aligns the architecture with the revenue base.

---

## What Changes for the Customer

**Today:**

1. Customer has OCP
2. "Want security? Evaluate this separate product called ACS"
3. Deploy Central, Sensor, Scanner — learn new UI, new RBAC, new concepts
4. Feels like a separate product → separate evaluation → friction

**ACS Next:**

1. Customer has OCP
2. Security data is already visible in OCP Console
3. Familiar tools (kubectl, GitOps), familiar RBAC (K8s RBAC)
4. "Want fleet visibility, reporting, and advanced policy? That's OPP"
5. Feels like "more of what I have" → natural upgrade → no separate eval

The sales motion shifts from "evaluate this security product" to "you're
already seeing security data — want that across your fleet?"

---

## Product Opportunities This Opens

The current architecture makes certain product decisions for us. ACS
Next returns them to PM.

| Product Question | Today | ACS Next |
|---|---|---|
| Can we offer a freemium tier? | No — Central is all-or-nothing | **Yes** — basic security with OCP, advanced with OPP |
| Can security feel like part of OCP? | Difficult — Central is a separate product | **Yes** — CRDs, OCP Console, K8s RBAC |
| Can we serve edge clusters? | No — secured cluster stack is too heavy | **Yes** — Collector + lightweight broker, hub does the rest |
| Can we have per-deployment feature sets? | No — Central is monolithic | **Yes** — optional components |
| Can advanced search be OPP-only? | No — search is baked into Central | **Yes** — Vuln Management Service is optional |
| Can we align with K8s RBAC? | Difficult — custom SAC engine | **Yes** — K8s RBAC is native |

### Three positioning plays

**1. Security as OCP table stakes.** Basic vulnerability visibility and
admission control ship with OCP. Every OCP customer gets security.
This is the strongest stickiness play — security becomes a reason to
stay on OCP.

**2. OPP security tier.** Fleet-wide vulnerability management, scheduled
reporting, advanced runtime detection, and exception management become
OPP value. Security becomes a top reason to upgrade from OCP to OPP.

**3. Edge and resource-constrained clusters — a new segment.** ACS
cannot serve these clusters today. Even the secured cluster
components — Sensor, Collector, Admission Controller, Scanner
indexer — are too heavy. ACS Next changes this: a Collector and
lightweight broker on the edge cluster stream runtime events to a hub
where Scanner, policy evaluation, and alerting run. The edge footprint
drops to ~150-200MB. Customers with hundreds of edge locations get
fleet-wide security visibility for the first time.

---

## What It Costs

| | One-Time Cost | Ongoing Cost per Quarter |
|---|---|---|
| **Stay on current architecture** | $0 | High: coordination overhead, slow features, RBAC convergence impasse, duplicating ACM capabilities |
| **ACS Next** | High: 12-18 month development | Lower: clear ownership, independent shipping, leverage ACM |

The 5.2 LTS release provides the transition window. Existing customers
stay on 5.2 (5 years of support). ACS Next targets 5.3+ or 6.0. No
customer disruption.

*See [proposal.md, Part IV](proposal.md#part-iv-costs-and-risks) for
detailed cost and risk analysis.*

---

## What Happens If We Don't

**For the business:**

* ACS remains misaligned with its revenue base — positioned as a
  standalone product while the vast majority of revenue comes from
  OCP/OPP customers
* No path to low-friction adoption (separate evaluation, separate budget)
* Edge clusters remain unserved — a growing segment with no ACS story
* Competitive disadvantage against K8s-native security tools

**For the roadmap:** These capabilities should come off near-term plans
because each is constrained by the current architecture's assumptions:

* RBAC convergence
* Policy placement via ACM Governance
* Deep ACM integration / single-pane-of-glass
* Configurable risk scoring (beyond incremental improvements)

This isn't punitive — it's practical. Continuing to iterate on these
within the current architecture's constraints is unlikely to produce a
breakthrough.

---

## How It Works (High Level)

ACS Next shifts from Central-as-hub to **each cluster owns its own
security**, with the **OPP portfolio providing fleet orchestration**.

```
Today:                              ACS Next:

Central (hub)                       OPP Portfolio (ACM)
  ├── Sensor (cluster A)              ├── Cluster A: ACS components
  ├── Sensor (cluster B)              ├── Cluster B: ACS components
  └── Sensor (cluster C)              └── Cluster C: ACS components

Central owns everything.            Each cluster is self-contained.
Custom RBAC, custom UI, custom API.  K8s RBAC, OCP Console, K8s API.
                                     ACM provides fleet aggregation.
```

**Per cluster:** Collector (eBPF) + Scanner + Admission Controller +
event broker. Security data stored as CRDs — standard K8s resources
that OCP Console displays and K8s RBAC controls.

**Fleet level (OPP):** Vuln Management Service on the ACM hub for
fleet-wide queries, scheduled reporting, and exception management.
ACM Search aggregates security CRs. ACM Governance distributes
security policies.

**What gets removed:** Central's custom RBAC engine, custom UI,
Central-Sensor sync protocol, custom auth providers, custom
multi-cluster aggregation. Replaced by K8s RBAC, OCP Console, ACM.
Estimated 25-30% codebase reduction.

*See [architecture.md](architecture.md) for detailed component design.*

---

## The Alternative Paths

| Capability | Path A: ACM Addon on Central | Path B: Incremental Decoupling | Path C: ACS Next |
|---|---|---|---|
| Time to value | 3-6 months | 6-12 months | 12-18 months |
| Portfolio integration | Partial | No | **Yes** |
| RBAC convergence | No | Partial | **Yes** |
| Low-friction adoption | No | No | **Yes** |
| Edge cluster support | No | No | **Yes** |
| OCP/OPP stickiness | Minimal | Minimal | **Yes** |
| New product tiers | No | No | **Yes** |

**Path A** gives near-term Console integration but doesn't solve
adoption, RBAC, or product tiering.

**Path B** improves engineering velocity but doesn't change the product
model. Risk of "big ball of mud" — more interfaces without cleaner
boundaries.

**Path C (ACS Next)** is the only path that enables the product
opportunities described above.

*See [proposal.md, Part V](proposal.md#the-alternative-paths-a-detailed-comparison)
for detailed comparison.*

---

## Recommended Next Step

**Phase 0: Validation prototype (1-2 months).** Minimal single-cluster
ACS demonstrating the core architecture:

* Event broker + CRD Projector + basic policy violation flow
* ACM Search indexing security CRs from a managed cluster
* OCP Console displaying violations

This validates the architecture before committing to full
implementation. If the prototype reveals fundamental issues, we've
invested months, not years.

---

*For the full strategic analysis, see [proposal.md](proposal.md).
For detailed technical architecture, see
[architecture.md](architecture.md).
For feature gap analysis, see [gap-analysis.md](gap-analysis.md).*
