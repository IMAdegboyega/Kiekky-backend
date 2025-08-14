// internal/posts/service.go
package posts

import (
	"database/sql"
	"errors"
	"mime/multipart"
	"path/filepath"
	"strings"
)

type Service struct {
	repo          *Repository
	uploadService *UploadService
}

func NewService(repo *Repository, uploadService *UploadService) *Service {
	return &Service{
		repo:          repo,
		uploadService: uploadService,
	}
}

func (s *Service) CreatePost(userID int64, req *CreatePostRequest) (*Post, error) {
	// Validate input
	if err := s.validateCreatePost(req); err != nil {
		return nil, err
	}
	
	// Create post
	post := &Post{
		UserID:     userID,
		Caption:    req.Caption,
		Visibility: req.Visibility,
	}
	
	if req.Location != "" {
		post.Location = sql.NullString{String: req.Location, Valid: true}
	}
	
	// Save post to database
	err := s.repo.CreatePost(post)
	if err != nil {
		return nil, err
	}
	
	// Add media if provided
	if len(req.MediaURLs) > 0 {
		media := make([]PostMedia, len(req.MediaURLs))
		for i, url := range req.MediaURLs {
			media[i] = PostMedia{
				PostID:    post.ID,
				MediaURL:  url,
				MediaType: s.getMediaType(url),
				Position:  i,
			}
		}
		
		err = s.repo.AddPostMedia(media)
		if err != nil {
			return nil, err
		}
		post.Media = media
	}
	
	// Get complete post data
	return s.repo.GetPostByID(post.ID, userID)
}

func (s *Service) GetPost(postID, userID int64) (*Post, error) {
	return s.repo.GetPostByID(postID, userID)
}

func (s *Service) UpdatePost(postID, userID int64, req *UpdatePostRequest) (*Post, error) {
	// Check if user owns the post
	isOwner, err := s.repo.IsPostOwner(postID, userID)
	if err != nil {
		return nil, err
	}
	if !isOwner {
		return nil, errors.New("unauthorized to update this post")
	}
	
	// Update post
	err = s.repo.UpdatePost(postID, req)
	if err != nil {
		return nil, err
	}
	
	// Return updated post
	return s.repo.GetPostByID(postID, userID)
}

func (s *Service) DeletePost(postID, userID int64) error {
	// Check if user owns the post
	isOwner, err := s.repo.IsPostOwner(postID, userID)
	if err != nil {
		return err
	}
	if !isOwner {
		return errors.New("unauthorized to delete this post")
	}
	
	return s.repo.DeletePost(postID)
}

func (s *Service) ToggleLike(postID, userID int64) (bool, error) {
	// Check if already liked
	post, err := s.repo.GetPostByID(postID, userID)
	if err != nil {
		return false, err
	}
	
	if post.IsLiked {
		err = s.repo.UnlikePost(postID, userID)
		return false, err
	} else {
		err = s.repo.LikePost(postID, userID)
		return true, err
	}
}

func (s *Service) GetPostLikes(postID int64, page, limit int) ([]Like, *PaginationMeta, error) {
	offset := (page - 1) * limit
	likes, total, err := s.repo.GetPostLikes(postID, limit, offset)
	if err != nil {
		return nil, nil, err
	}
	
	pagination := &PaginationMeta{
		Page:    page,
		Limit:   limit,
		Total:   total,
		HasNext: offset+limit < total,
	}
	
	return likes, pagination, nil
}

func (s *Service) AddComment(postID, userID int64, req *CommentRequest) (*Comment, error) {
	// Validate input
	if strings.TrimSpace(req.Content) == "" {
		return nil, errors.New("comment content cannot be empty")
	}
	
	comment := &Comment{
		PostID:   postID,
		UserID:   userID,
		ParentID: req.ParentID,
		Content:  req.Content,
	}
	
	err := s.repo.CreateComment(comment)
	if err != nil {
		return nil, err
	}
	
	return comment, nil
}

func (s *Service) GetPostComments(postID int64, page, limit int) ([]Comment, *PaginationMeta, error) {
	offset := (page - 1) * limit
	comments, total, err := s.repo.GetPostComments(postID, limit, offset)
	if err != nil {
		return nil, nil, err
	}
	
	pagination := &PaginationMeta{
		Page:    page,
		Limit:   limit,
		Total:   total,
		HasNext: offset+limit < total,
	}
	
	return comments, pagination, nil
}

func (s *Service) GetFeed(userID int64, page, limit int) (*FeedResponse, error) {
	offset := (page - 1) * limit
	posts, total, err := s.repo.GetFeed(userID, limit, offset)
	if err != nil {
		return nil, err
	}
	
	return &FeedResponse{
		Posts: posts,
		Pagination: PaginationMeta{
			Page:    page,
			Limit:   limit,
			Total:   total,
			HasNext: offset+limit < total,
		},
	}, nil
}

func (s *Service) GetExplorePosts(userID int64, page, limit int) (*FeedResponse, error) {
	offset := (page - 1) * limit
	posts, total, err := s.repo.GetExplorePosts(userID, limit, offset)
	if err != nil {
		return nil, err
	}
	
	return &FeedResponse{
		Posts: posts,
		Pagination: PaginationMeta{
			Page:    page,
			Limit:   limit,
			Total:   total,
			HasNext: offset+limit < total,
		},
	}, nil
}

func (s *Service) GetUserPosts(userID, requestingUserID int64, page, limit int) (*FeedResponse, error) {
	offset := (page - 1) * limit
	posts, total, err := s.repo.GetUserPosts(userID, requestingUserID, limit, offset)
	if err != nil {
		return nil, err
	}
	
	return &FeedResponse{
		Posts: posts,
		Pagination: PaginationMeta{
			Page:    page,
			Limit:   limit,
			Total:   total,
			HasNext: offset+limit < total,
		},
	}, nil
}

func (s *Service) validateCreatePost(req *CreatePostRequest) error {
	if strings.TrimSpace(req.Caption) == "" && len(req.MediaURLs) == 0 {
		return errors.New("post must have either caption or media")
	}
	
	if req.Visibility == "" {
		req.Visibility = "public"
	}
	
	if req.Visibility != "public" && req.Visibility != "private" && req.Visibility != "followers" {
		return errors.New("invalid visibility setting")
	}
	
	if len(req.MediaURLs) > 10 {
		return errors.New("maximum 10 media files allowed per post")
	}
	
	return nil
}

func (s *Service) getMediaType(url string) string {
	ext := strings.ToLower(filepath.Ext(url))
	switch ext {
	case ".mp4", ".avi", ".mov", ".wmv":
		return "video"
	default:
		return "image"
	}
}

// UploadMedia handles file upload to S3 or local storage
func (s *Service) UploadMedia(file multipart.File, header *multipart.FileHeader) (string, error) {
	return s.uploadService.UploadFile(file, header)
}