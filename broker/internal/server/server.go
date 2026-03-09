package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

type TLSConfig struct {
	CertFile string
	KeyFile  string
	CAFile   string
	Verify   bool // require client certs (mTLS)
}

type Config struct {
	Host      string
	Port      int
	StoreDir  string
	ClusterID string
	TLS       *TLSConfig
}

type Server struct {
	ns     *server.Server
	nc     *nats.Conn
	js     nats.JetStreamContext
	config Config
}

func New(cfg Config) (*Server, error) {
	opts := &server.Options{
		ServerName: fmt.Sprintf("acs-broker-%s", cfg.ClusterID),
		Host:       cfg.Host,
		Port:       cfg.Port,
		JetStream:  true,
		StoreDir:   cfg.StoreDir,
	}

	if cfg.TLS != nil {
		serverTLS, err := buildServerTLSConfig(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("building server TLS config: %w", err)
		}
		opts.TLSConfig = serverTLS
		opts.TLSVerify = cfg.TLS.Verify
		opts.TLSTimeout = 5
	}

	ns, err := server.NewServer(opts)
	if err != nil {
		return nil, fmt.Errorf("creating NATS server: %w", err)
	}

	return &Server{ns: ns, config: cfg}, nil
}

func (s *Server) Start() error {
	go s.ns.Start()

	if !s.ns.ReadyForConnections(10 * time.Second) {
		return fmt.Errorf("NATS server not ready")
	}

	// Build client connection options — connect to localhost for in-process
	clientURL := fmt.Sprintf("nats://127.0.0.1:%d", s.ns.Addr().(*net.TCPAddr).Port)
	connectOpts := []nats.Option{}
	if s.config.TLS != nil {
		tlsConfig, err := buildClientTLSConfig(s.config.TLS)
		if err != nil {
			return fmt.Errorf("building client TLS config: %w", err)
		}
		connectOpts = append(connectOpts, nats.Secure(tlsConfig))
		clientURL = fmt.Sprintf("tls://127.0.0.1:%d", s.ns.Addr().(*net.TCPAddr).Port)
	}

	nc, err := nats.Connect(clientURL, connectOpts...)
	if err != nil {
		return fmt.Errorf("connecting to NATS: %w", err)
	}
	s.nc = nc

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("getting JetStream context: %w", err)
	}
	s.js = js

	return s.createStreams()
}

func buildServerTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("loading server cert: %w", err)
	}

	caCert, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("reading CA cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS12,
	}

	if cfg.Verify {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig, nil
}

func buildClientTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("loading client cert: %w", err)
	}

	caCert, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("reading CA cert: %w", err)
	}

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func (s *Server) createStreams() error {
	streams := []nats.StreamConfig{
		{
			Name:     "PROCESS_EVENTS",
			Subjects: []string{"acs.*.process-events"},
			Storage:  nats.FileStorage,
			MaxAge:   15 * time.Minute,
		},
		{
			Name:     "NETWORK_EVENTS",
			Subjects: []string{"acs.*.network-events"},
			Storage:  nats.FileStorage,
			MaxAge:   15 * time.Minute,
		},
	}

	for _, cfg := range streams {
		_, err := s.js.AddStream(&cfg)
		if err != nil {
			return fmt.Errorf("creating stream %s: %w", cfg.Name, err)
		}
		log.Printf("Created stream: %s", cfg.Name)
	}

	return nil
}

func (s *Server) Shutdown() {
	if s.nc != nil {
		s.nc.Close()
	}
	s.ns.Shutdown()
}

// ClientURL returns the URL clients should use to connect to this server
func (s *Server) ClientURL() string {
	return s.ns.ClientURL()
}
