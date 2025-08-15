// internal/messaging/notifications.go

package messaging

import (
    "context"
    "fmt"
    "log"
    
    firebase "firebase.google.com/go/v4"
    "firebase.google.com/go/v4/messaging"
    "google.golang.org/api/option"
)

type PushService interface {
    SendNotification(ctx context.Context, userID int64, notification *PushNotification) error
    SendBulkNotifications(ctx context.Context, notifications []*BulkNotification) error
}

type PushNotification struct {
    Title    string            `json:"title"`
    Body     string            `json:"body"`
    ImageURL string            `json:"image_url,omitempty"`
    Data     map[string]string `json:"data,omitempty"`
    Badge    int              `json:"badge,omitempty"`
    Sound    string           `json:"sound,omitempty"`
}

type BulkNotification struct {
    UserIDs      []int64
    Notification *PushNotification
}

type pushService struct {
    fcmClient *messaging.Client
    repo      Repository
}

// NewPushService creates a new push notification service
func NewPushService(credentialsPath string, repo Repository) (PushService, error) {
    ctx := context.Background()
    
    opt := option.WithCredentialsFile(credentialsPath)
    app, err := firebase.NewApp(ctx, nil, opt)
    if err != nil {
        return nil, fmt.Errorf("error initializing firebase app: %v", err)
    }
    
    client, err := app.Messaging(ctx)
    if err != nil {
        return nil, fmt.Errorf("error getting messaging client: %v", err)
    }
    
    return &pushService{
        fcmClient: client,
        repo:      repo,
    }, nil
}

// SendNotification sends a push notification to a specific user
func (s *pushService) SendNotification(ctx context.Context, userID int64, notification *PushNotification) error {
    // Get user's push tokens
    tokens, err := s.repo.GetUserPushTokens(ctx, userID)
    if err != nil {
        return fmt.Errorf("failed to get user tokens: %v", err)
    }
    
    if len(tokens) == 0 {
        log.Printf("No push tokens found for user %d", userID)
        return nil
    }
    
    // Send to all user's devices
    for _, token := range tokens {
        if !token.IsActive {
            continue
        }
        
        message := &messaging.Message{
            Token: token.Token,
            Notification: &messaging.Notification{
                Title:    notification.Title,
                Body:     notification.Body,
                ImageURL: notification.ImageURL,
            },
            Data: notification.Data,
        }
        
        // Platform-specific configuration
        switch token.Platform {
        case "ios":
            message.APNS = &messaging.APNSConfig{
                Payload: &messaging.APNSPayload{
                    Aps: &messaging.Aps{
                        Badge: &notification.Badge,
                        Sound: notification.Sound,
                    },
                },
            }
        case "android":
            message.Android = &messaging.AndroidConfig{
                Priority: "high",
                Notification: &messaging.AndroidNotification{
                    Sound:    notification.Sound,
                    Priority: messaging.PriorityHigh,
                },
            }
        }
        
        // Send the message
        response, err := s.fcmClient.Send(ctx, message)
        if err != nil {
            log.Printf("Failed to send push notification to token %s: %v", token.Token, err)
            
            // Check if token is invalid and deactivate it
            if messaging.IsRegistrationTokenNotRegistered(err) {
                log.Printf("Token %s is not registered, deleting...", token.Token)
                s.repo.DeletePushToken(ctx, token.Token)
            }
            continue
        }
        
        log.Printf("Successfully sent push notification: %s", response)
    }
    
    return nil
}

// SendBulkNotifications sends notifications to multiple users
func (s *pushService) SendBulkNotifications(ctx context.Context, notifications []*BulkNotification) error {
    for _, bulk := range notifications {
        for _, userID := range bulk.UserIDs {
            // Send in goroutine for better performance
            go func(uid int64, notif *PushNotification) {
                if err := s.SendNotification(ctx, uid, notif); err != nil {
                    log.Printf("Failed to send bulk notification to user %d: %v", uid, err)
                }
            }(userID, bulk.Notification)
        }
    }
    return nil
}

// MockPushService for testing or when Firebase is not configured
type mockPushService struct{}

func NewMockPushService() PushService {
    return &mockPushService{}
}

func (m *mockPushService) SendNotification(ctx context.Context, userID int64, notification *PushNotification) error {
    log.Printf("Mock: Sending push notification to user %d: %s - %s", userID, notification.Title, notification.Body)
    return nil
}

func (m *mockPushService) SendBulkNotifications(ctx context.Context, notifications []*BulkNotification) error {
    for _, bulk := range notifications {
        log.Printf("Mock: Sending bulk notification to %d users", len(bulk.UserIDs))
    }
    return nil
}