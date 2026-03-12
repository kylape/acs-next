package policy

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	policyv1alpha1 "acs-next.stackrox.io/apis/policy.stackrox.io/v1alpha1"
	commonv1 "acs-next.stackrox.io/apis/common/v1"
	"acs-next.stackrox.io/apis/evaluator"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Violation represents a policy violation detected during evaluation.
type Violation struct {
	PolicyName string `json:"policy_name"`
	Message    string `json:"message"`
	Severity   string `json:"severity"`
}

// AlertPublisher publishes policy violation alerts.
type AlertPublisher interface {
	PublishAlert(source string, violation Violation, context map[string]string)
}

// StatusUpdater updates policy CRD status subresources.
type StatusUpdater interface {
	UpdateClusterPolicyStatus(ctx context.Context, name string, status policyv1alpha1.ClusterStackroxPolicyStatus) error
	UpdateNamespacedPolicyStatus(ctx context.Context, namespace, name string, status policyv1alpha1.StackroxPolicyStatus) error
}

// Engine holds CRD-based policies loaded from informer cache and evaluates them.
type Engine struct {
	mu              sync.RWMutex
	clusterPolicies map[string]*policyv1alpha1.ClusterStackroxPolicy
	nsPolicies      map[string]map[string]*policyv1alpha1.StackroxPolicy // namespace -> name -> policy
	statusUpdater   StatusUpdater
}

// NewEngine creates an empty Engine.
func NewEngine() *Engine {
	return &Engine{
		clusterPolicies: make(map[string]*policyv1alpha1.ClusterStackroxPolicy),
		nsPolicies:      make(map[string]map[string]*policyv1alpha1.StackroxPolicy),
	}
}

// SetStatusUpdater sets the status updater for writing violation data back to CRDs.
func (e *Engine) SetStatusUpdater(u StatusUpdater) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.statusUpdater = u
}

// SetClusterPolicy adds or updates a cluster-scoped policy in the engine.
func (e *Engine) SetClusterPolicy(p *policyv1alpha1.ClusterStackroxPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clusterPolicies[p.Name] = p.DeepCopy()
	log.Printf("Loaded cluster policy: %s (%s)", p.Spec.PolicyName, p.Name)
}

// DeleteClusterPolicy removes a cluster-scoped policy from the engine.
func (e *Engine) DeleteClusterPolicy(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.clusterPolicies, name)
	log.Printf("Removed cluster policy: %s", name)
}

// SetNamespacedPolicy adds or updates a namespace-scoped policy in the engine.
func (e *Engine) SetNamespacedPolicy(p *policyv1alpha1.StackroxPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	ns := p.Namespace
	if e.nsPolicies[ns] == nil {
		e.nsPolicies[ns] = make(map[string]*policyv1alpha1.StackroxPolicy)
	}
	e.nsPolicies[ns][p.Name] = p.DeepCopy()
	log.Printf("Loaded namespaced policy: %s/%s (%s)", ns, p.Name, p.Spec.PolicyName)
}

// DeleteNamespacedPolicy removes a namespace-scoped policy from the engine.
func (e *Engine) DeleteNamespacedPolicy(namespace, name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.nsPolicies[namespace] != nil {
		delete(e.nsPolicies[namespace], name)
	}
	log.Printf("Removed namespaced policy: %s/%s", namespace, name)
}

// EvaluateAdmission runs all DEPLOY-stage policies against a pod spec.
func (e *Engine) EvaluateAdmission(podSpec *corev1.PodSpec, namespace string) []Violation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ctx := &evaluator.EvalContext{
		PodSpec:   podSpec,
		Namespace: namespace,
	}

	var violations []Violation

	// Evaluate cluster-scoped policies
	for _, p := range e.clusterPolicies {
		if p.Spec.Disabled {
			continue
		}
		if !policyv1alpha1.ShouldApplyToAdmissionControl(p.Spec.LifecycleStages) {
			continue
		}
		result := evaluator.EvaluateSections(p.Spec.PolicySections, ctx)
		if result.Matched {
			msg := fmt.Sprintf("%s: %s", p.Spec.PolicyName, strings.Join(result.Messages, "; "))
			violations = append(violations, Violation{
				PolicyName: p.Spec.PolicyName,
				Message:    msg,
				Severity:   severityToLower(p.Spec.Severity),
			})
		}
	}

	// Evaluate namespace-scoped policies for this namespace
	if nsPolicies, ok := e.nsPolicies[namespace]; ok {
		for _, p := range nsPolicies {
			if p.Spec.Disabled {
				continue
			}
			if !policyv1alpha1.ShouldApplyToAdmissionControl(p.Spec.LifecycleStages) {
				continue
			}
			result := evaluator.EvaluateSections(p.Spec.PolicySections, ctx)
			if result.Matched {
				msg := fmt.Sprintf("%s: %s", p.Spec.PolicyName, strings.Join(result.Messages, "; "))
				violations = append(violations, Violation{
					PolicyName: p.Spec.PolicyName,
					Message:    msg,
					Severity:   severityToLower(p.Spec.Severity),
				})
			}
		}
	}

	return violations
}

