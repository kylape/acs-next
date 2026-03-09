package policy

import "fmt"

var suspiciousExecutables = map[string]bool{
	"/bin/sh":       true,
	"/bin/bash":     true,
	"/bin/nc":       true,
	"/usr/bin/curl": true,
	"/usr/bin/wget": true,
}

var sensitivePorts = map[int]string{
	22:   "SSH",
	3389: "RDP",
	4444: "Metasploit",
}

// SuspiciousProcess flags process events matching a deny list.
type SuspiciousProcess struct{}

func (p *SuspiciousProcess) Name() string { return "suspicious-process" }

func (p *SuspiciousProcess) EvaluateProcess(event ProcessEvent) []Violation {
	if suspiciousExecutables[event.Executable] {
		return []Violation{{
			PolicyName: p.Name(),
			Message: fmt.Sprintf("suspicious process %q detected in container %s/%s",
				event.Executable, event.Container.Namespace, event.Container.Pod),
			Severity: "high",
		}}
	}
	return nil
}

func (p *SuspiciousProcess) EvaluateNetwork(_ NetworkEvent) []Violation {
	return nil
}

// SensitivePortListen flags ACCEPT events on well-known sensitive ports.
type SensitivePortListen struct{}

func (p *SensitivePortListen) Name() string { return "sensitive-port-listen" }

func (p *SensitivePortListen) EvaluateProcess(_ ProcessEvent) []Violation {
	return nil
}

func (p *SensitivePortListen) EvaluateNetwork(event NetworkEvent) []Violation {
	if event.EventType != "ACCEPT" {
		return nil
	}
	if name, ok := sensitivePorts[event.DstPort]; ok {
		return []Violation{{
			PolicyName: p.Name(),
			Message: fmt.Sprintf("listening on sensitive %s port %d in container %s/%s",
				name, event.DstPort, event.Container.Namespace, event.Container.Pod),
			Severity: "medium",
		}}
	}
	return nil
}
