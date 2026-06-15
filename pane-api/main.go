// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"pane/pane-api/server"
)

func main() {
	port := 50051
	fmt.Printf("Starting Pane gRPC server on port %d...\n", port)
	srv, err := server.StartGrpcServer(port)
	if err != nil {
		log.Fatalf("Failed to start gRPC server: %v", err)
	}

	// Wait for termination signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("Shutting down Pane gRPC server...")
	srv.GracefulStop()
	fmt.Println("Server stopped.")
}
