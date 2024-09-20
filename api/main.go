// api/main.go

package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"dfs/clientlib"
)

type API struct {
	client *clientlib.Client
}

func NewAPI() *API {
	client := clientlib.NewClient()
	return &API{
		client: client,
	}
}

// uploadFile handles file uploads
func (api *API) uploadFile(c *gin.Context) {
	// Multipart form
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": "No file is received"})
		return
	}

	filePath := filepath.Join("uploads", filepath.Base(fileHeader.Filename))

	// Ensure the uploads directory exists
	if err := os.MkdirAll("uploads", os.ModePerm); err != nil {
		c.JSON(500, gin.H{"error": "Failed to create uploads directory"})
		return
	}

	// Save the uploaded file to the uploads directory
	if err := c.SaveUploadedFile(fileHeader, filePath); err != nil {
		c.JSON(500, gin.H{"error": "Failed to save file"})
		return
	}

	err = api.client.UploadFile(filePath)
	if err != nil {
		// Delete the temporary file only if upload failed
		os.Remove(filePath)
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	// Delete the temporary file after successful upload
	if err := os.Remove(filePath); err != nil {
		log.Printf("Warning: failed to delete temporary file %s: %v", filePath, err)
	}

	c.JSON(200, gin.H{"status": "File uploaded successfully"})
}

// downloadFile handles file downloads
func (api *API) downloadFile(c *gin.Context) {
	fileName := c.Param("filename")
	if fileName == "" {
		c.JSON(400, gin.H{"error": "Filename is required"})
		return
	}

	// Use the client library to download the file
	err := api.client.DownloadFile(fileName)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	downloadPath := filepath.Join("dfs_downloads", fileName)

	c.File(downloadPath)
}

// listFiles handles listing all available files
func (api *API) listFiles(c *gin.Context) {
	files, err := api.client.ListFiles()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"files": files})
}

func main() {
	api := NewAPI()

	router := gin.Default()

	router.Static("/static", "./static")

	router.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	// Define API routes
	router.POST("/upload", api.uploadFile)
	router.GET("/download/:filename", api.downloadFile)
	router.GET("/files", api.listFiles)

	// Ensure uploads and dfs_downloads directories exist
	if err := os.MkdirAll("uploads", os.ModePerm); err != nil {
		log.Fatalf("Failed to create uploads directory: %v", err)
	}
	if err := os.MkdirAll("dfs_downloads", os.ModePerm); err != nil {
		log.Fatalf("Failed to create dfs_downloads directory: %v", err)
	}

	err := router.Run(":8080")
	if err != nil {
		log.Fatalf("Failed to run API server: %v", err)
	}
}
