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

	t.Run("Spawn Env Restoration", func(t *testing.T) {

		// Pre-set an environment variable
		originalKey := "TEST_PANE_ORIGINAL_VAR"
		originalValue := "original_value"
		os.Setenv(originalKey, originalValue)
		defer os.Unsetenv(originalKey)

		// A variable that we will set during spawn but doesn't exist now
		newKey := "TEST_PANE_NEW_VAR"
		os.Unsetenv(newKey)

		// Construct a request that sets both
		specJson := `{"env": {"TEST_PANE_ORIGINAL_VAR": "overridden_value", "TEST_PANE_NEW_VAR": "new_value"}}`
		_, _ = client.Spawn(ctx, &pb.SpawnRequest{
			Id:       "test-env-vm",
			SpecJson: specJson,
		})

		// Check restoration
		if val := os.Getenv(originalKey); val != originalValue {
			t.Errorf("Expected original value '%s', got '%s'", originalValue, val)
		}

		if val, exists := os.LookupEnv(newKey); exists {
			t.Errorf("Expected new key to be unset, but got '%s'", val)
		}
	})
}
