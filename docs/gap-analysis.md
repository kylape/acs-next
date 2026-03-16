# ACS Next: Gap Analysis

*Status: Draft | Date: 2026-03-16*

---

## Purpose

This document identifies capabilities present in current StackRox/ACS that are
not explicitly addressed in the ACS Next architecture. This is not an argument
that every capability must be recreated — some may be intentionally removed,
replaced by portfolio alternatives, or deferred. The goal is awareness and
intentional decision-making.

This document covers two types of gaps:

1. **Feature gaps** — Capabilities in current ACS that need decisions for ACS Next
2. **Implementation detail gaps** — Specification depth needed before implementation
   (identified by comparison against detailed StackRox system specification)

**Feature Gap Structure:**

* **Covered**: Explicitly addressed in ACS Next architecture
* **Gap**: Not addressed; needs decision
* **Replaced**: Functionality replaced by portfolio or OCP-native alternative
* **Deferred**: Could be added later; not in initial scope

---

## Summary

### Feature Gaps

| Category | Capabilities Reviewed | Covered | Gaps | Replaced | Deferred |
|----------|----------------------|---------|------|----------|----------|
| Core Security | 8 | 6 | 0 | 1 | 1 |
| Vulnerability Management | 7 | 6 | 1 | 0 | 1 |
| Compliance | 6 | 1 | 0 | 5 | 0 |
| Network Security | 5 | 4 | 0 | 0 | 1 |
| Integrations | 5 | 5 | 0 | 0 | 0 |
| Administration | 9 | 7 | 0 | 2 | 0 |
| Sensor/Runtime | 6 | 6 | 0 | 0 | 0 |
| Reporting & Analytics | 4 | 4 | 0 | 0 | 0 |
| Image Management | 5 | 5 | 0 | 0 | 0 |
| VM Support | 3 | 1 | 0 | 0 | 2 |
| **Total** | **58** | **45** | **1** | **8** | **5** |

### Implementation Detail Gaps

