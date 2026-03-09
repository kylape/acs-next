package webhook

import (
	"encoding/json"
	"fmt"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"acs-next-admission-controller/internal/policy"
)

// extractPodSpec extracts a PodSpec from the raw object in the admission request.
func extractPodSpec(req *admissionv1.AdmissionRequest) (*corev1.PodSpec, error) {
	switch req.Kind.Kind {
	case "Pod":
		var pod corev1.Pod
		if err := json.Unmarshal(req.Object.Raw, &pod); err != nil {
			return nil, fmt.Errorf("unmarshal Pod: %w", err)
		}
		return &pod.Spec, nil

	case "Deployment":
		var deploy appsv1.Deployment
		if err := json.Unmarshal(req.Object.Raw, &deploy); err != nil {
			return nil, fmt.Errorf("unmarshal Deployment: %w", err)
		}
		return &deploy.Spec.Template.Spec, nil

	case "ReplicaSet":
		var rs appsv1.ReplicaSet
		if err := json.Unmarshal(req.Object.Raw, &rs); err != nil {
			return nil, fmt.Errorf("unmarshal ReplicaSet: %w", err)
		}
		return &rs.Spec.Template.Spec, nil

	default:
		return nil, fmt.Errorf("unsupported kind: %s", req.Kind.Kind)
	}
}

// buildResponse creates an AdmissionReview response from policy violations.
func buildResponse(uid string, violations []policy.Violation) *admissionv1.AdmissionReview {
	review := &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Response: &admissionv1.AdmissionResponse{
			UID: types.UID(uid),
		},
	}

	if len(violations) == 0 {
		review.Response.Allowed = true
		return review
	}

	review.Response.Allowed = false
	var msgs []string
	for _, v := range violations {
		msgs = append(msgs, fmt.Sprintf("[%s] %s: %s", v.Severity, v.PolicyName, v.Message))
	}
	review.Response.Result = &metav1.Status{
		Message: strings.Join(msgs, "; "),
	}
	return review
}
