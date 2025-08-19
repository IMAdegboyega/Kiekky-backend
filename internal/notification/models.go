// internal/notification/models.go

package notifications

import (
    "database/sql/driver"
    "encoding/json"
    "time"
)

// NotificationType represents different notification types
type NotificationType string

const (
    // Social notifications
    TypeLike           NotificationType = "like"
    TypeComment        NotificationType = "comment"
    TypeFollow         NotificationType = "follow"
    TypeMessage        NotificationType = "message"
    TypeMatch          NotificationType = "match"
    TypeStoryView      NotificationType = "story_view"
    TypeStoryReply     NotificationType = "story_reply"
    TypeMention        NotificationType = "mention"
    
    // System notifications
    TypeWelcome        NotificationType = "welcome"
    TypeProfileUpdate  NotificationType = "profile_update"
    TypeVerification   NotificationType = "verification"
    TypeSecurity       NotificationType = "security"
    TypePromotion      NotificationType = "promotion"
    TypeMaintenance    NotificationType = "maintenance"
)

// DeliveryChannel represents notification delivery channels
type DeliveryChannel string

const (
    ChannelPush    DeliveryChannel = "push"
    ChannelInApp   DeliveryChannel = "in_app"
    ChannelEmail   DeliveryChannel = "email"
    ChannelSMS     DeliveryChannel = "sms"
)

// Platform represents device platforms
type Platform string

const (
    PlatformIOS     Platform = "ios"
    PlatformAndroid Platform = "android"
    PlatformWeb     Platform = "web"
)

// Priority represents notification priority levels
type Priority string

const (
    PriorityHigh   Priority = "high"
    PriorityMedium Priority = "medium"
    PriorityLow    Priority = "low"
)

// Notification represents a notification entity
type Notification struct {
    ID          int64            `json:"id" db:"id"`
    UserID      int64            `json:"user_id" db:"user_id"`
    Type        NotificationType `json:"type" db:"type"`
    Title       string           `json:"title" db:"title"`
    Message     string           `json:"message" db:"message"`
    Data        NotificationData `json:"data" db:"data"`
    IsRead      bool             `json:"is_read" db:"is_read"`
    ReadAt      *time.Time       `json:"read_at,omitempty" db:"read_at"`
    CreatedAt   time.Time        `json:"created_at" db:"created_at"`
    
    // Additional fields for response
    Actor       *NotificationActor `json:"actor,omitempty"`
    ActionURL   string            `json:"action_url,omitempty"`
}

// NotificationData represents additional notification data
type NotificationData map[string]interface{}

// Scan implements sql.Scanner interface
func (nd *NotificationData) Scan(value interface{}) error {
    if value == nil {
        *nd = make(NotificationData)
        return nil
    }
    
    bytes, ok := value.([]byte)
    if !ok {
        return nil
    }
    
    return json.Unmarshal(bytes, nd)
}

// Value implements driver.Valuer interface
func (nd NotificationData) Value() (driver.Value, error) {
    if nd == nil {
        return "{}", nil
    }
    return json.Marshal(nd)
}

// NotificationActor represents the user who triggered the notification
type NotificationActor struct {
    ID             int64   `json:"id"`
    Username       string  `json:"username"`
    DisplayName    string  `json:"display_name"`
    ProfilePicture *string `json:"profile_picture,omitempty"`
}

