package integration

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"acs-next-broker/internal/server"

	"github.com/nats-io/nats.go"
)

// TestBrokerEndToEnd tests the full publish/subscribe flow
func TestBrokerEndToEnd(t *testing.T) {
	cfg := server.Config{
		Host:      "127.0.0.1",
		Port:      0,
		StoreDir:  t.TempDir(),
		ClusterID: "integration-test",
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Shutdown()

	// Create publisher connection
	publisher, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("Publisher failed to connect: %v", err)
	}
	defer publisher.Close()

	pubJS, err := publisher.JetStream()
	if err != nil {
		t.Fatalf("Failed to get JetStream context: %v", err)
	}

	// Create consumer connection
	consumer, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("Consumer failed to connect: %v", err)
	}
	defer consumer.Close()

	conJS, err := consumer.JetStream()
	if err != nil {
		t.Fatalf("Failed to get JetStream context: %v", err)
	}

	// Create a durable consumer
	_, err = conJS.AddConsumer("PROCESS_EVENTS", &nats.ConsumerConfig{
		Durable:       "test-consumer",
		DeliverPolicy: nats.DeliverAllPolicy,
		AckPolicy:     nats.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatalf("Failed to create consumer: %v", err)
	}

	// Subscribe using pull consumer
	sub, err := conJS.PullSubscribe("acs.*.process-events", "test-consumer")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	// Publish test messages
	testEvents := []map[string]interface{}{
		{"pid": 1001, "name": "process1", "cluster": "cluster-a"},
		{"pid": 1002, "name": "process2", "cluster": "cluster-a"},
		{"pid": 2001, "name": "process3", "cluster": "cluster-b"},
	}

	for i, event := range testEvents {
		cluster := event["cluster"].(string)
		subject := "acs." + cluster + ".process-events"

		data, _ := json.Marshal(event)
		_, err := pubJS.Publish(subject, data)
		if err != nil {
			t.Fatalf("Failed to publish message %d: %v", i, err)
		}
	}

	// Fetch and verify messages
	msgs, err := sub.Fetch(len(testEvents), nats.MaxWait(5*time.Second))
	if err != nil {
		t.Fatalf("Failed to fetch messages: %v", err)
	}

	if len(msgs) != len(testEvents) {
		t.Errorf("Got %d messages, want %d", len(msgs), len(testEvents))
	}

	for _, msg := range msgs {
		var event map[string]interface{}
		if err := json.Unmarshal(msg.Data, &event); err != nil {
			t.Errorf("Failed to unmarshal message: %v", err)
		}
		msg.Ack()
	}
}

// TestMultipleClusterIsolation verifies events from different clusters
// are properly isolated by subject pattern
func TestMultipleClusterIsolation(t *testing.T) {
	cfg := server.Config{
		Host:      "127.0.0.1",
		Port:      0,
		StoreDir:  t.TempDir(),
		ClusterID: "isolation-test",
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Shutdown()

	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("Failed to get JetStream context: %v", err)
	}

	// Publish to different clusters
	clusters := []string{"cluster-a", "cluster-b", "cluster-c"}
	for _, cluster := range clusters {
		subject := "acs." + cluster + ".process-events"
		_, err := js.Publish(subject, []byte(`{"cluster": "`+cluster+`"}`))
		if err != nil {
			t.Fatalf("Failed to publish to %s: %v", cluster, err)
		}
	}

	// Create consumer for specific cluster
	_, err = js.AddConsumer("PROCESS_EVENTS", &nats.ConsumerConfig{
		Durable:       "cluster-a-consumer",
		FilterSubject: "acs.cluster-a.process-events",
		DeliverPolicy: nats.DeliverAllPolicy,
		AckPolicy:     nats.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatalf("Failed to create filtered consumer: %v", err)
	}

	sub, err := js.PullSubscribe("acs.cluster-a.process-events", "cluster-a-consumer")
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}

	msgs, err := sub.Fetch(10, nats.MaxWait(2*time.Second))
	if err != nil && err != nats.ErrTimeout {
		t.Fatalf("Failed to fetch: %v", err)
	}

	// Should only get 1 message (from cluster-a)
	if len(msgs) != 1 {
		t.Errorf("Got %d messages, want 1 (only cluster-a)", len(msgs))
	}

	if len(msgs) > 0 {
		var event map[string]string
		json.Unmarshal(msgs[0].Data, &event)
		if event["cluster"] != "cluster-a" {
			t.Errorf("Got message from wrong cluster: %s", event["cluster"])
		}
	}
}

// TestConcurrentPublishers verifies the broker handles concurrent publishers
func TestConcurrentPublishers(t *testing.T) {
	cfg := server.Config{
		Host:      "127.0.0.1",
		Port:      0,
		StoreDir:  t.TempDir(),
		ClusterID: "concurrent-test",
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Shutdown()

	numPublishers := 5
	messagesPerPublisher := 100
	totalMessages := numPublishers * messagesPerPublisher

	var wg sync.WaitGroup
	errors := make(chan error, totalMessages)

	for p := 0; p < numPublishers; p++ {
		wg.Add(1)
		go func(publisherID int) {
			defer wg.Done()

			nc, err := nats.Connect(srv.ClientURL())
			if err != nil {
				errors <- err
				return
			}
			defer nc.Close()

			js, err := nc.JetStream()
			if err != nil {
				errors <- err
				return
			}

			for i := 0; i < messagesPerPublisher; i++ {
				subject := "acs.cluster.process-events"
				payload := []byte(`{"publisher": ` + string(rune('0'+publisherID)) + `, "seq": ` + string(rune('0'+i%10)) + `}`)

				_, err := js.Publish(subject, payload)
				if err != nil {
					errors <- err
					return
				}
			}
		}(p)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Publisher error: %v", err)
	}

	// Verify all messages were stored
	nc, _ := nats.Connect(srv.ClientURL())
	defer nc.Close()
	js, _ := nc.JetStream()

	info, err := js.StreamInfo("PROCESS_EVENTS")
	if err != nil {
		t.Fatalf("Failed to get stream info: %v", err)
	}

	if info.State.Msgs != uint64(totalMessages) {
		t.Errorf("Stream has %d messages, want %d", info.State.Msgs, totalMessages)
	}
}

// TestNetworkEventsStream verifies network events stream functionality
func TestNetworkEventsStream(t *testing.T) {
	cfg := server.Config{
		Host:      "127.0.0.1",
		Port:      0,
		StoreDir:  t.TempDir(),
		ClusterID: "network-test",
	}

	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := srv.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer srv.Shutdown()

	nc, err := nats.Connect(srv.ClientURL())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer nc.Close()

	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("Failed to get JetStream context: %v", err)
	}

	// Publish network events
	networkEvent := map[string]interface{}{
		"src_ip":     "10.0.0.1",
		"src_port":   45678,
		"dst_ip":     "10.0.0.2",
		"dst_port":   443,
		"protocol":   "TCP",
		"event_type": "CONNECT",
	}

	data, _ := json.Marshal(networkEvent)
	ack, err := js.Publish("acs.prod-cluster.network-events", data)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	if ack.Stream != "NETWORK_EVENTS" {
		t.Errorf("Published to wrong stream: %s", ack.Stream)
	}

	// Verify retrieval
	info, _ := js.StreamInfo("NETWORK_EVENTS")
	if info.State.Msgs != 1 {
		t.Errorf("Expected 1 message, got %d", info.State.Msgs)
	}
}
