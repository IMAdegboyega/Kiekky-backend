// internal/messaging/storage.go

package messaging

import (
    "bytes"
    "context"
    "fmt"
    "io"
    "mime/multipart"
    "path/filepath"
    "strings"
    "time"
    
    "github.com/aws/aws-sdk-go/aws"
    "github.com/aws/aws-sdk-go/aws/session"
    "github.com/aws/aws-sdk-go/service/s3"
    "github.com/google/uuid"
)

type StorageService interface {
    ProcessMedia(ctx context.Context, mediaURL string, messageType string) (*MediaInfo, error)
    UploadMedia(ctx context.Context, file io.Reader, filename string, contentType string) (string, error)
    UploadMultipartFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) (string, error)
    DeleteMedia(ctx context.Context, mediaURL string) error
    GenerateThumbnail(ctx context.Context, mediaURL string) (string, error)
    GetMediaMetadata(ctx context.Context, mediaURL string) (*MediaMetadata, error)
}

type MediaInfo struct {
    URL          string `json:"url"`
    ThumbnailURL string `json:"thumbnail_url,omitempty"`
    Size         int    `json:"size"`
    Duration     int    `json:"duration,omitempty"`
    Width        int    `json:"width,omitempty"`
    Height       int    `json:"height,omitempty"`
    MimeType     string `json:"mime_type"`
}

type MediaMetadata struct {
    Size         int64     `json:"size"`
    ContentType  string    `json:"content_type"`
    LastModified time.Time `json:"last_modified"`
}

type storageService struct {
    s3Client     *s3.S3
    bucketName   string
    cdnURL       string
    maxFileSize  int64
    allowedTypes []string
}

// NewStorageService creates a new storage service
func NewStorageService(awsSession *session.Session, bucketName, cdnURL string, maxFileSize int64) StorageService {
    return &storageService{
        s3Client:    s3.New(awsSession),
        bucketName:  bucketName,
        cdnURL:      cdnURL,
        maxFileSize: maxFileSize,
        allowedTypes: []string{
            "image/jpeg", "image/png", "image/gif", "image/webp",
            "video/mp4", "video/quicktime", "video/webm",
            "audio/mpeg", "audio/wav", "audio/ogg",
            "application/pdf", "application/zip",
        },
    }
}

// ProcessMedia processes uploaded media and generates metadata
func (s *storageService) ProcessMedia(ctx context.Context, mediaURL string, messageType string) (*MediaInfo, error) {
    info := &MediaInfo{
        URL:      mediaURL,
        MimeType: s.getMimeType(messageType, mediaURL),
    }
    
    // Get media metadata from S3
    metadata, err := s.GetMediaMetadata(ctx, mediaURL)
    if err == nil {
        info.Size = int(metadata.Size)
    }
    
    // Generate thumbnail for images and videos
    if messageType == "image" || messageType == "video" {
        thumbnailURL, err := s.GenerateThumbnail(ctx, mediaURL)
        if err == nil {
            info.ThumbnailURL = thumbnailURL
        }
    }
    
    // For videos and audio, get duration (would need ffprobe or similar)
    if messageType == "video" || messageType == "audio" {
        // Placeholder - implement with actual media processing library
        info.Duration = 0
    }
    
    return info, nil
}

