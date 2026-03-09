package server

import (
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Host:      "127.0.0.1",
				Port:      0, // random port
				StoreDir:  t.TempDir(),
				ClusterID: "test-cluster",
			},
			wantErr: false,
		},
		{
			name: "empty cluster ID uses default",
			config: Config{
				Host:      "127.0.0.1",
				Port:      0,
				StoreDir:  t.TempDir(),
				ClusterID: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, err := New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if srv == nil && !tt.wantErr {
				t.Error("New() returned nil server")
			}
		})
	}
}

func TestServerStartStop(t *testing.T) {
	cfg := Config{
		Host:      "127.0.0.1",
		Port:      0,
		StoreDir:  t.TempDir(),
		ClusterID: "test",
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Verify server is running by connecting
	nc, err := nats.Connect(srv.ns.ClientURL())
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	nc.Close()

	srv.Shutdown()
}

func TestStreamsCreated(t *testing.T) {
	cfg := Config{
		Host:      "127.0.0.1",
		Port:      0,
		StoreDir:  t.TempDir(),
		ClusterID: "test",
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer srv.Shutdown()

	// Connect and get JetStream context
	nc, err := nats.Connect(srv.ns.ClientURL())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("Failed to get JetStream context: %v", err)
	}

	// Verify PROCESS_EVENTS stream exists
	processStream, err := js.StreamInfo("PROCESS_EVENTS")
	if err != nil {
		t.Errorf("PROCESS_EVENTS stream not found: %v", err)
	}
	if processStream != nil {
		if len(processStream.Config.Subjects) != 1 || processStream.Config.Subjects[0] != "acs.*.process-events" {
			t.Errorf("PROCESS_EVENTS has wrong subjects: %v", processStream.Config.Subjects)
		}
	}

	// Verify NETWORK_EVENTS stream exists
	networkStream, err := js.StreamInfo("NETWORK_EVENTS")
	if err != nil {
		t.Errorf("NETWORK_EVENTS stream not found: %v", err)
	}
	if networkStream != nil {
		if len(networkStream.Config.Subjects) != 1 || networkStream.Config.Subjects[0] != "acs.*.network-events" {
			t.Errorf("NETWORK_EVENTS has wrong subjects: %v", networkStream.Config.Subjects)
		}
	}
}

func TestPublishToStreams(t *testing.T) {
	cfg := Config{
		Host:      "127.0.0.1",
		Port:      0,
		StoreDir:  t.TempDir(),
		ClusterID: "test",
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer srv.Shutdown()

	nc, err := nats.Connect(srv.ns.ClientURL())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("Failed to get JetStream context: %v", err)
	}

	tests := []struct {
		name    string
		subject string
		stream  string
		payload []byte
	}{
		{
			name:    "publish process event",
			subject: "acs.cluster1.process-events",
			stream:  "PROCESS_EVENTS",
			payload: []byte(`{"pid": 1234, "name": "test"}`),
		},
		{
			name:    "publish network event",
			subject: "acs.cluster1.network-events",
			stream:  "NETWORK_EVENTS",
			payload: []byte(`{"src_ip": "10.0.0.1", "dst_ip": "10.0.0.2"}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Publish message
			ack, err := js.Publish(tt.subject, tt.payload)
			if err != nil {
				t.Fatalf("Failed to publish: %v", err)
			}

			if ack.Stream != tt.stream {
				t.Errorf("Message published to wrong stream: got %s, want %s", ack.Stream, tt.stream)
			}

			// Verify message in stream
			info, err := js.StreamInfo(tt.stream)
			if err != nil {
				t.Fatalf("Failed to get stream info: %v", err)
			}

			if info.State.Msgs == 0 {
				t.Error("No messages in stream after publish")
			}
		})
	}
}

func TestStreamRetention(t *testing.T) {
	cfg := Config{
		Host:      "127.0.0.1",
		Port:      0,
		StoreDir:  t.TempDir(),
		ClusterID: "test",
	}

	srv, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer srv.Shutdown()

	nc, err := nats.Connect(srv.ns.ClientURL())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("Failed to get JetStream context: %v", err)
	}

	// Check PROCESS_EVENTS retention
	info, err := js.StreamInfo("PROCESS_EVENTS")
	if err != nil {
		t.Fatalf("Failed to get stream info: %v", err)
	}

	expectedMaxAge := 15 * time.Minute
	if info.Config.MaxAge != expectedMaxAge {
		t.Errorf("PROCESS_EVENTS MaxAge = %v, want %v", info.Config.MaxAge, expectedMaxAge)
	}

	if info.Config.Storage != nats.FileStorage {
		t.Errorf("PROCESS_EVENTS Storage = %v, want FileStorage", info.Config.Storage)
	}
}
