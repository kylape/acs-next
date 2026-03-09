# ACS Next Development Environment

Self-contained development environment for building ACS with limited external access.

**All code under `~/workspace/acs-next/` is throwaway prototype code.** This is a
Phase 0 architecture exploration — none of this code will be shipped or deployed to
production as-is. The goal is to validate design decisions and prove out the
event-driven architecture, not to produce production-ready artifacts.

## Git Server (Forgejo)

* **Internal URL**: `http://forgejo.forgejo.svc:3000`
* **External URL**: `https://forgejo-forgejo.apps.rosa.kl-01-29-additi.uxca.p3.openshiftapps.com`
* **Credentials**: `devadmin` / `admin123`
* **Namespace**: `forgejo`

Clone URLs use the format:
```
http://devadmin:admin123@forgejo.forgejo.svc:3000/devadmin/<repo>.git
```

Create new repos via API:
```bash
curl -X POST "http://forgejo.forgejo.svc:3000/api/v1/user/repos" \
  -H "Content-Type: application/json" \
  -u "devadmin:admin123" \
  -d '{"name": "repo-name", "private": false}'
```

## Container Registry

* **Internal URL**: `registry.dev-registry.svc:5000`
* **External URL**: `https://registry-dev-registry.apps.rosa.kl-01-29-additi.uxca.p3.openshiftapps.com`
* **Storage**: 500 Gi GP3 EBS (persistent)
* **Namespace**: `dev-registry`

Push images:
```bash
podman push <image> registry.dev-registry.svc:5000/<image>:<tag>
```

