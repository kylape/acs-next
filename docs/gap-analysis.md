# ACS Next: Gap Analysis

*Status: Draft | Date: 2026-03-16*

---

## Purpose

This document identifies capabilities present in current StackRox/ACS that are not explicitly addressed in the ACS Next architecture. This is not an argument that every capability must be recreated—some may be intentionally removed, replaced by portfolio alternatives, or deferred. The goal is awareness and intentional decision-making.

**Structure:**
* **Covered**: Explicitly addressed in ACS Next architecture
* **Gap**: Not addressed; needs decision
* **Replaced**: Functionality replaced by portfolio or OCP-native alternative
* **Deferred**: Could be added later; not in initial scope

---

## Summary

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

## Next Steps

1. **Scope document** clearly stating what's in v1 vs deferred
2. **Deployed Image Tracker design** — new component for re-scanning deployed images on vuln DB updates
3. **Network Policy Generator design** — separate consumer component (post-GA or OPP)
4. **Watched Images usage analysis** — gather customer data to prioritize continuous registry monitoring
5. **Broker retention and recovery design** — Define retention windows per stream,
   PVC sizing, consumer recovery behavior, and acceptable data loss windows

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
updated 2026-03-16 to reflect architecture changes and gap review decisions.
Key additions: Deployed Image Tracker component, Network Policy Generator
(deferred), K8s-native alternatives for administration features. It should
be updated as ACS Next design evolves.*
