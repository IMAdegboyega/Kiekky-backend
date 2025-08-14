//internals/profile/models.go

package profile

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Profile represents a user's profile
type Profile struct {
	ID                  int64              `json:"id" db:"id"`
	UserID              int64              `json:"user_id" db:"user_id"`
	Username            string             `json:"username" db:"username"`
	Email               string             `json:"email" db:"email"`
	DisplayName         string             `json:"display_name" db:"display_name"`
	ProfilePicture      *string            `json:"profile_picture" db:"profile_picture"`
	CoverPhoto          *string            `json:"cover_photo" db:"cover_photo"`
	Bio                 *string            `json:"bio" db:"bio"`
	DateOfBirth         *time.Time         `json:"date_of_birth" db:"date_of_birth"`
	Gender              *string            `json:"gender" db:"gender"`
	Location            *string            `json:"location" db:"location"`
	Latitude            *float64           `json:"latitude" db:"latitude"`
	Longitude           *float64           `json:"longitude" db:"longitude"`
	Interests           []string           `json:"interests" db:"interests"`
	LookingFor          *string            `json:"looking_for" db:"looking_for"`
	RelationshipStatus  *string            `json:"relationship_status" db:"relationship_status"`
	Height              *int               `json:"height" db:"height"` // in cm
	Education           *string            `json:"education" db:"education"`
	Work                *string            `json:"work" db:"work"`
	Languages           []string           `json:"languages" db:"languages"`
	Instagram           *string            `json:"instagram" db:"instagram"`
	Twitter             *string            `json:"twitter" db:"twitter"`
	Website             *string            `json:"website" db:"website"`
	PrivacySettings     PrivacySettings    `json:"privacy_settings" db:"privacy_settings"`
	NotificationSettings NotificationSettings `json:"notification_settings" db:"notification_settings"`
	EmailVerified       bool               `json:"email_verified" db:"email_verified"`
	PhoneVerified       bool               `json:"phone_verified" db:"phone_verified"`
	CompletionPercentage int               `json:"completion_percentage"`
	LastActive          time.Time          `json:"last_active" db:"last_active"`
	CreatedAt           time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time          `json:"updated_at" db:"updated_at"`
}

// PrivacySettings represents user privacy preferences
type PrivacySettings struct {
	ProfileVisibility   string `json:"profile_visibility"` // public, friends, private
	ShowEmail          bool   `json:"show_email"`
	ShowPhone          bool   `json:"show_phone"`
	ShowLocation       bool   `json:"show_location"`
	ShowOnlineStatus   bool   `json:"show_online_status"`
	AllowMessages      string `json:"allow_messages"` // everyone, friends, none
	AllowProfileViews  bool   `json:"allow_profile_views"`
}

// Scan implements the sql.Scanner interface for PrivacySettings
func (p *PrivacySettings) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	if bytes, ok := value.([]byte); ok {
		return json.Unmarshal(bytes, p)
	}
	return nil
}

// Value implements the driver.Valuer interface for PrivacySettings
func (p PrivacySettings) Value() (driver.Value, error) {
	return json.Marshal(p)
}

// NotificationSettings represents notification preferences
type NotificationSettings struct {
	EmailNotifications    bool `json:"email_notifications"`
	PushNotifications     bool `json:"push_notifications"`
	NewMessages          bool `json:"new_messages"`
	NewLikes             bool `json:"new_likes"`
	NewComments          bool `json:"new_comments"`
	NewFollowers         bool `json:"new_followers"`
	ProfileViews         bool `json:"profile_views"`
	MarketingEmails      bool `json:"marketing_emails"`
}

// Scan implements the sql.Scanner interface for NotificationSettings
func (n *NotificationSettings) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	if bytes, ok := value.([]byte); ok {
		return json.Unmarshal(bytes, n)
	}
	return nil
}

// Value implements the driver.Valuer interface for NotificationSettings
func (n NotificationSettings) Value() (driver.Value, error) {
	return json.Marshal(n)
}

