package main

import (
	"context"
	"flag"
	"io"
	"log"
	"time"

	"github.com/EternisAI/silo-proxy/proto"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	address = flag.String("address", "localhost:9090", "gRPC server address")
	agentID = flag.String("agent-id", "test-agent-1", "Agent ID for this connection")
	pings   = flag.Int("pings", 3, "Number of PING messages to send")
	delay   = flag.Duration("delay", 2*time.Second, "Delay between PING messages")
)

func main() {
	flag.Parse()

	log.Printf("Connecting to gRPC server at %s", *address)

	conn, err := grpc.NewClient(*address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := proto.NewProxyServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	stream, err := client.Stream(ctx)
	if err != nil {
		log.Fatalf("Failed to create stream: %v", err)
	}

	log.Printf("Stream created, sending first message with agent_id=%s", *agentID)

	firstMsg := &proto.ProxyMessage{
		Id:   uuid.New().String(),
		Type: proto.MessageType_PING,
		Metadata: map[string]string{
			"agent_id": *agentID,
		},
	}

	if err := stream.Send(firstMsg); err != nil {
		log.Fatalf("Failed to send first message: %v", err)
	}

	log.Printf("Sent PING id=%s", firstMsg.Id)

	done := make(chan struct{})
	errChan := make(chan error, 1)

	go receiveMessages(stream, done, errChan)

	sendPings(stream, *pings, *delay)

	if err := stream.CloseSend(); err != nil {
		log.Printf("Error closing send: %v", err)
	}

	select {
	case err := <-errChan:
		if err != nil && err != io.EOF {
			log.Printf("Receive error: %v", err)
		}
	case <-time.After(5 * time.Second):
		log.Println("Timeout waiting for responses")
	}

	close(done)
	log.Println("Test client finished")
}

func receiveMessages(stream proto.ProxyService_StreamClient, done chan struct{}, errChan chan error) {
	for {
		select {
		case <-done:
			return
		default:
			msg, err := stream.Recv()
			if err != nil {
				errChan <- err
				return
			}

			switch msg.Type {
			case proto.MessageType_PONG:
				log.Printf("Received PONG id=%s", msg.Id)
			case proto.MessageType_REQUEST:
				log.Printf("Received REQUEST id=%s payload_size=%d", msg.Id, len(msg.Payload))
			default:
				log.Printf("Received message type=%s id=%s", msg.Type, msg.Id)
			}
		}
	}
}

func sendPings(stream proto.ProxyService_StreamClient, count int, delay time.Duration) {
	for i := 1; i < count; i++ {
		time.Sleep(delay)

		pingMsg := &proto.ProxyMessage{
			Id:       uuid.New().String(),
			Type:     proto.MessageType_PING,
			Metadata: map[string]string{},
		}

		if err := stream.Send(pingMsg); err != nil {
			log.Printf("Failed to send PING: %v", err)
			return
		}

		log.Printf("Sent PING id=%s", pingMsg.Id)
	}
}
