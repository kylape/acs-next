package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"acs-next-admission-controller/internal/policy"
	"acs-next-admission-controller/internal/subscriber"
	"acs-next-admission-controller/internal/webhook"
)

func main() {
	engine := policy.NewEngine()

	// Start webhook server
	webhookCfg := webhook.Config{
		WebhookPort: getEnvInt("WEBHOOK_PORT", 8443),
		HealthPort:  getEnvInt("HEALTH_PORT", 8080),
		TLSCert:     getEnv("WEBHOOK_TLS_CERT", "/certs/webhook/tls.crt"),
		TLSKey:      getEnv("WEBHOOK_TLS_KEY", "/certs/webhook/tls.key"),
	}

	// Connect to NATS first so we can share the alert publisher with the webhook
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	natsURL := getEnv("NATS_URL", "nats://acs-broker.acs-next.svc:4222")
	subCfg := subscriber.Config{
		NATSURL: natsURL,
		TLSCert: getEnv("TLS_CERT", ""),
		TLSKey:  getEnv("TLS_KEY", ""),
		TLSCA:   getEnv("TLS_CA", ""),
	}

	var k8sClient kubernetes.Interface
	if restCfg, err := rest.InClusterConfig(); err == nil {
		k8sClient, err = kubernetes.NewForConfig(restCfg)
		if err != nil {
			log.Printf("Warning: failed to create K8s client: %v", err)
		}
	} else {
		log.Printf("Warning: not running in-cluster, K8s client unavailable: %v", err)
	}

	var alertPub policy.AlertPublisher
	sub, err := subscriber.New(subCfg, engine, k8sClient)
	if err != nil {
		log.Printf("Warning: failed to connect to NATS, runtime enforcement disabled: %v", err)
	} else {
		alertPub = sub.AlertPublisher()
		if err := sub.Start(ctx); err != nil {
			log.Printf("Warning: failed to start NATS subscriber: %v", err)
		} else {
			log.Printf("NATS subscriber connected to %s", natsURL)
		}
	}

	// Start webhook server (with alert publisher if NATS is available)
	webhookSrv, err := webhook.New(webhookCfg, engine, alertPub)
	if err != nil {
		log.Fatalf("Failed to create webhook server: %v", err)
	}

	if err := webhookSrv.Start(); err != nil {
		log.Fatalf("Failed to start webhook server: %v", err)
	}
	log.Printf("Webhook server started on port %d", webhookCfg.WebhookPort)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.Printf("Received signal %v, shutting down...", sig)
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	webhookSrv.Shutdown(shutdownCtx)

	if sub != nil {
		sub.Shutdown()
	}
	log.Println("Admission controller shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	s := os.Getenv(key)
	if s == "" {
		return defaultValue
	}
	var v int
	if _, err := fmt.Sscanf(s, "%d", &v); err != nil {
		return defaultValue
	}
	return v
}
