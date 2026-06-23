package server

import (
	"context"
	"os"
	"testing"
	"time"

	pb "pane/pane-api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestEnvRestoration(t *testing.T) {
	// Set an initial value
	os.Setenv("TEST_KEY", "original_value")
	defer os.Unsetenv("TEST_KEY")

	// Start server on a unique port
	server, err := StartGrpcServer(50054)
	if err != nil {
		t.Fatalf("Failed to start gRPC server: %v", err)
	}
	defer server.GracefulStop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "localhost:50054", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatalf("Failed to dial gRPC server: %v", err)
	}
	defer conn.Close()

	client := pb.NewPaneServiceClient(conn)

	// Issue spawn request with env that overrides TEST_KEY
	// This will fail because VM creation fails (no kernel/rootfs etc. or just mocked)
	// But it exercises the environment code inside Spawn
	_, _ = client.Spawn(ctx, &pb.SpawnRequest{
		Id: "test-env",
		SpecJson: `{"env": {"TEST_KEY": "new_value", "NEW_KEY": "some_value"}}`,
	})

	// Check if the original value was restored
	if val := os.Getenv("TEST_KEY"); val != "original_value" {
		t.Errorf("Expected TEST_KEY to be restored to 'original_value', got '%s'", val)
	}

	// Check if the new key was unset
	if _, ok := os.LookupEnv("NEW_KEY"); ok {
		t.Errorf("Expected NEW_KEY to be unset, but it was found")
	}
}
