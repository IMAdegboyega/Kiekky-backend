package stories

import (
    "time"
    "database/sql/driver"
    "github.com/lib/pq"
)

// Story represents a user story
type Story struct {
    ID             int64      `json:"id" db:"id"`
    UserID         int64      `json:"user_id" db:"user_id"`
    MediaURL       string     `json:"media_url" db:"media_url"`
    MediaType      string     `json:"media_type" db:"media_type"` // image or video
    ThumbnailURL   *string    `json:"thumbnail_url,omitempty" db:"thumbnail_url"`
    Caption        *string    `json:"caption,omitempty" db:"caption"`
    Duration       int        `json:"duration" db:"duration"` // seconds
    IsHighlighted  bool       `json:"is_highlighted" db:"is_highlighted"`
    HighlightTitle *string    `json:"highlight_title,omitempty" db:"highlight_title"`
    ExpiresAt      time.Time  `json:"expires_at" db:"expires_at"`
    CreatedAt      time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
    
    // Computed fields
    ViewCount      int        `json:"view_count,omitempty"`
    HasViewed      bool       `json:"has_viewed,omitempty"`
    IsExpired      bool       `json:"is_expired"`
    User           *StoryUser `json:"user,omitempty"`
}

// StoryUser represents user info in story response
type StoryUser struct {
    ID             int64   `json:"id"`
    Username       string  `json:"username"`
    DisplayName    string  `json:"display_name"`
    ProfilePicture *string `json:"profile_picture"`
}

// StoryView represents a story view
type StoryView struct {
    StoryID   int64      `json:"story_id" db:"story_id"`
    ViewerID  int64      `json:"viewer_id" db:"viewer_id"`
    ViewedAt  time.Time  `json:"viewed_at" db:"viewed_at"`
    Viewer    *StoryUser `json:"viewer,omitempty"`
}

// StoryReply represents a reply to a story
type StoryReply struct {
    ID        int64      `json:"id" db:"id"`
    StoryID   int64      `json:"story_id" db:"story_id"`
    UserID    int64      `json:"user_id" db:"user_id"`
    Message   *string    `json:"message,omitempty" db:"message"`
    Reaction  *string    `json:"reaction,omitempty" db:"reaction"`
    IsRead    bool       `json:"is_read" db:"is_read"`
    CreatedAt time.Time  `json:"created_at" db:"created_at"`
    User      *StoryUser `json:"user,omitempty"`
}

// StoryHighlight represents a collection of highlighted stories
type StoryHighlight struct {
    ID         int64          `json:"id" db:"id"`
    UserID     int64          `json:"user_id" db:"user_id"`
    Title      string         `json:"title" db:"title"`
    CoverImage *string        `json:"cover_image" db:"cover_image"`
    StoryIDs   pq.Int64Array  `json:"story_ids" db:"story_ids"`
    Stories    []*Story       `json:"stories,omitempty"`
    CreatedAt  time.Time      `json:"created_at" db:"created_at"`
    UpdatedAt  time.Time      `json:"updated_at" db:"updated_at"`
}

// CreateStoryRequest represents request to create a story
type CreateStoryRequest struct {
    MediaURL     string  `json:"media_url" validate:"required,url"`
    MediaType    string  `json:"media_type" validate:"required,oneof=image video"`
    ThumbnailURL string  `json:"thumbnail_url,omitempty" validate:"omitempty,url"`
    Caption      string  `json:"caption,omitempty" validate:"omitempty,max=500"`
    Duration     int     `json:"duration,omitempty" validate:"omitempty,min=1,max=60"`
}

// StoryReplyRequest represents request to reply to a story
type StoryReplyRequest struct {
    Message  string `json:"message,omitempty" validate:"omitempty,max=500"`
    Reaction string `json:"reaction,omitempty" validate:"omitempty,max=50"`
}

// CreateHighlightRequest represents request to create a highlight
type CreateHighlightRequest struct {
    Title      string  `json:"title" validate:"required,min=1,max=100"`
    StoryIDs   []int64 `json:"story_ids" validate:"required,min=1"`
    CoverImage string  `json:"cover_image,omitempty"`
}

// StoriesResponse represents paginated stories response
type StoriesResponse struct {
    Stories    []*Story `json:"stories"`
    TotalCount int      `json:"total_count"`
    HasMore    bool     `json:"has_more"`
}

// Scan implements sql.Scanner for pq.Int64Array
func (a *pq.Int64Array) Scan(src interface{}) error {
    return (*pq.Int64Array)(a).Scan(src)
}

// Value implements driver.Valuer for pq.Int64Array
func (a pq.Int64Array) Value() (driver.Value, error) {
    return pq.Array(a).Value()
}