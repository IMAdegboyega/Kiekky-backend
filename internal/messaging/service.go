// internal/messaging/service.go

package messaging

import (
    "context"
    "errors"
    "fmt"
    "time"
    "log"
    "io"
    "mime/multipart"

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

    // Hub management
    SetHub(hub *Hub)
    
    // Missing cleanup methods
    CleanupExpiredMessages(ctx context.Context) error
    CleanupOldReceipts(ctx context.Context, age time.Duration) error
    
    // Missing methods called by handlers
    GetPendingMessages(ctx context.Context, userID int64) ([]*Message, error)
    GetPushTokens(ctx context.Context, userID int64) ([]*PushToken, error)
    SendPushNotification(ctx context.Context, tokens []*PushToken, message WSMessage) error
    DeleteConversation(ctx context.Context, userID, conversationID int64) error
    AddParticipant(ctx context.Context, conversationID, userID int64) error
    RemoveParticipant(ctx context.Context, userID, conversationID, targetUserID int64) error
    MuteConversation(ctx context.Context, userID, conversationID int64) error
    UnmuteConversation(ctx context.Context, userID, conversationID int64) error
    ArchiveConversation(ctx context.Context, userID, conversationID int64) error
    UnarchiveConversation(ctx context.Context, userID, conversationID int64) error
    GetReactions(ctx context.Context, messageID int64) ([]*Reaction, error)
    UnregisterPushToken(ctx context.Context, token string) error
    SearchMessages(ctx context.Context, userID int64, query string) ([]*Message, error)
    GetBlockedUsers(ctx context.Context, userID int64) ([]*UserInfo, error)
    UploadMedia(ctx context.Context, userID int64, file io.Reader, header *multipart.FileHeader) (string, error)
    GetContactsOnlineStatus(ctx context.Context, userID int64) (map[int64]bool, error)
}

// Update service struct to export it:
type MessageService struct { // Changed from 'service' to 'MessageService' and exported
    repo           Repository
    hub            *Hub
    storageService StorageService
    pushService    PushService
}

// Update NewService to return concrete type for type assertion:
func NewService(repo Repository, storageService StorageService, pushService PushService) *MessageService {
    return &MessageService{
        repo:           repo,
        storageService: storageService,
        pushService:    pushService,
    }
}

// SetHub sets the hub after initialization to avoid circular dependency
func (s *MessageService) SetHub(hub *Hub) {
    s.hub = hub
}

// SendMessage sends a new message
func (s *MessageService) SendMessage(ctx context.Context, userID int64, req *SendMessageRequest) (*Message, error) {
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

func (s *MessageService) IsUserInConversation(ctx context.Context, userID, conversationID int64) bool {
    isIn, err := s.repo.IsUserInConversation(ctx, userID, conversationID)
    if err != nil {
        return false
    }
    return isIn
}

func (s *MessageService) IsBlocked(ctx context.Context, userID, targetUserID int64) bool {
    blocked, err := s.repo.IsBlocked(ctx, userID, targetUserID)
    if err != nil {
        return false
    }
    return blocked
}

func (s *MessageService) sendMessageNotifications(ctx context.Context, message *Message, participants []*Participant) {
    // Skip if no push service
    if s.pushService == nil {
        return
    }
    
    // Get sender info
    sender, _ := s.repo.GetUserInfo(ctx, message.SenderID)
    if sender == nil {
        return
    }
    
    // Prepare notification
    title := sender.DisplayName
    body := message.Content
    if body == nil || *body == "" {
        switch message.MessageType {
        case "image":
            body = ptr("Sent an image")
        case "video":
            body = ptr("Sent a video")
        case "audio":
            body = ptr("Sent an audio message")
        case "file":
            body = ptr("Sent a file")
        default:
            body = ptr("Sent a message")
        }
    }
    
    notification := &PushNotification{
        Title: title,
        Body:  *body,
        Data: map[string]string{
            "type":            "message",
            "conversation_id": fmt.Sprintf("%d", message.ConversationID),
            "message_id":      fmt.Sprintf("%d", message.ID),
            "sender_id":       fmt.Sprintf("%d", message.SenderID),
        },
    }
    
    // Send to offline participants
    for _, participant := range participants {
        if participant.UserID == message.SenderID {
            continue
        }
        
        // Check if user is online
        if s.hub != nil && s.hub.IsUserOnline(participant.UserID) {
            continue
        }
        
        // Send push notification
        go s.pushService.SendNotification(ctx, participant.UserID, notification)
    }
}

func (s *MessageService) CleanupExpiredMessages(ctx context.Context) error {
    // Delete messages that have expired (for disappearing messages feature)
    query := `
        DELETE FROM messages 
        WHERE expires_at IS NOT NULL 
        AND expires_at < NOW()
    `
    _, err := s.repo.(*postgresRepository).db.ExecContext(ctx, query)
    return err
}

func (s *MessageService) CleanupOldReceipts(ctx context.Context, age time.Duration) error {
    // Delete old read receipts for performance
    query := `
        DELETE FROM message_receipts 
        WHERE created_at < $1
    `
    _, err := s.repo.(*postgresRepository).db.ExecContext(ctx, query, time.Now().Add(-age))
    return err
}

func (s *MessageService) GetPendingMessages(ctx context.Context, userID int64) ([]*Message, error) {
    // Get undelivered messages for a user
    return s.repo.GetUndeliveredMessages(ctx, userID)
}

func (s *MessageService) MarkMessageDelivered(ctx context.Context, messageID int64) error {
    // Get message to find recipients
    message, err := s.repo.GetMessage(ctx, messageID)
    if err != nil {
        return err
    }
    
    // Get conversation participants
    participants, err := s.repo.GetConversationParticipants(ctx, message.ConversationID)
    if err != nil {
        return err
    }
    
    // Mark delivered for all participants except sender
    for _, p := range participants {
        if p.UserID != message.SenderID {
            s.repo.MarkMessageDelivered(ctx, messageID, p.UserID)
        }
    }
    
    return nil
}

func (s *MessageService) GetPushTokens(ctx context.Context, userID int64) ([]*PushToken, error) {
    return s.repo.GetUserPushTokens(ctx, userID)
}

func (s *MessageService) SendPushNotification(ctx context.Context, tokens []*PushToken, message WSMessage) error {
    // Convert WSMessage to notification format
    var title, body string
    
    // Parse message data
    switch message.Type {
    case "message":
        title = "New Message"
        body = "You have a new message"
    case "typing":
        return nil // Don't send push for typing
    default:
        title = "Notification"
        body = "You have a new notification"
    }
    
    notification := &PushNotification{
        Title: title,
        Body:  body,
        Data: map[string]string{
            "type": message.Type,
        },
    }
    
    // Send to each token
    for _, token := range tokens {
        if err := s.pushService.SendNotification(ctx, token.UserID, notification); err != nil {
            log.Printf("Failed to send push notification: %v", err)
        }
    }
    
    return nil
}

func (s *MessageService) GetUserContacts(ctx context.Context, userID int64) ([]int64, error) {
    return s.repo.GetUserContacts(ctx, userID)
}

func (s *MessageService) UpdateOnlineStatus(ctx context.Context, userID int64, isOnline bool) error {
    lastSeen := time.Now()
    return s.repo.UpdateUserOnlineStatus(ctx, userID, isOnline, lastSeen)
}

// Helper function
func ptr(s string) *string {
    return &s
}