// PushToken represents a device push token
type PushToken struct {
    ID        int64     `json:"id" db:"id"`
    UserID    int64     `json:"user_id" db:"user_id"`
    Platform  Platform  `json:"platform" db:"platform"`
    Token     string    `json:"token" db:"token"`
    DeviceID  string    `json:"device_id" db:"device_id"`
    IsActive  bool      `json:"is_active" db:"is_active"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// NotificationPreferences represents user notification preferences
type NotificationPreferences struct {
    ID              int64     `json:"id" db:"id"`
    UserID          int64     `json:"user_id" db:"user_id"`
    PushEnabled     bool      `json:"push_enabled" db:"push_enabled"`
    EmailEnabled    bool      `json:"email_enabled" db:"email_enabled"`
    SMSEnabled      bool      `json:"sms_enabled" db:"sms_enabled"`
    
    // Specific notification types
    Likes           bool      `json:"likes" db:"likes"`
    Comments        bool      `json:"comments" db:"comments"`
    Follows         bool      `json:"follows" db:"follows"`
    Messages        bool      `json:"messages" db:"messages"`
    Matches         bool      `json:"matches" db:"matches"`
    StoryViews      bool      `json:"story_views" db:"story_views"`
    StoryReplies    bool      `json:"story_replies" db:"story_replies"`
    Mentions        bool      `json:"mentions" db:"mentions"`
    Promotions      bool      `json:"promotions" db:"promotions"`
    
    UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// ScheduledNotification represents a scheduled notification
type ScheduledNotification struct {
    ID           int64            `json:"id" db:"id"`
    UserID       *int64           `json:"user_id,omitempty" db:"user_id"` // Null for broadcast
    Type         NotificationType `json:"type" db:"type"`
    Title        string           `json:"title" db:"title"`
    Message      string           `json:"message" db:"message"`
    Data         NotificationData `json:"data" db:"data"`
    Channels     []DeliveryChannel `json:"channels" db:"channels"`
    ScheduledFor time.Time        `json:"scheduled_for" db:"scheduled_for"`
    Status       string           `json:"status" db:"status"` // pending, sent, failed, cancelled
    SentAt       *time.Time       `json:"sent_at,omitempty" db:"sent_at"`
    CreatedAt    time.Time        `json:"created_at" db:"created_at"`
}

// NotificationTemplate represents a notification template
type NotificationTemplate struct {
    ID            int64            `json:"id" db:"id"`
    Type          NotificationType `json:"type" db:"type"`
    Language      string           `json:"language" db:"language"`
    TitleTemplate string           `json:"title_template" db:"title_template"`
    BodyTemplate  string           `json:"body_template" db:"body_template"`
    Variables     []string         `json:"variables" db:"variables"`
    CreatedAt     time.Time        `json:"created_at" db:"created_at"`
    UpdatedAt     time.Time        `json:"updated_at" db:"updated_at"`
}

// EmailNotification represents an email notification
type EmailNotification struct {
    To          string
    Subject     string
    Body        string
    HTML        string
    TemplateID  string
    Variables   map[string]interface{}
}

// SMSNotification represents an SMS notification
type SMSNotification struct {
    To          string
    Message     string
    TemplateID  string
    Variables   map[string]interface{}
}

// PushNotification represents a push notification
type PushNotification struct {
    Tokens      []string
    Title       string
    Body        string
    Data        map[string]string
    Badge       int
    Sound       string
    Priority    Priority
    CollapseKey string
    Image       string
}

// CreateNotificationRequest represents request to create a notification
type CreateNotificationRequest struct {
    UserID   int64            `json:"user_id" validate:"required"`
    Type     NotificationType `json:"type" validate:"required"`
    Title    string           `json:"title" validate:"required,max=200"`
    Message  string           `json:"message" validate:"required"`
    Data     NotificationData `json:"data,omitempty"`
    Channels []DeliveryChannel `json:"channels,omitempty"`
}

// BroadcastNotificationRequest represents request to broadcast notifications
type BroadcastNotificationRequest struct {
    UserIDs  []int64          `json:"user_ids,omitempty"` // Empty means all users
    Type     NotificationType `json:"type" validate:"required"`
    Title    string           `json:"title" validate:"required,max=200"`
    Message  string           `json:"message" validate:"required"`
    Data     NotificationData `json:"data,omitempty"`
    Channels []DeliveryChannel `json:"channels" validate:"required,min=1"`
}

// ScheduleNotificationRequest represents request to schedule a notification
type ScheduleNotificationRequest struct {
    UserID       *int64           `json:"user_id,omitempty"`
    Type         NotificationType `json:"type" validate:"required"`
    Title        string           `json:"title" validate:"required,max=200"`
    Message      string           `json:"message" validate:"required"`
    Data         NotificationData `json:"data,omitempty"`
    Channels     []DeliveryChannel `json:"channels" validate:"required,min=1"`
    ScheduledFor time.Time        `json:"scheduled_for" validate:"required"`
}

// RegisterPushTokenRequest represents request to register a push token
type RegisterPushTokenRequest struct {
    Platform Platform `json:"platform" validate:"required,oneof=ios android web"`
    Token    string   `json:"token" validate:"required"`
    DeviceID string   `json:"device_id" validate:"required"`
}

// UpdatePreferencesRequest represents request to update notification preferences
type UpdatePreferencesRequest struct {
    PushEnabled     *bool `json:"push_enabled,omitempty"`
    EmailEnabled    *bool `json:"email_enabled,omitempty"`
    SMSEnabled      *bool `json:"sms_enabled,omitempty"`
    Likes           *bool `json:"likes,omitempty"`
    Comments        *bool `json:"comments,omitempty"`
    Follows         *bool `json:"follows,omitempty"`
    Messages        *bool `json:"messages,omitempty"`
    Matches         *bool `json:"matches,omitempty"`
    StoryViews      *bool `json:"story_views,omitempty"`
    StoryReplies    *bool `json:"story_replies,omitempty"`
    Mentions        *bool `json:"mentions,omitempty"`
    Promotions      *bool `json:"promotions,omitempty"`
}

// NotificationsResponse represents paginated notifications response
type NotificationsResponse struct {
    Notifications []*Notification `json:"notifications"`
    TotalCount    int             `json:"total_count"`
    UnreadCount   int             `json:"unread_count"`
    HasMore       bool            `json:"has_more"`
}