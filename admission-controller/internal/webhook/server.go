package webhook

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"

	"acs-next-admission-controller/internal/policy"
)

// Config holds webhook server configuration.
type Config struct {
	WebhookPort int
	HealthPort  int
	TLSCert     string
	TLSKey      string
}

// Server is the admission webhook HTTPS server.
type Server struct {
	webhookServer *http.Server
	healthServer  *http.Server
	engine        *policy.Engine
	alertPub      policy.AlertPublisher
}

// New creates a new webhook Server.
func New(cfg Config, engine *policy.Engine, alertPub policy.AlertPublisher) (*Server, error) {
	cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
	if err != nil {
		return nil, fmt.Errorf("load TLS cert: %w", err)
	}

	mux := http.NewServeMux()
	s := &Server{engine: engine, alertPub: alertPub}
	mux.HandleFunc("/validate", s.handleValidate)

	s.webhookServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.WebhookPort),
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		},
	}

	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	s.healthServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HealthPort),
		Handler: healthMux,
	}

	return s, nil
}

// Start begins serving both the webhook and health endpoints.
func (s *Server) Start() error {
	go func() {
		log.Printf("Health server listening on %s", s.healthServer.Addr)
		if err := s.healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Health server error: %v", err)
		}
	}()

	go func() {
		log.Printf("Webhook server listening on %s", s.webhookServer.Addr)
		if err := s.webhookServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			log.Printf("Webhook server error: %v", err)
		}
	}()

	return nil
}

// Shutdown gracefully stops both servers.
func (s *Server) Shutdown(ctx context.Context) {
	s.webhookServer.Shutdown(ctx)
	s.healthServer.Shutdown(ctx)
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var review admissionv1.AdmissionReview
	if err := json.Unmarshal(body, &review); err != nil {
		http.Error(w, "invalid AdmissionReview", http.StatusBadRequest)
		return
	}

	if review.Request == nil {
		http.Error(w, "missing request in AdmissionReview", http.StatusBadRequest)
		return
	}

	podSpec, err := extractPodSpec(review.Request)
	if err != nil {
		log.Printf("Failed to extract pod spec: %v", err)
		// Allow on extraction failure — don't block unknown resources
		resp := buildResponse(string(review.Request.UID), nil)
		writeJSON(w, resp)
		return
	}

	namespace := review.Request.Namespace
	violations := s.engine.EvaluateAdmission(podSpec, namespace)

	if len(violations) > 0 {
		log.Printf("Admission denied for %s/%s: %d violations",
			namespace, review.Request.Name, len(violations))
		if s.alertPub != nil {
			for _, v := range violations {
				s.alertPub.PublishAlert("admission", v, map[string]string{
					"namespace": namespace,
					"name":      review.Request.Name,
					"kind":      review.Request.Kind.Kind,
				})
			}
		}
	}

	resp := buildResponse(string(review.Request.UID), violations)
	writeJSON(w, resp)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("Failed to write response: %v", err)
	}
}
