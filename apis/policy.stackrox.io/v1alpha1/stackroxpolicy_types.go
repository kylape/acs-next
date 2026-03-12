package v1alpha1

import (
	commonv1 "acs-next.stackrox.io/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StackroxPolicySpec defines the desired state of StackroxPolicy
type StackroxPolicySpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[^\n\r\$]{5,128}$`
	PolicyName string `json:"policyName"`

	// +kubebuilder:validation:Pattern=`^[^\$]{0,800}$`
	// +optional
	Description string `json:"description,omitempty"`

	// +optional
	Rationale string `json:"rationale,omitempty"`

	// +optional
	Remediation string `json:"remediation,omitempty"`

	// +optional
	Disabled bool `json:"disabled,omitempty"`

	// +kubebuilder:validation:MinItems=1
	Categories []string `json:"categories"`

	// +kubebuilder:validation:MinItems=1
	LifecycleStages []commonv1.LifecycleStage `json:"lifecycleStages"`

	// +optional
	EventSource commonv1.EventSource `json:"eventSource,omitempty"`

	// +optional
	Exclusions []commonv1.Exclusion `json:"exclusions,omitempty"`

	// +optional
	Scope []commonv1.NamespaceScopedScope `json:"scope,omitempty"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=UNSET_SEVERITY;LOW_SEVERITY;MEDIUM_SEVERITY;HIGH_SEVERITY;CRITICAL_SEVERITY
	Severity string `json:"severity"`

	// +optional
	EnforcementActions []commonv1.EnforcementAction `json:"enforcementActions,omitempty"`

	// +optional
	Notifiers []string `json:"notifiers,omitempty"`

	// +kubebuilder:validation:MinItems=1
	PolicySections []commonv1.PolicySection `json:"policySections"`

	// +optional
	MitreAttackVectors []commonv1.MitreAttackVectors `json:"mitreAttackVectors,omitempty"`
}

// NamespaceScopedViolationMetrics tracks violations for namespace-scoped policies
type NamespaceScopedViolationMetrics struct {
	// +optional
	TotalViolations int32 `json:"totalViolations,omitempty"`

	// +optional
	ActiveViolations int32 `json:"activeViolations,omitempty"`

	// +optional
	LastViolationTime *metav1.Time `json:"lastViolationTime,omitempty"`
}

// StackroxPolicyStatus defines the observed state of StackroxPolicy
type StackroxPolicyStatus struct {
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// +optional
	LocalPolicyID string `json:"localPolicyId,omitempty"`

	// +optional
	LastEvaluated *metav1.Time `json:"lastEvaluated,omitempty"`

	// +optional
	ViolationMetrics *NamespaceScopedViolationMetrics `json:"violationMetrics,omitempty"`

	// +kubebuilder:validation:MaxItems=20
	// +optional
	RecentViolations []RecentViolation `json:"recentViolations,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=srxp;sp
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Policy Name",type=string,JSONPath=`.spec.policyName`
// +kubebuilder:printcolumn:name="Severity",type=string,JSONPath=`.spec.severity`
// +kubebuilder:printcolumn:name="Accepted",type=string,JSONPath=`.status.conditions[?(@.type=="AcceptedByAdmissionControl")].status`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// StackroxPolicy is the schema for namespace-scoped policies in secured clusters
type StackroxPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StackroxPolicySpec   `json:"spec,omitempty"`
	Status StackroxPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StackroxPolicyList contains a list of StackroxPolicy
type StackroxPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StackroxPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StackroxPolicy{}, &StackroxPolicyList{})
}
