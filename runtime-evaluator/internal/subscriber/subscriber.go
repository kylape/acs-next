package subscriber

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	commonv1 "acs-next.stackrox.io/apis/common/v1"
	"acs-next.stackrox.io/apis/evaluator"
	policyv1alpha1 "acs-next.stackrox.io/apis/policy.stackrox.io/v1alpha1"

	"github.com/nats-io/nats.go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Config holds subscriber configuration.
type Config struct {
	NATSURL string
	TLSCert string
	TLSKey  string
	TLSCA   string
}

// ListOptions returns default list options.
func ListOptions() metav1.ListOptions {
	return metav1.ListOptions{}
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

// Engine holds CRD-based RUNTIME policies and evaluates them.
type Engine struct {
	mu              sync.RWMutex
	clusterPolicies map[string]*policyv1alpha1.ClusterStackroxPolicy
}

// NewEngine creates an empty Engine.
func NewEngine() *Engine {
	return &Engine{
		clusterPolicies: make(map[string]*policyv1alpha1.ClusterStackroxPolicy),
	}
}

// SetClusterPolicy adds or updates a cluster-scoped policy.
func (e *Engine) SetClusterPolicy(p *policyv1alpha1.ClusterStackroxPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.clusterPolicies[p.Name] = p.DeepCopy()
	log.Printf("Loaded cluster policy: %s (%s)", p.Spec.PolicyName, p.Name)
}

// DeleteClusterPolicy removes a cluster-scoped policy.
func (e *Engine) DeleteClusterPolicy(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.clusterPolicies, name)
	log.Printf("Removed cluster policy: %s", name)
}

// PolicyCount returns the number of loaded policies.
func (e *Engine) PolicyCount() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.clusterPolicies)
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
		if p.Spec.Disabled || !policyv1alpha1.ShouldApplyToSensor(p.Spec.LifecycleStages) {
			continue
		}
		result := evaluator.EvaluateSections(p.Spec.PolicySections, ctx)
		if result.Matched {
			violations = append(violations, Violation{
				PolicyName: p.Spec.PolicyName,
				Message: fmt.Sprintf("%s in container %s/%s",
					p.Spec.PolicyName, event.Container.Namespace, event.Container.Pod),
				Severity: severityToLower(p.Spec.Severity),
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
		if p.Spec.Disabled || !policyv1alpha1.ShouldApplyToSensor(p.Spec.LifecycleStages) {
			continue
		}
		result := evaluator.EvaluateSections(p.Spec.PolicySections, ctx)
		if result.Matched {
			violations = append(violations, Violation{
				PolicyName: p.Spec.PolicyName,
				Message: fmt.Sprintf("%s on port %d in container %s/%s",
					p.Spec.PolicyName, event.DstPort, event.Container.Namespace, event.Container.Pod),
				Severity: severityToLower(p.Spec.Severity),
			})
		}
	}
	return violations
}

// LoadDefaultPolicies loads hardcoded defaults when CRDs aren't available.
func (e *Engine) LoadDefaultPolicies() {
	defaults := []policyv1alpha1.ClusterStackroxPolicy{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "suspicious-process"},
			Spec: policyv1alpha1.ClusterStackroxPolicySpec{
				PolicyName: "Suspicious Process Execution", Severity: "HIGH_SEVERITY",
				Categories:      []string{"Anomalous Activity"},
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
				Categories:      []string{"Anomalous Activity"},
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
	log.Printf("Loaded %d default runtime policies", len(defaults))
}

// Violation represents a policy violation.
type Violation struct {
	PolicyName string `json:"policy_name"`
	Message    string `json:"message"`
	Severity   string `json:"severity"`
}

// Subscriber connects to NATS and processes runtime events.
type Subscriber struct {
	nc     *nats.Conn
	js     nats.JetStreamContext
	engine *Engine
}

// New creates a Subscriber connected to NATS.
func New(cfg Config, engine *Engine) (*Subscriber, error) {
	opts := []nats.Option{
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2 * time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			log.Printf("NATS disconnected: %v", err)
		}),
		nats.ReconnectHandler(func(_ *nats.Conn) {
			log.Println("NATS reconnected")
		}),
	}

	if cfg.TLSCert != "" && cfg.TLSKey != "" && cfg.TLSCA != "" {
		tlsCfg, err := buildTLSConfig(cfg.TLSCert, cfg.TLSKey, cfg.TLSCA)
		if err != nil {
			return nil, fmt.Errorf("build TLS config: %w", err)
		}
		opts = append(opts, nats.Secure(tlsCfg))
	}

	nc, err := nats.Connect(cfg.NATSURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("get JetStream context: %w", err)
	}

	return &Subscriber{nc: nc, js: js, engine: engine}, nil
}

