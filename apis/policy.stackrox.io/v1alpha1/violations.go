package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RecentViolation records a single policy violation event
type RecentViolation struct {
	Resource          ViolationResource `json:"resource"`
	Message           string            `json:"message"`
	Source            string            `json:"source"` // "admission" or "runtime"
	Timestamp         metav1.Time       `json:"timestamp"`
	EnforcementAction string            `json:"enforcementAction,omitempty"`
}

// ViolationResource identifies the resource that violated a policy
type ViolationResource struct {
	Kind          string `json:"kind"`
	Name          string `json:"name"`
	Namespace     string `json:"namespace,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	ImageName     string `json:"imageName,omitempty"`
}

const maxRecentViolations = 20

// AppendRecentViolation adds a violation to the list, keeping at most 20 entries.
// Oldest violations are dropped when the limit is reached.
func AppendRecentViolation(violations []RecentViolation, v RecentViolation) []RecentViolation {
	violations = append(violations, v)
	if len(violations) > maxRecentViolations {
		violations = violations[len(violations)-maxRecentViolations:]
	}
	return violations
}
