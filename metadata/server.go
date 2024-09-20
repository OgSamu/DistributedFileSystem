// metadata/server.go

package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	pb "dfs/proto/metadata"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server implements the MetadataServiceServer interface
type server struct {
	pb.UnimplementedMetadataServiceServer
	mu        sync.Mutex
	files     map[string]*FileMetadata
	storageNs []string
	chunkSize int64
}

// FileMetadata holds metadata for a single file
type FileMetadata struct {
	FileName   string
	FileSize   int64
	Chunks     []*ChunkInfo
	UploadDate string
}

// ChunkInfo holds information about a single chunk
type ChunkInfo struct {
	ChunkID     string
	StorageNode string
}

// NewServer initializes a new Metadata server
func NewServer(storageNodes []string, chunkSize int64) *server {
	return &server{
		files:     make(map[string]*FileMetadata),
		storageNs: storageNodes,
		chunkSize: chunkSize,
	}
}

// AllocateChunks allocates chunks for a new file
func (s *server) AllocateChunks(ctx context.Context, req *pb.CreateFileRequest) (*pb.AllocateChunksResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if file already exists
	if _, exists := s.files[req.FileName]; exists {
		return nil, status.Errorf(codes.AlreadyExists, "File %s already exists", req.FileName)
	}

	// Validate chunk size
	chunkSize := s.chunkSize
	if chunkSize <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "Invalid chunk size: %d", chunkSize)
	}

	// Calculate number of chunks
	numChunks := int((req.FileSize + chunkSize - 1) / chunkSize)
	if numChunks <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "File size too small or chunk size invalid")
	}

	// Assign storage nodes in a round-robin fashion
	chunks := make([]*ChunkInfo, numChunks)
	pbChunks := make([]*pb.ChunkInfo, numChunks)

	currentTime := time.Now().Format("2006-01-02 15:04:05")

	for i := 0; i < numChunks; i++ {
		chunkID := fmt.Sprintf("%s_%d", req.FileName, i)
		storageNode := s.storageNs[i%len(s.storageNs)] // Round-robin assignment

		chunkInfo := &ChunkInfo{
			ChunkID:     chunkID,
			StorageNode: storageNode,
		}
		chunks[i] = chunkInfo

		pbChunkInfo := &pb.ChunkInfo{
			ChunkId:     chunkID,
			StorageNode: storageNode,
		}
		pbChunks[i] = pbChunkInfo
	}

	// Store metadata
	fileMeta := &FileMetadata{
		FileName:   req.FileName,
		FileSize:   req.FileSize,
		Chunks:     chunks,
		UploadDate: currentTime,
	}
	s.files[req.FileName] = fileMeta

	log.Printf("Allocated %d chunks for file %s", numChunks, req.FileName)

	return &pb.AllocateChunksResponse{
		Chunks: pbChunks,
	}, nil
}

// GetFileInfo retrieves metadata for a specified file
func (s *server) GetFileInfo(ctx context.Context, req *pb.GetFileRequest) (*pb.GetFileResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fileMeta, exists := s.files[req.FileName]
	if !exists {
		return nil, status.Errorf(codes.NotFound, "File %s not found", req.FileName)
	}

	pbChunks := make([]*pb.ChunkInfo, len(fileMeta.Chunks))
	for i, chunk := range fileMeta.Chunks {
		pbChunks[i] = &pb.ChunkInfo{
			ChunkId:     chunk.ChunkID,
			StorageNode: chunk.StorageNode,
		}
	}

	return &pb.GetFileResponse{
		Chunks: pbChunks,
	}, nil
}

// ListFiles retrieves the list of all files with metadata
func (s *server) ListFiles(ctx context.Context, req *pb.ListFilesRequest) (*pb.ListFilesResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var files []*pb.FileInfo
	for _, fileMeta := range s.files {
		files = append(files, &pb.FileInfo{
			FileName:    fileMeta.FileName,
			FileSize:    fileMeta.FileSize,
			NumChunks:   int32(len(fileMeta.Chunks)),
			NumReplicas: 1, // Adjust based on actual replication logic
			UploadDate:  fileMeta.UploadDate,
		})
	}

	return &pb.ListFilesResponse{
		Files: files,
	}, nil
}
