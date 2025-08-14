// internal/posts/upload.go
package posts

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"
	
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
)

type UploadService struct {
	s3Client   *s3.S3
	bucketName string
	baseURL    string
	uploadDir  string // For local storage
	useS3      bool
}

func NewUploadService(config UploadConfig) *UploadService {
	us := &UploadService{
		bucketName: config.S3Bucket,
		baseURL:    config.BaseURL,
		uploadDir:  config.LocalUploadDir,
		useS3:      config.UseS3,
	}
	
	if config.UseS3 {
		sess := session.Must(session.NewSession(&aws.Config{
			Region: aws.String(config.AWSRegion),
		}))
		us.s3Client = s3.New(sess)
	} else {
		// Create upload directory if it doesn't exist
		if err := os.MkdirAll(config.LocalUploadDir, 0755); err != nil {
			panic("Failed to create upload directory: " + err.Error())
		}
	}
	
	return us
}

type UploadConfig struct {
	UseS3          bool
	S3Bucket       string
	AWSRegion      string
	LocalUploadDir string
	BaseURL        string
}

func (us *UploadService) UploadFile(file multipart.File, header *multipart.FileHeader) (string, error) {
	// Validate file
	if err := us.validateFile(header); err != nil {
		return "", err
	}
	
	// Generate unique filename
	filename := us.generateFilename(header.Filename)
	
	if us.useS3 {
		return us.uploadToS3(file, filename, header)
	}
	
	return us.uploadToLocal(file, filename)
}

func (us *UploadService) uploadToS3(file multipart.File, filename string, header *multipart.FileHeader) (string, error) {
	// Read file content
	buffer := bytes.NewBuffer(nil)
	if _, err := io.Copy(buffer, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	
	// Upload to S3
	key := fmt.Sprintf("posts/%s/%s", time.Now().Format("2006/01/02"), filename)
	
	_, err := us.s3Client.PutObject(&s3.PutObjectInput{
		Bucket:             aws.String(us.bucketName),
		Key:                aws.String(key),
		Body:               bytes.NewReader(buffer.Bytes()),
		ContentType:        aws.String(header.Header.Get("Content-Type")),
		ContentDisposition: aws.String("inline"),
		ACL:                aws.String("public-read"),
	})
	
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}
	
	// Return full URL
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", us.bucketName, key), nil
}

func (us *UploadService) uploadToLocal(file multipart.File, filename string) (string, error) {
	// Create date-based subdirectory
	dateDir := time.Now().Format("2006/01/02")
	fullDir := filepath.Join(us.uploadDir, "posts", dateDir)
	
	if err := os.MkdirAll(fullDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}
	
	// Create destination file
	destPath := filepath.Join(fullDir, filename)
	dest, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer dest.Close()
	
	// Copy file content
	if _, err := io.Copy(dest, file); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}
	
	// Return URL path
	urlPath := fmt.Sprintf("/uploads/posts/%s/%s", dateDir, filename)
	return us.baseURL + urlPath, nil
}

func (us *UploadService) validateFile(header *multipart.FileHeader) error {
	// Check file size (max 10MB)
	maxSize := int64(10 << 20) // 10MB
	if header.Size > maxSize {
		return fmt.Errorf("file size exceeds maximum of 10MB")
	}
	
	// Check file type
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
		".mp4":  true,
		".mov":  true,
		".avi":  true,
	}
	
	if !allowedExts[ext] {
		return fmt.Errorf("file type not allowed")
	}
	
	return nil
}

func (us *UploadService) generateFilename(originalName string) string {
	ext := filepath.Ext(originalName)
	name := uuid.New().String()
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s_%d%s", name, timestamp, ext)
}

func (us *UploadService) DeleteFile(fileURL string) error {
	if us.useS3 {
		return us.deleteFromS3(fileURL)
	}
	return us.deleteFromLocal(fileURL)
}

func (us *UploadService) deleteFromS3(fileURL string) error {
	// Extract key from URL
	key := strings.TrimPrefix(fileURL, fmt.Sprintf("https://%s.s3.amazonaws.com/", us.bucketName))
	
	_, err := us.s3Client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(us.bucketName),
		Key:    aws.String(key),
	})
	
	return err
}

func (us *UploadService) deleteFromLocal(fileURL string) error {
	// Extract path from URL
	urlPath := strings.TrimPrefix(fileURL, us.baseURL)
	localPath := filepath.Join(us.uploadDir, strings.TrimPrefix(urlPath, "/uploads/"))
	
	return os.Remove(localPath)
} 