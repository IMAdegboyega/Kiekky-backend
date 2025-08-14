// internal/profile/repository.go

package profile

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// Repository defines the profile repository interface
type Repository interface {
	// Profile CRUD
	GetProfileByUserID(ctx context.Context, userID int64) (*Profile, error)
	UpdateProfile(ctx context.Context, userID int64, req *UpdateProfileRequest, dob *time.Time) (*Profile, error)
	UpdateProfilePicture(ctx context.Context, userID int64, url string) error
	UpdateCoverPhoto(ctx context.Context, userID int64, url string) error
	
	// Settings
	UpdatePrivacySettings(ctx context.Context, userID int64, req *UpdatePrivacyRequest) error
	UpdateNotificationSettings(ctx context.Context, userID int64, req *UpdateNotificationRequest) error
	
	// Blocking
	BlockUser(ctx context.Context, userID int64, blockedID int64) error
	UnblockUser(ctx context.Context, userID int64, blockedID int64) error
	GetBlockedUsers(ctx context.Context, userID int64) ([]int64, error)
	IsBlocked(ctx context.Context, userID int64, targetID int64) (bool, error)
	
	// Discovery & Search
	DiscoverProfiles(ctx context.Context, userID int64, filter *DiscoverFilter, excludeIDs []int64) ([]*Profile, error)
	SearchUsers(ctx context.Context, filter *SearchFilter, excludeIDs []int64) ([]*Profile, error)
	
	// Profile Views
	RecordProfileView(ctx context.Context, viewerID int64, profileID int64) error
	GetProfileViews(ctx context.Context, userID int64, limit int) ([]*ProfileView, error)
}

// postgresRepository implements Repository using PostgreSQL
type postgresRepository struct {
	db *sqlx.DB
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *sqlx.DB) Repository {
	return &postgresRepository{db: db}
}

// GetProfileByUserID retrieves a profile by user ID
func (r *postgresRepository) GetProfileByUserID(ctx context.Context, userID int64) (*Profile, error) {
	var profile Profile
	query := `
		SELECT 
			u.id, u.id as user_id, u.username, u.email, u.display_name,
			u.profile_picture, u.cover_photo, u.bio, u.date_of_birth,
			u.gender, u.location, u.latitude, u.longitude,
			u.interests, u.looking_for, u.relationship_status,
			u.height, u.education, u.work, u.languages,
			u.instagram, u.twitter, u.website,
			u.privacy_settings, u.notification_settings,
			u.email_verified, u.phone_verified,
			u.last_active, u.created_at, u.updated_at
		FROM users u
		WHERE u.id = $1`

	err := r.db.GetContext(ctx, &profile, query, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrProfileNotFound
		}
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}

	return &profile, nil
}

// UpdateProfile updates a user's profile
func (r *postgresRepository) UpdateProfile(ctx context.Context, userID int64, req *UpdateProfileRequest, dob *time.Time) (*Profile, error) {
	// Build dynamic update query
	var setClauses []string
	var args []interface{}
	argCount := 1

	if req.DisplayName != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_name = $%d", argCount))
		args = append(args, *req.DisplayName)
		argCount++
	}
	if req.Bio != nil {
		setClauses = append(setClauses, fmt.Sprintf("bio = $%d", argCount))
		args = append(args, *req.Bio)
		argCount++
	}
	if dob != nil {
		setClauses = append(setClauses, fmt.Sprintf("date_of_birth = $%d", argCount))
		args = append(args, *dob)
		argCount++
	}
	if req.Gender != nil {
		setClauses = append(setClauses, fmt.Sprintf("gender = $%d", argCount))
		args = append(args, *req.Gender)
		argCount++
	}
	if req.Location != nil {
		setClauses = append(setClauses, fmt.Sprintf("location = $%d", argCount))
		args = append(args, *req.Location)
		argCount++
	}
	if req.Latitude != nil {
		setClauses = append(setClauses, fmt.Sprintf("latitude = $%d", argCount))
		args = append(args, *req.Latitude)
		argCount++
	}
	if req.Longitude != nil {
		setClauses = append(setClauses, fmt.Sprintf("longitude = $%d", argCount))
		args = append(args, *req.Longitude)
		argCount++
	}
	if req.Interests != nil {
		setClauses = append(setClauses, fmt.Sprintf("interests = $%d", argCount))
		args = append(args, pq.Array(req.Interests))
		argCount++
	}
	if req.LookingFor != nil {
		setClauses = append(setClauses, fmt.Sprintf("looking_for = $%d", argCount))
		args = append(args, *req.LookingFor)
		argCount++
	}
	if req.RelationshipStatus != nil {
		setClauses = append(setClauses, fmt.Sprintf("relationship_status = $%d", argCount))
		args = append(args, *req.RelationshipStatus)
		argCount++
	}
	if req.Height != nil {
		setClauses = append(setClauses, fmt.Sprintf("height = $%d", argCount))
		args = append(args, *req.Height)
		argCount++
	}
	if req.Education != nil {
		setClauses = append(setClauses, fmt.Sprintf("education = $%d", argCount))
		args = append(args, *req.Education)
		argCount++
	}
	if req.Work != nil {
		setClauses = append(setClauses, fmt.Sprintf("work = $%d", argCount))
		args = append(args, *req.Work)
		argCount++
	}
	if req.Languages != nil {
		setClauses = append(setClauses, fmt.Sprintf("languages = $%d", argCount))
		args = append(args, pq.Array(req.Languages))
		argCount++
	}
	if req.Instagram != nil {
		setClauses = append(setClauses, fmt.Sprintf("instagram = $%d", argCount))
		args = append(args, *req.Instagram)
		argCount++
	}
	if req.Twitter != nil {
		setClauses = append(setClauses, fmt.Sprintf("twitter = $%d", argCount))
		args = append(args, *req.Twitter)
		argCount++
	}
	if req.Website != nil {
		setClauses = append(setClauses, fmt.Sprintf("website = $%d", argCount))
		args = append(args, *req.Website)
		argCount++
	}

	// Always update updated_at
	setClauses = append(setClauses, fmt.Sprintf("updated_at = $%d", argCount))
	args = append(args, time.Now())
	argCount++

	// Add user ID to args
	args = append(args, userID)

	query := fmt.Sprintf(`
		UPDATE users 
		SET %s
		WHERE id = $%d`,
		strings.Join(setClauses, ", "),
		argCount,
	)

	_, err := r.db.ExecContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to update profile: %w", err)
	}

	// Return updated profile
	return r.GetProfileByUserID(ctx, userID)
}

