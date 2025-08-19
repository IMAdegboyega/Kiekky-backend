// internal/messaging/repository.go

package messaging

import (
    "context"
    "time"
)

type Repository interface {
    // Conversations
    CreateConversation(ctx context.Context, conv *Conversation) error
    GetConversation(ctx context.Context, id int64) (*Conversation, error)
    GetUserConversations(ctx context.Context, userID int64, limit, offset int) ([]*Conversation, error)
    UpdateConversation(ctx context.Context, id int64, updates map[string]interface{}) error
    DeleteConversation(ctx context.Context, id int64) error
    GetDirectConversation(ctx context.Context, user1ID, user2ID int64) (*Conversation, error)
    UpdateConversationLastMessage(ctx context.Context, convID, messageID int64, preview *string) error
    
    // Participants
    AddParticipant(ctx context.Context, participant *Participant) error
    RemoveParticipant(ctx context.Context, convID, userID int64) error
    GetConversationParticipants(ctx context.Context, convID int64) ([]*Participant, error)
    IsUserInConversation(ctx context.Context, userID, convID int64) (bool, error)
    UpdateLastRead(ctx context.Context, convID, userID, messageID int64) error
    IncrementUnreadCount(ctx context.Context, convID, userID int64) error
    ResetUnreadCount(ctx context.Context, convID, userID int64) error
    UpdateTypingStatus(ctx context.Context, convID, userID int64, isTyping bool) error
    
    // Messages
    CreateMessage(ctx context.Context, message *Message) error
    GetMessage(ctx context.Context, id int64) (*Message, error)
    GetConversationMessages(ctx context.Context, convID int64, limit, offset int) ([]*Message, error)
    GetUndeliveredMessages(ctx context.Context, userID int64) ([]*Message, error)
    UpdateMessage(ctx context.Context, id int64, content string) error
    DeleteMessage(ctx context.Context, id int64) error
    SearchMessages(ctx context.Context, userID int64, query string, limit int) ([]*Message, error)
    MarkMessageDelivered(ctx context.Context, messageID, userID int64) error
    
    // Receipts
    CreateReceipt(ctx context.Context, receipt *Receipt) error
    UpdateReceipt(ctx context.Context, receipt *Receipt) error
    GetMessageReceipts(ctx context.Context, messageID int64) ([]*Receipt, error)
    
    // Reactions
    AddReaction(ctx context.Context, reaction *Reaction) error
    RemoveReaction(ctx context.Context, messageID, userID int64, reaction string) error
    GetMessageReactions(ctx context.Context, messageID int64) ([]*Reaction, error)
    
    // Push tokens
    SavePushToken(ctx context.Context, userID int64, token, platform, deviceID string) error
    DeletePushToken(ctx context.Context, token string) error
    GetUserPushTokens(ctx context.Context, userID int64) ([]*PushToken, error)
    
    // Blocking
    BlockUser(ctx context.Context, userID, blockedUserID int64) error
    UnblockUser(ctx context.Context, userID, blockedUserID int64) error
    IsBlocked(ctx context.Context, userID, targetUserID int64) (bool, error)
    
    // User info
    GetUserInfo(ctx context.Context, userID int64) (*UserInfo, error)
    GetUserContacts(ctx context.Context, userID int64) ([]int64, error)
    UpdateUserOnlineStatus(ctx context.Context, userID int64, isOnline bool, lastSeen time.Time) error
    GetTypingUsers(ctx context.Context, conversationID int64) ([]int64, error)

    // Cleanup methods
    DeleteExpiredMessages(ctx context.Context) error
    DeleteOldReceipts(ctx context.Context, age time.Duration) error
}

// PushToken represents a device push notification token
type PushToken struct {
    ID         int64     `json:"id" db:"id"`
    UserID     int64     `json:"user_id" db:"user_id"`
    Token      string    `json:"token" db:"token"`
    Platform   string    `json:"platform" db:"platform"`
    DeviceID   string    `json:"device_id" db:"device_id"`
    IsActive   bool      `json:"is_active" db:"is_active"`
    LastUsedAt *time.Time `json:"last_used_at" db:"last_used_at"`
    CreatedAt  time.Time `json:"created_at" db:"created_at"`
    UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}