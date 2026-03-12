package subscriber

import (
	"encoding/json"
	"testing"
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

	var event ProcessEvent
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if event.ClusterID != "test-cluster" {
		t.Errorf("expected cluster_id test-cluster, got %s", event.ClusterID)
	}
	if event.Executable != "/bin/bash" {
		t.Errorf("expected executable /bin/bash, got %s", event.Executable)
	}
}

func TestEvaluateProcessEvent(t *testing.T) {
	e := NewEngine()
	e.LoadDefaultPolicies()

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
	e.LoadDefaultPolicies()

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

func TestSafeProcessNotFlagged(t *testing.T) {
	e := NewEngine()
	e.LoadDefaultPolicies()

	event := ProcessEvent{
		Executable: "/usr/bin/java",
		Container:  ContainerInfo{Namespace: "default", Pod: "test-pod"},
	}
	violations := e.EvaluateProcessEvent(event)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d", len(violations))
	}
}

func TestNormalPortNotFlagged(t *testing.T) {
	e := NewEngine()
	e.LoadDefaultPolicies()

	event := NetworkEvent{
		EventType: "ACCEPT",
		DstPort:   8080,
		Container: ContainerInfo{Namespace: "default", Pod: "test-pod"},
	}
	violations := e.EvaluateNetworkEvent(event)
	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d", len(violations))
	}
}
