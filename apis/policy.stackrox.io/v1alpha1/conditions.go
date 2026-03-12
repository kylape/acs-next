package v1alpha1

import (
	commonv1 "acs-next.stackrox.io/apis/common/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Condition types for StackroxPolicy and ClusterStackroxPolicy status
const (
	ConditionAcceptedBySensor           = "AcceptedBySensor"
	ConditionAcceptedByAdmissionControl = "AcceptedByAdmissionControl"
)

// Condition reasons for policy acceptance/rejection
const (
	ReasonPolicyLoaded   = "PolicyLoaded"
	ReasonPolicyUpdated  = "PolicyUpdated"
	ReasonPolicyDisabled = "PolicyDisabled"
	ReasonNotApplicable  = "NotApplicable"

	ReasonConversionError = "ConversionError"
	ReasonValidationError = "ValidationError"
	ReasonInvalidCriteria = "InvalidCriteria"
	ReasonInternalError   = "InternalError"
)

// Condition messages
const (
	MessagePolicyLoaded               = "Policy successfully loaded and is being evaluated"
	MessagePolicyUpdated              = "Policy successfully updated and is being evaluated"
	MessagePolicyDisabled             = "Policy is disabled and will not be evaluated"
	MessageRuntimeOnlyNotForAdmission = "Policy has only RUNTIME lifecycle stages, not applicable to admission control"
	MessageDeployOnlyNotForSensor     = "Policy has only DEPLOY lifecycle stages, not applicable to sensor runtime evaluation"
	MessageConversionError            = "Failed to convert policy spec to internal format"
	MessageValidationError            = "Policy validation failed"
	MessageInvalidCriteria            = "Policy criteria are invalid or unsupported"
	MessageInternalError              = "Internal error occurred while processing policy"
)

// NewCondition creates a new condition with the given parameters
func NewCondition(conditionType string, status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// SetCondition adds or updates a condition in the conditions list
func SetCondition(conditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	if conditions == nil {
		conditions = []metav1.Condition{}
	}

	for i, condition := range conditions {
		if condition.Type == newCondition.Type {
			if condition.Status != newCondition.Status {
				newCondition.LastTransitionTime = metav1.Now()
			} else {
				newCondition.LastTransitionTime = condition.LastTransitionTime
			}
			conditions[i] = newCondition
			return conditions
		}
	}

	conditions = append(conditions, newCondition)
	return conditions
}

// GetCondition returns the condition with the given type, or nil if not found
func GetCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

// IsConditionTrue returns true if the condition exists and has status True
func IsConditionTrue(conditions []metav1.Condition, conditionType string) bool {
	condition := GetCondition(conditions, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

// ShouldApplyToAdmissionControl returns true if the policy should be evaluated by admission control
func ShouldApplyToAdmissionControl(lifecycleStages []commonv1.LifecycleStage) bool {
	for _, stage := range lifecycleStages {
		if stage == commonv1.LifecycleStageDeploy {
			return true
		}
	}
	return false
}

// ShouldApplyToSensor returns true if the policy should be evaluated by sensor/runtime evaluator
func ShouldApplyToSensor(lifecycleStages []commonv1.LifecycleStage) bool {
	for _, stage := range lifecycleStages {
		if stage == commonv1.LifecycleStageRuntime {
			return true
		}
	}
	return false
}