// UpdateProfilePicture updates the profile picture URL
func (r *postgresRepository) UpdateProfilePicture(ctx context.Context, userID int64, url string) error {
	query := `UPDATE users SET profile_picture = $1, updated_at = $2 WHERE id = $3`
	
	var pictureValue interface{}
	if url == "" {
		pictureValue = nil
	} else {
		pictureValue = url
	}
	
	_, err := r.db.ExecContext(ctx, query, pictureValue, time.Now(), userID)
	return err
}

// UpdateCoverPhoto updates the cover photo URL
func (r *postgresRepository) UpdateCoverPhoto(ctx context.Context, userID int64, url string) error {
	query := `UPDATE users SET cover_photo = $1, updated_at = $2 WHERE id = $3`
	
	var coverValue interface{}
	if url == "" {
		coverValue = nil
	} else {
		coverValue = url
	}
	
	_, err := r.db.ExecContext(ctx, query, coverValue, time.Now(), userID)
	return err
}

// UpdatePrivacySettings updates privacy settings
func (r *postgresRepository) UpdatePrivacySettings(ctx context.Context, userID int64, req *UpdatePrivacyRequest) error {
	// First get current settings
	var current PrivacySettings
	query := `SELECT privacy_settings FROM users WHERE id = $1`
	err := r.db.GetContext(ctx, &current, query, userID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Update with new values
	if req.ProfileVisibility != nil {
		current.ProfileVisibility = *req.ProfileVisibility
	}
	if req.ShowEmail != nil {
		current.ShowEmail = *req.ShowEmail
	}
	if req.ShowPhone != nil {
		current.ShowPhone = *req.ShowPhone
	}
	if req.ShowLocation != nil {
		current.ShowLocation = *req.ShowLocation
	}
	if req.ShowOnlineStatus != nil {
		current.ShowOnlineStatus = *req.ShowOnlineStatus
	}
	if req.AllowMessages != nil {
		current.AllowMessages = *req.AllowMessages
	}
	if req.AllowProfileViews != nil {
		current.AllowProfileViews = *req.AllowProfileViews
	}

	// Save updated settings
	updateQuery := `UPDATE users SET privacy_settings = $1, updated_at = $2 WHERE id = $3`
	_, err = r.db.ExecContext(ctx, updateQuery, current, time.Now(), userID)
	return err
}

// UpdateNotificationSettings updates notification settings
func (r *postgresRepository) UpdateNotificationSettings(ctx context.Context, userID int64, req *UpdateNotificationRequest) error {
	// First get current settings
	var current NotificationSettings
	query := `SELECT notification_settings FROM users WHERE id = $1`
	err := r.db.GetContext(ctx, &current, query, userID)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// Update with new values
	if req.EmailNotifications != nil {
		current.EmailNotifications = *req.EmailNotifications
	}
	if req.PushNotifications != nil {
		current.PushNotifications = *req.PushNotifications
	}
	if req.NewMessages != nil {
		current.NewMessages = *req.NewMessages
	}
	if req.NewLikes != nil {
		current.NewLikes = *req.NewLikes
	}
	if req.NewComments != nil {
		current.NewComments = *req.NewComments
	}
	if req.NewFollowers != nil {
		current.NewFollowers = *req.NewFollowers
	}
	if req.ProfileViews != nil {
		current.ProfileViews = *req.ProfileViews
	}
	if req.MarketingEmails != nil {
		current.MarketingEmails = *req.MarketingEmails
	}

	// Save updated settings
	updateQuery := `UPDATE users SET notification_settings = $1, updated_at = $2 WHERE id = $3`
	_, err = r.db.ExecContext(ctx, updateQuery, current, time.Now(), userID)
	return err
}

// BlockUser creates a block record
func (r *postgresRepository) BlockUser(ctx context.Context, userID int64, blockedID int64) error {
	query := `INSERT INTO blocked_users (user_id, blocked_id, blocked_at) VALUES ($1, $2, $3)`
	_, err := r.db.ExecContext(ctx, query, userID, blockedID, time.Now())
	return err
}

// UnblockUser removes a block record
func (r *postgresRepository) UnblockUser(ctx context.Context, userID int64, blockedID int64) error {
	query := `DELETE FROM blocked_users WHERE user_id = $1 AND blocked_id = $2`
	_, err := r.db.ExecContext(ctx, query, userID, blockedID)
	return err
}

// GetBlockedUsers retrieves list of blocked user IDs
func (r *postgresRepository) GetBlockedUsers(ctx context.Context, userID int64) ([]int64, error) {
	var blockedIDs []int64
	query := `SELECT blocked_id FROM blocked_users WHERE user_id = $1`
	
	err := r.db.SelectContext(ctx, &blockedIDs, query, userID)
	if err != nil {
		return nil, err
	}
	
	return blockedIDs, nil
}

// IsBlocked checks if a user is blocked
func (r *postgresRepository) IsBlocked(ctx context.Context, userID int64, targetID int64) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM blocked_users WHERE user_id = $1 AND blocked_id = $2)`
	
	err := r.db.GetContext(ctx, &exists, query, userID, targetID)
	return exists, err
}

// DiscoverProfiles implements profile discovery with filters
func (r *postgresRepository) DiscoverProfiles(ctx context.Context, userID int64, filter *DiscoverFilter, excludeIDs []int64) ([]*Profile, error) {
	// Implementation would include complex filtering logic
	// This is a simplified version
	query := `
		SELECT 
			u.id, u.id as user_id, u.username, u.email, u.display_name,
			u.profile_picture, u.bio, u.gender, u.location,
			u.interests, u.looking_for, u.relationship_status
		FROM users u
		WHERE u.id != $1
		AND u.id != ALL($2)
		LIMIT $3 OFFSET $4`
	
	var profiles []*Profile
	err := r.db.SelectContext(ctx, &profiles, query, userID, pq.Array(excludeIDs), filter.Limit, filter.Offset)
	return profiles, err
}

// SearchUsers searches for users
func (r *postgresRepository) SearchUsers(ctx context.Context, filter *SearchFilter, excludeIDs []int64) ([]*Profile, error) {
	query := `
		SELECT 
			u.id, u.id as user_id, u.username, u.email, u.display_name,
			u.profile_picture, u.bio
		FROM users u
		WHERE (u.username ILIKE $1 OR u.display_name ILIKE $1)
		AND u.id != ALL($2)
		LIMIT $3 OFFSET $4`
	
	searchPattern := "%" + filter.Query + "%"
	
	var profiles []*Profile
	err := r.db.SelectContext(ctx, &profiles, query, searchPattern, pq.Array(excludeIDs), filter.Limit, filter.Offset)
	return profiles, err
}

// RecordProfileView records a profile view
func (r *postgresRepository) RecordProfileView(ctx context.Context, viewerID int64, profileID int64) error {
	query := `INSERT INTO profile_views (viewer_id, profile_id, viewed_at) VALUES ($1, $2, $3)`
	_, err := r.db.ExecContext(ctx, query, viewerID, profileID, time.Now())
	return err
}

// GetProfileViews gets recent profile views
func (r *postgresRepository) GetProfileViews(ctx context.Context, userID int64, limit int) ([]*ProfileView, error) {
	var views []*ProfileView
	query := `
		SELECT id, viewer_id, profile_id, viewed_at 
		FROM profile_views 
		WHERE profile_id = $1 
		ORDER BY viewed_at DESC 
		LIMIT $2`
	
	err := r.db.SelectContext(ctx, &views, query, userID, limit)
	return views, err
}