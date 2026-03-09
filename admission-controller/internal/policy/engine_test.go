package policy

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestNewEngineRegistersDefaults(t *testing.T) {
	e := NewEngine()
	if len(e.admissionPolicies) != 4 {
		t.Errorf("expected 4 admission policies, got %d", len(e.admissionPolicies))
	}
	if len(e.runtimePolicies) != 2 {
		t.Errorf("expected 2 runtime policies, got %d", len(e.runtimePolicies))
	}
}

func TestEvaluateAdmissionNoViolations(t *testing.T) {
	e := NewEngine()
	podSpec := &corev1.PodSpec{
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
	if len(violations) != 0 {
		t.Errorf("expected no violations, got %d: %v", len(violations), violations)
	}
}

func TestEvaluateAdmissionMultipleViolations(t *testing.T) {
	e := NewEngine()
	priv := true
	podSpec := &corev1.PodSpec{
		HostNetwork: true,
		Containers: []corev1.Container{{
			Name:  "app",
			Image: "nginx",
			SecurityContext: &corev1.SecurityContext{
				Privileged: &priv,
			},
		}},
	}
	violations := e.EvaluateAdmission(podSpec, "default")
	if len(violations) < 3 {
		t.Errorf("expected at least 3 violations (privileged, latest, no limits, hostNetwork), got %d: %v", len(violations), violations)
	}
}

func TestEvaluateProcessEvent(t *testing.T) {
	e := NewEngine()
	event := ProcessEvent{
		Executable: "/bin/bash",
		Container:  ContainerInfo{Namespace: "default", Pod: "test-pod"},
	}
	violations := e.EvaluateProcessEvent(event)
	if len(violations) != 1 {
		t.Errorf("expected 1 violation, got %d", len(violations))
	}
}

func TestEvaluateNetworkEvent(t *testing.T) {
	e := NewEngine()
	event := NetworkEvent{
		EventType: "ACCEPT",
		DstPort:   22,
		Container: ContainerInfo{Namespace: "default", Pod: "test-pod"},
	}
	violations := e.EvaluateNetworkEvent(event)
	if len(violations) != 1 {
		t.Errorf("expected 1 violation, got %d", len(violations))
	}
}
