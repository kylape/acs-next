package policy

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// NoPrivilegedContainers rejects pods with privileged containers.
type NoPrivilegedContainers struct{}

func (p *NoPrivilegedContainers) Name() string { return "no-privileged-containers" }

func (p *NoPrivilegedContainers) Evaluate(podSpec *corev1.PodSpec, _ string) []Violation {
	var violations []Violation
	for _, c := range allContainers(podSpec) {
		if c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
			violations = append(violations, Violation{
				PolicyName: p.Name(),
				Message:    fmt.Sprintf("container %q is privileged", c.Name),
				Severity:   "critical",
			})
		}
	}
	return violations
}

// NoLatestTag rejects containers using the :latest tag or no tag.
type NoLatestTag struct{}

func (p *NoLatestTag) Name() string { return "no-latest-tag" }

func (p *NoLatestTag) Evaluate(podSpec *corev1.PodSpec, _ string) []Violation {
	var violations []Violation
	for _, c := range allContainers(podSpec) {
		image := c.Image
		if image == "" {
			continue
		}
		// Split off digest if present
		if strings.Contains(image, "@") {
			continue // digest-pinned images are fine
		}
		parts := strings.Split(image, ":")
		if len(parts) == 1 || parts[len(parts)-1] == "latest" {
			violations = append(violations, Violation{
				PolicyName: p.Name(),
				Message:    fmt.Sprintf("container %q uses image %q (must specify a non-latest tag)", c.Name, image),
				Severity:   "high",
			})
		}
	}
	return violations
}

// RequireResourceLimits rejects containers without CPU and memory limits.
type RequireResourceLimits struct{}

func (p *RequireResourceLimits) Name() string { return "require-resource-limits" }

func (p *RequireResourceLimits) Evaluate(podSpec *corev1.PodSpec, _ string) []Violation {
	var violations []Violation
	for _, c := range allContainers(podSpec) {
		limits := c.Resources.Limits
		if limits == nil {
			violations = append(violations, Violation{
				PolicyName: p.Name(),
				Message:    fmt.Sprintf("container %q has no resource limits", c.Name),
				Severity:   "medium",
			})
			continue
		}
		var missing []string
		if _, ok := limits[corev1.ResourceCPU]; !ok {
			missing = append(missing, "cpu")
		}
		if _, ok := limits[corev1.ResourceMemory]; !ok {
			missing = append(missing, "memory")
		}
		if len(missing) > 0 {
			violations = append(violations, Violation{
				PolicyName: p.Name(),
				Message:    fmt.Sprintf("container %q is missing resource limits: %s", c.Name, strings.Join(missing, ", ")),
				Severity:   "medium",
			})
		}
	}
	return violations
}

// NoHostNetwork rejects pods with hostNetwork enabled.
type NoHostNetwork struct{}

func (p *NoHostNetwork) Name() string { return "no-host-network" }

func (p *NoHostNetwork) Evaluate(podSpec *corev1.PodSpec, _ string) []Violation {
	if podSpec.HostNetwork {
		return []Violation{{
			PolicyName: p.Name(),
			Message:    "pod uses hostNetwork",
			Severity:   "high",
		}}
	}
	return nil
}

func allContainers(podSpec *corev1.PodSpec) []corev1.Container {
	all := make([]corev1.Container, 0, len(podSpec.InitContainers)+len(podSpec.Containers))
	all = append(all, podSpec.InitContainers...)
	all = append(all, podSpec.Containers...)
	return all
}
