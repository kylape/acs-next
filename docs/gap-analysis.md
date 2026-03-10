# ACS Next: Gap Analysis

*Status: Draft | Date: 2026-02-27*

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
| Vulnerability Management | 7 | 5 | 2 | 0 | 0 |
| Compliance | 6 | 1 | 0 | 5 | 0 |
| Network Security | 5 | 3 | 2 | 0 | 0 |
| Integrations | 5 | 3 | 2 | 0 | 0 |
| Administration | 8 | 2 | 4 | 2 | 0 |
| Sensor/Runtime | 6 | 4 | 2 | 0 | 0 |
| Reporting & Analytics | 4 | 1 | 3 | 0 | 0 |
| **Total** | **49** | **25** | **15** | **8** | **1** |

---

## Detailed Gap Analysis

### 1. Core Security Features

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| Policy Engine | Boolean policy with field matching, lifecycle stages | **Covered** | Embedded in Collector, Admission Control, Scanner |
| Alert Management | AlertService with filtering, grouping, lifecycle | **Covered** | Alerting Service component |
| Runtime Detection | Process, network, file access monitoring | **Covered** | Collector with embedded policy engine |
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
| Vulnerability Trends | Historical CVE data, trend analysis | **Gap** | Requires Persistence Service; not explicit |
| Base Image Tracking | BaseImageService, base layer matching | **Gap** | Not mentioned in architecture |
| Component Analysis | Package-level vulnerability attribution | **Covered** | Implicit in scanner functionality |
| Watched Images | WatchedImageService for specific images | **Gap** | Not mentioned |

**Recommendation:**
* Vulnerability Exceptions are customer-critical; need CRD-based workflow or API
* Base Image Tracking could be scanner metadata
* Watched Images may be handled via policy targeting

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
| Network Policies | NetworkPolicyService for policy management | **Gap** | Not explicit; may not be in scope |
| Network Policy Generation | Automatic NetworkPolicy recommendations | **Gap** | Not mentioned |
| Network Graph | Visualization of network connections | **Covered** | Could be OCP Console extension |

**Recommendation:**
* Network Policy Generation is valuable but complex; could be post-GA
* Network Policies management may be out of scope (K8s-native already)

---

### 5. Integrations

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| **Notification Integrations** | | | |
| Jira, Slack, Teams, PagerDuty | 14+ notifier types | **Covered** | External Notifiers component |
| Splunk, Sumo Logic, Syslog | SIEM integrations | **Covered** | External Notifiers component |
| AWS Security Hub, Sentinel | Cloud SIEM integrations | **Gap** | Need to ensure parity |
| **Registry Integrations** | | | |
| ECR, GCR, ACR, Quay, etc. | 11+ registry types | **Gap** | Scanner needs registry access |
| **Auth Providers** | | | |
| OIDC, SAML, LDAP, OCP Auth | 7+ auth types | **Replaced** | K8s RBAC + portfolio identity |
| **Backup Integrations** | | | |
| S3, GCS, Azure Blob | External backup targets | **Gap** | PostgreSQL backup if Persistence Service |
| **Signature Verification** | | | |
| Cosign, Sigstore, Fulcio | Image signature verification | **Covered** | SignatureVerifier CRD; Admission Control enforces |

**Recommendation:**
* Registry integrations are critical for Scanner; need to preserve
* Auth provider diversity replaced by K8s + portfolio; acceptable
* Signature verification increasingly important; should be in Scanner

---

### 6. Administration

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| User Management | UserService, groups, roles | **Replaced** | K8s RBAC + portfolio |
| API Token Management | APITokenService, expiration tracking | **Gap** | Service accounts instead? |
| Custom RBAC | SAC engine, scopes | **Replaced** | K8s RBAC |
| Feature Flags | FeatureFlagService | **Gap** | Operator CR flags? |
| Declarative Config | GitOps via CRDs | **Covered** | Native CRD model |
| Administration Events | Audit logging for admin actions | **Gap** | K8s audit logs? |
| Credential Expiry | Certificate, token expiration tracking | **Gap** | Not mentioned |
| System Info | Version, deployment info, health | **Gap** | Operator status CRs? |

**Recommendation:**
* API tokens become K8s service accounts—acceptable
* Admin events should use K8s audit logs—acceptable
* Feature flags could be operator CR spec fields
* System health should be operator status conditions

---

### 7. Sensor/Runtime Capabilities

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| Process Monitoring | Process indicators, lineage, enrichment | **Covered** | Collector publishes process-events |
| Network Flow Collection | Flow enrichment, pod/deployment correlation | **Covered** | Collector publishes network-flows |
| File Activity Monitoring | Sensitive file access detection | **Covered** | Collector publishes file-events |
| Audit Log Collection | K8s audit log streaming | **Covered** | Audit Logs component |
| Sensor Upgrades | Automated sensor version management | **Gap** | Operator handles? |
| Probe Management | eBPF probe distribution | **Gap** | Collector self-contained? |

**Recommendation:**
* Sensor upgrades become operator-managed; acceptable
* Probe management could be container image updates

---