Gaps identified by comparison against detailed StackRox system specification.
See [Implementation Detail Gaps](#implementation-detail-gaps-vs-stackrox-specification)
section for details.

| Priority | Count | Blocks Phase 0? | Blocks GA? |
|----------|-------|-----------------|------------|
| Critical | 6 | 4 yes, 2 no | All |
| High | 4 | No | All |
| **Total** | **10** | **4** | **10** |

---

## Detailed Gap Analysis

### 1. Core Security Features

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| Policy Engine | Boolean policy with field matching, lifecycle stages | **Covered** | Shared Go library embedded in CI Gateway (build), Admission Control (deploy), Runtime Evaluator (runtime) |
| Alert Management | AlertService with filtering, grouping, lifecycle | **Covered** | Violations to broker → Notifiers component; AlertManager via Prometheus metrics |
| Runtime Detection | Process, network, file access monitoring | **Covered** | Collector publishes raw events; Runtime Evaluator (separate) evaluates policies |
| Admission Control | Deploy-time policy enforcement via webhook | **Covered** | Dedicated Admission Control component |
| Risk Scoring | RiskService with multipliers, ranking | **Covered** | Risk Scorer component (optional) |
| Baselines | Process and network baselines, anomaly detection | **Covered** | Baselines component (optional) |
| MITRE ATT&CK | Policy mapping to MITRE framework | **Replaced** | Not explicit; could be metadata on policy CRDs |
| Policy Categories | Built-in and custom categorization | **Deferred** | Not explicit; could be labels on policy CRDs |

---

### 2. Vulnerability Management

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| CVE Detection | Image and node vulnerability scanning | **Covered** | Scanner component |
| SBOM Generation | Software bill of materials | **Covered** | Scanner publishes sbom-updates feed |
| Vulnerability Exceptions | Deferrals (time-based, fixable), False Positives | **Covered** | CRD with status subresource; K8s RBAC for approval workflow |
| Vulnerability Trends | Historical CVE data, trend analysis | **Covered** | Prometheus metrics + OCP dashboards; Thanos for long-term retention |
| Base Image Tracking | BaseImageService, base layer matching | **Deferred** | Still in progress in current ACS; policy engine support under review |
| Component Analysis | Package-level vulnerability attribution | **Covered** | Implicit in scanner functionality |
| Watched Images | WatchedImageService for specific images | **Gap** | Continuous registry monitoring; CI Gateway covers on-demand scanning |

**Decisions:**
* Base Image Tracking — deferred; current ACS implementation still in progress
* Watched Images — gap; needs customer usage data to prioritize. CI Gateway covers on-demand use case; continuous monitoring is the delta

---

### 3. Compliance

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| Compliance Standards | PCI-DSS 3.2, HIPAA 164, NIST 800-53, NIST 800-190 | **Replaced** | Dissolved; compliance-operator provides |
| Compliance Checks | 76+ checks across standards | **Replaced** | Compliance-operator owns checks |
| Compliance Aggregation | By cluster, namespace, deployment, node | **Replaced** | Compliance-operator + OCP Console |
| Compliance Scanning | On-demand and scheduled scans | **Replaced** | Compliance-operator scheduling |
| Compliance Reports | ComplianceRunResults, export | **Replaced** | Compliance-operator results |
| Compliance Operator Integration | Proxying compliance-operator results | **Covered** | Direct use of compliance-operator |

**Decision confirmed:** ACS Next dissolves compliance framework; compliance-operator is the answer. This is a significant scope reduction and simplification.

---

### 4. Network Security

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| Network Flow Monitoring | Connection tracking, flow enrichment | **Covered** | Collector publishes network-flows |
| Network Baselines | Learned patterns, anomaly detection | **Covered** | Baselines component |
| Network Policies | NetworkPolicyService for policy management | **Covered** | Out of scope; K8s-native tooling (kubectl, GitOps, OCP Console) sufficient |
| Network Policy Generation | Automatic NetworkPolicy recommendations | **Deferred** | Separate consumer component; subscribes to network-flows, generates recommendations; post-GA or OPP feature |
| Network Graph | Visualization of network connections | **Covered** | OCP Console extension |

**Decisions:**
* Network Policies Management — out of scope; K8s-native tooling is sufficient
* Network Policy Generation — in scope as separate optional consumer component, deferred to post-GA or OPP tier

---

### 5. Integrations

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| **Notification Integrations** | | | |
| Jira, Slack, Teams, PagerDuty | 14+ notifier types | **Covered** | External Notifiers component |
| Splunk, Sumo Logic, Syslog | SIEM integrations | **Covered** | External Notifiers component |
| AWS Security Hub, Sentinel | Cloud SIEM integrations | **Covered** | Notifier CRD with type-specific config; cloud auth via workload identity |
| **Registry Integrations** | | | |
| ECR, GCR, ACR, Quay, etc. | 11+ registry types | **Covered** | `ImageRegistry` CRD with `credentialsRef` and workload identity support |
| **Auth Providers** | | | |
| OIDC, SAML, LDAP, OCP Auth | 7+ auth types | **Replaced** | K8s RBAC + portfolio identity |
| **Backup Integrations** | | | |
| S3, GCS, Azure Blob | External backup targets | **Covered** | Vuln Management Service uses SQLite (local) or BYODB (customer-managed backups) |
| **Signature Verification** | | | |
| Cosign, Sigstore, Fulcio | Image signature verification | **Covered** | SignatureVerifier CRD; Admission Control enforces |

**Decisions:**
* AWS Security Hub / Sentinel — covered; just additional `type` values in Notifier CRD with cloud auth support
* Backup — covered; SQLite file backup or customer manages their own PostgreSQL backups

---

### 6. Administration

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| User Management | UserService, groups, roles | **Replaced** | K8s RBAC + portfolio |
| API Token Management | APITokenService, expiration tracking | **Covered** | K8s ServiceAccounts with token requests |
| Custom RBAC | SAC engine, scopes | **Replaced** | K8s RBAC |
| Feature Flags | FeatureFlagService | **Covered** | ConfigMap or environment variables (not CRD spec — avoids enshrining in API) |
| Declarative Config | GitOps via CRDs | **Covered** | Native CRD model |
| Administration Events | Audit logging for admin actions | **Covered** | K8s audit logs capture all API server activity including CR changes |
| Credential Expiry | Certificate, token expiration tracking | **Covered** | Operator status conditions + Prometheus metrics (`acs_certificate_expiry_seconds`) |
| System Info | Version, deployment info, health | **Covered** | Operator status conditions on component CRs |
| High Availability | Central HA is architecturally constrained | **Covered** | Modular components are independently scalable; no single-point-of-failure aggregator |

**Decisions:**
* API Tokens → K8s ServiceAccounts
* Feature Flags → ConfigMap/env vars (not CRD spec to avoid enshrining experimental features in API)
* Admin Events → K8s audit logs
* Credential Expiry → Operator monitors secrets, exposes status + metrics
* System Info → Operator status conditions

---

### 7. Sensor/Runtime Capabilities

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| Process Monitoring | Process indicators, lineage, enrichment | **Covered** | Collector publishes process-events |
| Network Flow Collection | Flow enrichment, pod/deployment correlation | **Covered** | Collector publishes network-flows |
| File Activity Monitoring | Sensitive file access detection | **Covered** | Collector publishes file-events |
| Audit Log Collection | K8s audit log streaming | **Covered** | Audit Logs component |
| Sensor Upgrades | Automated sensor version management | **Covered** | N/A in ACS Next; operator-managed deployments, interface is broker subjects + protobufs |
| Probe Management | eBPF probe distribution | **Covered** | N/A in ACS Next; eBPF probes ship with Collector image, kernel modules deprecated |

**Decisions:**
* Sensor Upgrades — not applicable; no "Sensor" in ACS Next, components are operator-managed
* Probe Management — not applicable; probes bundled in Collector container image

---

### 8. Reporting & Analytics

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| Scheduled Reports | ReportConfigurationService, automation | **Covered** | Vuln Management Service internal component; VulnerabilityReport CRDs |
| On-Demand Reports | ReportService, vulnerabilities, compliance | **Covered** | `roxctl` at single-cluster; Vuln Management Service at fleet level |
| Vulnerability Export | VulnMgmtService, stream export | **Covered** | Vuln Management Service export API; `roxctl` for single-cluster |
| Search/Query | SearchService, unified search | **Covered** | Single-cluster: kubectl/CRs; Fleet: Vuln Management Service API; ACM Search optional |

**Recommendation:**
* Reporting designed as Vuln Management Service internal component with CRD-based configuration
* Export via `roxctl` (single-cluster) and Vuln Management Service API (fleet)
* Search covered by ACM Search

---

### 9. Image Management (Detailed)

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| Image Scanning | Scanner V4 with indexer/matcher | **Covered** | Scanner component |
| Image Metadata Caching | TTL-based caching, reprocessing | **Covered** | JetStream caching with `MaxMsgsPerSubject: 1`, `MaxAge: 1 hour` |
| Delegated Registry Scanning | Pull-through registry support | **Covered** | Scanner topology options (Local/Split/Delegated) cover this |
| Image Re-scanning | Periodic re-scan of deployed images | **Covered** | New "Deployed Image Tracker" component; watches deployments, triggers re-scans on vuln DB updates |
| Registry Authentication | Per-registry credentials | **Covered** | `ImageRegistry` CRD `credentialsRef` points to K8s Secrets |

**Decisions:**
* Delegated Registry Scanning — covered by Scanner topology options
* Image Re-scanning — new "Deployed Image Tracker" component:
  * Watches Pods/Deployments via K8s API
  * Maintains image → deployment inventory
  * Subscribes to vuln DB update notifications
  * Publishes to `acs.scan-requests` for deployed images
  * Optional consumer component

---

### 10. Virtual Machine Support

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| VM Inventory | VirtualMachineService | **Covered** | Mentioned in architecture |
| VM Vulnerability Scanning | VM index reports | **Deferred** | Post initial release; architecture supports VM agent → broker → Scanner |
| VM Agent Integration | Fact agent for VM data | **Deferred** | Post initial release; current ACS scope is narrow (CVE list per VM) |

**Decisions:**
* VM Support — deferred from initial release
* Current ACS VM scope is narrow: CVE list per VM, CVE-centric views ("which VMs affected by this CVE?")
* Architecture supports future VM support: VM agent publishes to broker, Scanner/Vuln Management Service consumes

---

## Remaining Gaps

### Gaps Requiring Customer Data

1. **Watched Images**
   * Current: Monitor specific images outside of deployed workloads
   * CI Gateway covers on-demand scanning (`POST /v1/scan`)
   * Gap is continuous registry monitoring (re-scan on vuln DB update)
   * **Decision:** Needs customer usage data to prioritize

### Deferred (Post-GA or OPP)

2. **Network Policy Generation**
   * Automatic NetworkPolicy recommendations based on observed traffic
   * Separate consumer component subscribing to `network-flows`
   * **Decision:** In scope as optional component, post-GA or OPP tier

3. **Base Image Tracking**
   * Track base layers, attribute vulnerabilities to base vs app
   * Still in progress in current ACS (policy engine support under review)
   * **Decision:** Deferred until current ACS implementation matures

4. **VM Support**
   * VM vulnerability scanning and agent integration
   * Architecture supports: VM agent → broker → Scanner
   * Current ACS scope is narrow (CVE list per VM)
   * **Decision:** Deferred from initial release

---

## Explicitly Removed Capabilities (Intentional)

These capabilities are intentionally not in ACS Next:

| Capability | Replacement | Rationale |
|------------|-------------|-----------|
| Compliance Framework | compliance-operator | Avoid duplication; OCP-native |
| Custom RBAC (SAC) | K8s RBAC | Portfolio integration goal |
| Central Aggregation | NATS leaf nodes + Vuln Management Service | Direct streaming; ACM optional for fleet orchestration |
| Custom Auth Providers | K8s + portfolio identity | Simplification |
| Central-Sensor sync protocol | Not needed | Single-cluster model; no network partition to handle |

**Central-Sensor sync scope:** The current sync machinery spans 90+ files and includes bidirectional gRPC streaming (50+ message types from Sensor, 30+ from Central), dual-sided hash deduplication, chunked deduper state transfer on reconnect, 7-layer stream wrappers, strict sync sequencing, and reconciliation logic. This complexity exists to handle network partitions between Central and Sensor. In ACS Next, components are co-located—this machinery becomes unnecessary.

---

## Feature Mapping to Components

| ACS Next Component | Current ACS Equivalent | Coverage |
|-------------------|------------------------|----------|
| Collector | Collector + ProcessSignal + NetworkFlow | Good; stays simple (eBPF only) |
| Runtime Evaluator | Sensor policy evaluation | New; subscribes to Collector, enriches with K8s context |
| CI Gateway | Central image check API | New; external REST API for CI/CD, embeds policy engine |
| Admission Control | AdmissionControl + Sensor detection | Good; embeds policy engine |
| Scanner | Scanner V4 | Good; `ImageRegistry` CRD for registry auth |
| Deployed Image Tracker | Central re-scan scheduling | New; watches deployments, triggers re-scans on vuln DB updates |
| Broker | Central event handling | New pattern; NATS leaf nodes for multi-cluster |
| CRD Projector | N/A (new) | New; projects violations/scans to CRs |
| Vuln Management Service (hub) | Central PostgreSQL | Fleet-level queries; SQLite or BYODB |
| External Notifiers | Notifiers package | Good; Notifier CRD with type-specific config |
| Risk Scorer | Risk service | Good |
| Baselines | ProcessBaseline + NetworkBaseline | Good |
| Network Policy Generator | NetworkPolicyService | Deferred; separate consumer, post-GA or OPP |

---

## Implementation Detail Gaps (vs. StackRox Specification)

The following gaps were identified by comparing ACS Next architecture documents
against a detailed StackRox system specification. These are implementation
details that need specification before Phase 0 can fully validate the
architecture.

### Critical (Pre-Implementation)

#### 1. Policy Expression Language Specification

**Current ACS:** Boolean expression structure with PolicySection → PolicyGroup →
PolicyValue hierarchy. Sections are OR'd, groups within sections are AND'd.
Field metadata registry (966+ lines) maps field names to object paths and
matcher types. Linked match filtering ensures matches from repeated sub-objects
(e.g., containers) come from the same sub-object.

**ACS Next gap:** `StackroxPolicy` CRD mentioned but schema not specified:

* How do `policy_sections`, `policy_groups`, `policy_values` map to CRD structure?
* Field registry — which fields are supported, what are their types?
* Linked match filtering — how does this work with CRD-based policies?
* Augmented objects — how are computed fields (imageAge, permissionLevel) handled?

**Risk:** Policy compatibility issues during migration. Complex semantics can't
be simplified without breaking existing policies.

**Recommendation:** Add policy CRD schema specification. Document field registry.
Provide policy migration guide.

---

#### 2. Deployment Data Model and Enrichment

**Current ACS:** Sophisticated deployment model with:

* Hash-based deduplication at 4 layers (Resolver, Detector, Deduper, Central)
* Service account permission level computation (NONE → DEFAULT → ELEVATED → CLUSTER_ADMIN)
* Network policy resolution and service correlation
* RBAC enrichment (ServiceAccount → RoleBindings → Roles → PolicyRules)

**ACS Next gap:** "Runtime Evaluator has full deployment context" stated but not
specified:

* How is deployment state maintained without Central's in-memory store?
* Is 4-layer deduplication needed, or does NATS pub/sub obviate it?
* How does RBAC enrichment work — live K8s API queries or cached?
* What's the latency impact of on-demand enrichment?

**Risk:** Missing enrichment breaks policies using `Minimum RBAC Permissions`,
`Service Account`, or network policy fields.

**Recommendation:** Document Runtime Evaluator's deployment tracking. Specify
enrichment scope and caching strategy.

---

#### 3. Network Flow Pipeline Specification

**Current ACS:** Sophisticated network flow handling:

* ConnTracker state machine (INITIATED → ESTABLISHED → CLOSED)
* Afterglow suppression (30s window to suppress transient connections)
* Delta computation (send only new/closed connections)
* External IP classification (private IPs → DEPLOYMENT, public → EXTERNAL_SOURCE)
* Rate limiting (1000 connections/container/interval)

**ACS Next gap:** `network-flows` subject mentioned but not specified:

* Message format (connection tuples, timestamps, direction, protocol)
* Where does afterglow and delta computation happen? (Collector or Baselines?)
* How is external IP classification performed?
* Network baseline storage and locking semantics

**Risk:** Missing afterglow could 10x+ event volume. Missing delta computation
causes duplicate reporting.

**Recommendation:** Document Collector's network flow publishing semantics.
Specify Baselines component's connection tracking state.

---

#### 4. Process Indicator Handling and Filtering

**Current ACS:** Critical process handling:

* Stable ID generation (UUID v5 from pod_id, container_name, exec_file_path, name, args)
* Similarity-based filter dropping 40-50% of redundant processes
* Rate limiting (100 signals/sec per container)
* Process lineage tracking (parent process chain)

**ACS Next gap:** `process-events` subject mentioned but not specified:

* Message schema — does it include lineage?
* Where does similarity filtering run — Collector or Runtime Evaluator?
* Rate limiting thresholds and overflow behavior
* Stable ID generation — preserved or dropped?

**Risk:** Without similarity filtering, process volume could be 2x expected.
Without stable IDs, baseline correlation breaks.

**Recommendation:** Document process event schema with lineage. Specify
similarity filter location and configuration.

---

#### 5. Alert Lifecycle and Deduplication

**Current ACS:** Alert state machine:

* States: ACTIVE (enforcement applied), ATTEMPTED (inform-only), RESOLVED
* Deduplication key: `(policy_id, entity_id, lifecycle_stage)`
* New violation for existing key → append to existing alert
* Enforcement count tracking, first/last occurrence timestamps

**ACS Next gap:** `PolicyViolation` CRs exist but lifecycle not specified:

* How does a violation become resolved? (Pod deleted? Policy fixed? Manual?)
* Deduplication — same violation updates existing CR or creates new?
* How is enforcement count tracked?
* First/last seen timestamps in CR?

**Risk:** CR proliferation without deduplication. Missing resolution detection
leaves stale violations.

**Recommendation:** Document PolicyViolation lifecycle. Specify CRD Projector's
deduplication and resolution detection logic.

---

#### 6. Image Scan State Tracking

**Current ACS:** Image scan states:

* Unscanned → Scan in progress → Scanned
* `MISSING_SCAN_DATA` note for scan failures
* Multi-layer enrichment cache (in-memory 4hr TTL, PostgreSQL, Scanner manifest)
* ScanStats computed and cached (CVE counts by severity)

**ACS Next gap:** JetStream caching mentioned but scan lifecycle not specified:

* How is "scan in progress" tracked to prevent duplicate scans?
* How are scan failures recorded?
* Cache invalidation on vulnerability database updates?
* ScanStats computation — where does it happen?

**Risk:** Missing state tracking causes duplicate scans or stale data.

**Recommendation:** Document scan state machine. Specify ImageScanSummary
update semantics on re-scan.

---

### High Priority (Pre-GA)

#### 7. Enforcement Actions Beyond Admission

**Current ACS:** Runtime enforcement actions:

* `SCALE_TO_ZERO` — set replicas to 0, save original in annotation
* `KILL_POD` — delete pods matching deployment's pod labels
* Break-glass bypass via `admission.stackrox.io/break-glass` annotation

**ACS Next gap:** Enforcement execution not specified:

* Which component executes SCALE_TO_ZERO and KILL_POD?
* How is enforcement audit trail maintained?
* Break-glass annotation support?

**Risk:** Runtime enforcement won't work without explicit implementation.
Production operations may be blocked without bypass.

**Recommendation:** Document enforcement executor (likely Runtime Evaluator).
Specify break-glass support.

---

#### 8. Risk Scoring Algorithm

**Current ACS:** Multiplicative risk model with 8 multipliers:

1. Policy Violations (max 4.0)
2. Process Baseline Violations (max 4.0)
3. Image Vulnerabilities (max 4.0)
4. Service Configuration (max 2.0)
5. Network Reachability (max 2.0)
6. Risky Component Count (max 1.5)
7. Component Count (max 1.5)
8. Image Age (max 1.5)

Uses normalization function with saturation thresholds.

**ACS Next gap:** Risk Scorer component mentioned but algorithm not specified:

* Are all 8 multipliers preserved?
* Same multiplicative model or simplified?
* Normalization parameters?

**Risk:** Risk scores differ between ACS and ACS Next, confusing users.

**Recommendation:** Document risk scoring algorithm or explicitly note changes.

---

#### 9. Collector eBPF Specifics

**Current ACS:** Collector details:

* Monitored syscalls: `sys_execve`, `sys_connect`, `sys_accept4`, `sys_close`
* BPF ring buffer delivery to userspace
* `/proc/net/tcp` fallback every 30s for missed connections
* Rate limiting configuration via environment variables

**ACS Next gap:** "Collector stays simple (eBPF only)" but specifics not confirmed:

* Same syscall coverage?
* Fallback mechanism preserved?
* Rate limiting thresholds?

**Risk:** Missing fallback could miss connections. Missing rate limiting could
overload broker.

**Recommendation:** Reference existing Collector spec or document expected
behavior.

---

#### 10. Failure Mode Quantification

**Current ACS:** Specific failure handling:

* Bounded queues: 10,000 process indicators, 10,000 network flows, 1,000 file events
* Exponential backoff: 5s initial, 5min max, infinite retries
* Offline mode: continue monitoring, cache policies, buffer events

**ACS Next gap:** Broker recovery documented qualitatively but not quantified:

* Concrete queue sizes for each stream?
* Backpressure behavior — drop oldest or block publishers?
* What's "acceptable to lose" in terms of time windows?

**Risk:** Unclear failure behavior makes operations difficult.

**Recommendation:** Add concrete numbers to broker.md. Specify backpressure
policy per stream.

---

### Implementation Gap Summary

| Gap | Category | Blocks Phase 0? | Blocks GA? |
|-----|----------|-----------------|------------|
| Policy expression language | Critical | Yes | Yes |
| Deployment enrichment | Critical | Yes | Yes |
| Network flow pipeline | Critical | Yes | Yes |
| Process indicator handling | Critical | Yes | Yes |
| Alert lifecycle | Critical | No | Yes |
| Image scan state | Critical | No | Yes |
| Enforcement actions | High | No | Yes |
| Risk scoring algorithm | High | No | Yes |
| Collector eBPF specifics | High | No | Yes |
| Failure mode quantification | High | No | Yes |

**Phase 0 blocking gaps** require specification before the prototype can
validate the architecture — without them, we can't confirm policy compatibility
or event pipeline behavior.

**GA blocking gaps** can be deferred to later phases but must be resolved before
production deployment.

---

## Next Steps

### Implementation Detail Gaps (Phase 0 Blocking)

1. **Policy CRD schema specification** — Document `StackroxPolicy` structure
   including sections/groups/values, field registry, linked match filtering
2. **Runtime Evaluator deployment tracking** — Specify how deployment context
   is maintained, RBAC enrichment, caching strategy
3. **Event schema definitions** — Protobuf schemas for `process-events`,
   `network-flows`, `runtime-events` including lineage and connection state
4. **Process/network pipeline semantics** — Document similarity filtering
   location, afterglow suppression, delta computation, rate limiting thresholds

### Implementation Detail Gaps (GA Blocking)

5. **PolicyViolation lifecycle** — Document state transitions, deduplication
   logic, resolution detection in CRD Projector
6. **Image scan state machine** — Specify scan-in-progress tracking, failure
   handling, cache invalidation
7. **Enforcement executor design** — SCALE_TO_ZERO/KILL_POD implementation,
   break-glass bypass support
8. **Risk scoring algorithm** — Document multipliers and normalization or note
   intentional changes
9. **Failure mode quantification** — Concrete queue sizes, backpressure
   policies, acceptable data loss windows

### Feature Gaps

10. **Scope document** clearly stating what's in v1 vs deferred
11. **Deployed Image Tracker design** — new component for re-scanning deployed images on vuln DB updates
12. **Network Policy Generator design** — separate consumer component (post-GA or OPP)
13. **Watched Images usage analysis** — gather customer data to prioritize continuous registry monitoring

**Gaps closed in this review:**
* Registry integrations — `ImageRegistry` CRD with credentials support
* Image caching — JetStream-based result caching
* Registry authentication — CRD `credentialsRef` to K8s Secrets
* Delegated registry scanning — Scanner topology options
* Image re-scanning — Deployed Image Tracker component
* AWS Security Hub / Sentinel — Notifier CRD with cloud auth
* API tokens — K8s ServiceAccounts
* Feature flags — ConfigMap/env vars
* Admin events — K8s audit logs
* Credential expiry — Operator status + Prometheus metrics
* System info — Operator status conditions
* Sensor upgrades — N/A (operator-managed)
* Probe management — N/A (bundled in image)

---

## Appendix: Current ACS Service Inventory

For reference, here are all identified services in current ACS:

**Central Services (60+):**
* AlertService, APITokenService, AuthProviderService, AuthService
* BaseImageService, ClusterCVEService, ClusterInitService, ClusterService
* ComplianceManagementService, ComplianceProfileService, ComplianceResultsService
* ComplianceRuleService, ComplianceScanConfigurationService, ComplianceService
* ConfigService, CredentialExpiryService, CVEService, DBService
* DeclarativeConfigHealthService, DeclarativeConfigService
* DelegatedRegistryConfigService, DeploymentService, DiscoveredClustersService
* ExternalBackupService, FeatureFlagService, GroupService, ImageIntegrationService
* ImageService, MetadataService, MitreAttackService, NamespaceService
* NetworkBaselineService, NetworkGraphService, NetworkPolicyService
* NodeCVEService, NodeService, NotifierService, PodService
* PolicyCategoryService, PolicyService, ProcessBaselineResultsService
* ProcessBaselineService, ProcessService, ProcessListeningOnPortService
* RankingService, ReportConfigurationService, ReportService
* ResourceCollectionService, RiskService, RoleService
* RbacService, SearchService, SecretService, SensorService
* SensorUpgradeConfigService, SensorUpgradeService, ServiceAccountService
* ServiceIdentityService, SignatureIntegrationService, SystemInfoService
* TelemetryService, UserService, VirtualMachineService
* VulnerabilityExceptionService, VulnMgmtService, WatchedImageService

**Notifier Types (14):**
* Jira, Email, ACSCS Email, CSCC (GCP), Slack, Teams
* Splunk, PagerDuty, SumoLogic, AWS Security Hub
* Syslog, Microsoft Sentinel, Generic Webhook, Network Policy

**Registry Types (11):**
* Docker Registry, ECR, Artifact Registry (GCP), ACR (Azure)
* Quay, IBM Container Registry, Artifactory, Nexus
* GitHub Container Registry, RHEL Registry, Generic

**Auth Providers (7):**
* Basic, OIDC, SAML, User PKI, OpenShift Auth, Google IAP, Token

---

*This gap analysis is based on codebase exploration as of 2026-02-27,
updated 2026-03-16 to reflect architecture changes and gap review decisions,
and further updated 2026-03-16 with implementation detail gaps identified by
comparison against detailed StackRox system specification. Key additions:
implementation detail gaps (policy schema, event pipelines, alert lifecycle),
Deployed Image Tracker component, Network Policy Generator (deferred),
K8s-native alternatives for administration features. It should be updated as
ACS Next design evolves.*
