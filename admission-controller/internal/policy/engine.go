package policy

import (
	corev1 "k8s.io/api/core/v1"
)

// Violation represents a policy violation detected during evaluation.
type Violation struct {
	PolicyName string `json:"policy_name"`
	Message    string `json:"message"`
	Severity   string `json:"severity"`
}

// ProcessEvent represents a runtime process event from NATS.
type ProcessEvent struct {
	ClusterID   string        `json:"cluster_id"`
	Timestamp   string        `json:"timestamp"`
	Container   ContainerInfo `json:"container"`
	ProcessName string        `json:"process_name"`
	Executable  string        `json:"executable"`
	Args        []string      `json:"args"`
	PID         int           `json:"pid"`
	ParentPID   int           `json:"parent_pid"`
	UID         int           `json:"uid"`
}

// NetworkEvent represents a runtime network event from NATS.
type NetworkEvent struct {
	ClusterID string        `json:"cluster_id"`
	Timestamp string        `json:"timestamp"`
	Container ContainerInfo `json:"container"`
	SrcIP     string        `json:"src_ip"`
	SrcPort   int           `json:"src_port"`
	DstIP     string        `json:"dst_ip"`
	DstPort   int           `json:"dst_port"`
	Protocol  string        `json:"protocol"`
	EventType string        `json:"event_type"`
}

// ContainerInfo identifies the container associated with an event.
type ContainerInfo struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Image     string `json:"image"`
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
}

// AdmissionPolicy evaluates pod specs at admission time.
type AdmissionPolicy interface {
	Name() string
	Evaluate(podSpec *corev1.PodSpec, namespace string) []Violation
}

// RuntimePolicy evaluates runtime events from NATS.
type RuntimePolicy interface {
	Name() string
	EvaluateProcess(event ProcessEvent) []Violation
	EvaluateNetwork(event NetworkEvent) []Violation
}

// Engine holds registered policies and evaluates them.
type Engine struct {
	admissionPolicies []AdmissionPolicy
	runtimePolicies   []RuntimePolicy
}

// NewEngine creates an Engine with default policies registered.
func NewEngine() *Engine {
	e := &Engine{}
	e.RegisterAdmissionPolicy(&NoPrivilegedContainers{})
	e.RegisterAdmissionPolicy(&NoLatestTag{})
	e.RegisterAdmissionPolicy(&RequireResourceLimits{})
	e.RegisterAdmissionPolicy(&NoHostNetwork{})
	e.RegisterRuntimePolicy(&SuspiciousProcess{})
	e.RegisterRuntimePolicy(&SensitivePortListen{})
	return e
}

// RegisterAdmissionPolicy adds an admission policy to the engine.
func (e *Engine) RegisterAdmissionPolicy(p AdmissionPolicy) {
	e.admissionPolicies = append(e.admissionPolicies, p)
}

// RegisterRuntimePolicy adds a runtime policy to the engine.
func (e *Engine) RegisterRuntimePolicy(p RuntimePolicy) {
	e.runtimePolicies = append(e.runtimePolicies, p)
}

// AlertPublisher publishes policy violation alerts.
type AlertPublisher interface {
	PublishAlert(source string, violation Violation, context map[string]string)
}

// EvaluateAdmission runs all admission policies against a pod spec.
func (e *Engine) EvaluateAdmission(podSpec *corev1.PodSpec, namespace string) []Violation {
	var violations []Violation
	for _, p := range e.admissionPolicies {
		violations = append(violations, p.Evaluate(podSpec, namespace)...)
	}
	return violations
}

// EvaluateProcessEvent runs all runtime policies against a process event.
func (e *Engine) EvaluateProcessEvent(event ProcessEvent) []Violation {
	var violations []Violation
	for _, p := range e.runtimePolicies {
		violations = append(violations, p.EvaluateProcess(event)...)
	}
	return violations
}

// EvaluateNetworkEvent runs all runtime policies against a network event.
func (e *Engine) EvaluateNetworkEvent(event NetworkEvent) []Violation {
	var violations []Violation
	for _, p := range e.runtimePolicies {
		violations = append(violations, p.EvaluateNetwork(event)...)
	}
	return violations
}
