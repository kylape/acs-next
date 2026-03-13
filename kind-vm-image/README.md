# KinD VM Image

Pre-baked KubeVirt containerDisk image with KinD, podman, kubectl, and helm
pre-installed. Used by the acs-next e2e pipeline to quickly spin up ephemeral
Kubernetes clusters for integration testing.

## What's Included

* **Base OS**: Fedora (from OpenShift Virtualization golden images)
* **Container runtime**: Podman (KinD uses `KIND_EXPERIMENTAL_PROVIDER=podman`)
* **KinD**: v0.22.0
* **kubectl**: v1.29.2
* **Helm**: v3.14.0
* **cloud-init**: For first-boot configuration
* **kind-setup.sh**: Creates KinD cluster with registry mirror on boot

## How It Works

1. The Tekton build pipeline clones the Fedora golden image into a PVC
2. `disk-virt-customize` installs all packages and tools into the qcow2
3. `disk-uploader` wraps the qcow2 in a containerDisk and pushes to the registry

At runtime:

1. The e2e pipeline creates a KubeVirt VM from this containerDisk
2. Cloud-init calls `kind-setup.sh` with `REGISTRY_IP` and `K8S_VERSION` env vars
3. `kind-setup.sh` creates a KinD cluster and configures containerd to mirror
   the cluster's internal registry via the ClusterIP
4. Readiness is signaled via `/tmp/kind-ready`

## Building

### Via Tekton Pipeline (Recommended)

```bash
kubectl apply -f .tekton/kind-vm-image-build.yaml
kubectl create -f - <<EOF
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  generateName: kind-vm-image-build-
  namespace: acs-next
spec:
  pipelineRef:
    name: kind-vm-image-build
  taskRunTemplate:
    serviceAccountName: kind-vm-e2e-sa
    podTemplate:
      nodeSelector:
        kubernetes.io/arch: amd64
  workspaces:
    - name: source
      volumeClaimTemplate:
        spec:
          accessModes: [ReadWriteOnce]
          storageClassName: gp3-csi
          resources:
            requests:
              storage: 1Gi
EOF
```

## Files

* `kind-setup.sh` — Startup script baked into the VM image
* `customize-commands.txt` — virt-customize commands for the build pipeline
