package evaluator

import (
	"testing"

	commonv1 "acs-next.stackrox.io/apis/common/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestPrivilegedContainer(t *testing.T) {
	priv := true
	sections := []commonv1.PolicySection{{
		PolicyGroups: []commonv1.PolicyGroup{{
			FieldName: "Privileged Container",
		}},
	}}

	tests := []struct {
		name    string
		podSpec *corev1.PodSpec
		want    bool
	}{
		{
			name: "privileged container matches",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:            "app",
					SecurityContext: &corev1.SecurityContext{Privileged: &priv},
				}},
			},
			want: true,
		},
		{
			name: "non-privileged does not match",
			podSpec: &corev1.PodSpec{
				Containers: []corev1.Container{{Name: "app"}},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvalContext{PodSpec: tt.podSpec}
			result := EvaluateSections(sections, ctx)
			if result.Matched != tt.want {
				t.Errorf("got matched=%v, want %v", result.Matched, tt.want)
			}
		})
	}
}

func TestImageTag(t *testing.T) {
	sections := []commonv1.PolicySection{{
		PolicyGroups: []commonv1.PolicyGroup{{
			FieldName: "Image Tag",
			Values:    []commonv1.PolicyValue{{Value: "latest"}},
		}},
	}}

	tests := []struct {
		name  string
		image string
		want  bool
	}{
		{"latest tag matches", "nginx:latest", true},
		{"no tag matches", "nginx", true},
		{"specific tag no match", "nginx:1.25", false},
		{"digest no match", "nginx@sha256:abc123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvalContext{
				PodSpec: &corev1.PodSpec{
					Containers: []corev1.Container{{Name: "app", Image: tt.image}},
				},
			}
			result := EvaluateSections(sections, ctx)
			if result.Matched != tt.want {
				t.Errorf("image %q: got matched=%v, want %v", tt.image, result.Matched, tt.want)
			}
		})
	}
}

func TestResourceLimits(t *testing.T) {
	cpuSection := []commonv1.PolicySection{{
		PolicyGroups: []commonv1.PolicyGroup{{
			FieldName: "Container CPU Limit",
		}},
	}}

	memSection := []commonv1.PolicySection{{
		PolicyGroups: []commonv1.PolicyGroup{{
			FieldName: "Container Memory Limit",
		}},
	}}

	withLimits := &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name: "app",
			Resources: corev1.ResourceRequirements{
				Limits: corev1.ResourceList{
					corev1.ResourceCPU:    resource.MustParse("100m"),
					corev1.ResourceMemory: resource.MustParse("128Mi"),
				},
			},
		}},
	}

	noLimits := &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "app"}},
	}

	// CPU limit present -> no match (field checks for *missing* limit)
	result := EvaluateSections(cpuSection, &EvalContext{PodSpec: withLimits})
	if result.Matched {
		t.Error("expected no CPU limit match when limits are set")
	}

	// CPU limit missing -> match
	result = EvaluateSections(cpuSection, &EvalContext{PodSpec: noLimits})
	if !result.Matched {
		t.Error("expected CPU limit match when limits are missing")
	}

	// Memory limit present -> no match
	result = EvaluateSections(memSection, &EvalContext{PodSpec: withLimits})
	if result.Matched {
		t.Error("expected no memory limit match when limits are set")
	}

	// Memory limit missing -> match
	result = EvaluateSections(memSection, &EvalContext{PodSpec: noLimits})
	if !result.Matched {
		t.Error("expected memory limit match when limits are missing")
	}
}

func TestHostNetwork(t *testing.T) {
	sections := []commonv1.PolicySection{{
		PolicyGroups: []commonv1.PolicyGroup{{
			FieldName: "Host Network",
		}},
	}}

	ctx := &EvalContext{PodSpec: &corev1.PodSpec{HostNetwork: true, Containers: []corev1.Container{{Name: "app"}}}}
	result := EvaluateSections(sections, ctx)
	if !result.Matched {
		t.Error("expected host network match")
	}

	ctx = &EvalContext{PodSpec: &corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}}
	result = EvaluateSections(sections, ctx)
	if result.Matched {
		t.Error("expected no host network match")
	}
}

