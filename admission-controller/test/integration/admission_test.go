package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"acs-next-admission-controller/internal/policy"
)

func startEmbeddedNATS(t *testing.T) *server.Server {
	t.Helper()
	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		NoLog:     true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		t.Fatalf("failed to create NATS server: %v", err)
	}

	go ns.Start()

	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("NATS server not ready")
	}

	// Create streams
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("failed to get JetStream: %v", err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "PROCESS_EVENTS",
		Subjects: []string{"acs.*.process-events"},
		Storage:  nats.FileStorage,
		MaxAge:   15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("failed to create PROCESS_EVENTS stream: %v", err)
	}

	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "NETWORK_EVENTS",
		Subjects: []string{"acs.*.network-events"},
		Storage:  nats.FileStorage,
		MaxAge:   15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("failed to create NETWORK_EVENTS stream: %v", err)
	}

	return ns
}

func TestWebhookEndToEnd(t *testing.T) {
	engine := policy.NewEngine()
	engine.LoadDefaultPolicies()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		r.Body.Read(body)
		r.Body.Close()

		var review admissionv1.AdmissionReview
		json.Unmarshal(body, &review)

		var pod corev1.Pod
		json.Unmarshal(review.Request.Object.Raw, &pod)

		violations := engine.EvaluateAdmission(&pod.Spec, review.Request.Namespace)

		resp := &admissionv1.AdmissionReview{
			TypeMeta: metav1.TypeMeta{APIVersion: "admission.k8s.io/v1", Kind: "AdmissionReview"},
			Response: &admissionv1.AdmissionResponse{
				UID:     review.Request.UID,
				Allowed: len(violations) == 0,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	ts := httptest.NewServer(handler)
	defer ts.Close()

	// Test: compliant pod allowed
	t.Run("compliant pod allowed", func(t *testing.T) {
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
		review := makeReview(t, pod)
		resp := sendReview(t, ts.URL, review)
		if !resp.Response.Allowed {
			t.Error("expected compliant pod to be allowed")
		}
	})

	// Test: privileged pod denied
	t.Run("privileged pod denied", func(t *testing.T) {
		priv := true
		pod := corev1.Pod{
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{{
					Name:  "bad",
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
		review := makeReview(t, pod)
		resp := sendReview(t, ts.URL, review)
		if resp.Response.Allowed {
			t.Error("expected privileged pod to be denied")
		}
	})
}

func TestRuntimePolicyWithNATS(t *testing.T) {
	ns := startEmbeddedNATS(t)
	defer ns.Shutdown()

	engine := policy.NewEngine()
	engine.LoadDefaultPolicies()

	// Publish a suspicious process event
	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("failed to get JetStream: %v", err)
	}

	event := policy.ProcessEvent{
		ClusterID:   "test-cluster",
		Executable:  "/bin/bash",
		ProcessName: "bash",
		Container: policy.ContainerInfo{
			Namespace: "default",
			Pod:       "test-pod",
			Name:      "app",
		},
	}
	data, _ := json.Marshal(event)
	_, err = js.Publish("acs.test-cluster.process-events", data)
	if err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	// Consume and evaluate
	_, err = js.AddConsumer("PROCESS_EVENTS", &nats.ConsumerConfig{
		Durable:       "test-runtime-consumer",
		DeliverPolicy: nats.DeliverAllPolicy,
		AckPolicy:     nats.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatalf("failed to create consumer: %v", err)
	}

	sub, err := js.PullSubscribe("acs.*.process-events", "test-runtime-consumer")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	msgs, err := sub.Fetch(1, nats.MaxWait(5*time.Second))
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}

	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	var received policy.ProcessEvent
	json.Unmarshal(msgs[0].Data, &received)
	msgs[0].Ack()

	violations := engine.EvaluateProcessEvent(received)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].PolicyName != "Suspicious Process Execution" {
		t.Errorf("expected Suspicious Process Execution policy, got %s", violations[0].PolicyName)
	}
}

func TestNetworkEventRuntimePolicy(t *testing.T) {
	ns := startEmbeddedNATS(t)
	defer ns.Shutdown()

	engine := policy.NewEngine()
	engine.LoadDefaultPolicies()

	nc, err := nats.Connect(ns.ClientURL())
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("failed to get JetStream: %v", err)
	}

	event := policy.NetworkEvent{
		ClusterID: "test-cluster",
		EventType: "ACCEPT",
		DstPort:   22,
		Container: policy.ContainerInfo{
			Namespace: "default",
			Pod:       "test-pod",
		},
	}
	data, _ := json.Marshal(event)
	_, err = js.Publish("acs.test-cluster.network-events", data)
	if err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	_, err = js.AddConsumer("NETWORK_EVENTS", &nats.ConsumerConfig{
		Durable:       "test-network-consumer",
		DeliverPolicy: nats.DeliverAllPolicy,
		AckPolicy:     nats.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatalf("failed to create consumer: %v", err)
	}

	sub, err := js.PullSubscribe("acs.*.network-events", "test-network-consumer")
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	msgs, err := sub.Fetch(1, nats.MaxWait(5*time.Second))
	if err != nil {
		t.Fatalf("failed to fetch: %v", err)
	}

	var received policy.NetworkEvent
	json.Unmarshal(msgs[0].Data, &received)
	msgs[0].Ack()

	violations := engine.EvaluateNetworkEvent(received)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d", len(violations))
	}
	if violations[0].PolicyName != "Sensitive Port Listening" {
		t.Errorf("expected Sensitive Port Listening policy, got %s", violations[0].PolicyName)
	}
}

func makeReview(t *testing.T, pod corev1.Pod) []byte {
	t.Helper()
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

func sendReview(t *testing.T, url string, body []byte) *admissionv1.AdmissionReview {
	t.Helper()
	resp, err := http.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("failed to send review: %v", err)
	}
	defer resp.Body.Close()

	var review admissionv1.AdmissionReview
	json.NewDecoder(resp.Body).Decode(&review)
	return &review
}