### 8. Reporting & Analytics

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| Scheduled Reports | ReportConfigurationService, automation | **Gap** | Not mentioned |
| On-Demand Reports | ReportService, vulnerabilities, compliance | **Gap** | Not mentioned |
| Vulnerability Export | VulnMgmtService, stream export | **Gap** | Direct subscription? |
| Search/Query | SearchService, unified search | **Covered** | ACM Search for CRs |

**Recommendation:**
* Reporting could be post-GA or portfolio feature
* Export could be CLI tooling (roxctl equivalent)
* Search covered by ACM Search

---

### 9. Image Management (Detailed)

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| Image Scanning | Scanner V4 with indexer/matcher | **Covered** | Scanner component |
| Image Metadata Caching | TTL-based caching, reprocessing | **Gap** | Implementation detail |
| Delegated Registry Scanning | Pull-through registry support | **Gap** | Scanner configuration |
| Image Re-scanning | Periodic re-scan on demand | **Gap** | Scanner feature |
| Registry Authentication | Per-registry credentials | **Gap** | Scanner secrets management |

**Recommendation:**
* These are implementation details for Scanner component
* Should be captured in Scanner design doc

---

### 10. Virtual Machine Support

| Capability | Current ACS | ACS Next Status | Notes |
|------------|-------------|-----------------|-------|
| VM Inventory | VirtualMachineService | **Covered** | Mentioned in architecture |
| VM Vulnerability Scanning | VM index reports | **Gap** | Scanner scope? |
| VM Agent Integration | Fact agent for VM data | **Gap** | Architecture TBD |

**Recommendation:**
* VM support mentioned but not detailed
* Needs dedicated design section

---

## Critical Gaps Requiring Decisions

### High Priority (Customer-Facing, Pre-GA)

1. **Registry Integrations**
   * Current: 11+ registry types with auth
   * Need: Scanner must authenticate to private registries
   * Decision needed: How does Scanner access registry credentials?

2. **External Notifier Parity**
   * Current: 14+ notifier types including cloud SIEMs
   * Need: External Notifiers component must support same targets
   * Decision needed: Which notifiers are P0 vs P1?

### Medium Priority (Post-GA or Optional)

3. **Scheduled Reports**
   * Current: Automated report generation and delivery
   * Alternative: CLI export + external scheduling
   * Decision needed: Build or defer?

4. **Network Policy Generation**
   * Current: Automatic NetworkPolicy recommendations
   * Alternative: Out of scope (K8s-native tooling)
   * Decision needed: Include or explicitly exclude?

5. **Base Image Tracking**
   * Current: Track base layers, attribute vulnerabilities
   * Alternative: Scanner metadata
   * Decision needed: Feature scope?

### Low Priority (Implementation Details)

6. **Feature Flags**
   * Alternative: Operator CR spec fields

7. **System Health/Info**
   * Alternative: Operator status conditions

8. **Probe Management**
    * Alternative: Container image updates

---

## Explicitly Removed Capabilities (Intentional)

These capabilities are intentionally not in ACS Next:

| Capability | Replacement | Rationale |
|------------|-------------|-----------|
| Compliance Framework | compliance-operator | Avoid duplication; OCP-native |
| Custom RBAC (SAC) | K8s RBAC | Portfolio integration goal |
| Central Aggregation | ACM addon | Portfolio owns multi-cluster |
| Custom Auth Providers | K8s + portfolio identity | Simplification |
| Central-Sensor sync protocol | Not needed | Single-cluster model; no network partition to handle |

**Central-Sensor sync scope:** The current sync machinery spans 90+ files and includes bidirectional gRPC streaming (50+ message types from Sensor, 30+ from Central), dual-sided hash deduplication, chunked deduper state transfer on reconnect, 7-layer stream wrappers, strict sync sequencing, and reconciliation logic. This complexity exists to handle network partitions between Central and Sensor. In ACS Next, components are co-located—this machinery becomes unnecessary.

---

## Feature Mapping to Components

| ACS Next Component | Current ACS Equivalent | Coverage |
|-------------------|------------------------|----------|
| Collector | Collector + ProcessSignal + NetworkFlow | Good |
| Admission Control | AdmissionControl + Sensor detection | Good |
| Scanner | Scanner V4 | Needs registry integration |
| Broker | Central event handling | New pattern |
| CRD Projector | N/A (new) | - |
| Persistence Service | Central PostgreSQL | Subset |
| Alerting Service | Alert manager integration | Good |
| External Notifiers | Notifiers package | Needs parity check |
| Risk Scorer | Risk service | Good |
| Baselines | ProcessBaseline + NetworkBaseline | Good |

---

## Next Steps

1. **Design decisions** on high-priority gaps (vulnerability exceptions, registry auth, signatures)
2. **Scope document** clearly stating what's in v1 vs deferred
3. **Notifier parity** audit to identify P0 notifiers
4. **Scanner design doc** covering implementation details
5. **VM support design doc** if in scope
6. **Broker retention and recovery design** — Define retention windows per stream, PVC sizing, consumer recovery behavior, and acceptable data loss windows (see architecture doc "Consumer Recovery and Failure Modes" section)

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

*This gap analysis is based on codebase exploration as of 2026-02-27. It should be updated as ACS Next design evolves.*
