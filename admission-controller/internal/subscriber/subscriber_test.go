package subscriber

import (
	"encoding/json"
	"testing"

	"acs-next-admission-controller/internal/policy"
)

func TestProcessEventDeserialization(t *testing.T) {
	data := `{
		"cluster_id": "test-cluster",
		"timestamp": "2026-03-06T12:00:00Z",
		"container": {
			"id": "abc123",
			"name": "app",
			"image": "nginx:1.25",
			"namespace": "default",
			"pod": "test-pod"
		},
		"process_name": "bash",
		"executable": "/bin/bash",
		"args": ["-c", "whoami"],
		"pid": 1234,
		"parent_pid": 1,
		"uid": 0
	}`

	var event policy.ProcessEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if event.ClusterID != "test-cluster" {
		t.Errorf("expected cluster_id test-cluster, got %s", event.ClusterID)
	}
	if event.Executable != "/bin/bash" {
		t.Errorf("expected executable /bin/bash, got %s", event.Executable)
	}
	if event.Container.Pod != "test-pod" {
		t.Errorf("expected pod test-pod, got %s", event.Container.Pod)
	}
}

func TestNetworkEventDeserialization(t *testing.T) {
	data := `{
		"cluster_id": "test-cluster",
		"timestamp": "2026-03-06T12:00:00Z",
		"container": {
			"id": "abc123",
			"name": "app",
			"image": "nginx:1.25",
			"namespace": "default",
			"pod": "test-pod"
		},
		"src_ip": "10.0.0.1",
		"src_port": 45678,
		"dst_ip": "10.0.0.2",
		"dst_port": 22,
		"protocol": "TCP",
		"event_type": "ACCEPT"
	}`

	var event policy.NetworkEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if event.EventType != "ACCEPT" {
		t.Errorf("expected event_type ACCEPT, got %s", event.EventType)
	}
	if event.DstPort != 22 {
		t.Errorf("expected dst_port 22, got %d", event.DstPort)
	}
}

func TestFormatAnnotation(t *testing.T) {
	violations := []policy.Violation{
		{PolicyName: "suspicious-process", Message: "/bin/bash detected"},
		{PolicyName: "sensitive-port-listen", Message: "port 22 open"},
	}

	result := FormatAnnotation(violations)
	expected := "suspicious-process: /bin/bash detected; sensitive-port-listen: port 22 open"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormatAnnotationEmpty(t *testing.T) {
	result := FormatAnnotation(nil)
	if result != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}
