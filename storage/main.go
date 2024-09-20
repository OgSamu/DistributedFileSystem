// storage/main.go

package main

import (
	"flag"
	"log"
	"net"

	pb "dfs/proto/storage"

	"google.golang.org/grpc"
)

func main() {
	// Command-line flags
	port := flag.String("port", ":50052", "Port to listen on")
	storageDir := flag.String("storage_dir", "storage_data", "Directory to store chunk data")
	flag.Parse()

	// Listen on the specified port
	lis, err := net.Listen("tcp", *port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", *port, err)
	}

	// Create a new Storage server
	srv := NewServer(*storageDir)

	// Create a new gRPC server
	grpcServer := grpc.NewServer()

	// Register the StorageService with the gRPC server
	pb.RegisterStorageServiceServer(grpcServer, srv)

	log.Printf("Storage Service is running on port %s", *port)

	// Start serving
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to serve gRPC server: %v", err)
	}
}
