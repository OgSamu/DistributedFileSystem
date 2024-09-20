// metadata/main.go

package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"

	pb "dfs/proto/metadata"

	"google.golang.org/grpc"
)

func main() {
	// Command-line flags
	port := flag.String("port", ":50051", "The server port")
	storageNodes := flag.String("storage_nodes", "localhost:50052,localhost:50053", "Comma-separated list of storage node addresses")
	chunkSizeMB := flag.Int64("chunk_size_mb", 64, "Chunk size in megabytes")
	flag.Parse()

	// Parse storage node addresses
	storageNs := strings.Split(*storageNodes, ",")
	for i, node := range storageNs {
		storageNs[i] = strings.TrimSpace(node)
	}

	// Convert chunk size from MB to bytes
	chunkSize := *chunkSizeMB * 1024 * 1024

	// Create a new Metadata server
	srv := NewServer(storageNs, chunkSize)

	// Listen on the specified port
	lis, err := net.Listen("tcp", *port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", *port, err)
	}

	// Create a new gRPC server
	grpcServer := grpc.NewServer()

	// Register the MetadataService with the gRPC server
	pb.RegisterMetadataServiceServer(grpcServer, srv)

	// Channel to listen for interrupt or terminate signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Run the server in a goroutine
	go func() {
		log.Printf("Metadata Service is running on port %s", *port)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC server: %v", err)
		}
	}()

	// Block until a signal is received
	<-quit
	log.Println("Shutting down Metadata Service...")

	// Gracefully stop the server
	grpcServer.GracefulStop()

	log.Println("Metadata Service stopped.")
}