// UpdatePolicyStatuses writes violation data back to the CRD status subresource.
func (e *Engine) UpdatePolicyStatuses(ctx context.Context, violations []Violation, resource policyv1alpha1.ViolationResource) {
	e.mu.RLock()
	updater := e.statusUpdater
	e.mu.RUnlock()

	if updater == nil {
		return
	}

	now := metav1.Now()

	// Build a set of violated policy names
	violatedPolicies := make(map[string]Violation)
	for _, v := range violations {
		violatedPolicies[v.PolicyName] = v
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, p := range e.clusterPolicies {
		v, matched := violatedPolicies[p.Spec.PolicyName]
		if !matched {
			continue
		}

		status := p.Status.DeepCopy()
		status.LastEvaluated = &now
		status.LocalPolicyID = policyID(p.Spec.PolicyName)

		// Set accepted condition
		status.Conditions = policyv1alpha1.SetCondition(status.Conditions,
			policyv1alpha1.NewCondition(policyv1alpha1.ConditionAcceptedByAdmissionControl,
				metav1.ConditionTrue, policyv1alpha1.ReasonPolicyLoaded, policyv1alpha1.MessagePolicyLoaded))

		if status.ViolationMetrics == nil {
			status.ViolationMetrics = &policyv1alpha1.ClusterScopedViolationMetrics{}
		}
		status.ViolationMetrics.TotalViolations++
		status.ViolationMetrics.ActiveViolations++
		status.ViolationMetrics.LastViolationTime = &now

		enforcementAction := ""
		if len(p.Spec.EnforcementActions) > 0 {
			enforcementAction = string(p.Spec.EnforcementActions[0])
		}

		status.RecentViolations = policyv1alpha1.AppendRecentViolation(status.RecentViolations,
			policyv1alpha1.RecentViolation{
				Resource:          resource,
				Message:           v.Message,
				Source:            "admission",
				Timestamp:         now,
				EnforcementAction: enforcementAction,
			})

		if err := updater.UpdateClusterPolicyStatus(ctx, p.Name, *status); err != nil {
			log.Printf("Failed to update cluster policy status %s: %v", p.Name, err)
		}
	}
}

// PolicyCount returns the number of loaded policies.
func (e *Engine) PolicyCount() (cluster, namespaced int) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	cluster = len(e.clusterPolicies)
	for _, nsp := range e.nsPolicies {
		namespaced += len(nsp)
	}
	return
}

func severityToLower(severity string) string {
	switch severity {
	case "CRITICAL_SEVERITY":
		return "critical"
	case "HIGH_SEVERITY":
		return "high"
	case "MEDIUM_SEVERITY":
		return "medium"
	case "LOW_SEVERITY":
		return "low"
	default:
		return "unset"
	}
}

func policyID(policyName string) string {
	h := sha256.Sum256([]byte(policyName))
	return fmt.Sprintf("%x", h[:16])
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

// EvaluateProcessEvent runs all RUNTIME policies against a process event.
func (e *Engine) EvaluateProcessEvent(event ProcessEvent) []Violation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	ctx := &evaluator.EvalContext{
		ProcessName: event.ProcessName,
		Executable:  event.Executable,
	}

	var violations []Violation
	for _, p := range e.clusterPolicies {
		if p.Spec.Disabled {
			continue
		}
		if !policyv1alpha1.ShouldApplyToSensor(p.Spec.LifecycleStages) {
			continue
		}
		result := evaluator.EvaluateSections(p.Spec.PolicySections, ctx)
		if result.Matched {
			msg := fmt.Sprintf("%s in container %s/%s",
				p.Spec.PolicyName, event.Container.Namespace, event.Container.Pod)
			violations = append(violations, Violation{
				PolicyName: p.Spec.PolicyName,
				Message:    msg,
				Severity:   severityToLower(p.Spec.Severity),
			})
		}
	}

	return violations
}

// EvaluateNetworkEvent runs all RUNTIME policies against a network event.
func (e *Engine) EvaluateNetworkEvent(event NetworkEvent) []Violation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if event.EventType != "ACCEPT" {
		return nil
	}

	ctx := &evaluator.EvalContext{
		DstPort:   event.DstPort,
		EventType: event.EventType,
	}

	var violations []Violation
	for _, p := range e.clusterPolicies {
		if p.Spec.Disabled {
			continue
		}
		if !policyv1alpha1.ShouldApplyToSensor(p.Spec.LifecycleStages) {
			continue
		}
		result := evaluator.EvaluateSections(p.Spec.PolicySections, ctx)
		if result.Matched {
			msg := fmt.Sprintf("%s on port %d in container %s/%s",
				p.Spec.PolicyName, event.DstPort, event.Container.Namespace, event.Container.Pod)
			violations = append(violations, Violation{
				PolicyName: p.Spec.PolicyName,
				Message:    msg,
				Severity:   severityToLower(p.Spec.Severity),
			})
		}
	}

	return violations
}

