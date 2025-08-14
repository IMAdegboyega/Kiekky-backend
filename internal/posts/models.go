// internal/posts/models.go
package posts

import (
	"database/sql"
	"time"
)

type Post struct {
	ID            int64          `json:"id"`
	UserID        int64          `json:"user_id"`
	Caption       string         `json:"caption"`
	Location      sql.NullString `json:"location,omitempty"`
	Visibility    string         `json:"visibility"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	
	// Joined fields
	User          *UserInfo      `json:"user,omitempty"`
	Media         []PostMedia    `json:"media,omitempty"`
	LikesCount    int            `json:"likes_count"`
	CommentsCount int            `json:"comments_count"`
	IsLiked       bool           `json:"is_liked"`
}

type PostMedia struct {
	ID        int64     `json:"id"`
	PostID    int64     `json:"post_id"`
	MediaURL  string    `json:"url"`
	MediaType string    `json:"type"`
	Position  int       `json:"position"`
}

type UserInfo struct {
	ID             int64  `json:"id"`
	Username       string `json:"username"`
	ProfilePicture string `json:"profile_picture,omitempty"`
}

type Like struct {
	PostID    int64     `json:"post_id"`
	UserID    int64     `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	User      *UserInfo `json:"user,omitempty"`
}

type Comment struct {
	ID        int64      `json:"id"`
	PostID    int64      `json:"post_id"`
	UserID    int64      `json:"user_id"`
	ParentID  *int64     `json:"parent_id,omitempty"`
	Content   string     `json:"content"`
	CreatedAt time.Time  `json:"created_at"`
	User      *UserInfo  `json:"user,omitempty"`
	Replies   []Comment  `json:"replies,omitempty"`
}

type CreatePostRequest struct {
	Caption    string   `json:"caption"`
	Location   string   `json:"location,omitempty"`
	Visibility string   `json:"visibility"`
	MediaURLs  []string `json:"media_urls,omitempty"`
}

type UpdatePostRequest struct {
	Caption    string `json:"caption,omitempty"`
	Location   string `json:"location,omitempty"`
	Visibility string `json:"visibility,omitempty"`
}

type CommentRequest struct {
	Content  string `json:"content"`
	ParentID *int64 `json:"parent_id,omitempty"`
}

type PaginationParams struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
}

type PaginationMeta struct {
	Page    int  `json:"page"`
	Limit   int  `json:"limit"`
	Total   int  `json:"total"`
	HasNext bool `json:"has_next"`
}

type FeedResponse struct {
	Posts      []Post         `json:"posts"`
	Pagination PaginationMeta `json:"pagination"`
}