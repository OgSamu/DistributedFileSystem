// clientlib/client.go

package clientlib

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	metadataPb "dfs/proto/metadata"
	storagePb "dfs/proto/storage"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client represents the client interacting with Metadata and Storage services
type Client struct {
	metadataClient metadataPb.MetadataServiceClient
	storageClients map[string]storagePb.StorageServiceClient
	chunkSize      int64
	mu             sync.Mutex
}

// NewClient initializes a new Client
func NewClient() *Client {
	// Connect to Metadata Service
	metadataConn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to Metadata Service: %v", err)
	}
	metadataClient := metadataPb.NewMetadataServiceClient(metadataConn)

	return &Client{
		metadataClient: metadataClient,
		storageClients: make(map[string]storagePb.StorageServiceClient),
		chunkSize:      64 * 1024 * 1024, // 64MB
	}
}

// UploadFile uploads a file to the distributed file system
func (c *Client) UploadFile(filePath string) error {
	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %v", err)
	}

	fileName := filepath.Base(filePath)
	fileSize := fileInfo.Size()

	// Request chunk allocation from Metadata Service
	allocResp, err := c.metadataClient.AllocateChunks(context.Background(), &metadataPb.CreateFileRequest{
		FileName: fileName,
		FileSize: fileSize,
	})
	if err != nil {
		return fmt.Errorf("failed to allocate chunks: %v", err)
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	// Upload each chunk
	var wg sync.WaitGroup
	progress := make(chan int64)
	totalSize := fileSize

	// Start a goroutine to display upload progress
	go func() {
		var uploaded int64
		for n := range progress {
			uploaded += n
			percent := float64(uploaded) / float64(totalSize) * 100
			fmt.Printf("\rUploading... %.2f%% complete", percent)
		}
		fmt.Println("\nUpload complete.")
	}()

	errChan := make(chan error, len(allocResp.Chunks))

	for i, chunkInfo := range allocResp.Chunks {
		wg.Add(1)
		go func(i int, chunkInfo *metadataPb.ChunkInfo) {
			defer wg.Done()
			// Read chunk data
			chunkOffset := int64(i) * c.chunkSize
			chunkSize := c.chunkSize
			if chunkOffset+chunkSize > fileSize {
				chunkSize = fileSize - chunkOffset
			}
			chunkData := make([]byte, chunkSize)
			n, err := file.ReadAt(chunkData, chunkOffset)
			if err != nil && err != io.EOF {
				errChan <- fmt.Errorf("failed to read chunk %s: %v", chunkInfo.ChunkId, err)
				return
			}
			chunkData = chunkData[:n]

			// Connect to Storage Node
			storageClient, err := c.getStorageClient(chunkInfo.StorageNode)
			if err != nil {
				errChan <- fmt.Errorf("failed to connect to storage node: %v", err)
				return
			}

			// Store the chunk
			_, err = storageClient.StoreChunk(context.Background(), &storagePb.StoreChunkRequest{
				ChunkId: chunkInfo.ChunkId,
				Data:    chunkData,
			})
			if err != nil {
				errChan <- fmt.Errorf("failed to store chunk %s: %v", chunkInfo.ChunkId, err)
				return
			}

			// Update progress
			progress <- int64(n)

			log.Printf("Chunk %s uploaded successfully.", chunkInfo.ChunkId)
		}(i, chunkInfo)
	}
	wg.Wait()
	close(progress)
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		for err := range errChan {
			log.Println(err)
		}
		return fmt.Errorf("upload failed due to errors during chunk upload")
	}

	return nil
}

// DownloadFile downloads a file from the distributed file system
func (c *Client) DownloadFile(fileName string) error {
	// Request file info from Metadata Service
	fileInfoResp, err := c.metadataClient.GetFileInfo(context.Background(), &metadataPb.GetFileRequest{
		FileName: fileName,
	})
	if err != nil {
		return fmt.Errorf("failed to get file info: %v", err)
	}

	// Define the download path
	downloadDir := "dfs_downloads"

	// Create the download directory if it doesn't exist
	err = os.MkdirAll(downloadDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create download directory: %v", err)
	}

	// Create the full path for the downloaded file
	downloadPath := filepath.Join(downloadDir, fileName)

	// Create or truncate the local file
	file, err := os.Create(downloadPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	// Download each chunk
	var wg sync.WaitGroup
	progress := make(chan int64)
	totalSize := int64(len(fileInfoResp.Chunks)) * c.chunkSize

	// Start a goroutine to display download progress
	go func() {
		var downloaded int64
		for n := range progress {
			downloaded += n
			percent := float64(downloaded) / float64(totalSize) * 100
			fmt.Printf("\rDownloading... %.2f%% complete", percent)
		}
		fmt.Println("\nDownload complete.")
	}()

	errChan := make(chan error, len(fileInfoResp.Chunks))
	chunkDataMap := make([][]byte, len(fileInfoResp.Chunks))

	for i, chunkInfo := range fileInfoResp.Chunks {
		wg.Add(1)
		go func(i int, chunkInfo *metadataPb.ChunkInfo) {
			defer wg.Done()
			// Connect to Storage Node
			storageClient, err := c.getStorageClient(chunkInfo.StorageNode)
			if err != nil {
				errChan <- fmt.Errorf("failed to connect to storage node: %v", err)
				return
			}

			// Retrieve the chunk
			resp, err := storageClient.RetrieveChunk(context.Background(), &storagePb.RetrieveChunkRequest{
				ChunkId: chunkInfo.ChunkId,
			})
			if err != nil {
				errChan <- fmt.Errorf("failed to retrieve chunk %s: %v", chunkInfo.ChunkId, err)
				return
			}

			chunkDataMap[i] = resp.Data

			// Update progress
			progress <- int64(len(resp.Data))

			log.Printf("Chunk %s downloaded successfully.", chunkInfo.ChunkId)
		}(i, chunkInfo)
	}
	wg.Wait()
	close(progress)
	close(errChan)

	// Check for errors
	if len(errChan) > 0 {
		for err := range errChan {
			log.Println(err)
		}
		return fmt.Errorf("download failed due to errors during chunk download")
	}

	// Write chunks to file
	for i, data := range chunkDataMap {
		if data == nil {
			return fmt.Errorf("missing data for chunk %d", i)
		}
		_, err := file.WriteAt(data, int64(i)*c.chunkSize)
		if err != nil {
			return fmt.Errorf("failed to write chunk %d to file: %v", i, err)
		}
	}

	return nil
}

// ListFiles retrieves the list of all files from the Metadata Service
func (c *Client) ListFiles() ([]*metadataPb.FileInfo, error) { // Changed to []*FileInfo
	listResp, err := c.metadataClient.ListFiles(context.Background(), &metadataPb.ListFilesRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %v", err)
	}
	return listResp.Files, nil // Now returning []*FileInfo
}

// getStorageClient retrieves or creates a StorageServiceClient for the given address with retry logic
func (c *Client) getStorageClient(address string) (storagePb.StorageServiceClient, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if client, exists := c.storageClients[address]; exists {
		return client, nil
	}

	var conn *grpc.ClientConn
	var err error
	for i := 0; i < 3; i++ { // Retry up to 3 times
		conn, err = grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			break
		}
		log.Printf("Retrying to connect to storage node at %s (attempt %d)", address, i+1)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to storage node at %s after retries: %v", address, err)
	}

	client := storagePb.NewStorageServiceClient(conn)
	c.storageClients[address] = client
	return client, nil
}
