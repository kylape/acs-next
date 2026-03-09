package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	"acs-next-admission-controller/internal/policy"
)

func makeAdmissionReview(pod corev1.Pod) []byte {
	raw, _ := json.Marshal(pod)
	review := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"},
		Request: &admissionv1.AdmissionRequest{
			UID:       "test-uid",
			Kind:      metav1.GroupVersionKind{Kind: "Pod"},
			Namespace: "default",
			Name:      "test-pod",
			Object:    runtime.RawExtension{Raw: raw},
		},
	}
	body, _ := json.Marshal(review)
	return body
}

func TestValidateAllowed(t *testing.T) {
	engine := policy.NewEngine()
	s := &Server{engine: engine, alertPub: nil}

	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "app",
				Image: "nginx:1.25",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			}},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(makeAdmissionReview(pod)))
	w := httptest.NewRecorder()
	s.handleValidate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var review admissionv1.AdmissionReview
	json.Unmarshal(w.Body.Bytes(), &review)
	if !review.Response.Allowed {
		t.Errorf("expected allowed, got denied: %s", review.Response.Result.Message)
	}
}

func TestValidatePrivilegedRejected(t *testing.T) {
	engine := policy.NewEngine()
	s := &Server{engine: engine, alertPub: nil}

	priv := true
	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "app",
				Image: "nginx:1.25",
				SecurityContext: &corev1.SecurityContext{
					Privileged: &priv,
				},
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			}},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(makeAdmissionReview(pod)))
	w := httptest.NewRecorder()
	s.handleValidate(w, req)

	var review admissionv1.AdmissionReview
	json.Unmarshal(w.Body.Bytes(), &review)
	if review.Response.Allowed {
		t.Error("expected denied for privileged container")
	}
}

func TestValidateLatestTagRejected(t *testing.T) {
	engine := policy.NewEngine()
	s := &Server{engine: engine, alertPub: nil}

	pod := corev1.Pod{
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "app",
				Image: "nginx",
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("100m"),
						corev1.ResourceMemory: resource.MustParse("128Mi"),
					},
				},
			}},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader(makeAdmissionReview(pod)))
	w := httptest.NewRecorder()
	s.handleValidate(w, req)

	var review admissionv1.AdmissionReview
	json.Unmarshal(w.Body.Bytes(), &review)
	if review.Response.Allowed {
		t.Error("expected denied for image with no tag")
	}
}

func TestValidateBadRequest(t *testing.T) {
	engine := policy.NewEngine()
	s := &Server{engine: engine, alertPub: nil}

	req := httptest.NewRequest(http.MethodPost, "/validate", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	s.handleValidate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestValidateMethodNotAllowed(t *testing.T) {
	engine := policy.NewEngine()
	s := &Server{engine: engine, alertPub: nil}

	req := httptest.NewRequest(http.MethodGet, "/validate", nil)
	w := httptest.NewRecorder()
	s.handleValidate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}
