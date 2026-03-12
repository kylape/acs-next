package policy

import (
	corev1 "k8s.io/api/core/v1"
)

// allContainers returns all init + regular containers in a pod spec.
func allContainers(podSpec *corev1.PodSpec) []corev1.Container {
	all := make([]corev1.Container, 0, len(podSpec.InitContainers)+len(podSpec.Containers))
	all = append(all, podSpec.InitContainers...)
	all = append(all, podSpec.Containers...)
	return all
}
