// internal/messaging/service.go

package messaging

import (
    "context"
    "errors"
    "fmt"
    "time"
)

var (
    ErrConversationNotFound = errors.New("conversation not found")
    ErrMessageNotFound = errors.New("message not found")
    ErrUnauthorized = errors.New("unauthorized")
    ErrBlocked = errors.New("user is blocked")
    ErrNotParticipant = errors.New("not a participant in this conversation")
)

type Service interface {
    // Conversation management
    CreateConversation(ctx context.Context, userID int64, req *CreateConversationRequest) (*Conversation, error)
    GetConversation(ctx context.Context, conversationID, userID int64) (*Conversation, error)
    GetUserConversations(ctx context.Context, userID int64, limit, offset int) ([]*Conversation, error)
    GetConversationParticipants(ctx context.Context, conversationID int64) ([]*Participant, error)
    IsUserInConversation(ctx context.Context, userID, conversationID int64) bool
    
    // Messages
    SendMessage(ctx context.Context, userID int64, req *SendMessageRequest) (*Message, error)
    GetMessage(ctx context.Context, messageID int64) (*Message, error)
    GetConversationMessages(ctx context.Context, conversationID, userID int64, limit, offset int) ([]*Message, error)
    EditMessage(ctx context.Context, messageID, userID int64, content string) (*Message, error)
    DeleteMessage(ctx context.Context, messageID, userID int64) error
    
    // Message status
    MarkMessageDelivered(ctx context.Context, messageID, userID int64) error
    MarkMessagesRead(ctx context.Context, userID int64, messageIDs []int64) ([]*Receipt, error)
    GetUndeliveredMessages(ctx context.Context, userID int64) ([]*Message, error)
    
    // Reactions
    AddReaction(ctx context.Context, userID, messageID int64, reaction string) (*Reaction, error)
    RemoveReaction(ctx context.Context, userID, messageID int64, reaction string) error
    
    // Typing indicators
    UpdateTypingStatus(ctx context.Context, userID, conversationID int64, isTyping bool) error
    
    // Online status
    UpdateOnlineStatus(ctx context.Context, userID int64, isOnline bool) error
    
    // Push notifications
    RegisterPushToken(ctx context.Context, userID int64, req *PushTokenRequest) error
    
    // Blocking
    BlockUser(ctx context.Context, userID, blockedUserID int64) error
    UnblockUser(ctx context.Context, userID, blockedUserID int64) error
    IsBlocked(ctx context.Context, userID, targetUserID int64) bool
    
    // Utilities
    GetUserContacts(ctx context.Context, userID int64) ([]int64, error)
    GetOrCreateDirectConversation(ctx context.Context, user1ID, user2ID int64) (*Conversation, error)
}

type service struct {
    repo           Repository
    hub            *Hub
    storageService StorageService
    pushService    PushService
}

func NewService(repo Repository, storageService StorageService, pushService PushService) Service {
    return &service{
        repo:           repo,
        storageService: storageService,
        pushService:    pushService,
    }
}

// SetHub sets the hub after initialization to avoid circular dependency
func (s *service) SetHub(hub *Hub) {
    s.hub = hub
}

// SendMessage sends a new message
func (s *service) SendMessage(ctx context.Context, userID int64, req *SendMessageRequest) (*Message, error) {
    // Verify user is participant
    if !s.IsUserInConversation(ctx, userID, req.ConversationID) {
        return nil, ErrNotParticipant
    }
    
    // Check for blocked users
    participants, _ := s.repo.GetConversationParticipants(ctx, req.ConversationID)
    for _, p := range participants {
        if p.UserID != userID && s.IsBlocked(ctx, userID, p.UserID) {
            return nil, ErrBlocked
        }
    }
    
    // Handle media upload if needed
    var mediaURL, thumbnailURL string
    var mediaSize, mediaDuration int
    
    if req.MediaURL != "" {
        // Process media (generate thumbnail, get metadata)
        mediaInfo, err := s.storageService.ProcessMedia(ctx, req.MediaURL, req.MessageType)
        if err != nil {
            return nil, err
        }
        
        mediaURL = mediaInfo.URL
        thumbnailURL = mediaInfo.ThumbnailURL
        mediaSize = mediaInfo.Size
        mediaDuration = mediaInfo.Duration
    }
    
    // Create message
    message := &Message{
        ConversationID:    req.ConversationID,
        SenderID:         userID,
        ParentMessageID:  req.ParentMessageID,
        Content:          &req.Content,
        MessageType:      req.MessageType,
        MediaURL:         &mediaURL,
        MediaThumbnailURL: &thumbnailURL,
        MediaSize:        &mediaSize,
        MediaDuration:    &mediaDuration,
        Metadata:         req.Metadata,
        CreatedAt:        time.Now(),
    }
    
    // Save to database
    if err := s.repo.CreateMessage(ctx, message); err != nil {
        return nil, err
    }
    
    // Update conversation last message
    s.repo.UpdateConversationLastMessage(ctx, req.ConversationID, message.ID, message.Content)
    
    // Update unread counts for other participants
    for _, p := range participants {
        if p.UserID != userID {
            s.repo.IncrementUnreadCount(ctx, req.ConversationID, p.UserID)
        }
    }
    
    // Load sender info
    message.Sender, _ = s.repo.GetUserInfo(ctx, userID)
    
    // Send push notifications to offline users
    go s.sendMessageNotifications(ctx, message, participants)
    
    return message, nil
}

// Additional implementation methods...