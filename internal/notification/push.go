// internal/notification/push.go

package notifications

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "log"
    "os"
    
    firebase "firebase.google.com/go/v4"
    "firebase.google.com/go/v4/messaging"
    "google.golang.org/api/option"
)

// FCMPushService implements push notifications using Firebase Cloud Messaging
type FCMPushService struct {
    client *messaging.Client
}

// NewFCMPushService creates a new FCM push service
func NewFCMPushService(ctx context.Context) (PushService, error) {
    // Get credentials from environment
    credentialsPath := os.Getenv("FIREBASE_CREDENTIALS_PATH")
    if credentialsPath == "" {
        credentialsJSON := os.Getenv("FIREBASE_CREDENTIALS_JSON")
        if credentialsJSON == "" {
            return nil, errors.New("FIREBASE_CREDENTIALS_PATH or FIREBASE_CREDENTIALS_JSON must be set")
        }
        
        // Use JSON credentials directly
        opt := option.WithCredentialsJSON([]byte(credentialsJSON))
        app, err := firebase.NewApp(ctx, nil, opt)
        if err != nil {
            return nil, fmt.Errorf("failed to initialize Firebase app: %v", err)
        }
        
        client, err := app.Messaging(ctx)
        if err != nil {
            return nil, fmt.Errorf("failed to get messaging client: %v", err)
        }
        
        return &FCMPushService{client: client}, nil
    }
    
    // Use credentials file
    opt := option.WithCredentialsFile(credentialsPath)
    app, err := firebase.NewApp(ctx, nil, opt)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize Firebase app: %v", err)
    }
    
    client, err := app.Messaging(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get messaging client: %v", err)
    }
    
    return &FCMPushService{client: client}, nil
}

// SendPush sends a push notification to specified devices
func (s *FCMPushService) SendPush(ctx context.Context, notification *PushNotification) error {
    if len(notification.Tokens) == 0 {
        return errors.New("no tokens provided")
    }
    
    // Create base message
    baseMessage := &messaging.Notification{
        Title: notification.Title,
        Body:  notification.Body,
    }
    
    if notification.Image != "" {
        baseMessage.ImageURL = notification.Image
    }
    
    // Convert data map
    data := notification.Data
    if data == nil {
        data = make(map[string]string)
    }
    
    // Add notification metadata
    data["title"] = notification.Title
    data["body"] = notification.Body
    
    // Handle Android and iOS specific configurations
    androidConfig := &messaging.AndroidConfig{
        Priority: s.mapPriority(notification.Priority),
        Notification: &messaging.AndroidNotification{
            Sound:       notification.Sound,
            ClickAction: "FLUTTER_NOTIFICATION_CLICK",
        },
    }
    
    if notification.CollapseKey != "" {
        androidConfig.CollapseKey = notification.CollapseKey
    }
    
    apnsConfig := &messaging.APNSConfig{
        Headers: map[string]string{
            "apns-priority": s.getAPNSPriority(notification.Priority),
        },
        Payload: &messaging.APNSPayload{
            Aps: &messaging.Aps{
                Alert: &messaging.ApsAlert{
                    Title: notification.Title,
                    Body:  notification.Body,
                },
                Badge: &notification.Badge,
                Sound: notification.Sound,
            },
        },
    }
    
    // For single token, send individual message
    if len(notification.Tokens) == 1 {
        message := &messaging.Message{
            Token:        notification.Tokens[0],
            Notification: baseMessage,
            Data:         data,
            Android:      androidConfig,
            APNS:         apnsConfig,
        }
        
        response, err := s.client.Send(ctx, message)
        if err != nil {
            log.Printf("Failed to send push notification: %v", err)
            return err
        }
        
        log.Printf("Successfully sent push notification: %s", response)
        return nil
    }
    
    // For multiple tokens, use batch send
    messages := make([]*messaging.Message, 0, len(notification.Tokens))
    for _, token := range notification.Tokens {
        message := &messaging.Message{
            Token:        token,
            Notification: baseMessage,
            Data:         data,
            Android:      androidConfig,
            APNS:         apnsConfig,
        }
        messages = append(messages, message)
    }
    
    batchResponse, err := s.client.SendAll(ctx, messages)
    if err != nil {
        log.Printf("Failed to send batch push notifications: %v", err)
        return err
    }
    
    if batchResponse.FailureCount > 0 {
        log.Printf("Failed to send %d out of %d push notifications", 
            batchResponse.FailureCount, len(messages))
        
        // Log individual failures
        for idx, resp := range batchResponse.Responses {
            if resp.Error != nil {
                log.Printf("Failed to send to token %s: %v", 
                    notification.Tokens[idx], resp.Error)
            }
        }
    }
    
    log.Printf("Successfully sent %d push notifications", batchResponse.SuccessCount)
    return nil
}

// SendBatchPush sends multiple push notifications
func (s *FCMPushService) SendBatchPush(ctx context.Context, notifications []*PushNotification) error {
    for _, notification := range notifications {
        if err := s.SendPush(ctx, notification); err != nil {
            log.Printf("Failed to send push notification in batch: %v", err)
            // Continue with other notifications
        }
    }
    
    return nil
}

// mapPriority maps our priority to FCM priority
func (s *FCMPushService) mapPriority(priority Priority) string {
    switch priority {
    case PriorityHigh:
        return "high"
    case PriorityLow:
        return "normal"
    default:
        return "high"
    }
}

// getAPNSPriority gets APNS priority string
func (s *FCMPushService) getAPNSPriority(priority Priority) string {
    switch priority {
    case PriorityHigh:
        return "10"
    case PriorityLow:
        return "5"
    default:
        return "10"
    }
}

// MockPushService is a mock implementation for testing
type MockPushService struct {
    SentNotifications []*PushNotification
}

func NewMockPushService() PushService {
    return &MockPushService{
        SentNotifications: make([]*PushNotification, 0),
    }
}

func (m *MockPushService) SendPush(ctx context.Context, notification *PushNotification) error {
    m.SentNotifications = append(m.SentNotifications, notification)
    log.Printf("Mock: Sending push notification to %d devices: %s", 
        len(notification.Tokens), notification.Title)
    return nil
}

func (m *MockPushService) SendBatchPush(ctx context.Context, notifications []*PushNotification) error {
    for _, n := range notifications {
        m.SendPush(ctx, n)
    }
    return nil
}

// APNSPushService implements push notifications using Apple Push Notification Service
// This is a placeholder - actual implementation would require APNS certificates
type APNSPushService struct {
    // APNS client configuration
}

func NewAPNSPushService(config map[string]string) (PushService, error) {
    // Implementation would go here
    return &APNSPushService{}, nil
}

func (s *APNSPushService) SendPush(ctx context.Context, notification *PushNotification) error {
    // APNS implementation
    return errors.New("APNS not implemented")
}

func (s *APNSPushService) SendBatchPush(ctx context.Context, notifications []*PushNotification) error {
    // APNS batch implementation
    return errors.New("APNS batch not implemented")
}