// UpdateProfileRequest represents a profile update request
type UpdateProfileRequest struct {
	DisplayName        *string              `json:"display_name" validate:"omitempty,min=2,max=100"`
	Bio                *string              `json:"bio" validate:"omitempty,max=500"`
	DateOfBirth        *string              `json:"date_of_birth" validate:"omitempty"`
	Gender             *string              `json:"gender" validate:"omitempty,oneof=male female other"`
	Location           *string              `json:"location" validate:"omitempty,max=100"`
	Latitude           *float64             `json:"latitude" validate:"omitempty,latitude"`
	Longitude          *float64             `json:"longitude" validate:"omitempty,longitude"`
	Interests          []string             `json:"interests" validate:"omitempty,max=10,dive,min=1,max=50"`
	LookingFor         *string              `json:"looking_for" validate:"omitempty,max=50"`
	RelationshipStatus *string              `json:"relationship_status" validate:"omitempty,oneof=single married divorced widowed complicated"`
	Height             *int                 `json:"height" validate:"omitempty,min=100,max=250"`
	Education          *string              `json:"education" validate:"omitempty,max=200"`
	Work               *string              `json:"work" validate:"omitempty,max=200"`
	Languages          []string             `json:"languages" validate:"omitempty,max=5,dive,min=2,max=50"`
	Instagram          *string              `json:"instagram" validate:"omitempty,max=50"`
	Twitter            *string              `json:"twitter" validate:"omitempty,max=50"`
	Website            *string              `json:"website" validate:"omitempty,url,max=200"`
}

// ProfileSetupRequest represents initial profile setup
type ProfileSetupRequest struct {
	DisplayName string   `json:"display_name" validate:"required,min=2,max=100"`
	DateOfBirth string   `json:"date_of_birth" validate:"required"`
	Gender      string   `json:"gender" validate:"required,oneof=male female other"`
	Bio         string   `json:"bio" validate:"omitempty,max=500"`
	Interests   []string `json:"interests" validate:"required,min=1,max=10"`
	LookingFor  string   `json:"looking_for" validate:"required,max=50"`
}

// UpdatePrivacyRequest represents privacy settings update
type UpdatePrivacyRequest struct {
	ProfileVisibility  *string `json:"profile_visibility" validate:"omitempty,oneof=public friends private"`
	ShowEmail         *bool   `json:"show_email"`
	ShowPhone         *bool   `json:"show_phone"`
	ShowLocation      *bool   `json:"show_location"`
	ShowOnlineStatus  *bool   `json:"show_online_status"`
	AllowMessages     *string `json:"allow_messages" validate:"omitempty,oneof=everyone friends none"`
	AllowProfileViews *bool   `json:"allow_profile_views"`
}

// UpdateNotificationRequest represents notification settings update
type UpdateNotificationRequest struct {
	EmailNotifications *bool `json:"email_notifications"`
	PushNotifications  *bool `json:"push_notifications"`
	NewMessages       *bool `json:"new_messages"`
	NewLikes          *bool `json:"new_likes"`
	NewComments       *bool `json:"new_comments"`
	NewFollowers      *bool `json:"new_followers"`
	ProfileViews      *bool `json:"profile_views"`
	MarketingEmails   *bool `json:"marketing_emails"`
}

// ProfileView represents a profile view record
type ProfileView struct {
	ID         int64     `json:"id" db:"id"`
	ViewerID   int64     `json:"viewer_id" db:"viewer_id"`
	ProfileID  int64     `json:"profile_id" db:"profile_id"`
	ViewedAt   time.Time `json:"viewed_at" db:"viewed_at"`
}

// BlockedUser represents a blocked user record
type BlockedUser struct {
	ID          int64     `json:"id" db:"id"`
	UserID      int64     `json:"user_id" db:"user_id"`
	BlockedID   int64     `json:"blocked_id" db:"blocked_id"`
	BlockedAt   time.Time `json:"blocked_at" db:"blocked_at"`
}

// DiscoverFilter represents filters for discovering profiles
type DiscoverFilter struct {
	Gender             *string  `json:"gender"`
	MinAge             *int     `json:"min_age"`
	MaxAge             *int     `json:"max_age"`
	Location           *string  `json:"location"`
	MaxDistance        *int     `json:"max_distance"` // in km
	Interests          []string `json:"interests"`
	RelationshipStatus *string  `json:"relationship_status"`
	LookingFor         *string  `json:"looking_for"`
	Limit              int      `json:"limit"`
	Offset             int      `json:"offset"`
}

// SearchFilter represents filters for searching users
type SearchFilter struct {
	Query  string `json:"query" validate:"required,min=2"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// ProfileCompletion represents profile completion details
type ProfileCompletion struct {
	Percentage int                       `json:"percentage"`
	Missing    []string                  `json:"missing_fields"`
	Completed  []string                  `json:"completed_fields"`
	Details    ProfileCompletionDetails `json:"details"`
}

// ProfileCompletionDetails represents detailed completion status
type ProfileCompletionDetails struct {
	BasicInfo      bool `json:"basic_info"`
	ProfilePicture bool `json:"profile_picture"`
	Bio            bool `json:"bio"`
	Interests      bool `json:"interests"`
	Location       bool `json:"location"`
	Social         bool `json:"social"`
}