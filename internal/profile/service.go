// internal/profile/service.go

package profile

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrProfileNotFound     = errors.New("profile not found")
	ErrUnauthorized        = errors.New("unauthorized access")
	ErrUserBlocked         = errors.New("user is blocked")
	ErrInvalidImageFormat  = errors.New("invalid image format")
	ErrImageTooLarge       = errors.New("image size exceeds limit")
	ErrProfileIncomplete   = errors.New("profile is incomplete")
	ErrAlreadyBlocked      = errors.New("user is already blocked")
	ErrCannotBlockSelf     = errors.New("cannot block yourself")
)

// Service defines the profile service interface
type Service interface {
	// Profile CRUD
	GetProfile(ctx context.Context, userID int64, viewerID int64) (*Profile, error)
	GetMyProfile(ctx context.Context, userID int64) (*Profile, error)
	UpdateProfile(ctx context.Context, userID int64, req *UpdateProfileRequest) (*Profile, error)
	SetupProfile(ctx context.Context, userID int64, req *ProfileSetupRequest) (*Profile, error)
	
	// Profile Pictures
	UploadProfilePicture(ctx context.Context, userID int64, file multipart.File, header *multipart.FileHeader) (string, error)
	UploadCoverPhoto(ctx context.Context, userID int64, file multipart.File, header *multipart.FileHeader) (string, error)
	DeleteProfilePicture(ctx context.Context, userID int64) error
	DeleteCoverPhoto(ctx context.Context, userID int64) error
	
	// Profile Completion
	GetProfileCompletion(ctx context.Context, userID int64) (*ProfileCompletion, error)
	
	// Privacy & Settings
	UpdatePrivacySettings(ctx context.Context, userID int64, req *UpdatePrivacyRequest) error
	UpdateNotificationSettings(ctx context.Context, userID int64, req *UpdateNotificationRequest) error
	
	// Blocking
	BlockUser(ctx context.Context, userID int64, blockedID int64) error
	UnblockUser(ctx context.Context, userID int64, blockedID int64) error
	GetBlockedUsers(ctx context.Context, userID int64) ([]int64, error)
	IsBlocked(ctx context.Context, userID int64, targetID int64) (bool, error)
	
	// Discovery & Search
	DiscoverProfiles(ctx context.Context, userID int64, filter *DiscoverFilter) ([]*Profile, error)
	SearchUsers(ctx context.Context, userID int64, filter *SearchFilter) ([]*Profile, error)
	
	// Profile Views
	RecordProfileView(ctx context.Context, viewerID int64, profileID int64) error
	GetProfileViews(ctx context.Context, userID int64, limit int) ([]*ProfileView, error)
}

// service implements the profile service
type service struct {
	repo          Repository
	uploadService UploadService
}

// NewService creates a new profile service
func NewService(repo Repository, uploadService UploadService) Service {
	return &service{
		repo:          repo,
		uploadService: uploadService,
	}
}

// GetProfile retrieves a user's profile
func (s *service) GetProfile(ctx context.Context, userID int64, viewerID int64) (*Profile, error) {
	// Check if viewer is blocked
	blocked, err := s.IsBlocked(ctx, userID, viewerID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, ErrUserBlocked
	}

	// Get profile
	profile, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Apply privacy settings if not own profile
	if userID != viewerID {
		profile = s.applyPrivacySettings(profile, viewerID)
		
		// Record profile view
		go func() {
			_ = s.RecordProfileView(context.Background(), viewerID, userID)
		}()
	}

	// Calculate completion percentage
	completion, _ := s.calculateCompletion(profile)
	profile.CompletionPercentage = completion.Percentage

	return profile, nil
}

// GetMyProfile retrieves the current user's full profile
func (s *service) GetMyProfile(ctx context.Context, userID int64) (*Profile, error) {
	profile, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Calculate completion percentage
	completion, _ := s.calculateCompletion(profile)
	profile.CompletionPercentage = completion.Percentage

	return profile, nil
}

