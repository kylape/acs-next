package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	brokerv1 "acs-next/api/gen/go/acs/broker/v1"
)

var (
	natsURL   string
	streamName string
	subject   string
	startSeq  uint64
	follow    bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "nats-cli",
		Short: "ACS Next NATS CLI - stream and decode protobuf events",
	}

	rootCmd.PersistentFlags().StringVarP(&natsURL, "url", "u", "nats://localhost:4222", "NATS server URL")

	// streams command - list available streams
	streamsCmd := &cobra.Command{
		Use:   "streams",
		Short: "List available JetStream streams",
		RunE:  listStreams,
	}

	// stream info command
	streamInfoCmd := &cobra.Command{
		Use:   "info [stream]",
		Short: "Show stream information",
		Args:  cobra.ExactArgs(1),
		RunE:  streamInfo,
	}

	// subscribe command - subscribe to a stream and decode messages
	subscribeCmd := &cobra.Command{
		Use:   "subscribe [stream]",
		Short: "Subscribe to a stream and decode protobuf messages",
		Long: `Subscribe to a JetStream stream and decode protobuf messages.

Supported streams:
  PROCESS_EVENTS  - Process execution events (ProcessEvent protobuf)
  NETWORK_EVENTS  - Network connection events (NetworkEvent protobuf)

Examples:
  nats-cli subscribe PROCESS_EVENTS
  nats-cli subscribe PROCESS_EVENTS --follow
  nats-cli subscribe NETWORK_EVENTS --start-seq 100`,
		Args: cobra.ExactArgs(1),
		RunE: subscribe,
	}
	subscribeCmd.Flags().Uint64Var(&startSeq, "start-seq", 0, "Start from specific sequence number (0 = latest)")
	subscribeCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow new messages (like tail -f)")

	// get command - get a specific message
	getCmd := &cobra.Command{
		Use:   "get [stream] [sequence]",
		Short: "Get and decode a specific message by sequence number",
		Args:  cobra.ExactArgs(2),
		RunE:  getMessage,
	}

	rootCmd.AddCommand(streamsCmd, streamInfoCmd, subscribeCmd, getCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func connect() (*nats.Conn, jetstream.JetStream, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, nil, fmt.Errorf("connect to NATS: %w", err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, nil, fmt.Errorf("create JetStream context: %w", err)
	}

	return nc, js, nil
}

func listStreams(cmd *cobra.Command, args []string) error {
	nc, js, err := connect()
	if err != nil {
		return err
	}
	defer nc.Close()

	ctx := context.Background()
	streamLister := js.ListStreams(ctx)

	fmt.Println("Available streams:")
	fmt.Println()

	for info := range streamLister.Info() {
		fmt.Printf("  %-20s  %d messages  %s\n",
			info.Config.Name,
			info.State.Msgs,
			strings.Join(info.Config.Subjects, ", "))
	}

	if err := streamLister.Err(); err != nil {
		return fmt.Errorf("list streams: %w", err)
	}

	return nil
}

func streamInfo(cmd *cobra.Command, args []string) error {
	nc, js, err := connect()
	if err != nil {
		return err
	}
	defer nc.Close()

	ctx := context.Background()
	stream, err := js.Stream(ctx, args[0])
	if err != nil {
		return fmt.Errorf("get stream: %w", err)
	}

	info, err := stream.Info(ctx)
	if err != nil {
		return fmt.Errorf("get stream info: %w", err)
	}

	fmt.Printf("Stream: %s\n", info.Config.Name)
	fmt.Printf("Subjects: %s\n", strings.Join(info.Config.Subjects, ", "))
	fmt.Printf("Messages: %d\n", info.State.Msgs)
	fmt.Printf("Bytes: %d\n", info.State.Bytes)
	fmt.Printf("First Seq: %d\n", info.State.FirstSeq)
	fmt.Printf("Last Seq: %d\n", info.State.LastSeq)
	fmt.Printf("Consumers: %d\n", info.State.Consumers)
	fmt.Printf("Max Age: %s\n", info.Config.MaxAge)

	return nil
}

func subscribe(cmd *cobra.Command, args []string) error {
	streamName := args[0]

	nc, js, err := connect()
	if err != nil {
		return err
	}
	defer nc.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down...")
		cancel()
	}()

	stream, err := js.Stream(ctx, streamName)
	if err != nil {
		return fmt.Errorf("get stream: %w", err)
	}

	// Determine delivery policy
	var deliverPolicy jetstream.DeliverPolicy
	var optStartSeq uint64

	if startSeq > 0 {
		deliverPolicy = jetstream.DeliverByStartSequencePolicy
		optStartSeq = startSeq
	} else if follow {
		deliverPolicy = jetstream.DeliverNewPolicy
	} else {
		deliverPolicy = jetstream.DeliverAllPolicy
	}

	consumerCfg := jetstream.ConsumerConfig{
		DeliverPolicy: deliverPolicy,
		AckPolicy:     jetstream.AckNonePolicy,
	}
	if optStartSeq > 0 {
		consumerCfg.OptStartSeq = optStartSeq
	}

	consumer, err := stream.CreateOrUpdateConsumer(ctx, consumerCfg)
	if err != nil {
		return fmt.Errorf("create consumer: %w", err)
	}

	fmt.Printf("Subscribed to %s (Ctrl+C to exit)\n", streamName)
	fmt.Println()

	msgCh := make(chan jetstream.Msg, 100)

	// Start consuming
	cons, err := consumer.Consume(func(msg jetstream.Msg) {
		select {
		case msgCh <- msg:
		case <-ctx.Done():
		}
	})
	if err != nil {
		return fmt.Errorf("start consumer: %w", err)
	}
	defer cons.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-msgCh:
			if err := printMessage(streamName, msg); err != nil {
				fmt.Fprintf(os.Stderr, "decode error: %v\n", err)
			}
		}
	}
}

