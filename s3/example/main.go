package main

import (
	"context"
	"log"
	"os"
	"strings"

	"github.com/michaldziurowski/one/s3"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	os.Setenv("APP_NAME", "test-bucket")
	os.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
	os.Setenv("AWS_REGION", "us-east-1")
	os.Setenv("AWS_ACCESS_KEY_ID", "test")
	os.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	close, err := s3.Init()
	if err != nil {
		log.Fatalf("Failed to initialize S3: %v", err)
	}
	defer close()
	log.Println("S3 initialized successfully")

	ctx := context.Background()

	content := strings.NewReader("Hello, S3! This is a test file content.")
	err = s3.Upload(ctx, "test/hello.txt", content)
	if err != nil {
		log.Fatalf("Failed to upload file: %v", err)
	}

	log.Println("File uploaded successfully to test/hello.txt")

	jsonContent := strings.NewReader(`{"message": "Hello from JSON", "timestamp": "2024-01-01T00:00:00Z"}`)
	err = s3.Upload(ctx, "data/sample.json", jsonContent)
	if err != nil {
		log.Fatalf("Failed to upload JSON file: %v", err)
	}

	log.Println("JSON file uploaded successfully to data/sample.json")

	log.Println("Example completed successfully!")
}