// UpdateProfile updates a user's profile
func (s *service) UpdateProfile(ctx context.Context, userID int64, req *UpdateProfileRequest) (*Profile, error) {
	// Parse date of birth if provided
	var dob *time.Time
	if req.DateOfBirth != nil && *req.DateOfBirth != "" {
		parsed, err := time.Parse("2006-01-02", *req.DateOfBirth)
		if err != nil {
			return nil, fmt.Errorf("invalid date format, use YYYY-MM-DD")
		}
		dob = &parsed
	}

	// Update profile in repository
	profile, err := s.repo.UpdateProfile(ctx, userID, req, dob)
	if err != nil {
		return nil, err
	}

	// Calculate completion percentage
	completion, _ := s.calculateCompletion(profile)
	profile.CompletionPercentage = completion.Percentage

	return profile, nil
}

// SetupProfile handles initial profile setup
func (s *service) SetupProfile(ctx context.Context, userID int64, req *ProfileSetupRequest) (*Profile, error) {
	// Parse date of birth
	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		return nil, fmt.Errorf("invalid date format, use YYYY-MM-DD")
	}

	// Convert to UpdateProfileRequest
	updateReq := &UpdateProfileRequest{
		DisplayName: &req.DisplayName,
		DateOfBirth: &req.DateOfBirth,
		Gender:      &req.Gender,
		Bio:         &req.Bio,
		Interests:   req.Interests,
		LookingFor:  &req.LookingFor,
	}

	return s.repo.UpdateProfile(ctx, userID, updateReq, &dob)
}

// UploadProfilePicture uploads a profile picture
func (s *service) UploadProfilePicture(ctx context.Context, userID int64, file multipart.File, header *multipart.FileHeader) (string, error) {
	// Validate file
	if err := s.validateImage(header); err != nil {
		return "", err
	}

	// Upload to storage
	url, err := s.uploadService.UploadFile(ctx, file, header, "profile-pictures")
	if err != nil {
		return "", fmt.Errorf("failed to upload profile picture: %w", err)
	}

	// Update profile with new picture URL
	if err := s.repo.UpdateProfilePicture(ctx, userID, url); err != nil {
		// Try to delete uploaded file
		_ = s.uploadService.DeleteFile(ctx, url)
		return "", err
	}

	return url, nil
}

// UploadCoverPhoto uploads a cover photo
func (s *service) UploadCoverPhoto(ctx context.Context, userID int64, file multipart.File, header *multipart.FileHeader) (string, error) {
	// Validate file
	if err := s.validateImage(header); err != nil {
		return "", err
	}

	// Upload to storage
	url, err := s.uploadService.UploadFile(ctx, file, header, "cover-photos")
	if err != nil {
		return "", fmt.Errorf("failed to upload cover photo: %w", err)
	}

	// Update profile with new cover photo URL
	if err := s.repo.UpdateCoverPhoto(ctx, userID, url); err != nil {
		// Try to delete uploaded file
		_ = s.uploadService.DeleteFile(ctx, url)
		return "", err
	}

	return url, nil
}

// DeleteProfilePicture removes the profile picture
func (s *service) DeleteProfilePicture(ctx context.Context, userID int64) error {
	// Get current profile picture URL
	profile, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return err
	}

	if profile.ProfilePicture != nil && *profile.ProfilePicture != "" {
		// Delete from storage
		_ = s.uploadService.DeleteFile(ctx, *profile.ProfilePicture)
	}

	// Update profile to remove picture
	return s.repo.UpdateProfilePicture(ctx, userID, "")
}

// DeleteCoverPhoto removes the cover photo
func (s *service) DeleteCoverPhoto(ctx context.Context, userID int64) error {
	// Get current cover photo URL
	profile, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return err
	}

	if profile.CoverPhoto != nil && *profile.CoverPhoto != "" {
		// Delete from storage
		_ = s.uploadService.DeleteFile(ctx, *profile.CoverPhoto)
	}

	// Update profile to remove cover photo
	return s.repo.UpdateCoverPhoto(ctx, userID, "")
}

