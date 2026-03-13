# **ACS Next Strategy Brief**

## **Opportunity**

ACS is a powerful security platform, but adoption has been a persistent challenge. The technology itself is strong — customers who deploy it see clear value. The problem is getting them there. As a separate product, ACS introduces its own RBAC, authentication, configuration paradigm, and notification framework — effectively asking customers to stand up and manage a parallel platform alongside OCP. The result is a significant gap between current adoption and the platform's potential.

ACS Next significantly reduces that friction by re-architecting ACS as lightweight, modular components for OCP, enabling incremental adoption that favors breadth over depth. Rather than asking customers to adopt an entire separate stack, targeted ACS capabilities can be placed in front of the majority of customers — integrated into the management surfaces and operational patterns they already use. And because the architecture is modular, PMs can explore diverse upsell paths with minimal engineering effort, experimenting with different combinations of capabilities to find what drives the most organic growth.

Note that ACS Next is *not* a play to compete broadly in the CNAPP market. It is a portfolio alignment play — deepening ACS's value for the OCP and OPP customers who already represent the majority of ACS revenue.

ACS can be one of two things: a platform that enables strategic experimentation and portfolio-led growth, or a mature enterprise product that executes reliably on known customer needs. Both are legitimate. ACS Next is a bet on the former. The LTS window is the moment to make that choice consciously, because without a conscious choice, we default to incremental maintenance over strategic growth.

## **What Changes for the Customer**

With ACS Next, security capabilities surface as a natural extension of OCP rather than a separate product to deploy and learn. Customers interact with familiar tools, familiar RBAC, and the OCP Console — there's no separate platform to stand up before they start seeing value. The exact shape of that initial experience is a product decision, but the architecture makes it possible for customers to encounter ACS value before they've made any conscious adoption step. The sales motion at renewal shifts from "you're paying for this product you haven't deployed" to "you're already using this — here's how to get more."

## **Product Opportunities This Opens**

The current architecture makes certain product decisions for us. ACS Next returns them to PM.

| Product Question | Today | ACS Next |
| :---- | :---- | :---- |
| Can we offer a freemium tier? | **No** — Central is all-or-nothing | **Yes** — e.g., basic security with OCP, advanced with OPP |
| Can security feel like part of OCP? | **Difficult** — Central is a separate product | **Yes** — CRDs, OCP Console, K8s RBAC |
| Can we serve edge clusters? | **No** — secured cluster stack is too heavy | **Yes** — Collector streams to a remote broker, no local infrastructure needed |
| Can we differentiate tiers by capability? | **Difficult** — Central is monolithic, so capabilities can't be separated from the product | **Yes** — independent components map naturally to product tiers |
| Can we align with K8s RBAC? | **Difficult** — custom SAC engine | **Yes** — K8s RBAC is native |

### **Three positioning plays**

**1\. Security as OCP table stakes.** Basic vulnerability visibility and admission control ship with OCP. Every OCP customer gets security. This is the strongest stickiness play — security becomes a reason to stay on OCP.

**2\. OPP security tier.** Fleet-wide vulnerability management, scheduled reporting, advanced runtime detection, and exception management become OPP value. Security becomes a top reason to upgrade from OCP to OPP.

**3\. Edge and resource-constrained clusters — a new segment.** ACS cannot serve these clusters today. Even the secured cluster components — Sensor, Collector, Admission Controller, Scanner indexer — are too heavy. ACS Next's modular architecture is flexible enough to offer a runtime-only security footprint on edge clusters, streaming events to a hub for policy evaluation and alerting. Customers with hundreds of edge locations get fleet-wide security visibility for the first time.

## **The Investment**

ACS Next is a significant investment — 12 to 18 months of dedicated development effort to re-architect a product that customers depend on today. That cost is real and should not be understated.

But the cost of staying on the current architecture is also significant, even if it's less visible. Each quarter spent on the current system carries coordination overhead, slow feature delivery, costly RBAC convergence workarounds, and continued duplication of capabilities that ACM already provides. These costs compound — and the current architecture has reached the point where incremental improvement can no longer address them.

The investment requires careful resource planning — engineers will need to transition gradually from the current product, and feature development on the existing platform must continue at a pace that doesn't erode customer confidence. The 5.2 LTS release provides a natural transition window for this: existing customers stay on 5.2 with five years of support while ACS Next targets 5.3+ or 6.0.

## **The Cost of Not Investing**

The following capabilities are on or near the roadmap, but each is fundamentally constrained by the current architecture's assumptions. These aren't abstract technical concerns — they're the reason certain high-priority roadmap items consistently take longer than expected or require compromises that satisfy no one.

* RBAC convergence without complex workarounds  
* Policy placement via ACM Governance  
* Deep ACM integration and single-pane-of-glass management  
* High availability

Each of these can be pursued on the current architecture, but will require significant complexity and effort to deliver individually. With ACS Next, they emerge naturally from the architecture itself — they aren't separate features to build, they're inherent properties of the new system.

## **How It Works (High Level)**

![][image1]

ACS Next shifts from a centralized hub to components that run natively on each cluster, with ACM providing fleet-level orchestration. The modular design allows for different topologies depending on customer environment and product tier — what follows is one potential configuration.

**Per cluster:** Lightweight components — Collector (eBPF), Scanner, Admission Controller, and an event broker — run on each cluster. Security data is stored as CRDs, making it accessible through standard Kubernetes tooling: OCP Console displays it, K8s RBAC controls access to it, and kubectl works as expected.

**Fleet level (OPP):** ACM provides the multi-cluster layer. ACM Governance distributes security policies. A Vulnerability Management Service on the hub enables fleet-wide queries, scheduled reporting, and exception management.

**What this replaces:** Central's custom RBAC engine, custom UI, Central-Sensor sync protocol, custom auth providers, and custom multi-cluster aggregation — all replaced by platform-native equivalents in K8s and ACM. This represents an estimated 25–30% maintenance cost reduction.

## **Recommended Next Step**

Before committing to full implementation, we recommend a short validation phase to test the core architectural assumptions.

**Phase 0: Validation prototype (1–2 months).** A minimal single-cluster deployment demonstrating the event broker, CRD Projector, and a basic policy violation flow — enough to validate that the architecture works before investing further.
