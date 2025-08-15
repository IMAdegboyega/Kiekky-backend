package notifications

import (
    "context"
    "errors"
    "fmt"
    "log"
    "time"
)

var (
    ErrNotificationNotFound = errors.New("notification not found")
    ErrUnauthorized        = errors.New("unauthorized")
    ErrInvalidChannel      = errors.New("invalid delivery channel")
    ErrTemplateNotFound    = errors.New("template not found")
)

type Service interface {
    // Core notification operations
    SendNotification(ctx context.Context, req *CreateNotificationRequest) (*Notification, error)
    SendBatchNotifications(ctx context.Context, req *BroadcastNotificationRequest) error
    GetNotifications(ctx context.Context, userID int64, limit, offset int, unreadOnly bool) (*NotificationsResponse, error)
    GetNotification(ctx context.Context, notificationID int64, userID int64) (*Notification, error)
    MarkAsRead(ctx context.Context, notificationID int64, userID int64) error
    MarkAllAsRead(ctx context.Context, userID int64) error
    DeleteNotification(ctx context.Context, notificationID int64, userID int64) error
    
    // Push token management
    RegisterPushToken(ctx context.Context, userID int64, req *RegisterPushTokenRequest) error
    UnregisterPushToken(ctx context.Context, token string) error
    
    // Preferences
    GetPreferences(ctx context.Context, userID int64) (*NotificationPreferences, error)
    UpdatePreferences(ctx context.Context, userID int64, req *UpdatePreferencesRequest) error
    
    // Scheduled notifications
    ScheduleNotification(ctx context.Context, req *ScheduleNotificationRequest) (*ScheduledNotification, error)
    CancelScheduledNotification(ctx context.Context, scheduledID int64, userID int64) error
    ProcessScheduledNotifications(ctx context.Context) error
    
    // Utility methods
    SendWelcomeNotification(ctx context.Context, userID int64) error
    SendFollowNotification(ctx context.Context, followerID, followedID int64) error
    SendLikeNotification(ctx context.Context, likerID, postOwnerID int64, postID int64) error
    SendCommentNotification(ctx context.Context, commenterID, postOwnerID int64, postID int64, comment string) error
    SendMessageNotification(ctx context.Context, senderID, receiverID int64, message string) error
    SendMatchNotification(ctx context.Context, user1ID, user2ID int64) error
    
    // Cleanup
    CleanupOldNotifications(ctx context.Context, olderThan time.Duration) error
}

// External service interfaces
type PushService interface {
    SendPush(ctx context.Context, notification *PushNotification) error
    SendBatchPush(ctx context.Context, notifications []*PushNotification) error
}

type EmailService interface {
    SendEmail(ctx context.Context, notification *EmailNotification) error
    SendBatchEmails(ctx context.Context, notifications []*EmailNotification) error
}

type SMSService interface {
    SendSMS(ctx context.Context, notification *SMSNotification) error
    SendBatchSMS(ctx context.Context, notifications []*SMSNotification) error
}

type TemplateService interface {
    RenderTemplate(ctx context.Context, templateType NotificationType, language string, data map[string]interface{}) (title, body string, err error)
}

type service struct {
    repo            Repository
    pushService     PushService
    emailService    EmailService
    smsService      SMSService
    templateService TemplateService
}

func NewService(
    repo Repository,
    pushService PushService,
    emailService EmailService,
    smsService SMSService,
    templateService TemplateService,
) Service {
    return &service{
        repo:            repo,
        pushService:     pushService,
        emailService:    emailService,
        smsService:      smsService,
        templateService: templateService,
    }
}

// SendNotification sends a notification to a user
func (s *service) SendNotification(ctx context.Context, req *CreateNotificationRequest) (*Notification, error) {
    // Check user preferences
    prefs, err := s.repo.GetUserPreferences(ctx, req.UserID)
    if err != nil {
        log.Printf("Failed to get user preferences: %v", err)
        // Continue with default preferences
    }
    
    // Create in-app notification
    notification := &Notification{
        UserID:  req.UserID,
        Type:    req.Type,
        Title:   req.Title,
        Message: req.Message,
        Data:    req.Data,
        IsRead:  false,
    }
    
    if err := s.repo.CreateNotification(ctx, notification); err != nil {
        return nil, err
    }
    
    // Determine delivery channels
    channels := req.Channels
    if len(channels) == 0 {
        channels = s.getDefaultChannels(req.Type, prefs)
    }
    
    // Send through requested channels
    for _, channel := range channels {
        switch channel {
        case ChannelPush:
            if prefs.PushEnabled && s.shouldSendForType(req.Type, prefs) {
                go s.sendPushNotification(ctx, req.UserID, notification)
            }
        case ChannelEmail:
            if prefs.EmailEnabled && s.shouldSendForType(req.Type, prefs) {
                go s.sendEmailNotification(ctx, req.UserID, notification)
            }
        case ChannelSMS:
            if prefs.SMSEnabled && s.shouldSendForType(req.Type, prefs) {
                go s.sendSMSNotification(ctx, req.UserID, notification)
            }
        }
    }
    
    return notification, nil
}

