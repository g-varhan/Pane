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

func TestHandler_EnvRace(t *testing.T) {
	// This tests that our environment variable tracking correctly restores
	// the environment to its original state.

	// Create a mock server
	srv := &PaneServer{}

	// Set an initial environment variable
	t.Setenv("TEST_PANE_ENV", "original_value")

	// Prepare spawn request that sets the same environment variable
	req := &pb.SpawnRequest{
		Id: "test-vm",
		SpecJson: `{"vmm":"firecracker","cpus":1,"memory":"128MiB","disk":{"path":"/tmp/disk","format":"raw"},"kernel":"/tmp/kernel","cmdline":"console=ttyS0","env":{"TEST_PANE_ENV":"overwritten_value","TEST_PANE_NEW_ENV":"new_value"}}`,
	}

	// Call Spawn (this will fail because FFI isn't set up, but we only care about the env vars)
	// We run it and let it fail, then check the env vars.
	srv.Spawn(context.Background(), req)

	// Check if the original environment variable was restored
	val, ok := os.LookupEnv("TEST_PANE_ENV")
	if !ok || val != "original_value" {
		t.Errorf("Expected TEST_PANE_ENV to be 'original_value', got %q (exists: %v)", val, ok)
	}

	// Check if the new environment variable was unset
	val, ok = os.LookupEnv("TEST_PANE_NEW_ENV")
	if ok {
		t.Errorf("Expected TEST_PANE_NEW_ENV to be unset, got %q", val)
	}
}
