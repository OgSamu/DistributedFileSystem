// client/main.go

package main

import (
	"flag"
	"fmt"
	"log"

	"dfs/clientlib"
)

func main() {
	operation := flag.String("op", "", "Operation to perform: upload/download/list")
	fileName := flag.String("file", "", "File name (required for upload/download)")
	flag.Parse()

	c := clientlib.NewClient()

	switch *operation {
	case "upload":
		if *fileName == "" {
			log.Fatalf("Upload operation requires -file parameter")
		}
		err := c.UploadFile(*fileName)
		if err != nil {
			log.Fatalf("Upload failed: %v", err)
		}
		fmt.Println("File uploaded successfully.")
	case "download":
		if *fileName == "" {
			log.Fatalf("Download operation requires -file parameter")
		}
		err := c.DownloadFile(*fileName)
		if err != nil {
			log.Fatalf("Download failed: %v", err)
		}
		fmt.Println("File downloaded successfully.")
	case "list":
		files, err := c.ListFiles()
		if err != nil {
			log.Fatalf("List files failed: %v", err)
		}
		fmt.Println("Available Files:")
		for _, file := range files {
			fmt.Printf("- %s (Size: %.2f MB, Chunks: %d, Replicas: %d, Uploaded: %s)\n",
				file.FileName,
				float64(file.FileSize)/(1024*1024),
				file.NumChunks,
				file.NumReplicas,
				file.UploadDate)
		}
	default:
		fmt.Println("Invalid operation. Use -op=upload, -op=download, or -op=list.")
	}
}
