#!/bin/bash
# kind-setup.sh — Creates a KinD cluster inside the VM with optional registry mirror.
# Invoked by cloud-init runcmd with optional REGISTRY_IP and K8S_VERSION env vars.
set -euo pipefail

REGISTRY_IP="${REGISTRY_IP:-}"  # Optional: if set, configures registry mirror
K8S_VERSION="${K8S_VERSION:-v1.29.2}"
CLUSTER_NAME="${CLUSTER_NAME:-e2e}"

export KIND_EXPERIMENTAL_PROVIDER=podman

if [[ -n "${REGISTRY_IP}" ]]; then
  echo "Starting KinD cluster: ${CLUSTER_NAME} (k8s ${K8S_VERSION}, registry ${REGISTRY_IP}:5000)"
else
  echo "Starting KinD cluster: ${CLUSTER_NAME} (k8s ${K8S_VERSION}, no registry mirror)"
fi

# Wait for podman
echo "Waiting for podman..."
for i in $(seq 1 60); do
  if podman info &>/dev/null; then
    echo "Podman is ready"
    break
  fi
  sleep 2
done

# Create KinD cluster config
cat > /tmp/kind-config.yaml <<EOF
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
name: ${CLUSTER_NAME}
containerdConfigPatches:
- |-
  [plugins."io.containerd.grpc.v1.cri".registry]
    config_path = "/etc/containerd/certs.d"
networking:
  apiServerAddress: "0.0.0.0"
  apiServerPort: 6443
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
EOF

# Create KinD cluster
kind create cluster \
  --name "${CLUSTER_NAME}" \
  --image "kindest/node:${K8S_VERSION}" \
  --config /tmp/kind-config.yaml \
  --wait 120s

# Configure containerd registry mirrors on each KinD node (if REGISTRY_IP is set).
# This mirrors the pattern from kind-with-registry.sh — each node gets a
# hosts.toml that maps the registry name to the actual ClusterIP.
if [[ -n "${REGISTRY_IP}" ]]; then
  REGISTRY_DIR="/etc/containerd/certs.d"
  for node in $(kind get nodes --name "${CLUSTER_NAME}"); do
    # Mirror: registry.dev-registry.svc:5000 → ClusterIP
    podman exec "${node}" mkdir -p "${REGISTRY_DIR}/registry.dev-registry.svc:5000"
    cat <<TOML | podman exec -i "${node}" cp /dev/stdin "${REGISTRY_DIR}/registry.dev-registry.svc:5000/hosts.toml"
server = "http://${REGISTRY_IP}:5000"

[host."http://${REGISTRY_IP}:5000"]
  capabilities = ["pull", "resolve"]
  skip_verify = true
TOML

    # Also map the raw ClusterIP:5000
    podman exec "${node}" mkdir -p "${REGISTRY_DIR}/${REGISTRY_IP}:5000"
    cat <<TOML | podman exec -i "${node}" cp /dev/stdin "${REGISTRY_DIR}/${REGISTRY_IP}:5000/hosts.toml"
server = "http://${REGISTRY_IP}:5000"

[host."http://${REGISTRY_IP}:5000"]
  capabilities = ["pull", "resolve"]
  skip_verify = true
TOML
  done
fi

# Export kubeconfig
SERVE_DIR="/home/stackrox/serve"
mkdir -p "${SERVE_DIR}"
kind get kubeconfig --name "${CLUSTER_NAME}" > "${SERVE_DIR}/kubeconfig"
chown -R stackrox:stackrox "${SERVE_DIR}"

# Signal readiness
touch "${SERVE_DIR}/ready"

# Start HTTP server to expose kubeconfig and readiness to the pipeline.
# The VMI pod IP is reachable from other pods in the cluster, so the
# pipeline task can just curl http://<VMI_IP>:8080/kubeconfig.
echo "Starting HTTP server on :8080..."
cd "${SERVE_DIR}" && python3 -m http.server 8080 &

echo "KinD cluster '${CLUSTER_NAME}' is ready"