**Important**: Kubernetes pods must pull via the external route URL with TLS,
not the internal `registry.dev-registry.svc:5000` address (kubelet can't resolve it).
Use: `registry-dev-registry.apps.rosa.kl-01-29-additi.uxca.p3.openshiftapps.com`

## Node Configuration

| Node | Instance Type | NVMe Storage | Architecture | Notes |
|------|---------------|--------------|--------------|-------|
| ip-10-0-0-166 | c6id.metal | 2x 1.9TB | x86_64 | Tainted for VMs |
| ip-10-0-0-216 | m7gd.metal | 2x 1.9TB | arm64 | Tainted for VMs (`workload-type=kubevirt-vm:NoSchedule`) |
| ip-10-0-0-219 | m5d.2xlarge | 1x 300GB | x86_64 | General workloads |
| ip-10-0-0-57 | m6gd.xlarge | 1x 237GB | arm64 | General workloads |

Metal nodes are reserved for KubeVirt VMs. Use GP3 EBS for persistent storage needs.

**Build pipelines pin to amd64 nodes** via `nodeSelector: kubernetes.io/arch: amd64`
in PipelineRun podTemplate. Mixed-arch cluster means builds on arm64 produce
images that can't run on amd64.

## Storage Classes

* `gp3-csi` — GP3 EBS, persistent, recommended for most workloads
* `lvms-nvme` — Local NVMe, high performance, node-bound, ephemeral

## Network Access

This environment has **limited external network access**:

* Use internal Forgejo for git: `http://forgejo.forgejo.svc:3000`
* Use internal registry: `registry.dev-registry.svc:5000`
* External pulls (quay.io, docker.io) may be unavailable
* Download dependencies during image build, not at runtime

## ArgoCD

ArgoCD manages all platform resources via the `acs-next-platform` Application.

* **Namespace**: `openshift-gitops`
* **URL**: `https://openshift-gitops-server-openshift-gitops.apps.rosa.kl-01-29-additi.uxca.p3.openshiftapps.com`
* **Password**: `kubectl get secret -n openshift-gitops openshift-gitops-cluster -o jsonpath='{.data.admin\.password}' | base64 -d`
* **Application source**: `platform/` directory in `acs-next` repo on Forgejo
* **Sync**: Automated with prune and self-heal
* **Repo creds**: Stored in `forgejo-repo-creds` secret in `openshift-gitops`

Tekton resources (Pipelines, Triggers, etc.) require `Replace=true` sync
annotation — applied via kustomize patches in `tekton/kustomization.yaml`.
ArgoCD's default apply conflicts with Tekton's validation webhook for
resolver-based taskRef.

The ArgoCD Application manifest is at `argocd/application.yaml`
(outside the synced `platform/` path to avoid self-management loops).

## Platform (GitOps)

The `platform/` directory contains K8s manifests synced by ArgoCD:
* `platform/forgejo/` — Git server deployment
* `platform/registry/` — Container registry deployment
* `platform/broker/` — ACS Broker (embedded NATS) deployment
* `platform/admission-controller/` — Admission controller + ValidatingWebhookConfiguration
* `platform/collector/` — Collector DaemonSet
* `platform/tekton/` — CI infrastructure (triggers with path-based CEL filters, event listener, route)
* `argocd/` — ArgoCD Application definition (not synced by ArgoCD)

## ACS Next Broker (Phase 0 Prototype)

Event-driven architecture centered on an embedded NATS broker with JetStream.

### Monorepo Structure

All components live in a single `acs-next` repo on Forgejo (`devadmin/acs-next`).
Local clone: `~/workspace/acs-next-monorepo/`

| Directory | Language | Description |
|-----------|----------|-------------|
| `broker/` | Go 1.23 | Embedded NATS broker with JetStream |
| `api/` | Protobuf | Event message definitions (buf.build) |
| `admission-controller/` | Go 1.25 | Admission webhook + NATS runtime enforcement |
| `collector-nats/` | C++ | Standalone NATS client library for collector |
| `collector/` | C++ | Full StackRox collector with NATS support |
| `platform/` | K8s YAML | GitOps manifests (synced by ArgoCD) |
| `argocd/` | K8s YAML | ArgoCD Application definition |
| `.tekton/` | YAML | All CI pipeline definitions |

Go modules use `go.work` at the root to link `broker/` and `admission-controller/`.

### Architecture

```
Collector (C++) --[plain NATS]--> NATS (port 4222) --> ACS Broker (Go)
                                                  ├── PROCESS_EVENTS stream (acs.*.process-events)
                                                  └── NETWORK_EVENTS stream (acs.*.network-events)
                                                            │
                                                  Admission Controller (Go)
                                                  ├── Subscribes to both streams (durable pull consumers)
                                                  ├── Evaluates runtime policies
                                                  ├── Publishes violations to acs.alerts
                                                  └── ValidatingWebhook (port 8443)
                                                       ├── Intercepts pod CREATE in labeled namespaces
                                                       ├── Evaluates admission policies
                                                       └── Publishes violations to acs.alerts
```

* Streams use file storage with 15-minute max age
* Subject pattern: `acs.<cluster-id>.<event-type>`
* Consumers can filter by cluster using subject filters
* JetStream data persisted via 10Gi gp3-csi PVC

### mTLS

* Self-signed CA (`CN=ACS NATS CA`) with 10-year validity
* Server cert with SANs: `acs-broker`, `acs-broker.acs-next.svc.cluster.local`,
  `localhost`, `127.0.0.1`
* Client cert for collector/consumer authentication
* Secrets: `nats-server-tls`, `nats-client-tls` in `acs-next` namespace
* TLS enabled via env vars: `TLS_CERT`, `TLS_KEY`, `TLS_CA`, `TLS_VERIFY`
* CA strategy: self-signed for Phase 0; cert-manager for production/vanilla k8s,
  OpenShift service-ca as alternative on OpenShift

### Broker Key Files

```
acs-next-broker/
├── cmd/broker/main.go                  # Entry point, TLS config from env
├── internal/server/server.go           # Embedded NATS + JetStream + mTLS
├── internal/server/server_test.go      # Unit tests (5 tests)
├── test/integration/broker_test.go     # Integration tests (4 tests)
├── Dockerfile                          # Multi-stage build (golang:1.23 -> ubi9-minimal)
├── Makefile                            # Build/test targets
├── go.mod / go.sum
└── README.md
```

### Collector NATS Library

```
collector-nats/
├── include/NatsClient.h                # C++ NATS client interface
├── src/NatsClient.cpp                  # Implementation using cnats
├── test/nats_client_test.cpp           # Unit tests (3) + integration tests (3)
├── CMakeLists.txt                      # CMake build (uses find_library for cnats)
├── Dockerfile                          # Build + test image (UBI9)
└── Makefile
```

### Collector (Full StackRox Collector with NATS)

The full StackRox collector built with NATS support, deployed as a privileged
DaemonSet. Publishes process events to the broker via NATS.

**Key files:**
* `collector/lib/CollectorConfig.cpp` — reads `NATS_URL` and `CLUSTER_ID` env vars
* `collector/lib/system-inspector/Service.cpp` — selects NATS client when `NATS_URL` is set
* `collector/lib/NatsSignalServiceClient.h/.cpp` — NATS signal client (`#ifdef HAVE_NATS`)
* `collector/lib/CMakeLists.txt` — uses `find_library(nats)` to detect cnats
* `collector/container/Dockerfile.acs-next` — 2-stage build using pre-built builder
* `collector/container/Dockerfile.builder` — builder image with all deps + cnats

**Build architecture:**
* Pre-built builder image (`collector-builder:latest`) — CentOS Stream 10 with
  17 third-party libs compiled from source + cnats v3.9.1. Built once, ~40 min.
* Collector build uses builder as base — only compiles collector source. ~17 min
  total (11 min clone with submodules from GitHub + 6 min compile).
* Submodules fetched from GitHub during clone (SUBMODULES=true on git-clone task).
* Runtime image uses UBI10 (must match CentOS Stream 10 glibc).

**Build gotchas:**
* cnats installs to `/usr/local/lib64/` not `/usr/local/lib/` on CentOS Stream 10
* `find_package(cnats CONFIG)` silently fails — use `find_library(NATS_LIB nats)`
  + `find_path(NATS_INCLUDE_DIR nats/nats.h)` instead
* `libnats.so` must be copied into runtime image (dynamically linked)
* `self-checks` binary must also be built and copied (collector crashes without it)
* UBI9 runtime won't work — glibc 2.34 is too old for CentOS Stream 10 binaries

**Deployment gotchas:**
* DaemonSet with `hostNetwork: true` needs `dnsPolicy: ClusterFirstWithHostNet`
  (default `ClusterFirst` won't resolve cluster service DNS)
* Broker TLS disabled for Phase 0 (collector NATS client doesn't support TLS yet)
* Requires privileged SCC (`collector-scc`) for hostPID + hostNetwork

**Container images:**
* Builder: `registry-dev-registry.apps.rosa.../acs-next/collector-builder:latest`
* Collector: `registry-dev-registry.apps.rosa.../acs-next/collector:dev`

**Pipelines:**
* `collector-acs-next-build` — git-clone (SUBMODULES=true) + buildah
* `collector-builder-build` — one-time builder image build
* Trigger: `collector-build-push-trigger` (CEL filter: `body.repository.name == 'collector'`)

**Forgejo source management:**
* Pushed as orphan branch (no 3.4GB git history, source tree is ~3.7MB)
* Source file updates via Forgejo API (`PUT /contents/<path>`) — each creates a
  commit and triggers a webhook build. Cancel intermediate builds.
* Verify local files have changes before base64-encoding for API upload — orphan
  branch checkouts can revert working-tree modifications.

### Admission Controller

The admission controller provides both admission-time and runtime security
policy enforcement. Go 1.25 (required by k8s client-go v0.35).

**Admission policies** (ValidatingWebhook, intercepts pod CREATE):

| Policy | Severity | Check |
|--------|----------|-------|
| `no-privileged-containers` | critical | Rejects `securityContext.privileged: true` |
| `no-latest-tag` | high | Rejects images with `:latest` or no tag |
| `require-resource-limits` | medium | Rejects containers missing cpu/memory limits |
| `no-host-network` | high | Rejects `hostNetwork: true` |

**Runtime policies** (NATS subscriber, evaluates process/network events):

| Policy | Severity | Check |
|--------|----------|-------|
| `suspicious-process` | high | Flags `/bin/bash`, `/bin/sh`, `/bin/nc`, `/usr/bin/curl`, `/usr/bin/wget` |
| `sensitive-port-listen` | medium | Flags ACCEPT on ports 22, 3389, 4444 |

**Enforcement** (Phase 0 — non-destructive):
* Log structured JSON violation
* Annotate pod with `acs-next.stackrox.io/violations`
* Publish alert to `acs.alerts` NATS subject (both admission and runtime)

**Webhook configuration**:
* Opt-in namespace selector: `acs-next.stackrox.io/enforce: "true"`
* `failurePolicy: Ignore` — won't break the cluster if webhook is down
* Self-signed TLS cert (Secret `admission-controller-tls`)
* Cert must have `extendedKeyUsage: serverAuth` for OpenShift API server

**AlertPublisher interface** (`internal/policy/engine.go`):
* Shared by webhook server and NATS subscriber
* `NATSAlertPublisher` implementation in subscriber package
* Publishes to `acs.alerts` with `source` field (`"admission"` or `"runtime"`)

### Admission Controller Key Files

```
acs-next-admission-controller/
├── cmd/admission-controller/main.go     # Entry point, wires webhook + NATS subscriber
├── internal/
│   ├── webhook/
│   │   ├── server.go                    # HTTPS server, POST /validate, health /healthz
│   │   ├── review.go                    # AdmissionReview helpers, pod spec extraction
│   │   └── server_test.go              # 5 tests
│   ├── policy/
│   │   ├── engine.go                    # Policy engine, AlertPublisher interface
│   │   ├── admission.go                 # 4 admission policies
│   │   ├── runtime.go                   # 2 runtime policies
│   │   └── *_test.go                    # 22 tests total
│   └── subscriber/
│       ├── subscriber.go                # NATS JetStream consumer, NATSAlertPublisher
│       └── subscriber_test.go           # 4 tests
├── test/integration/
│   └── admission_test.go               # 4 integration tests (embedded NATS)
├── Dockerfile                           # Multi-stage build (golang:1.25 -> ubi9-minimal)
├── Makefile                             # Build/test targets
└── .tekton/                             # CI pipelines
```

### API Proto Key Files

```
acs-next-api/
├── buf.yaml / buf.gen.yaml             # Buf configuration
└── proto/acs/broker/v1/
    ├── common.proto                    # ContainerInfo shared type
    ├── process.proto                   # ProcessEvent message
    └── network.proto                   # NetworkEvent message
```

### Testing

**Broker unit tests** (`internal/server/server_test.go`):
* `TestNew` — Server creation with various configs
* `TestServerStartStop` — Start/stop lifecycle
* `TestStreamsCreated` — Verifies PROCESS_EVENTS and NETWORK_EVENTS streams
* `TestPublishToStreams` — Publish to both streams
* `TestStreamRetention` — MaxAge and storage config validation

**Broker integration tests** (`test/integration/broker_test.go`):
* `TestBrokerEndToEnd` — Full publish/subscribe with pull consumers
* `TestMultipleClusterIsolation` — Filtered consumers per cluster
* `TestConcurrentPublishers` — 5 publishers x 100 messages concurrently
* `TestNetworkEventsStream` — Network events publish and verify

**Collector NATS unit tests** (`test/nats_client_test.cpp`):
* `test_build_subject` — Verify publish fails when disconnected
* `test_lifecycle` — Start/stop lifecycle safety
* `test_publish_disconnected` — Publish returns false when not connected

**Admission controller unit tests** (`internal/*/`):
* Policy engine tests — registration, evaluation with no/multiple violations
* Admission policy tests — table-driven for each policy (privileged, latest tag,
  resource limits, host network) with pass/fail cases
* Runtime policy tests — suspicious process detection, sensitive port detection
* Webhook tests — allowed/denied responses, bad requests, method not allowed
* Subscriber tests — JSON deserialization, annotation formatting

**Admission controller integration tests** (`test/integration/admission_test.go`):
* `TestWebhookEndToEnd` — compliant pod allowed, privileged pod denied
* `TestRuntimePolicyWithNATS` — publish process event, verify policy fires
* `TestNetworkEventRuntimePolicy` — publish network event, verify policy fires

**Run tests**: `make test` in each repo

### CI/CD (Tekton)

All CI is triggered automatically on push via Forgejo webhooks.

**Pipeline definitions live in `.tekton/` at the repo root.**

| Pipeline file | What it does |
|---------------|--------------|
| `broker-build.yaml`, `broker-test.yaml` | Go test + buildah push |
| `admission-controller-build.yaml`, `admission-controller-test.yaml` | Go test + buildah push |
| `collector-build.yaml` | buildah push (SUBMODULES=true, 30Gi workspace) |
| `collector-nats-test.yaml` | cmake build/test via buildah |
| `api-generate.yaml` | buf generate |

All pipelines clone the single `acs-next` repo with `SUBMODULES=false` except
the collector build which uses `SUBMODULES=true` for GitHub submodules.

**Triggers** (in `platform/tekton/triggers/acs-next-triggers.yaml`):
* Path-based CEL filters — e.g., `body.commits.exists(c, c.modified.exists(f, f.startsWith('broker/')))`
* Single TriggerBinding, 5 TriggerTemplates, 5 Triggers
* Single EventListener (`forgejo-listener`)

To update a pipeline, edit `.tekton/*.yaml`, then apply:
```bash
kubectl apply -f .tekton/
```

**EventListener Route**:
`https://forgejo-webhook-acs-next.apps.rosa.kl-01-29-additi.uxca.p3.openshiftapps.com`

Single Forgejo webhook on the `acs-next` repo.

### Deployed Resources (namespace: acs-next)

* `deployment/acs-broker` — 1 replica, TLS disabled (Phase 0), PVC storage, imagePullPolicy: Always
* `service/acs-broker` — ClusterIP on port 4222
* `pvc/broker-jetstream-data` — 10Gi gp3-csi for JetStream persistence
* `secret/nats-server-tls` — Server TLS cert/key/CA (mounted but not used while TLS disabled)
* `secret/nats-client-tls` — Client TLS cert/key/CA
* `daemonset/collector` — privileged, hostPID, hostNetwork, amd64 only, NATS client
* `serviceaccount/collector` — SA for collector pods
* `scc/collector-scc` — SecurityContextConstraints for privileged collector
* `deployment/acs-admission-controller` — 1 replica, webhook + NATS subscriber
* `service/acs-admission-controller` — ClusterIP port 443 → 8443
* `validatingwebhookconfiguration/acs-admission-controller` — opt-in via namespace label
* `secret/admission-controller-tls` — Webhook server TLS cert/key/CA
* `clusterrole/acs-admission-controller` — pods get/list/patch for annotations
* `deployment/el-forgejo-listener` — Tekton EventListener
* `route/forgejo-webhook` — Edge-terminated TLS route for webhooks

### Container Images

* Broker: `registry-dev-registry.apps.rosa.../acs-next/broker:dev`
* Collector: `registry-dev-registry.apps.rosa.../acs-next/collector:dev`
* Collector Builder: `registry-dev-registry.apps.rosa.../acs-next/collector-builder:latest`
* Collector NATS: `registry-dev-registry.apps.rosa.../acs-next/collector-nats:dev`
* Admission Controller: `registry-dev-registry.apps.rosa.../acs-next/admission-controller:dev`

### Known Issues / Lessons Learned

* kubelet cannot resolve `registry.dev-registry.svc` — use external route URL
* `kubectl apply` doesn't work with `generateName` — use `kubectl create -f -`
* Tekton tasks in `openshift-pipelines` ns use uppercase params (`URL`, `REVISION`)
* ArgoCD default apply conflicts with Tekton resolver-based taskRef — use Replace=true
* Build pipelines must pin to amd64 nodes (mixed-arch cluster)
* OpenShift SCC prevents `privileged: true` — use buildah ClusterTask (vfs storage)
* Mutable `:dev` tags need `imagePullPolicy: Always`
* `hostNetwork: true` DaemonSets need `dnsPolicy: ClusterFirstWithHostNet` for cluster DNS
* Collector builder uses CentOS Stream 10 — runtime must be UBI10 (glibc version match)
* cnats `find_package(cnats CONFIG)` silently fails — use `find_library` instead
* cnats installs to `/usr/local/lib64/` on CentOS Stream 10, not `/usr/local/lib/`
* Collector source in monorepo has truncated history (no 3.4GB git history)
* Webhook TLS certs need `extendedKeyUsage: serverAuth` for OpenShift API server
* Webhook `caBundle` must match the CA that signed the server cert (fingerprint
  mismatch causes "tls: bad certificate")
* Use opt-in `namespaceSelector` for webhooks on OpenShift — system namespaces
  like `openshift-backplane` get blocked otherwise
* No podman socket in devcontainer — use Tekton pipelines for image builds
* k8s client-go v0.35 requires Go 1.25 (Dockerfile must use `golang:1.25`)

## Directory Structure

```
acs-next/                              # Single Forgejo repo (devadmin/acs-next)
├── CLAUDE.md                          # This file (development reference)
├── go.work                            # Go workspace linking broker + admission-controller
├── .tekton/                           # All CI pipeline definitions
├── broker/                            # Embedded NATS broker (Go 1.23)
├── api/                               # Protobuf definitions (buf.build)
├── admission-controller/              # Admission webhook + NATS subscriber (Go 1.25)
├── collector-nats/                    # Standalone NATS client library (C++)
├── collector/                         # Full StackRox collector with NATS support (C++)
│   ├── collector/container/Dockerfile.acs-next
│   ├── collector/container/Dockerfile.builder
│   ├── collector/lib/NatsSignalServiceClient.*
│   └── collector/lib/CMakeLists.txt
├── platform/                          # K8s manifests (synced by ArgoCD)
│   ├── forgejo/                       # Git server
│   ├── registry/                      # Container registry
│   ├── broker/                        # Broker deployment
│   ├── admission-controller/          # Webhook deployment
│   ├── collector/                     # Collector DaemonSet
│   └── tekton/                        # CI triggers (path-based CEL filters)
└── argocd/                            # ArgoCD Application definition
```
