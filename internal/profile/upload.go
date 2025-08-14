// internal/profile/upload.go

package profile

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"bytes"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
)

// UploadService defines the file upload service interface
type UploadService interface {
	UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, folder string) (string, error)
	DeleteFile(ctx context.Context, url string) error
}

// LocalUploadService implements local file storage
type LocalUploadService struct {
	uploadDir string
	baseURL   string
}

// NewLocalUploadService creates a new local upload service
func NewLocalUploadService(uploadDir, baseURL string) UploadService {
	return &LocalUploadService{
		uploadDir: uploadDir,
		baseURL:   baseURL,
	}
}

// UploadFile uploads a file to local storage
func (s *LocalUploadService) UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, folder string) (string, error) {
	// Create upload directory if it doesn't exist
	fullPath := filepath.Join(s.uploadDir, folder)
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Generate unique filename
	ext := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%s_%d%s", uuid.New().String(), time.Now().Unix(), ext)
	filePath := filepath.Join(fullPath, filename)

	// Create destination file
	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dst.Close()

	// Copy file content
	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	// Return URL
	url := fmt.Sprintf("%s/%s/%s", s.baseURL, folder, filename)
	return url, nil
}

// DeleteFile deletes a file from local storage
func (s *LocalUploadService) DeleteFile(ctx context.Context, url string) error {
	// Extract file path from URL
	// This is a simplified implementation
	// You might need to adjust based on your URL structure
	
	// Remove base URL to get relative path
	relativePath := url[len(s.baseURL):]
	if relativePath[0] == '/' {
		relativePath = relativePath[1:]
	}
	
	filePath := filepath.Join(s.uploadDir, relativePath)
	
	// Delete file
	if err := os.Remove(filePath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete file: %w", err)
		}
	}
	
	return nil
}

// S3UploadService implements AWS S3 file storage
type S3UploadService struct {
	s3Client   *s3.S3
	bucket     string
	region     string
	baseURL    string
}

// NewS3UploadService creates a new S3 upload service
func NewS3UploadService(bucket, region string) (UploadService, error) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %w", err)
	}

	s3Client := s3.New(sess)
	baseURL := fmt.Sprintf("https://%s.s3.%s.amazonaws.com", bucket, region)

	return &S3UploadService{
		s3Client: s3Client,
		bucket:   bucket,
		region:   region,
		baseURL:  baseURL,
	}, nil
}

// UploadFile uploads a file to S3
func (s *S3UploadService) UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, folder string) (string, error) {
	// Generate unique key
	ext := filepath.Ext(header.Filename)
	key := fmt.Sprintf("%s/%s_%d%s", folder, uuid.New().String(), time.Now().Unix(), ext)

	// Read file content
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Detect content type
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Upload to S3
	_, err = s.s3Client.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body: 		 bytes.NewReader(fileBytes),
		ContentType: aws.String(contentType),
		ACL:         aws.String("public-read"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Return URL
	url := fmt.Sprintf("%s/%s", s.baseURL, key)
	return url, nil
}

// DeleteFile deletes a file from S3
func (s *S3UploadService) DeleteFile(ctx context.Context, url string) error {
	// Extract key from URL
	key := url[len(s.baseURL)+1:] // +1 for the slash

	// Delete from S3
	_, err := s.s3Client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}