func getMessage(cmd *cobra.Command, args []string) error {
	streamName := args[0]
	var seq uint64
	if _, err := fmt.Sscanf(args[1], "%d", &seq); err != nil {
		return fmt.Errorf("invalid sequence number: %s", args[1])
	}

	nc, js, err := connect()
	if err != nil {
		return err
	}
	defer nc.Close()

	ctx := context.Background()
	stream, err := js.Stream(ctx, streamName)
	if err != nil {
		return fmt.Errorf("get stream: %w", err)
	}

	msg, err := stream.GetMsg(ctx, seq)
	if err != nil {
		return fmt.Errorf("get message: %w", err)
	}

	return printRawMessage(streamName, msg)
}

func printMessage(streamName string, msg jetstream.Msg) error {
	meta, err := msg.Metadata()
	if err != nil {
		return err
	}

	decoded, err := decodeMessage(streamName, msg.Data())
	if err != nil {
		return err
	}

	fmt.Printf("[%s] seq=%d subject=%s\n",
		meta.Timestamp.Format(time.RFC3339),
		meta.Sequence.Stream,
		msg.Subject())
	fmt.Println(decoded)
	fmt.Println()

	return nil
}

func printRawMessage(streamName string, msg *jetstream.RawStreamMsg) error {
	decoded, err := decodeMessage(streamName, msg.Data)
	if err != nil {
		return err
	}

	fmt.Printf("[%s] seq=%d subject=%s\n",
		msg.Time.Format(time.RFC3339),
		msg.Sequence,
		msg.Subject)
	fmt.Println(decoded)

	return nil
}

func decodeMessage(streamName string, data []byte) (string, error) {
	var msg proto.Message

	switch streamName {
	case "PROCESS_EVENTS":
		msg = &brokerv1.ProcessEvent{}
	case "NETWORK_EVENTS":
		msg = &brokerv1.NetworkEvent{}
	default:
		// Unknown stream - return raw JSON with base64 data
		return fmt.Sprintf(`{"raw_base64": "%s"}`, data), nil
	}

	if err := proto.Unmarshal(data, msg); err != nil {
		return "", fmt.Errorf("unmarshal protobuf: %w", err)
	}

	jsonBytes, err := protojson.MarshalOptions{
		Multiline: true,
		Indent:    "  ",
	}.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal to JSON: %w", err)
	}

	return string(jsonBytes), nil
}
