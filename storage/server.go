// storage/server.go

package main

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	pb "dfs/proto/storage"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server implements the StorageServiceServer interface
type server struct {
	pb.UnimplementedStorageServiceServer
	storageDir string
}

// NewServer initializes a new Storage server
func NewServer(storageDir string) *server {
	// Ensure the storage directory exists
	if err := os.MkdirAll(storageDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create storage directory: %v", err)
	}

	return &server{
		storageDir: storageDir,
	}
}

// StoreChunk saves a chunk of data to the storage node
func (s *server) StoreChunk(ctx context.Context, req *pb.StoreChunkRequest) (*pb.StoreChunkResponse, error) {
	chunkPath := filepath.Join(s.storageDir, req.ChunkId)
	err := ioutil.WriteFile(chunkPath, req.Data, 0644)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to store chunk: %v", err)
	}

	log.Printf("Stored chunk %s", req.ChunkId)

	return &pb.StoreChunkResponse{
		Success: true,
	}, nil
}

// RetrieveChunk retrieves a chunk of data from the storage node
func (s *server) RetrieveChunk(ctx context.Context, req *pb.RetrieveChunkRequest) (*pb.RetrieveChunkResponse, error) {
	chunkPath := filepath.Join(s.storageDir, req.ChunkId)
	data, err := ioutil.ReadFile(chunkPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "Chunk %s not found", req.ChunkId)
		}
		return nil, status.Errorf(codes.Internal, "Failed to read chunk: %v", err)
	}

	log.Printf("Retrieved chunk %s", req.ChunkId)

	return &pb.RetrieveChunkResponse{
		Data: data,
	}, nil
}
