package policy

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestNoPrivilegedContainers(t *testing.T) {
	e := NewEngine()
	e.LoadDefaultPolicies()

	priv := true
	notPriv := false

	tests := []struct {
		name      string
		podSpec   *corev1.PodSpec
		wantCount int
	}{
		{
			name: "privileged container rejected",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:            "app",
					Image:           "nginx:1.25",
					SecurityContext: &corev1.SecurityContext{Privileged: &priv},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				}},
			},
			wantCount: 1,
		},
		{
			name: "non-privileged container allowed",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:            "app",
					Image:           "nginx:1.25",
					SecurityContext: &corev1.SecurityContext{Privileged: &notPriv},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				}},
			},
			wantCount: 0,
		},
		{
			name: "init container also checked",
			podSpec: &corev1.PodSpec{
				InitContainers: []corev1.Container{{
					Name:            "init",
					Image:           "busybox:1.36",
					SecurityContext: &corev1.SecurityContext{Privileged: &priv},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				}},
				Containers: []corev1.Container{{
					Name:  "app",
					Image: "nginx:1.25",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				}},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := e.EvaluateAdmission(tt.podSpec, "default")
			// Count only "No Privileged Containers" violations
			count := 0
			for _, v := range violations {
				if v.PolicyName == "No Privileged Containers" {
					count++
				}
			}
			if count != tt.wantCount {
				t.Errorf("got %d privileged violations, want %d (all violations: %v)", count, tt.wantCount, violations)
			}
		})
	}
}

func TestNoLatestTag(t *testing.T) {
	e := NewEngine()
	e.LoadDefaultPolicies()

	tests := []struct {
		name  string
		image string
		want  int // 1 = violated, 0 = allowed
	}{
		{"explicit tag allowed", "nginx:1.25", 0},
		{"digest allowed", "nginx@sha256:abc123", 0},
		{"latest tag rejected", "nginx:latest", 1},
		{"no tag rejected", "nginx", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podSpec := &corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "app",
					Image: tt.image,
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				}},
			}
			violations := e.EvaluateAdmission(podSpec, "default")
			count := 0
			for _, v := range violations {
				if v.PolicyName == "No Latest Image Tag" {
					count++
				}
			}
			if count != tt.want {
				t.Errorf("image %q: got %d tag violations, want %d", tt.image, count, tt.want)
			}
		})
	}
}

func TestRequireResourceLimits(t *testing.T) {
	e := NewEngine()
	e.LoadDefaultPolicies()

	tests := []struct {
		name      string
		podSpec   *corev1.PodSpec
		wantCount int
	}{
		{
			name: "both limits set",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "app",
					Image: "nginx:1.25",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				}},
			},
			wantCount: 0,
		},
		{
			name: "no limits at all",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{Name: "app", Image: "nginx:1.25"}},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := e.EvaluateAdmission(tt.podSpec, "default")
			count := 0
			for _, v := range violations {
				if v.PolicyName == "Require Resource Limits" {
					count++
				}
			}
			if count != tt.wantCount {
				t.Errorf("got %d resource limit violations, want %d", count, tt.wantCount)
			}
		})
	}
}

func TestNoHostNetwork(t *testing.T) {
	e := NewEngine()
	e.LoadDefaultPolicies()

	tests := []struct {
		name        string
		hostNetwork bool
		wantCount   int
	}{
		{"host network rejected", true, 1},
		{"no host network allowed", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podSpec := &corev1.PodSpec{
				HostNetwork: tt.hostNetwork,
				Containers: []corev1.Container{{
					Name:  "app",
					Image: "nginx:1.25",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				}},
			}
			violations := e.EvaluateAdmission(podSpec, "default")
			count := 0
			for _, v := range violations {
				if v.PolicyName == "No Host Network" {
					count++
				}
			}
			if count != tt.wantCount {
				t.Errorf("got %d host network violations, want %d", count, tt.wantCount)
			}
		})
	}
}