// SendBatchNotifications sends notifications to multiple users
func (s *service) SendBatchNotifications(ctx context.Context, req *BroadcastNotificationRequest) error {
    userIDs := req.UserIDs
    
    // If no specific users, get all users based on preferences
    if len(userIDs) == 0 {
        // This would need to be implemented based on your user service
        // For now, we'll assume userIDs are provided
        if len(userIDs) == 0 {
            return errors.New("no users specified for broadcast")
        }
    }
    
    // Create notifications for all users
    notifications := make([]*Notification, 0, len(userIDs))
    for _, userID := range userIDs {
        notification := &Notification{
            UserID:  userID,
            Type:    req.Type,
            Title:   req.Title,
            Message: req.Message,
            Data:    req.Data,
            IsRead:  false,
        }
        notifications = append(notifications, notification)
    }
    
    // Batch create in-app notifications
    if err := s.repo.CreateBatchNotifications(ctx, notifications); err != nil {
        return err
    }
    
    // Send through requested channels
    for _, channel := range req.Channels {
        switch channel {
        case ChannelPush:
            go s.sendBatchPushNotifications(ctx, userIDs, req.Title, req.Message, req.Data)
        case ChannelEmail:
            go s.sendBatchEmailNotifications(ctx, userIDs, req.Title, req.Message, req.Data)
        case ChannelSMS:
            go s.sendBatchSMSNotifications(ctx, userIDs, req.Message)
        }
    }
    
    return nil
}

// GetNotifications retrieves notifications for a user
func (s *service) GetNotifications(ctx context.Context, userID int64, limit, offset int, unreadOnly bool) (*NotificationsResponse, error) {
    if limit == 0 {
        limit = 20
    }
    
    notifications, err := s.repo.GetUserNotifications(ctx, userID, limit, offset, unreadOnly)
    if err != nil {
        return nil, err
    }
    
    totalCount, err := s.repo.GetUserNotificationCount(ctx, userID, false)
    if err != nil {
        totalCount = len(notifications)
    }
    
    unreadCount, err := s.repo.GetUserNotificationCount(ctx, userID, true)
    if err != nil {
        unreadCount = 0
    }
    
    // Enrich notifications with actor information if needed
    for _, n := range notifications {
        s.enrichNotification(ctx, n)
    }
    
    return &NotificationsResponse{
        Notifications: notifications,
        TotalCount:    totalCount,
        UnreadCount:   unreadCount,
        HasMore:       offset+len(notifications) < totalCount,
    }, nil
}

// GetNotification retrieves a specific notification
func (s *service) GetNotification(ctx context.Context, notificationID int64, userID int64) (*Notification, error) {
    notification, err := s.repo.GetNotification(ctx, notificationID)
    if err != nil {
        return nil, err
    }
    
    if notification == nil {
        return nil, ErrNotificationNotFound
    }
    
    if notification.UserID != userID {
        return nil, ErrUnauthorized
    }
    
    s.enrichNotification(ctx, notification)
    return notification, nil
}

// MarkAsRead marks a notification as read
func (s *service) MarkAsRead(ctx context.Context, notificationID int64, userID int64) error {
    return s.repo.MarkAsRead(ctx, notificationID, userID)
}

// MarkAllAsRead marks all notifications as read for a user
func (s *service) MarkAllAsRead(ctx context.Context, userID int64) error {
    return s.repo.MarkAllAsRead(ctx, userID)
}

// DeleteNotification deletes a notification
func (s *service) DeleteNotification(ctx context.Context, notificationID int64, userID int64) error {
    return s.repo.DeleteNotification(ctx, notificationID, userID)
}

// RegisterPushToken registers a push token for a user
func (s *service) RegisterPushToken(ctx context.Context, userID int64, req *RegisterPushTokenRequest) error {
    token := &PushToken{
        UserID:   userID,
        Platform: req.Platform,
        Token:    req.Token,
        DeviceID: req.DeviceID,
        IsActive: true,
    }
    
    return s.repo.SavePushToken(ctx, token)
}

// UnregisterPushToken unregisters a push token
func (s *service) UnregisterPushToken(ctx context.Context, token string) error {
    return s.repo.DeletePushToken(ctx, token)
}

