package notifications

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    
    "github.com/twilio/twilio-go"
    twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

// TwilioSMSService implements SMS notifications using Twilio
type TwilioSMSService struct {
    client *twilio.RestClient
    from   string
}

// NewTwilioSMSService creates a new Twilio SMS service
func NewTwilioSMSService() (SMSService, error) {
    accountSID := os.Getenv("TWILIO_ACCOUNT_SID")
    authToken := os.Getenv("TWILIO_AUTH_TOKEN")
    from := os.Getenv("TWILIO_PHONE_NUMBER")
    
    if accountSID == "" || authToken == "" || from == "" {
        return nil, fmt.Errorf("incomplete Twilio configuration")
    }
    
    client := twilio.NewRestClientWithParams(twilio.ClientParams{
        Username: accountSID,
        Password: authToken,
    })
    
    return &TwilioSMSService{
        client: client,
        from:   from,
    }, nil
}

// SendSMS sends a single SMS
func (s *TwilioSMSService) SendSMS(ctx context.Context, notification *SMSNotification) error {
    params := &twilioApi.CreateMessageParams{}
    params.SetTo(notification.To)
    params.SetFrom(s.from)
    params.SetBody(notification.Message)
    
    resp, err := s.client.Api.CreateMessage(params)
    if err != nil {
        log.Printf("Failed to send SMS to %s: %v", notification.To, err)
        return err
    }
    
    if resp.Sid != nil {
        log.Printf("Successfully sent SMS to %s with SID: %s", notification.To, *resp.Sid)
    }
    
    return nil
}

// SendBatchSMS sends multiple SMS messages
func (s *TwilioSMSService) SendBatchSMS(ctx context.Context, notifications []*SMSNotification) error {
    for _, notification := range notifications {
        if err := s.SendSMS(ctx, notification); err != nil {
            log.Printf("Failed to send SMS in batch: %v", err)
            // Continue with other messages
        }
    }
    
    return nil
}

// AfricasTalkingSMSService implements SMS notifications using Africa's Talking
type AfricasTalkingSMSService struct {
    username string
    apiKey   string
    from     string
}

// NewAfricasTalkingSMSService creates a new Africa's Talking SMS service
func NewAfricasTalkingSMSService() (SMSService, error) {
    username := os.Getenv("AT_USERNAME")
    apiKey := os.Getenv("AT_API_KEY")
    from := os.Getenv("AT_SENDER_ID")
    
    if username == "" || apiKey == "" {
        return nil, fmt.Errorf("incomplete Africa's Talking configuration")
    }
    
    return &AfricasTalkingSMSService{
        username: username,
        apiKey:   apiKey,
        from:     from,
    }, nil
}

// SendSMS sends a single SMS via Africa's Talking
func (s *AfricasTalkingSMSService) SendSMS(ctx context.Context, notification *SMSNotification) error {
    // Africa's Talking implementation would go here
    log.Printf("Africa's Talking: Sending SMS to %s: %s", notification.To, notification.Message)
    return nil
}

// SendBatchSMS sends multiple SMS messages via Africa's Talking
func (s *AfricasTalkingSMSService) SendBatchSMS(ctx context.Context, notifications []*SMSNotification) error {
    // Africa's Talking supports batch sending
    recipients := make([]string, len(notifications))
    for i, n := range notifications {
        recipients[i] = n.To
    }
    
    // Actual implementation would send to all recipients at once
    log.Printf("Africa's Talking: Sending batch SMS to %d recipients", len(recipients))
    return nil
}

// MockSMSService is a mock implementation for testing
type MockSMSService struct {
    SentMessages []*SMSNotification
}

func NewMockSMSService() SMSService {
    return &MockSMSService{
        SentMessages: make([]*SMSNotification, 0),
    }
}

func (m *MockSMSService) SendSMS(ctx context.Context, notification *SMSNotification) error {
    m.SentMessages = append(m.SentMessages, notification)
    log.Printf("Mock: Sending SMS to %s: %s", notification.To, notification.Message)
    return nil
}

func (m *MockSMSService) SendBatchSMS(ctx context.Context, notifications []*SMSNotification) error {
    for _, n := range notifications {
        m.SendSMS(ctx, n)
    }
    return nil
}