func TestProcessName(t *testing.T) {
	sections := []commonv1.PolicySection{{
		PolicyGroups: []commonv1.PolicyGroup{{
			FieldName: "Process Name",
			Values: []commonv1.PolicyValue{
				{Value: "/bin/bash"},
				{Value: "/bin/sh"},
			},
		}},
	}}

	tests := []struct {
		name string
		exec string
		want bool
	}{
		{"bash matches", "/bin/bash", true},
		{"sh matches", "/bin/sh", true},
		{"java no match", "/usr/bin/java", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvalContext{Executable: tt.exec}
			result := EvaluateSections(sections, ctx)
			if result.Matched != tt.want {
				t.Errorf("executable %q: got matched=%v, want %v", tt.exec, result.Matched, tt.want)
			}
		})
	}
}

func TestPort(t *testing.T) {
	sections := []commonv1.PolicySection{{
		PolicyGroups: []commonv1.PolicyGroup{{
			FieldName: "Port",
			Values: []commonv1.PolicyValue{
				{Value: "22"},
				{Value: "3389"},
			},
		}},
	}}

	tests := []struct {
		name string
		port int
		want bool
	}{
		{"SSH port matches", 22, true},
		{"RDP port matches", 3389, true},
		{"normal port no match", 8080, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &EvalContext{DstPort: tt.port}
			result := EvaluateSections(sections, ctx)
			if result.Matched != tt.want {
				t.Errorf("port %d: got matched=%v, want %v", tt.port, result.Matched, tt.want)
			}
		})
	}
}

func TestNegation(t *testing.T) {
	sections := []commonv1.PolicySection{{
		PolicyGroups: []commonv1.PolicyGroup{{
			FieldName: "Privileged Container",
			Negate:    true,
		}},
	}}

	// Non-privileged + negate = match
	ctx := &EvalContext{PodSpec: &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "app"}},
	}}
	result := EvaluateSections(sections, ctx)
	if !result.Matched {
		t.Error("expected negated non-privileged to match")
	}

	// Privileged + negate = no match
	priv := true
	ctx = &EvalContext{PodSpec: &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:            "app",
			SecurityContext: &corev1.SecurityContext{Privileged: &priv},
		}},
	}}
	result = EvaluateSections(sections, ctx)
	if result.Matched {
		t.Error("expected negated privileged to not match")
	}
}

func TestMultipleGroupsANDed(t *testing.T) {
	sections := []commonv1.PolicySection{{
		PolicyGroups: []commonv1.PolicyGroup{
			{FieldName: "Privileged Container"},
			{FieldName: "Host Network"},
		},
	}}

	priv := true

	// Both conditions met
	ctx := &EvalContext{PodSpec: &corev1.PodSpec{
		HostNetwork: true,
		Containers: []corev1.Container{{
			Name:            "app",
			SecurityContext: &corev1.SecurityContext{Privileged: &priv},
		}},
	}}
	result := EvaluateSections(sections, ctx)
	if !result.Matched {
		t.Error("expected both conditions to match")
	}

	// Only privileged
	ctx = &EvalContext{PodSpec: &corev1.PodSpec{
		Containers: []corev1.Container{{
			Name:            "app",
			SecurityContext: &corev1.SecurityContext{Privileged: &priv},
		}},
	}}
	result = EvaluateSections(sections, ctx)
	if result.Matched {
		t.Error("expected partial match to fail with AND")
	}
}

func TestUnsupportedField(t *testing.T) {
	sections := []commonv1.PolicySection{{
		PolicyGroups: []commonv1.PolicyGroup{{
			FieldName: "Unknown Field",
		}},
	}}

	ctx := &EvalContext{PodSpec: &corev1.PodSpec{
		Containers: []corev1.Container{{Name: "app"}},
	}}
	result := EvaluateSections(sections, ctx)
	if result.Matched {
		t.Error("expected unsupported field to not match")
	}
}
