package policy

import "testing"

func TestSuspiciousProcess(t *testing.T) {
	e := NewEngine()
	e.LoadDefaultPolicies()

	tests := []struct {
		name       string
		executable string
		wantCount  int
	}{
		{"bash detected", "/bin/bash", 1},
		{"sh detected", "/bin/sh", 1},
		{"nc detected", "/bin/nc", 1},
		{"curl detected", "/usr/bin/curl", 1},
		{"wget detected", "/usr/bin/wget", 1},
		{"java safe", "/usr/bin/java", 0},
		{"python safe", "/usr/bin/python3", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := ProcessEvent{
				Executable: tt.executable,
				Container:  ContainerInfo{Namespace: "default", Pod: "test-pod"},
			}
			violations := e.EvaluateProcessEvent(event)
			if len(violations) != tt.wantCount {
				t.Errorf("executable %q: got %d violations, want %d", tt.executable, len(violations), tt.wantCount)
			}
		})
	}
}

func TestSensitivePortListen(t *testing.T) {
	e := NewEngine()
	e.LoadDefaultPolicies()

	tests := []struct {
		name      string
		eventType string
		dstPort   int
		wantCount int
	}{
		{"SSH accept flagged", "ACCEPT", 22, 1},
		{"RDP accept flagged", "ACCEPT", 3389, 1},
		{"metasploit accept flagged", "ACCEPT", 4444, 1},
		{"SSH connect not flagged", "CONNECT", 22, 0},
		{"normal port accept not flagged", "ACCEPT", 8080, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := NetworkEvent{
				EventType: tt.eventType,
				DstPort:   tt.dstPort,
				Container: ContainerInfo{Namespace: "default", Pod: "test-pod"},
			}
			violations := e.EvaluateNetworkEvent(event)
			if len(violations) != tt.wantCount {
				t.Errorf("got %d violations, want %d", len(violations), tt.wantCount)
			}
		})
	}
}
