// SPDX-License-Identifier: Apache-2.0

package server

import (
	"context"
	"os"
	"testing"
	"time"

	pb "pane/pane-api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

func TestSpawnEnvPreservation(t *testing.T) {
	// Set a preexisting environment variable
	os.Setenv("TEST_ENV_VAR", "original_value")
	defer os.Unsetenv("TEST_ENV_VAR")

	server := &PaneServer{}

	// Create a dummy SpawnRequest that provides invalid arguments but has a JSON spec
	// that defines the same environment variable. We expect Spawn to fail eventually
	// (because the VM doesn't exist/can't be spawned), but the environment variable
	// should have been restored before returning.
	req := &pb.SpawnRequest{
		Id:       "test-vm-env",
		SpecJson: `{"env": {"TEST_ENV_VAR": "new_value", "NEW_VAR": "foo"}}`,
	}

	// This will fail because ffi.Spawn fails, but that's fine for testing the env state
	_, _ = server.Spawn(context.Background(), req)

	// Check if the original value was preserved
	val, ok := os.LookupEnv("TEST_ENV_VAR")
	if !ok || val != "original_value" {
		t.Errorf("Expected TEST_ENV_VAR to be 'original_value', got '%s'", val)
	}

	// Check if the newly added var was removed
	_, ok = os.LookupEnv("NEW_VAR")
	if ok {
		t.Errorf("Expected NEW_VAR to be unset")
	}
}

func TestStartGrpcServer(t *testing.T) {
	server, err := StartGrpcServer(50052)
	if err != nil {
		t.Fatalf("Failed to start gRPC server: %v", err)
	}
	defer server.GracefulStop()
}

func TestPaneServiceValidation(t *testing.T) {
	server, err := StartGrpcServer(50053)
	if err != nil {
		t.Fatalf("Failed to start gRPC server: %v", err)
	}
	defer server.GracefulStop()

	// Connect to server
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "localhost:50053",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		t.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	client := pb.NewPaneServiceClient(conn)

	t.Run("Spawn Validation", func(t *testing.T) {
		_, err := client.Spawn(ctx, &pb.SpawnRequest{
			Id: "",
		})
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error, got %v", err)
		}
	})

	t.Run("Destroy Invalid ID", func(t *testing.T) {
		_, err := client.Destroy(ctx, &pb.DestroyRequest{
			Id: "",
		})
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument error, got %v", err)
		}
	})

	t.Run("Destroy Success", func(t *testing.T) {
		resp, err := client.Destroy(ctx, &pb.DestroyRequest{
			Id: "non-existent-vm",
		})
		if err != nil {
			t.Fatalf("Expected success for Destroy on non-existent VM, got error: %v", err)
		}
		if resp.Id != "non-existent-vm" {
			t.Errorf("Expected response ID 'non-existent-vm', got '%s'", resp.Id)
		}
	})
}