// GetProfileCompletion calculates profile completion percentage
func (s *service) GetProfileCompletion(ctx context.Context, userID int64) (*ProfileCompletion, error) {
	profile, err := s.repo.GetProfileByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return s.calculateCompletion(profile)
}

// UpdatePrivacySettings updates privacy settings
func (s *service) UpdatePrivacySettings(ctx context.Context, userID int64, req *UpdatePrivacyRequest) error {
	return s.repo.UpdatePrivacySettings(ctx, userID, req)
}

// UpdateNotificationSettings updates notification settings
func (s *service) UpdateNotificationSettings(ctx context.Context, userID int64, req *UpdateNotificationRequest) error {
	return s.repo.UpdateNotificationSettings(ctx, userID, req)
}

// BlockUser blocks another user
func (s *service) BlockUser(ctx context.Context, userID int64, blockedID int64) error {
	if userID == blockedID {
		return ErrCannotBlockSelf
	}

	// Check if already blocked
	exists, err := s.repo.IsBlocked(ctx, userID, blockedID)
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyBlocked
	}

	return s.repo.BlockUser(ctx, userID, blockedID)
}

// UnblockUser unblocks a user
func (s *service) UnblockUser(ctx context.Context, userID int64, blockedID int64) error {
	return s.repo.UnblockUser(ctx, userID, blockedID)
}

// GetBlockedUsers gets list of blocked user IDs
func (s *service) GetBlockedUsers(ctx context.Context, userID int64) ([]int64, error) {
	return s.repo.GetBlockedUsers(ctx, userID)
}

// IsBlocked checks if a user is blocked
func (s *service) IsBlocked(ctx context.Context, userID int64, targetID int64) (bool, error) {
	// Check both directions
	blocked, err := s.repo.IsBlocked(ctx, userID, targetID)
	if err != nil {
		return false, err
	}
	if blocked {
		return true, nil
	}

	// Check if target has blocked the user
	return s.repo.IsBlocked(ctx, targetID, userID)
}

