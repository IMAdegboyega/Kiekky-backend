package stories

import (
    "context"
    "errors"
    "fmt"
    "mime/multipart"
    "os"
    "path/filepath"
    "strings"
    "time"
)

var (
    ErrStoryNotFound = errors.New("story not found")
    ErrUnauthorized  = errors.New("unauthorized")
    ErrStoryExpired  = errors.New("story has expired")
    ErrInvalidMedia  = errors.New("invalid media file")
    ErrInvalidReply  = errors.New("message or reaction is required")
)

type Service interface {
    // Story CRUD
    CreateStory(ctx context.Context, userID int64, req *CreateStoryRequest) (*Story, error)
    GetStory(ctx context.Context, storyID int64, viewerID int64) (*Story, error)
    GetUserStories(ctx context.Context, userID int64, viewerID int64) ([]*Story, error)
    GetActiveStories(ctx context.Context, viewerID int64, limit int, offset int) (*StoriesResponse, error)
    DeleteStory(ctx context.Context, storyID int64, userID int64) error
    
    // Story interactions
    ViewStory(ctx context.Context, storyID int64, viewerID int64) error
    ReplyToStory(ctx context.Context, storyID int64, userID int64, req *StoryReplyRequest) (*StoryReply, error)
    GetStoryViews(ctx context.Context, storyID int64, userID int64) ([]*StoryView, error)
    GetStoryReplies(ctx context.Context, storyID int64, userID int64) ([]*StoryReply, error)
    MarkReplyAsRead(ctx context.Context, replyID int64, userID int64) error
    
    // Highlights
    CreateHighlight(ctx context.Context, userID int64, req *CreateHighlightRequest) (*StoryHighlight, error)
    GetUserHighlights(ctx context.Context, userID int64) ([]*StoryHighlight, error)
    DeleteHighlight(ctx context.Context, highlightID int64, userID int64) error
    
    // Media upload
    UploadStoryMedia(ctx context.Context, userID int64, file multipart.File, header *multipart.FileHeader) (string, error)
    
    // Cleanup
    CleanupExpiredStories(ctx context.Context) error
}

// UploadService interface for media uploads
type UploadService interface {
    UploadFile(ctx context.Context, file multipart.File, header *multipart.FileHeader, folder string) (string, error)
    DeleteFile(ctx context.Context, fileURL string) error
}

type service struct {
    repo          Repository
    uploadService UploadService
    expiryHours   int
}

func NewService(repo Repository, uploadService UploadService) Service {
    expiryHours := 24 // Default to 24 hours
    if hours := os.Getenv("STORY_EXPIRY_HOURS"); hours != "" {
        // Parse hours from env if available
        fmt.Sscanf(hours, "%d", &expiryHours)
    }
    
    return &service{
        repo:          repo,
        uploadService: uploadService,
        expiryHours:   expiryHours,
    }
}

// CreateStory creates a new story
func (s *service) CreateStory(ctx context.Context, userID int64, req *CreateStoryRequest) (*Story, error) {
    // Set default duration if not provided
    duration := req.Duration
    if duration == 0 {
        if req.MediaType == "image" {
            duration = 5
        } else {
            duration = 15
        }
    }
    
    // Calculate expiry time
    expiresAt := time.Now().Add(time.Duration(s.expiryHours) * time.Hour)
    
    story := &Story{
        UserID:       userID,
        MediaURL:     req.MediaURL,
        MediaType:    req.MediaType,
        Duration:     duration,
        ExpiresAt:    expiresAt,
    }
    
    if req.ThumbnailURL != "" {
        story.ThumbnailURL = &req.ThumbnailURL
    }
    if req.Caption != "" {
        story.Caption = &req.Caption
    }
    
    if err := s.repo.CreateStory(ctx, story); err != nil {
        return nil, err
    }
    
    // Get user info
    user, err := s.repo.GetStoryUser(ctx, userID)
    if err == nil {
        story.User = user
    }
    
    return story, nil
}

// GetStory retrieves a story by ID
func (s *service) GetStory(ctx context.Context, storyID int64, viewerID int64) (*Story, error) {
    story, err := s.repo.GetStoryWithUser(ctx, storyID, viewerID)
    if err != nil {
        return nil, err
    }
    
    // Check if story is expired
    if time.Now().After(story.ExpiresAt) && !story.IsHighlighted {
        return nil, ErrStoryExpired
    }
    
    return story, nil
}

