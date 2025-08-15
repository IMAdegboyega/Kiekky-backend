// internal/messaging/models.go

package messaging

import (
    "time"
    "encoding/json"
)

// Conversation represents a chat conversation
type Conversation struct {
    ID                  int64           `json:"id" db:"id"`
    Type                string          `json:"type" db:"type"`
    Name                *string         `json:"name,omitempty" db:"name"`
    AvatarURL           *string         `json:"avatar_url,omitempty" db:"avatar_url"`
    CreatedBy           *int64          `json:"created_by,omitempty" db:"created_by"`
    IsActive            bool            `json:"is_active" db:"is_active"`
    LastMessageAt       *time.Time      `json:"last_message_at,omitempty" db:"last_message_at"`
    LastMessagePreview  *string         `json:"last_message_preview,omitempty" db:"last_message_preview"`
    Metadata            json.RawMessage `json:"metadata,omitempty" db:"metadata"`
    CreatedAt           time.Time       `json:"created_at" db:"created_at"`
    UpdatedAt           time.Time       `json:"updated_at" db:"updated_at"`
    
    // Computed fields
    Participants        []*Participant  `json:"participants,omitempty"`
    UnreadCount         int             `json:"unread_count,omitempty"`
    LastMessage         *Message        `json:"last_message,omitempty"`
}

// Participant represents a conversation participant
type Participant struct {
    ID                    int64      `json:"id" db:"id"`
    ConversationID        int64      `json:"conversation_id" db:"conversation_id"`
    UserID                int64      `json:"user_id" db:"user_id"`
    Role                  string     `json:"role" db:"role"`
    JoinedAt              time.Time  `json:"joined_at" db:"joined_at"`
    LastReadAt            *time.Time `json:"last_read_at,omitempty" db:"last_read_at"`
    LastReadMessageID     *int64     `json:"last_read_message_id,omitempty" db:"last_read_message_id"`
    IsMuted               bool       `json:"is_muted" db:"is_muted"`
    IsArchived            bool       `json:"is_archived" db:"is_archived"`
    NotificationPreference string    `json:"notification_preference" db:"notification_preference"`
    UnreadCount           int        `json:"unread_count" db:"unread_count"`
    IsTyping              bool       `json:"is_typing" db:"is_typing"`
    
    // Joined fields
    User                  *UserInfo  `json:"user,omitempty"`
}

// Message represents a chat message
type Message struct {
    ID                int64           `json:"id" db:"id"`
    ConversationID    int64           `json:"conversation_id" db:"conversation_id"`
    SenderID          int64           `json:"sender_id" db:"sender_id"`
    ParentMessageID   *int64          `json:"parent_message_id,omitempty" db:"parent_message_id"`
    Content           *string         `json:"content,omitempty" db:"content"`
    MessageType       string          `json:"message_type" db:"message_type"`
    MediaURL          *string         `json:"media_url,omitempty" db:"media_url"`
    MediaThumbnailURL *string         `json:"media_thumbnail_url,omitempty" db:"media_thumbnail_url"`
    MediaSize         *int            `json:"media_size,omitempty" db:"media_size"`
    MediaDuration     *int            `json:"media_duration,omitempty" db:"media_duration"`
    Metadata          json.RawMessage `json:"metadata,omitempty" db:"metadata"`
    IsEdited          bool            `json:"is_edited" db:"is_edited"`
    EditedAt          *time.Time      `json:"edited_at,omitempty" db:"edited_at"`
    IsDeleted         bool            `json:"is_deleted" db:"is_deleted"`
    DeletedAt         *time.Time      `json:"deleted_at,omitempty" db:"deleted_at"`
    DeliveredAt       *time.Time      `json:"delivered_at,omitempty" db:"delivered_at"`
    CreatedAt         time.Time       `json:"created_at" db:"created_at"`
    
    // Computed fields
    Sender            *UserInfo       `json:"sender,omitempty"`
    ParentMessage     *Message        `json:"parent_message,omitempty"`
    Receipts          []*Receipt      `json:"receipts,omitempty"`
    Reactions         []*Reaction     `json:"reactions,omitempty"`
    IsRead            bool            `json:"is_read,omitempty"`
}

// Receipt represents message delivery/read receipt
type Receipt struct {
    ID          int64      `json:"id" db:"id"`
    MessageID   int64      `json:"message_id" db:"message_id"`
    UserID      int64      `json:"user_id" db:"user_id"`
    DeliveredAt *time.Time `json:"delivered_at,omitempty" db:"delivered_at"`
    ReadAt      *time.Time `json:"read_at,omitempty" db:"read_at"`
    User        *UserInfo  `json:"user,omitempty"`
}

// WebSocket message types
type WSMessage struct {
    Type      string          `json:"type"`
    Data      json.RawMessage `json:"data"`
    Timestamp time.Time       `json:"timestamp"`
}

type WSMessageType string

const (
    WSTypeMessage        WSMessageType = "message"
    WSTypeTyping         WSMessageType = "typing"
    WSTypeStopTyping     WSMessageType = "stop_typing"
    WSTypeDelivered      WSMessageType = "delivered"
    WSTypeRead           WSMessageType = "read"
    WSTypeOnline         WSMessageType = "online"
    WSTypeOffline        WSMessageType = "offline"
    WSTypeReaction       WSMessageType = "reaction"
    WSTypeMessageDeleted WSMessageType = "message_deleted"
    WSTypeMessageEdited  WSMessageType = "message_edited"
)

// Request DTOs
type CreateConversationRequest struct {
    Type         string   `json:"type" validate:"required,oneof=direct group"`
    Name         string   `json:"name" validate:"required_if=Type group"`
    ParticipantIDs []int64 `json:"participant_ids" validate:"required,min=1"`
}

type SendMessageRequest struct {
    ConversationID  int64           `json:"conversation_id" validate:"required"`
    Content         string          `json:"content" validate:"required_without=MediaURL"`
    MessageType     string          `json:"message_type" validate:"required,oneof=text image video audio file location sticker"`
    MediaURL        string          `json:"media_url" validate:"omitempty,url"`
    ParentMessageID *int64          `json:"parent_message_id,omitempty"`
    Metadata        json.RawMessage `json:"metadata,omitempty"`
}