// GetPreferences retrieves user notification preferences
func (s *service) GetPreferences(ctx context.Context, userID int64) (*NotificationPreferences, error) {
    return s.repo.GetUserPreferences(ctx, userID)
}

// UpdatePreferences updates user notification preferences
func (s *service) UpdatePreferences(ctx context.Context, userID int64, req *UpdatePreferencesRequest) error {
    updates := make(map[string]interface{})
    
    if req.PushEnabled != nil {
        updates["push_enabled"] = *req.PushEnabled
    }
    if req.EmailEnabled != nil {
        updates["email_enabled"] = *req.EmailEnabled
    }
    if req.SMSEnabled != nil {
        updates["sms_enabled"] = *req.SMSEnabled
    }
    if req.Likes != nil {
        updates["likes"] = *req.Likes
    }
    if req.Comments != nil {
        updates["comments"] = *req.Comments
    }
    if req.Follows != nil {
        updates["follows"] = *req.Follows
    }
    if req.Messages != nil {
        updates["messages"] = *req.Messages
    }
    if req.Matches != nil {
        updates["matches"] = *req.Matches
    }
    if req.StoryViews != nil {
        updates["story_views"] = *req.StoryViews
    }
    if req.StoryReplies != nil {
        updates["story_replies"] = *req.StoryReplies
    }
    if req.Mentions != nil {
        updates["mentions"] = *req.Mentions
    }
    if req.Promotions != nil {
        updates["promotions"] = *req.Promotions
    }
    
    return s.repo.UpdateUserPreferences(ctx, userID, updates)
}

// ScheduleNotification schedules a notification for later
func (s *service) ScheduleNotification(ctx context.Context, req *ScheduleNotificationRequest) (*ScheduledNotification, error) {
    scheduled := &ScheduledNotification{
        UserID:       req.UserID,
        Type:         req.Type,
        Title:        req.Title,
        Message:      req.Message,
        Data:         req.Data,
        Channels:     req.Channels,
        ScheduledFor: req.ScheduledFor,
        Status:       "pending",
    }
    
    if err := s.repo.CreateScheduledNotification(ctx, scheduled); err != nil {
        return nil, err
    }
    
    return scheduled, nil
}

// CancelScheduledNotification cancels a scheduled notification
func (s *service) CancelScheduledNotification(ctx context.Context, scheduledID int64, userID int64) error {
    // TODO: Verify ownership
    return s.repo.CancelScheduledNotification(ctx, scheduledID)
}

// ProcessScheduledNotifications processes pending scheduled notifications
func (s *service) ProcessScheduledNotifications(ctx context.Context) error {
    notifications, err := s.repo.GetPendingScheduledNotifications(ctx, time.Now())
    if err != nil {
        return err
    }
    
    for _, scheduled := range notifications {
        // Create and send the notification
        req := &CreateNotificationRequest{
            UserID:   *scheduled.UserID,
            Type:     scheduled.Type,
            Title:    scheduled.Title,
            Message:  scheduled.Message,
            Data:     scheduled.Data,
            Channels: scheduled.Channels,
        }
        
        _, err := s.SendNotification(ctx, req)
        
        status := "sent"
        sentAt := time.Now()
        if err != nil {
            status = "failed"
            log.Printf("Failed to send scheduled notification %d: %v", scheduled.ID, err)
        }
        
        s.repo.UpdateScheduledNotificationStatus(ctx, scheduled.ID, status, &sentAt)
    }
    
    return nil
}

// Utility notification methods

func (s *service) SendWelcomeNotification(ctx context.Context, userID int64) error {
    req := &CreateNotificationRequest{
        UserID:  userID,
        Type:    TypeWelcome,
        Title:   "Welcome to Kiekky! 🎉",
        Message: "Start exploring and connecting with amazing people around you.",
        Data: NotificationData{
            "action": "onboarding",
        },
    }
    
    _, err := s.SendNotification(ctx, req)
    return err
}

func (s *service) SendFollowNotification(ctx context.Context, followerID, followedID int64) error {
    // Get follower info (would need user service)
    followerName := fmt.Sprintf("User %d", followerID)
    
    req := &CreateNotificationRequest{
        UserID:  followedID,
        Type:    TypeFollow,
        Title:   "New Follower! 👥",
        Message: fmt.Sprintf("%s started following you", followerName),
        Data: NotificationData{
            "follower_id": followerID,
            "action":      "profile",
        },
    }
    
    _, err := s.SendNotification(ctx, req)
    return err
}

