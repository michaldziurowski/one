// Package s3 provides an AWS S3 abstraction with bucket management and high-performance file upload support.
//
// This package offers a simple interface for S3 operations using the AWS SDK Go v2 with
// the s3manager for optimized uploads. It automatically manages bucket creation based on 
// the APP_NAME environment variable and provides high-performance file upload functionality.
//
// Key features:
//   - Automatic bucket creation and management based on APP_NAME
//   - High-performance uploads using s3manager with automatic multipart upload
//   - Parallel upload of large files for improved throughput
//   - Memory-efficient streaming uploads without buffering entire files
//   - Configurable part size (10MB) and concurrency (5 goroutines) for optimal performance
//   - Automatic retry and error recovery for robust uploads
//   - Support for both LocalStack (development) and AWS S3 (production)
//   - Context-aware operations with proper error handling
//   - Cleanup function pattern consistent with other packages
//
// Environment variables:
//   - APP_NAME: Required, used as bucket name
//   - AWS_ENDPOINT_URL: Optional, for LocalStack or custom S3-compatible services
//   - AWS_REGION: Optional, defaults to us-east-1
//   - AWS_ACCESS_KEY_ID: AWS credentials
//   - AWS_SECRET_ACCESS_KEY: AWS credentials
//
// Example usage:
//
//	close, err := s3.Init()
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer close()
//
//	// Upload a file (automatically uses multipart upload for large files)
//	file, _ := os.Open("example.txt")
//	defer file.Close()
//
//	err = s3.Upload(ctx, "files/example.txt", file)
//	if err != nil {
//		// handle error
//	}
//
// Performance characteristics:
//   - Files smaller than 10MB: single-part upload
//   - Files larger than 10MB: automatic multipart upload with 5 concurrent parts
//   - Memory usage: minimal buffering, streams data efficiently
//   - Network resilience: automatic retry on part failures
package s3

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

var (
	client     *s3.Client
	uploader   *manager.Uploader
	bucketName string
)

func Init() (func(), error) {
	appName := os.Getenv("APP_NAME")
	if appName == "" {
		return nil, fmt.Errorf("APP_NAME environment variable is required")
	}

	bucketName = appName

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	client = s3.NewFromConfig(cfg, func(o *s3.Options) {
		if os.Getenv("AWS_ENDPOINT_URL") != "" {
			o.UsePathStyle = true
		}
	})

	uploader = manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = 10 * 1024 * 1024 // 10MB per part
		u.Concurrency = 5             // 5 concurrent uploads
	})

	if err := ensureBucket(context.TODO()); err != nil {
		return nil, fmt.Errorf("failed to ensure bucket exists: %w", err)
	}

	closeFunc := func() {
		client = nil
		uploader = nil
		bucketName = ""
	}

	return closeFunc, nil
}

func Upload(ctx context.Context, key string, reader io.Reader) error {
	if uploader == nil {
		return fmt.Errorf("S3 uploader not initialized, call Init() first")
	}

	_, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   reader,
	})
	if err != nil {
		return fmt.Errorf("failed to upload object: %w", err)
	}

	return nil
}

func ensureBucket(ctx context.Context) error {
	_, err := client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(bucketName),
	})
	if err != nil {
		var notFound *types.NotFound
		var noSuchBucket *types.NoSuchBucket
		if !errors.As(err, &notFound) && !errors.As(err, &noSuchBucket) {
			return fmt.Errorf("failed to check if bucket exists: %w", err)
		}

		_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
			Bucket: aws.String(bucketName),
		})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return nil
}