// DiscoverProfiles discovers profiles based on filters
func (s *service) DiscoverProfiles(ctx context.Context, userID int64, filter *DiscoverFilter) ([]*Profile, error) {
	// Get blocked users to exclude
	blockedUsers, err := s.repo.GetBlockedUsers(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Get profiles
	profiles, err := s.repo.DiscoverProfiles(ctx, userID, filter, blockedUsers)
	if err != nil {
		return nil, err
	}

	// Apply privacy settings
	for i, profile := range profiles {
		profiles[i] = s.applyPrivacySettings(profile, userID)
	}

	return profiles, nil
}

// SearchUsers searches for users by query
func (s *service) SearchUsers(ctx context.Context, userID int64, filter *SearchFilter) ([]*Profile, error) {
	// Get blocked users to exclude
	blockedUsers, err := s.repo.GetBlockedUsers(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Search profiles
	profiles, err := s.repo.SearchUsers(ctx, filter, blockedUsers)
	if err != nil {
		return nil, err
	}

	// Apply privacy settings
	for i, profile := range profiles {
		profiles[i] = s.applyPrivacySettings(profile, userID)
	}

	return profiles, nil
}

// RecordProfileView records a profile view
func (s *service) RecordProfileView(ctx context.Context, viewerID int64, profileID int64) error {
	if viewerID == profileID {
		return nil // Don't record self views
	}

	return s.repo.RecordProfileView(ctx, viewerID, profileID)
}

// GetProfileViews gets recent profile views
func (s *service) GetProfileViews(ctx context.Context, userID int64, limit int) ([]*ProfileView, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	return s.repo.GetProfileViews(ctx, userID, limit)
}

// Helper methods

// validateImage validates uploaded image
func (s *service) validateImage(header *multipart.FileHeader) error {
	// Check file size (5MB max)
	maxSize := int64(5 * 1024 * 1024)
	if header.Size > maxSize {
		return ErrImageTooLarge
	}

	// Check file extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	allowedExts := map[string]bool{
		".jpg":  true,
		".jpeg": true,
		".png":  true,
		".gif":  true,
		".webp": true,
	}

	if !allowedExts[ext] {
		return ErrInvalidImageFormat
	}

	return nil
}

// applyPrivacySettings applies privacy settings to a profile
func (s *service) applyPrivacySettings(profile *Profile, viewerID int64) *Profile {
	// Clone profile to avoid modifying original
	p := *profile

	// Apply visibility settings
	if p.PrivacySettings.ProfileVisibility == "private" {
		// Hide most information for private profiles
		p.Bio = nil
		p.DateOfBirth = nil
		p.Interests = []string{}
		p.Instagram = nil
		p.Twitter = nil
		p.Website = nil
	}

	// Hide email if not allowed
	if !p.PrivacySettings.ShowEmail {
		p.Email = ""
	}

	// Hide location if not allowed
	if !p.PrivacySettings.ShowLocation {
		p.Location = nil
		p.Latitude = nil
		p.Longitude = nil
	}

	return &p
}

// calculateCompletion calculates profile completion
func (s *service) calculateCompletion(profile *Profile) (*ProfileCompletion, error) {
	completion := &ProfileCompletion{
		Missing:   []string{},
		Completed: []string{},
		Details:   ProfileCompletionDetails{},
	}

	totalFields := 0
	completedFields := 0

	// Check basic info
	basicFields := []struct {
		value interface{}
		name  string
	}{
		{profile.DisplayName, "display_name"},
		{profile.DateOfBirth, "date_of_birth"},
		{profile.Gender, "gender"},
	}

	basicComplete := true
	for _, field := range basicFields {
		totalFields++
		if s.isFieldComplete(field.value) {
			completedFields++
			completion.Completed = append(completion.Completed, field.name)
		} else {
			basicComplete = false
			completion.Missing = append(completion.Missing, field.name)
		}
	}
	completion.Details.BasicInfo = basicComplete

	// Check profile picture
	totalFields++
	if profile.ProfilePicture != nil && *profile.ProfilePicture != "" {
		completedFields++
		completion.Completed = append(completion.Completed, "profile_picture")
		completion.Details.ProfilePicture = true
	} else {
		completion.Missing = append(completion.Missing, "profile_picture")
	}

	// Check bio
	totalFields++
	if profile.Bio != nil && *profile.Bio != "" {
		completedFields++
		completion.Completed = append(completion.Completed, "bio")
		completion.Details.Bio = true
	} else {
		completion.Missing = append(completion.Missing, "bio")
	}

	// Check interests
	totalFields++
	if len(profile.Interests) > 0 {
		completedFields++
		completion.Completed = append(completion.Completed, "interests")
		completion.Details.Interests = true
	} else {
		completion.Missing = append(completion.Missing, "interests")
	}

	// Check location
	totalFields++
	if profile.Location != nil && *profile.Location != "" {
		completedFields++
		completion.Completed = append(completion.Completed, "location")
		completion.Details.Location = true
	} else {
		completion.Missing = append(completion.Missing, "location")
	}

	// Check social media (at least one)
	totalFields++
	if (profile.Instagram != nil && *profile.Instagram != "") ||
		(profile.Twitter != nil && *profile.Twitter != "") ||
		(profile.Website != nil && *profile.Website != "") {
		completedFields++
		completion.Completed = append(completion.Completed, "social_media")
		completion.Details.Social = true
	} else {
		completion.Missing = append(completion.Missing, "social_media")
	}

	// Calculate percentage
	if totalFields > 0 {
		completion.Percentage = (completedFields * 100) / totalFields
	}

	return completion, nil
}

// isFieldComplete checks if a field is complete
func (s *service) isFieldComplete(value interface{}) bool {
	if value == nil {
		return false
	}

	switch v := value.(type) {
	case string:
		return v != ""
	case *string:
		return v != nil && *v != ""
	case []string:
		return len(v) > 0
	default:
		return true
	}
}