func (s *service) SendLikeNotification(ctx context.Context, likerID, postOwnerID int64, postID int64) error {
    likerName := fmt.Sprintf("User %d", likerID)
    
    req := &CreateNotificationRequest{
        UserID:  postOwnerID,
        Type:    TypeLike,
        Title:   "Someone liked your post! ❤️",
        Message: fmt.Sprintf("%s liked your post", likerName),
        Data: NotificationData{
            "liker_id": likerID,
            "post_id":  postID,
            "action":   "post",
        },
    }
    
    _, err := s.SendNotification(ctx, req)
    return err
}

func (s *service) SendCommentNotification(ctx context.Context, commenterID, postOwnerID int64, postID int64, comment string) error {
    commenterName := fmt.Sprintf("User %d", commenterID)
    
    // Truncate comment if too long
    if len(comment) > 50 {
        comment = comment[:47] + "..."
    }
    
    req := &CreateNotificationRequest{
        UserID:  postOwnerID,
        Type:    TypeComment,
        Title:   "New comment on your post 💬",
        Message: fmt.Sprintf("%s: %s", commenterName, comment),
        Data: NotificationData{
            "commenter_id": commenterID,
            "post_id":      postID,
            "comment":      comment,
            "action":       "post",
        },
    }
    
    _, err := s.SendNotification(ctx, req)
    return err
}

func (s *service) SendMessageNotification(ctx context.Context, senderID, receiverID int64, message string) error {
    senderName := fmt.Sprintf("User %d", senderID)
    
    // Truncate message if too long
    if len(message) > 50 {
        message = message[:47] + "..."
    }
    
    req := &CreateNotificationRequest{
        UserID:  receiverID,
        Type:    TypeMessage,
        Title:   fmt.Sprintf("Message from %s 💌", senderName),
        Message: message,
        Data: NotificationData{
            "sender_id": senderID,
            "action":    "chat",
        },
    }
    
    _, err := s.SendNotification(ctx, req)
    return err
}

func (s *service) SendMatchNotification(ctx context.Context, user1ID, user2ID int64) error {
    // Send to both users
    req1 := &CreateNotificationRequest{
        UserID:  user1ID,
        Type:    TypeMatch,
        Title:   "It's a Match! 💕",
        Message: "You have a new match! Start a conversation now.",
        Data: NotificationData{
            "matched_user_id": user2ID,
            "action":          "chat",
        },
    }
    
    req2 := &CreateNotificationRequest{
        UserID:  user2ID,
        Type:    TypeMatch,
        Title:   "It's a Match! 💕",
        Message: "You have a new match! Start a conversation now.",
        Data: NotificationData{
            "matched_user_id": user1ID,
            "action":          "chat",
        },
    }
    
    s.SendNotification(ctx, req1)
    s.SendNotification(ctx, req2)
    
    return nil
}

// CleanupOldNotifications removes old notifications
func (s *service) CleanupOldNotifications(ctx context.Context, olderThan time.Duration) error {
    before := time.Now().Add(-olderThan)
    return s.repo.DeleteOldNotifications(ctx, before)
}

// Helper methods

func (s *service) getDefaultChannels(notificationType NotificationType, prefs *NotificationPreferences) []DeliveryChannel {
    channels := []DeliveryChannel{ChannelInApp}
    
    if prefs.PushEnabled {
        channels = append(channels, ChannelPush)
    }
    
    // Critical notifications also go to email
    switch notificationType {
    case TypeSecurity, TypeVerification:
        if prefs.EmailEnabled {
            channels = append(channels, ChannelEmail)
        }
    }
    
    return channels
}

func (s *service) shouldSendForType(notificationType NotificationType, prefs *NotificationPreferences) bool {
    switch notificationType {
    case TypeLike:
        return prefs.Likes
    case TypeComment:
        return prefs.Comments
    case TypeFollow:
        return prefs.Follows
    case TypeMessage:
        return prefs.Messages
    case TypeMatch:
        return prefs.Matches
    case TypeStoryView:
        return prefs.StoryViews
    case TypeStoryReply:
        return prefs.StoryReplies
    case TypeMention:
        return prefs.Mentions
    case TypePromotion:
        return prefs.Promotions
    default:
        return true
    }
}