// LoadDefaultPolicies loads hardcoded defaults when no CRDs are available.
// This provides backward compatibility during the transition.
func (e *Engine) LoadDefaultPolicies() {
	defaults := []policyv1alpha1.ClusterStackroxPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "no-privileged-containers"},
			Spec: policyv1alpha1.ClusterStackroxPolicySpec{
				PolicyName: "No Privileged Containers", Severity: "CRITICAL_SEVERITY",
				Categories: []string{"Security Best Practices"},
				LifecycleStages: []commonv1.LifecycleStage{commonv1.LifecycleStageDeploy},
				EnforcementActions: []commonv1.EnforcementAction{commonv1.EnforcementActionFailKubeRequest},
				PolicySections: []commonv1.PolicySection{{
					PolicyGroups: []commonv1.PolicyGroup{{FieldName: "Privileged Container"}},
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "no-latest-tag"},
			Spec: policyv1alpha1.ClusterStackroxPolicySpec{
				PolicyName: "No Latest Image Tag", Severity: "HIGH_SEVERITY",
				Categories: []string{"DevOps Best Practices"},
				LifecycleStages: []commonv1.LifecycleStage{commonv1.LifecycleStageDeploy},
				EnforcementActions: []commonv1.EnforcementAction{commonv1.EnforcementActionFailKubeRequest},
				PolicySections: []commonv1.PolicySection{{
					PolicyGroups: []commonv1.PolicyGroup{{FieldName: "Image Tag", Values: []commonv1.PolicyValue{{Value: "latest"}}}},
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "require-resource-limits"},
			Spec: policyv1alpha1.ClusterStackroxPolicySpec{
				PolicyName: "Require Resource Limits", Severity: "MEDIUM_SEVERITY",
				Categories: []string{"DevOps Best Practices"},
				LifecycleStages: []commonv1.LifecycleStage{commonv1.LifecycleStageDeploy},
				EnforcementActions: []commonv1.EnforcementAction{commonv1.EnforcementActionFailKubeRequest},
				PolicySections: []commonv1.PolicySection{
					{PolicyGroups: []commonv1.PolicyGroup{{FieldName: "Container CPU Limit"}}},
					{PolicyGroups: []commonv1.PolicyGroup{{FieldName: "Container Memory Limit"}}},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "no-host-network"},
			Spec: policyv1alpha1.ClusterStackroxPolicySpec{
				PolicyName: "No Host Network", Severity: "HIGH_SEVERITY",
				Categories: []string{"Security Best Practices"},
				LifecycleStages: []commonv1.LifecycleStage{commonv1.LifecycleStageDeploy},
				EnforcementActions: []commonv1.EnforcementAction{commonv1.EnforcementActionFailKubeRequest},
				PolicySections: []commonv1.PolicySection{{
					PolicyGroups: []commonv1.PolicyGroup{{FieldName: "Host Network"}},
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "suspicious-process"},
			Spec: policyv1alpha1.ClusterStackroxPolicySpec{
				PolicyName: "Suspicious Process Execution", Severity: "HIGH_SEVERITY",
				Categories: []string{"Anomalous Activity"},
				LifecycleStages: []commonv1.LifecycleStage{commonv1.LifecycleStageRuntime},
				PolicySections: []commonv1.PolicySection{{
					PolicyGroups: []commonv1.PolicyGroup{{
						FieldName: "Process Name",
						Values: []commonv1.PolicyValue{
							{Value: "/bin/bash"}, {Value: "/bin/sh"}, {Value: "/bin/nc"},
							{Value: "/usr/bin/curl"}, {Value: "/usr/bin/wget"},
						},
					}},
				}},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "sensitive-port-listen"},
			Spec: policyv1alpha1.ClusterStackroxPolicySpec{
				PolicyName: "Sensitive Port Listening", Severity: "MEDIUM_SEVERITY",
				Categories: []string{"Anomalous Activity"},
				LifecycleStages: []commonv1.LifecycleStage{commonv1.LifecycleStageRuntime},
				PolicySections: []commonv1.PolicySection{{
					PolicyGroups: []commonv1.PolicyGroup{{
						FieldName: "Port",
						Values: []commonv1.PolicyValue{
							{Value: "22"}, {Value: "3389"}, {Value: "4444"},
						},
					}},
				}},
			},
		},
	}

	for i := range defaults {
		e.SetClusterPolicy(&defaults[i])
	}
	log.Printf("Loaded %d default policies", len(defaults))
}

// Ignore is a helper to suppress unused import warnings for json and time.
var _ = json.Marshal
var _ = time.Now
