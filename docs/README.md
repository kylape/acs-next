# ACS Next Design Documents

## Strategic Documents

* **[Business Case Brief](brief.md)** — 15-minute read. The business case
  for ACS Next: portfolio alignment, adoption friction, product
  opportunities, and what happens if we don't invest. **Start here.**

* **[Rationale](rationale.md)** — The evidence behind the brief: why the
  current architecture constrains us, what alternatives we considered,
  costs/risks, and common questions.

* **[Gap Analysis](gap-analysis.md)** — What's missing: feature-by-feature
  comparison with current ACS, identifying gaps and decisions needed.

## Architecture

* **[Architecture Overview](architecture/)** — How ACS Next works: component
  design, event-driven broker, data flows, and key design decisions.

Detailed documentation:

| Document | Content |
|----------|---------|
| [Scanner](architecture/components/scanner.md) | Indexer/matcher architecture, deployment topologies |
| [Broker](architecture/components/broker.md) | Embedded NATS, JetStream, feeds, recovery |
| [Policy Engine](architecture/components/policy-engine.md) | Engine options, signature verification |
| [Consumers](architecture/components/consumers.md) | CRD Projector, Alerting, Notifiers, Risk Scorer |
| [Data Architecture](architecture/data-architecture.md) | Persistence strategy, CRD scaling, Prometheus |
| [Multi-Cluster](architecture/multi-cluster.md) | Vuln Management Service, Fleet RBAC, Reporting |
| [Deployment](architecture/deployment.md) | Profiles, topologies, operator |
| [CRDs](architecture/crds.md) | Full CRD reference and inventory |