// UploadMedia uploads a file to S3
func (s *storageService) UploadMedia(ctx context.Context, file io.Reader, filename string, contentType string) (string, error) {
    // Validate content type
    if !s.isAllowedType(contentType) {
        return "", fmt.Errorf("file type %s not allowed", contentType)
    }
    
    // Generate unique key
    ext := filepath.Ext(filename)
    key := fmt.Sprintf("messages/%s/%s%s", 
        time.Now().Format("2006/01/02"),
        uuid.New().String(),
        ext,
    )
    
    // Read file into buffer to check size
    buf := new(bytes.Buffer)
    size, err := io.Copy(buf, file)
    if err != nil {
        return "", fmt.Errorf("failed to read file: %v", err)
    }
    
    // Check file size
    if size > s.maxFileSize {
        return "", fmt.Errorf("file size %d exceeds maximum allowed size %d", size, s.maxFileSize)
    }
    
    // Upload to S3
    _, err = s.s3Client.PutObjectWithContext(ctx, &s3.PutObjectInput{
        Bucket:        aws.String(s.bucketName),
        Key:           aws.String(key),
        Body:          bytes.NewReader(buf.Bytes()),
        ContentType:   aws.String(contentType),
        ContentLength: aws.Int64(size),
        ACL:           aws.String("public-read"),
        Metadata: map[string]*string{
            "uploaded-at": aws.String(time.Now().Format(time.RFC3339)),
            "file-name":   aws.String(filename),
        },
    })
    
    if err != nil {
        return "", fmt.Errorf("failed to upload to S3: %v", err)
    }
    
    // Return CDN URL
    return fmt.Sprintf("%s/%s", s.cdnURL, key), nil
}

// UploadMultipartFile handles multipart file upload
func (s *storageService) UploadMultipartFile(ctx context.Context, file multipart.File, header *multipart.FileHeader) (string, error) {
    defer file.Close()
    
    // Detect content type
    buffer := make([]byte, 512)
    _, err := file.Read(buffer)
    if err != nil {
        return "", err
    }
    contentType := http.DetectContentType(buffer)
    
    // Reset file reader
    file.Seek(0, 0)
    
    return s.UploadMedia(ctx, file, header.Filename, contentType)
}

// DeleteMedia deletes media from S3
func (s *storageService) DeleteMedia(ctx context.Context, mediaURL string) error {
    // Extract key from URL
    key := strings.TrimPrefix(mediaURL, s.cdnURL+"/")
    
    _, err := s.s3Client.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
        Bucket: aws.String(s.bucketName),
        Key:    aws.String(key),
    })
    
    return err
}

// GenerateThumbnail generates a thumbnail for images/videos
func (s *storageService) GenerateThumbnail(ctx context.Context, mediaURL string) (string, error) {
    // This would typically use an image processing service like AWS Lambda
    // or a library like imaging/ffmpeg for video thumbnails
    // For now, returning a placeholder implementation
    
    // In production, you would:
    // 1. Download the original media
    // 2. Generate thumbnail using imaging library
    // 3. Upload thumbnail to S3
    // 4. Return thumbnail URL
    
    return "", fmt.Errorf("thumbnail generation not implemented")
}

// GetMediaMetadata gets metadata for a media file
func (s *storageService) GetMediaMetadata(ctx context.Context, mediaURL string) (*MediaMetadata, error) {
    // Extract key from URL
    key := strings.TrimPrefix(mediaURL, s.cdnURL+"/")
    
    result, err := s.s3Client.HeadObjectWithContext(ctx, &s3.HeadObjectInput{
        Bucket: aws.String(s.bucketName),
        Key:    aws.String(key),
    })
    
    if err != nil {
        return nil, err
    }
    
    return &MediaMetadata{
        Size:         *result.ContentLength,
        ContentType:  *result.ContentType,
        LastModified: *result.LastModified,
    }, nil
}

// Helper methods
func (s *storageService) isAllowedType(contentType string) bool {
    for _, allowed := range s.allowedTypes {
        if allowed == contentType {
            return true
        }
    }
    return false
}

func (s *storageService) getMimeType(messageType, filename string) string {
    switch messageType {
    case "image":
        if strings.HasSuffix(filename, ".png") {
            return "image/png"
        }
        if strings.HasSuffix(filename, ".gif") {
            return "image/gif"
        }
        return "image/jpeg"
    case "video":
        return "video/mp4"
    case "audio":
        return "audio/mpeg"
    case "file":
        return "application/octet-stream"
    default:
        return "application/octet-stream"
    }
}