// GetUserStories retrieves all stories for a user
func (s *service) GetUserStories(ctx context.Context, userID int64, viewerID int64) ([]*Story, error) {
    stories, err := s.repo.GetUserStories(ctx, userID, false)
    if err != nil {
        return nil, err
    }
    
    // Get user info once
    user, err := s.repo.GetStoryUser(ctx, userID)
    if err == nil {
        for _, story := range stories {
            story.User = user
            if viewerID > 0 {
                story.HasViewed, _ = s.repo.HasViewed(ctx, story.ID, viewerID)
            }
        }
    }
    
    return stories, nil
}

// GetActiveStories retrieves active stories feed
func (s *service) GetActiveStories(ctx context.Context, viewerID int64, limit int, offset int) (*StoriesResponse, error) {
    if limit == 0 {
        limit = 20
    }
    
    stories, err := s.repo.GetActiveStories(ctx, viewerID, limit, offset)
    if err != nil {
        return nil, err
    }
    
    totalCount, err := s.repo.GetActiveStoriesCount(ctx, viewerID)
    if err != nil {
        totalCount = len(stories)
    }
    
    return &StoriesResponse{
        Stories:    stories,
        TotalCount: totalCount,
        HasMore:    offset+len(stories) < totalCount,
    }, nil
}

// DeleteStory deletes a story
func (s *service) DeleteStory(ctx context.Context, storyID int64, userID int64) error {
    story, err := s.repo.GetStory(ctx, storyID)
    if err != nil {
        return err
    }
    
    // Check ownership
    if story.UserID != userID {
        return ErrUnauthorized
    }
    
    // Delete media file
    if s.uploadService != nil {
        s.uploadService.DeleteFile(ctx, story.MediaURL)
        if story.ThumbnailURL != nil {
            s.uploadService.DeleteFile(ctx, *story.ThumbnailURL)
        }
    }
    
    return s.repo.DeleteStory(ctx, storyID)
}

// ViewStory records a story view
func (s *service) ViewStory(ctx context.Context, storyID int64, viewerID int64) error {
    story, err := s.repo.GetStory(ctx, storyID)
    if err != nil {
        return err
    }
    
    // Check if story is expired
    if time.Now().After(story.ExpiresAt) && !story.IsHighlighted {
        return ErrStoryExpired
    }
    
    // Don't record own views
    if story.UserID == viewerID {
        return nil
    }
    
    return s.repo.RecordView(ctx, storyID, viewerID)
}

// ReplyToStory creates a reply to a story
func (s *service) ReplyToStory(ctx context.Context, storyID int64, userID int64, req *StoryReplyRequest) (*StoryReply, error) {
    // Validate request
    if req.Message == "" && req.Reaction == "" {
        return nil, ErrInvalidReply
    }
    
    story, err := s.repo.GetStory(ctx, storyID)
    if err != nil {
        return nil, err
    }
    
    // Check if story is expired
    if time.Now().After(story.ExpiresAt) && !story.IsHighlighted {
        return nil, ErrStoryExpired
    }
    
    reply := &StoryReply{
        StoryID: storyID,
        UserID:  userID,
    }
    
    if req.Message != "" {
        reply.Message = &req.Message
    }
    if req.Reaction != "" {
        reply.Reaction = &req.Reaction
    }
    
    if err := s.repo.CreateReply(ctx, reply); err != nil {
        return nil, err
    }
    
    // Get user info
    user, err := s.repo.GetStoryUser(ctx, userID)
    if err == nil {
        reply.User = user
    }
    
    return reply, nil
}

// GetStoryViews retrieves all views for a story
func (s *service) GetStoryViews(ctx context.Context, storyID int64, userID int64) ([]*StoryView, error) {
    story, err := s.repo.GetStory(ctx, storyID)
    if err != nil {
        return nil, err
    }
    
    // Only story owner can see views
    if story.UserID != userID {
        return nil, ErrUnauthorized
    }
    
    return s.repo.GetStoryViews(ctx, storyID)
}

// GetStoryReplies retrieves all replies for a story
func (s *service) GetStoryReplies(ctx context.Context, storyID int64, userID int64) ([]*StoryReply, error) {
    story, err := s.repo.GetStory(ctx, storyID)
    if err != nil {
        return nil, err
    }
    
    // Only story owner can see replies
    if story.UserID != userID {
        return nil, ErrUnauthorized
    }
    
    return s.repo.GetStoryReplies(ctx, storyID)
}

