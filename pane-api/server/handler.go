package server

import (
	"context"
	"errors"
	"fmt"
	"net"

	"pane/pane-api/ffi"
	pb "pane/pane-api/proto"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PaneServer implements the gRPC PaneService interface
type PaneServer struct {
	pb.UnimplementedPaneServiceServer
}

// Spawn handles booting a new microVM instance
func (s *PaneServer) Spawn(ctx context.Context, req *pb.SpawnRequest) (*pb.SpawnResponse, error) {
	if req.Id == "" || req.KernelPath == "" || req.RootfsPath == "" {
		return nil, status.Error(codes.InvalidArgument, "id, kernel_path, and rootfs_path cannot be empty")
	}

	cid, pid, err := ffi.Spawn(req.Id, req.KernelPath, req.RootfsPath, req.BootArgs, req.VcpuCount, req.MemSizeMib)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to spawn VM: %v", err)
	}

	return &pb.SpawnResponse{
		Id:       req.Id,
		VsockCid: cid,
		Pid:      pid,
	}, nil
}

// Exec executes a command inside the VM and streams stdout/stderr chunks back
func (s *PaneServer) Exec(req *pb.ExecRequest, stream pb.PaneService_ExecServer) error {
	if req.Id == "" || req.Command == "" {
		return status.Error(codes.InvalidArgument, "id and command cannot be empty")
	}

	// Channel to receive gRPC chunks from the CGo callback
	type grpcChunk struct {
		data     []byte
		isStderr bool
		exitCode int32
		err      error
	}
	ch := make(chan grpcChunk, 16)

	// Define Go callback
	callback := func(data []byte, isStderr bool, exitCode int32) {
		ch <- grpcChunk{
			data:     data,
			isStderr: isStderr,
			exitCode: exitCode,
		}
	}

	// Run CGo execution in a background goroutine so we can select on context cancellation
	go func() {
		err := ffi.Exec(req.Id, req.Command, req.Args, callback)
		if err != nil {
			ch <- grpcChunk{err: err}
		}
		close(ch)
	}()

	// Loop and stream chunks to gRPC client
	for {
		select {
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "stream execution canceled by client")
		case chunk, ok := <-ch:
			if !ok {
				return nil
			}
			if chunk.err != nil {
				return status.Errorf(codes.Internal, "execution error: %v", chunk.err)
			}

			resp := &pb.ExecResponse{
				Data:     chunk.data,
				IsStderr: chunk.isStderr,
				ExitCode: chunk.exitCode,
			}

			if err := stream.Send(resp); err != nil {
				return status.Errorf(codes.Unavailable, "failed to send gRPC stream message: %v", err)
			}

			if chunk.exitCode >= 0 {
				return nil // Finished cleanly
			}
		}
	}
}

// Snapshot freezes execution and captures state to files
func (s *PaneServer) Snapshot(ctx context.Context, req *pb.SnapshotRequest) (*pb.SnapshotResponse, error) {
	if req.Id == "" || req.SnapshotPath == "" || req.MemFilePath == "" {
		return nil, status.Error(codes.InvalidArgument, "id, snapshot_path, and mem_file_path cannot be empty")
	}

	err := ffi.Snapshot(req.Id, req.SnapshotPath, req.MemFilePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to take snapshot: %v", err)
	}

	return &pb.SnapshotResponse{
		Id: req.Id,
	}, nil
}

// Fork creates a microVM clone from a snapshot
func (s *PaneServer) Fork(ctx context.Context, req *pb.ForkRequest) (*pb.ForkResponse, error) {
	if req.Id == "" || req.SnapshotPath == "" || req.MemFilePath == "" || req.NewRootfsPath == "" {
		return nil, status.Error(codes.InvalidArgument, "missing required fields for fork request")
	}

	pid, err := ffi.Fork(req.Id, req.SnapshotPath, req.MemFilePath, req.NewRootfsPath, req.NewVsockCid)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to fork VM: %v", err)
	}

	return &pb.ForkResponse{
		Id:       req.Id,
		VsockCid: req.NewVsockCid,
		Pid:      pid,
	}, nil
}

// Destroy stops microVM process and cleans up cgroups/sockets
func (s *PaneServer) Destroy(ctx context.Context, req *pb.DestroyRequest) (*pb.DestroyResponse, error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "id cannot be empty")
	}

	err := ffi.Destroy(req.Id)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to destroy VM: %v", err)
	}

	return &pb.DestroyResponse{
		Id: req.Id,
	}, nil
}

// StartGrpcServer runs the gRPC listener on the specified port
func StartGrpcServer(port int) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterPaneServiceServer(grpcServer, &PaneServer{})

	go func() {
		if err := grpcServer.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			fmt.Printf("gRPC server error: %v\n", err)
		}
	}()

	return grpcServer, nil
}