func (s *service) enrichNotification(ctx context.Context, notification *Notification) {
    // Add actor information based on notification data
    if actorID, ok := notification.Data["actor_id"].(float64); ok {
        // Would need to fetch user info from user service
        notification.Actor = &NotificationActor{
            ID:          int64(actorID),
            Username:    fmt.Sprintf("user_%d", int64(actorID)),
            DisplayName: fmt.Sprintf("User %d", int64(actorID)),
        }
    }
    
    // Add action URL based on notification type
    switch notification.Type {
    case TypeLike, TypeComment:
        if postID, ok := notification.Data["post_id"].(float64); ok {
            notification.ActionURL = fmt.Sprintf("/posts/%d", int64(postID))
        }
    case TypeFollow:
        if followerID, ok := notification.Data["follower_id"].(float64); ok {
            notification.ActionURL = fmt.Sprintf("/users/%d", int64(followerID))
        }
    case TypeMessage, TypeMatch:
        notification.ActionURL = "/messages"
    }
}

// Channel-specific sending methods

func (s *service) sendPushNotification(ctx context.Context, userID int64, notification *Notification) {
    if s.pushService == nil {
        return
    }
    
    tokens, err := s.repo.GetUserPushTokens(ctx, userID, nil)
    if err != nil || len(tokens) == 0 {
        return
    }
    
    tokenStrings := make([]string, len(tokens))
    for i, token := range tokens {
        tokenStrings[i] = token.Token
    }
    
    push := &PushNotification{
        Tokens: tokenStrings,
        Title:  notification.Title,
        Body:   notification.Message,
        Data: map[string]string{
            "notification_id": fmt.Sprintf("%d", notification.ID),
            "type":           string(notification.Type),
        },
    }
    
    if err := s.pushService.SendPush(ctx, push); err != nil {
        log.Printf("Failed to send push notification: %v", err)
    }
}

func (s *service) sendEmailNotification(ctx context.Context, userID int64, notification *Notification) {
    if s.emailService == nil {
        return
    }
    
    // Would need to get user email from user service
    email := &EmailNotification{
        To:      fmt.Sprintf("user%d@example.com", userID),
        Subject: notification.Title,
        Body:    notification.Message,
    }
    
    if err := s.emailService.SendEmail(ctx, email); err != nil {
        log.Printf("Failed to send email notification: %v", err)
    }
}

func (s *service) sendSMSNotification(ctx context.Context, userID int64, notification *Notification) {
    if s.smsService == nil {
        return
    }
    
    // Would need to get user phone from user service
    sms := &SMSNotification{
        To:      fmt.Sprintf("+1234567890%d", userID),
        Message: fmt.Sprintf("%s: %s", notification.Title, notification.Message),
    }
    
    if err := s.smsService.SendSMS(ctx, sms); err != nil {
        log.Printf("Failed to send SMS notification: %v", err)
    }
}

func (s *service) sendBatchPushNotifications(ctx context.Context, userIDs []int64, title, message string, data NotificationData) {
    if s.pushService == nil {
        return
    }
    
    tokens, err := s.repo.GetAllActivePushTokens(ctx, userIDs)
    if err != nil || len(tokens) == 0 {
        return
    }
    
    // Group tokens by platform for optimized sending
    platformTokens := make(map[Platform][]string)
    for _, token := range tokens {
        platformTokens[token.Platform] = append(platformTokens[token.Platform], token.Token)
    }
    
    for platform, tokenList := range platformTokens {
        push := &PushNotification{
            Tokens: tokenList,
            Title:  title,
            Body:   message,
            Data: map[string]string{
                "platform": string(platform),
            },
        }
        
        if err := s.pushService.SendBatchPush(ctx, []*PushNotification{push}); err != nil {
            log.Printf("Failed to send batch push notifications: %v", err)
        }
    }
}

func (s *service) sendBatchEmailNotifications(ctx context.Context, userIDs []int64, title, message string, data NotificationData) {
    if s.emailService == nil {
        return
    }
    
    // Would need to get user emails from user service
    emails := make([]*EmailNotification, 0, len(userIDs))
    for _, userID := range userIDs {
        email := &EmailNotification{
            To:      fmt.Sprintf("user%d@example.com", userID),
            Subject: title,
            Body:    message,
        }
        emails = append(emails, email)
    }
    
    if err := s.emailService.SendBatchEmails(ctx, emails); err != nil {
        log.Printf("Failed to send batch email notifications: %v", err)
    }
}

func (s *service) sendBatchSMSNotifications(ctx context.Context, userIDs []int64, message string) {
    if s.smsService == nil {
        return
    }
    
    // Would need to get user phones from user service
    smsList := make([]*SMSNotification, 0, len(userIDs))
    for _, userID := range userIDs {
        sms := &SMSNotification{
            To:      fmt.Sprintf("+1234567890%d", userID),
            Message: message,
        }
        smsList = append(smsList, sms)
    }
    
    if err := s.smsService.SendBatchSMS(ctx, smsList); err != nil {
        log.Printf("Failed to send batch SMS notifications: %v", err)
    }
}