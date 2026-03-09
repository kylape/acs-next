package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"acs-next-broker/internal/server"
)

func main() {
	cfg := server.Config{
		Host:      "0.0.0.0",
		Port:      4222,
		StoreDir:  getEnv("STORE_DIR", "/data/jetstream"),
		ClusterID: getEnv("CLUSTER_ID", "local"),
	}

	// Configure TLS if cert files are provided
	tlsCert := getEnv("TLS_CERT", "")
	tlsKey := getEnv("TLS_KEY", "")
	tlsCA := getEnv("TLS_CA", "")
	if tlsCert != "" && tlsKey != "" && tlsCA != "" {
		cfg.TLS = &server.TLSConfig{
			CertFile: tlsCert,
			KeyFile:  tlsKey,
			CAFile:   tlsCA,
			Verify:   getEnv("TLS_VERIFY", "true") == "true",
		}
		log.Printf("TLS enabled (mTLS verify: %v)", cfg.TLS.Verify)
	}

	srv, err := server.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	log.Printf("ACS Broker started on %s:%d", cfg.Host, cfg.Port)

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh

	log.Printf("Received signal %v, shutting down...", sig)
	srv.Shutdown()
	log.Println("Broker shutdown complete")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