// Start begins consuming events from both streams.
func (s *Subscriber) Start(ctx context.Context) error {
	if err := s.startConsumer(ctx, "PROCESS_EVENTS", "acs.*.process-events",
		"runtime-process-events", s.handleProcessMessage); err != nil {
		return fmt.Errorf("start process consumer: %w", err)
	}

	if err := s.startConsumer(ctx, "NETWORK_EVENTS", "acs.*.network-events",
		"runtime-network-events", s.handleNetworkMessage); err != nil {
		return fmt.Errorf("start network consumer: %w", err)
	}

	log.Println("Runtime evaluator subscriber started")
	return nil
}

// Shutdown closes the NATS connection.
func (s *Subscriber) Shutdown() {
	if s.nc != nil {
		s.nc.Close()
	}
}

func (s *Subscriber) startConsumer(ctx context.Context, stream, subject, durableName string,
	handler func(*nats.Msg)) error {
	_, err := s.js.AddConsumer(stream, &nats.ConsumerConfig{
		Durable:       durableName,
		FilterSubject: subject,
		DeliverPolicy: nats.DeliverNewPolicy,
		AckPolicy:     nats.AckExplicitPolicy,
	})
	if err != nil {
		return fmt.Errorf("create consumer %s: %w", durableName, err)
	}

	sub, err := s.js.PullSubscribe(subject, durableName)
	if err != nil {
		return fmt.Errorf("subscribe %s: %w", durableName, err)
	}

	go func() {
		log.Printf("Consumer %s started on %s", durableName, subject)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			msgs, err := sub.Fetch(10, nats.MaxWait(5*time.Second))
			if err != nil {
				if err == nats.ErrTimeout {
					continue
				}
				if ctx.Err() != nil {
					return
				}
				log.Printf("Fetch error on %s: %v", durableName, err)
				continue
			}

			for _, msg := range msgs {
				handler(msg)
				msg.Ack()
			}
		}
	}()

	return nil
}

func (s *Subscriber) handleProcessMessage(msg *nats.Msg) {
	var event ProcessEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal process event: %v", err)
		return
	}

	violations := s.engine.EvaluateProcessEvent(event)
	for _, v := range violations {
		s.publishAlert(v, event.Container)
	}
}

func (s *Subscriber) handleNetworkMessage(msg *nats.Msg) {
	var event NetworkEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal network event: %v", err)
		return
	}

	violations := s.engine.EvaluateNetworkEvent(event)
	for _, v := range violations {
		s.publishAlert(v, event.Container)
	}
}

func (s *Subscriber) publishAlert(v Violation, container ContainerInfo) {
	violationJSON, _ := json.Marshal(map[string]interface{}{
		"type":      "runtime_violation",
		"policy":    v.PolicyName,
		"severity":  v.Severity,
		"message":   v.Message,
		"namespace": container.Namespace,
		"pod":       container.Pod,
		"container": container.Name,
		"image":     container.Image,
	})
	log.Printf("VIOLATION: %s", violationJSON)

	alert := map[string]interface{}{
		"source":   "runtime",
		"policy":   v.PolicyName,
		"severity": v.Severity,
		"message":  v.Message,
		"context": map[string]string{
			"namespace": container.Namespace,
			"pod":       container.Pod,
			"container": container.Name,
			"image":     container.Image,
		},
	}
	data, _ := json.Marshal(alert)
	if err := s.nc.Publish("acs.alerts", data); err != nil {
		log.Printf("Failed to publish alert: %v", err)
	}
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

func buildTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load key pair: %w", err)
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read CA file: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}
