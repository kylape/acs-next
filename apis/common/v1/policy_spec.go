package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LifecycleStage defines the stage in the lifecycle where a policy applies
// +kubebuilder:validation:Enum=DEPLOY;RUNTIME
type LifecycleStage string

const (
	LifecycleStageDeploy  LifecycleStage = "DEPLOY"
	LifecycleStageRuntime LifecycleStage = "RUNTIME"
)

// EventSource defines which events should trigger policy execution
// +kubebuilder:validation:Enum=NOT_APPLICABLE;DEPLOYMENT_EVENT;AUDIT_LOG_EVENT;NODE_EVENT
type EventSource string

const (
	EventSourceNotApplicable   EventSource = "NOT_APPLICABLE"
	EventSourceDeploymentEvent EventSource = "DEPLOYMENT_EVENT"
	EventSourceAuditLogEvent   EventSource = "AUDIT_LOG_EVENT"
	EventSourceNodeEvent       EventSource = "NODE_EVENT"
)

// EnforcementAction defines enforcement actions for policy violations
// +kubebuilder:validation:Enum=UNSET_ENFORCEMENT;SCALE_TO_ZERO_ENFORCEMENT;UNSATISFIABLE_NODE_CONSTRAINT_ENFORCEMENT;KILL_POD_ENFORCEMENT;FAIL_KUBE_REQUEST_ENFORCEMENT;FAIL_DEPLOYMENT_CREATE_ENFORCEMENT;FAIL_DEPLOYMENT_UPDATE_ENFORCEMENT
type EnforcementAction string

const (
	EnforcementActionUnset                       EnforcementAction = "UNSET_ENFORCEMENT"
	EnforcementActionScaleToZero                 EnforcementAction = "SCALE_TO_ZERO_ENFORCEMENT"
	EnforcementActionUnsatisfiableNodeConstraint EnforcementAction = "UNSATISFIABLE_NODE_CONSTRAINT_ENFORCEMENT"
	EnforcementActionKillPod                     EnforcementAction = "KILL_POD_ENFORCEMENT"
	EnforcementActionFailKubeRequest             EnforcementAction = "FAIL_KUBE_REQUEST_ENFORCEMENT"
	EnforcementActionFailDeploymentCreate        EnforcementAction = "FAIL_DEPLOYMENT_CREATE_ENFORCEMENT"
	EnforcementActionFailDeploymentUpdate        EnforcementAction = "FAIL_DEPLOYMENT_UPDATE_ENFORCEMENT"
)

// NamespaceScopedScope defines scope for namespace-scoped policies
type NamespaceScopedScope struct {
	// WorkloadSelector is a Kubernetes label selector for workloads
	// +optional
	WorkloadSelector *metav1.LabelSelector `json:"workloadSelector,omitempty"`
}

// Scope defines scope for cluster-scoped policies
type Scope struct {
	// Namespace is a direct namespace name to target
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// NamespaceSelector is a Kubernetes label selector for namespaces
	// +optional
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`

	// WorkloadSelector is a Kubernetes label selector for workloads
	// +optional
	WorkloadSelector *metav1.LabelSelector `json:"workloadSelector,omitempty"`
}

// Exclusion defines what should be excluded from policy
type Exclusion struct {
	// Name is a descriptive name for this exclusion
	// +optional
	Name string `json:"name,omitempty"`

	// Deployment specifies deployment-based exclusions
	// +optional
	Deployment *ExclusionDeployment `json:"deployment,omitempty"`

	// Image specifies image-based exclusions
	// +optional
	Image *ExclusionImage `json:"image,omitempty"`

	// Expiration is when this exclusion expires
	// +optional
	Expiration *metav1.Time `json:"expiration,omitempty"`

	// WorkloadSelector excludes workloads matching this label selector
	// +optional
	WorkloadSelector *metav1.LabelSelector `json:"workloadSelector,omitempty"`
}

// ExclusionDeployment defines deployment-based exclusions
type ExclusionDeployment struct {
	// Name is the deployment name to exclude
	// +optional
	Name string `json:"name,omitempty"`

	// Scope narrows the exclusion to specific namespaces
	// +optional
	Scope *Scope `json:"scope,omitempty"`
}

// ExclusionImage defines image-based exclusions
type ExclusionImage struct {
	// Name is the image name to exclude (supports wildcards)
	// +optional
	Name string `json:"name,omitempty"`
}

// PolicySection defines a section of policy criteria
type PolicySection struct {
	// SectionName is a user-friendly name for this section
	// +optional
	SectionName string `json:"sectionName,omitempty"`

	// PolicyGroups is the set of policy groups that make up this section
	PolicyGroups []PolicyGroup `json:"policyGroups"`
}

// BooleanOperator defines how policy values are combined
// +kubebuilder:validation:Enum=OR;AND
type BooleanOperator string

const (
	BooleanOperatorOr  BooleanOperator = "OR"
	BooleanOperatorAnd BooleanOperator = "AND"
)

// PolicyGroup defines a group of policy criteria
type PolicyGroup struct {
	// FieldName defines which field on a deployment or image this PolicyGroup evaluates
	FieldName string `json:"fieldName"`

	// BooleanOperator determines if the values are combined with OR or AND
	// +optional
	BooleanOperator BooleanOperator `json:"booleanOperator,omitempty"`

	// Negate determines if the evaluation of this PolicyGroup is negated
	// +optional
	Negate bool `json:"negate,omitempty"`

	// Values is the list of values for the specified field
	// +optional
	Values []PolicyValue `json:"values,omitempty"`
}

// PolicyValue defines a single policy value
type PolicyValue struct {
	// Value is the string value
	Value string `json:"value,omitempty"`
}

// MitreAttackVectors defines MITRE ATT&CK framework mapping
type MitreAttackVectors struct {
	// Tactic is the MITRE ATT&CK tactic
	// +optional
	Tactic string `json:"tactic,omitempty"`

	// Techniques are the MITRE ATT&CK techniques
	// +optional
	Techniques []string `json:"techniques,omitempty"`
}