// MarkReplyAsRead marks a reply as read
func (s *service) MarkReplyAsRead(ctx context.Context, replyID int64, userID int64) error {
    reply, err := s.repo.GetReply(ctx, replyID)
    if err != nil {
        return err
    }
    
    // Get story to check ownership
    story, err := s.repo.GetStory(ctx, reply.StoryID)
    if err != nil {
        return err
    }
    
    // Only story owner can mark replies as read
    if story.UserID != userID {
        return ErrUnauthorized
    }
    
    return s.repo.MarkReplyAsRead(ctx, replyID)
}

// CreateHighlight creates a story highlight
func (s *service) CreateHighlight(ctx context.Context, userID int64, req *CreateHighlightRequest) (*StoryHighlight, error) {
    // Verify ownership of all stories
    for _, storyID := range req.StoryIDs {
        story, err := s.repo.GetStory(ctx, storyID)
        if err != nil {
            return nil, err
        }
        if story.UserID != userID {
            return nil, ErrUnauthorized
        }
        
        // Mark stories as highlighted
        story.IsHighlighted = true
        story.HighlightTitle = &req.Title
        s.repo.CreateStory(ctx, story) // Update story
    }
    
    highlight := &StoryHighlight{
        UserID:   userID,
        Title:    req.Title,
        StoryIDs: req.StoryIDs,
    }
    
    if req.CoverImage != "" {
        highlight.CoverImage = &req.CoverImage
    }
    
    if err := s.repo.CreateHighlight(ctx, highlight); err != nil {
        return nil, err
    }
    
    // Load stories
    for _, storyID := range highlight.StoryIDs {
        story, err := s.repo.GetStoryWithUser(ctx, storyID, 0)
        if err == nil {
            highlight.Stories = append(highlight.Stories, story)
        }
    }
    
    return highlight, nil
}

// GetUserHighlights retrieves all highlights for a user
func (s *service) GetUserHighlights(ctx context.Context, userID int64) ([]*StoryHighlight, error) {
    highlights, err := s.repo.GetUserHighlights(ctx, userID)
    if err != nil {
        return nil, err
    }
    
    // Load stories for each highlight
    for _, highlight := range highlights {
        for _, storyID := range highlight.StoryIDs {
            story, err := s.repo.GetStoryWithUser(ctx, storyID, 0)
            if err == nil {
                highlight.Stories = append(highlight.Stories, story)
            }
        }
    }
    
    return highlights, nil
}

// DeleteHighlight deletes a highlight
func (s *service) DeleteHighlight(ctx context.Context, highlightID int64, userID int64) error {
    highlight, err := s.repo.GetHighlight(ctx, highlightID)
    if err != nil {
        return err
    }
    
    // Check ownership
    if highlight.UserID != userID {
        return ErrUnauthorized
    }
    
    return s.repo.DeleteHighlight(ctx, highlightID)
}

// UploadStoryMedia uploads story media file
func (s *service) UploadStoryMedia(ctx context.Context, userID int64, file multipart.File, header *multipart.FileHeader) (string, error) {
    // Validate file type
    ext := strings.ToLower(filepath.Ext(header.Filename))
    validExts := map[string]bool{
        ".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
        ".mp4": true, ".mov": true, ".avi": true,
    }
    
    if !validExts[ext] {
        return "", ErrInvalidMedia
    }
    
    // Check file size (100MB max)
    maxSize := int64(100 * 1024 * 1024)
    if header.Size > maxSize {
        return "", errors.New("file size exceeds 100MB limit")
    }
    
    // Upload to storage
    folder := fmt.Sprintf("stories/%d", userID)
    url, err := s.uploadService.UploadFile(ctx, file, header, folder)
    if err != nil {
        return "", err
    }
    
    return url, nil
}

// CleanupExpiredStories removes expired stories
func (s *service) CleanupExpiredStories(ctx context.Context) error {
    before := time.Now()
    
    // Get media URLs before deletion
    mediaURLs, err := s.repo.GetExpiredStoryMedia(ctx, before)
    if err != nil {
        return err
    }
    
    // Delete from database
    if err := s.repo.DeleteExpiredStories(ctx, before); err != nil {
        return err
    }
    
    // Delete media files
    if s.uploadService != nil {
        for _, url := range mediaURLs {
            s.uploadService.DeleteFile(ctx, url)
        }
    }
    
    return nil
}