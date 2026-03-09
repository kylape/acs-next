package policy

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestNoPrivilegedContainers(t *testing.T) {
	p := &NoPrivilegedContainers{}
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
					SecurityContext: &corev1.SecurityContext{Privileged: &priv},
				}},
			},
			wantCount: 1,
		},
		{
			name: "non-privileged container allowed",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:            "app",
					SecurityContext: &corev1.SecurityContext{Privileged: &notPriv},
				}},
			},
			wantCount: 0,
		},
		{
			name: "no security context allowed",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{Name: "app"}},
			},
			wantCount: 0,
		},
		{
			name: "init container also checked",
			podSpec: &corev1.PodSpec{
				InitContainers: []corev1.Container{{
					Name:            "init",
					SecurityContext: &corev1.SecurityContext{Privileged: &priv},
				}},
				Containers: []corev1.Container{{Name: "app"}},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := p.Evaluate(tt.podSpec, "default")
			if len(violations) != tt.wantCount {
				t.Errorf("got %d violations, want %d", len(violations), tt.wantCount)
			}
		})
	}
}

func TestNoLatestTag(t *testing.T) {
	p := &NoLatestTag{}

	tests := []struct {
		name      string
		image     string
		wantCount int
	}{
		{"explicit tag allowed", "nginx:1.25", 0},
		{"digest allowed", "nginx@sha256:abc123", 0},
		{"latest tag rejected", "nginx:latest", 1},
		{"no tag rejected", "nginx", 1},
		{"registry with tag allowed", "registry.example.com/nginx:v1", 0},
		{"registry no tag rejected", "registry.example.com/nginx", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			podSpec := &corev1.PodSpec{
				Containers: []corev1.Container{{Name: "app", Image: tt.image}},
			}
			violations := p.Evaluate(podSpec, "default")
			if len(violations) != tt.wantCount {
				t.Errorf("image %q: got %d violations, want %d", tt.image, len(violations), tt.wantCount)
			}
		})
	}
}

func TestRequireResourceLimits(t *testing.T) {
	p := &RequireResourceLimits{}

	tests := []struct {
		name      string
		podSpec   *corev1.PodSpec
		wantCount int
	}{
		{
			name: "both limits set",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "app",
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
				Containers: []corev1.Container{{Name: "app"}},
			},
			wantCount: 1,
		},
		{
			name: "missing memory limit",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
					},
				}},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			violations := p.Evaluate(tt.podSpec, "default")
			if len(violations) != tt.wantCount {
				t.Errorf("got %d violations, want %d", len(violations), tt.wantCount)
			}
		})
	}
}

func TestNoHostNetwork(t *testing.T) {
	p := &NoHostNetwork{}

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
				Containers:  []corev1.Container{{Name: "app"}},
			}
			violations := p.Evaluate(podSpec, "default")
			if len(violations) != tt.wantCount {
				t.Errorf("got %d violations, want %d", len(violations), tt.wantCount)
			}
		})
	}
}
