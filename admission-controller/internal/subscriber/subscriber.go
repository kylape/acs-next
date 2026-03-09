package subscriber

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"acs-next-admission-controller/internal/policy"
)

// Config holds subscriber configuration.
type Config struct {
	NATSURL string
	TLSCert string
	TLSKey  string
	TLSCA   string
}

// Subscriber connects to NATS and processes runtime events.
type Subscriber struct {
	nc       *nats.Conn
	js       nats.JetStreamContext
	engine   *policy.Engine
	k8s      kubernetes.Interface
	alertPub *NATSAlertPublisher
}

// New creates a Subscriber connected to the NATS broker.
func New(cfg Config, engine *policy.Engine, k8s kubernetes.Interface) (*Subscriber, error) {
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

	return &Subscriber{
		nc:       nc,
		js:       js,
		engine:   engine,
		k8s:      k8s,
		alertPub: NewNATSAlertPublisher(nc),
	}, nil
}

// Start begins consuming events from both streams.
func (s *Subscriber) Start(ctx context.Context) error {
	if err := s.startConsumer(ctx, "PROCESS_EVENTS", "acs.*.process-events",
		"admission-process-events", s.handleProcessMessage); err != nil {
		return fmt.Errorf("start process consumer: %w", err)
	}

	if err := s.startConsumer(ctx, "NETWORK_EVENTS", "acs.*.network-events",
		"admission-network-events", s.handleNetworkMessage); err != nil {
		return fmt.Errorf("start network consumer: %w", err)
	}

	log.Println("NATS subscriber started")
	return nil
}

// AlertPublisher returns the NATS alert publisher for use by other components.
func (s *Subscriber) AlertPublisher() policy.AlertPublisher {
	return s.alertPub
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
	var event policy.ProcessEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal process event: %v", err)
		return
	}

	violations := s.engine.EvaluateProcessEvent(event)
	for _, v := range violations {
		s.enforce(v, event.Container)
	}
}

func (s *Subscriber) handleNetworkMessage(msg *nats.Msg) {
	var event policy.NetworkEvent
	if err := json.Unmarshal(msg.Data, &event); err != nil {
		log.Printf("Failed to unmarshal network event: %v", err)
		return
	}

	violations := s.engine.EvaluateNetworkEvent(event)
	for _, v := range violations {
		s.enforce(v, event.Container)
	}
}

func (s *Subscriber) enforce(v policy.Violation, container policy.ContainerInfo) {
	// 1. Log the violation
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

	// 2. Annotate the pod (if we have a K8s client and pod info)
	if s.k8s != nil && container.Namespace != "" && container.Pod != "" {
		annotation := fmt.Sprintf("%s: %s", v.PolicyName, v.Message)
		patch := fmt.Sprintf(`{"metadata":{"annotations":{"acs-next.stackrox.io/violations":%q}}}`,
			annotation)

		_, err := s.k8s.CoreV1().Pods(container.Namespace).Patch(
			context.Background(),
			container.Pod,
			types.MergePatchType,
			[]byte(patch),
			metav1.PatchOptions{},
		)
		if err != nil {
			log.Printf("Failed to annotate pod %s/%s: %v", container.Namespace, container.Pod, err)
		}
	}

	// 3. Publish alert to NATS
	s.alertPub.PublishAlert("runtime", v, map[string]string{
		"namespace": container.Namespace,
		"pod":       container.Pod,
		"container": container.Name,
		"image":     container.Image,
	})
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

// NATSAlertPublisher publishes policy violation alerts to NATS.
type NATSAlertPublisher struct {
	nc *nats.Conn
}

// NewNATSAlertPublisher creates a publisher backed by a NATS connection.
func NewNATSAlertPublisher(nc *nats.Conn) *NATSAlertPublisher {
	return &NATSAlertPublisher{nc: nc}
}

// PublishAlert publishes a violation alert to the acs.alerts subject.
func (p *NATSAlertPublisher) PublishAlert(source string, v policy.Violation, ctx map[string]string) {
	if p.nc == nil {
		return
	}
	alert := map[string]interface{}{
		"source":   source,
		"policy":   v.PolicyName,
		"severity": v.Severity,
		"message":  v.Message,
		"context":  ctx,
	}
	data, _ := json.Marshal(alert)
	if err := p.nc.Publish("acs.alerts", data); err != nil {
		log.Printf("Failed to publish alert: %v", err)
	}
}

// FormatAnnotation builds the annotation value for a violation.
func FormatAnnotation(violations []policy.Violation) string {
	var parts []string
	for _, v := range violations {
		parts = append(parts, fmt.Sprintf("%s: %s", v.PolicyName, v.Message))
	}
	return strings.Join(parts, "; ")
}
