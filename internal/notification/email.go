// internal/notification/email.go

package notifications

import (
    "bytes"
    "context"
    "crypto/tls"
    "fmt"
    "html/template"
    "log"
    "net/smtp"
    "os"
    "strings"
    
    "gopkg.in/gomail.v2"
)

// SMTPEmailService implements email notifications using SMTP
type SMTPEmailService struct {
    host     string
    port     int
    username string
    password string
    from     string
    fromName string
    dialer   *gomail.Dialer
}

// NewSMTPEmailService creates a new SMTP email service
func NewSMTPEmailService() (EmailService, error) {
    host := os.Getenv("SMTP_HOST")
    port := 587 // Default SMTP port
    if portStr := os.Getenv("SMTP_PORT"); portStr != "" {
        fmt.Sscanf(portStr, "%d", &port)
    }
    
    username := os.Getenv("SMTP_USERNAME")
    password := os.Getenv("SMTP_PASSWORD")
    from := os.Getenv("SMTP_FROM")
    fromName := os.Getenv("SMTP_FROM_NAME")
    
    if fromName == "" {
        fromName = "Kiekky"
    }
    
    if host == "" || username == "" || password == "" || from == "" {
        return nil, fmt.Errorf("incomplete SMTP configuration")
    }
    
    dialer := gomail.NewDialer(host, port, username, password)
    dialer.TLSConfig = &tls.Config{InsecureSkipVerify: false}
    
    return &SMTPEmailService{
        host:     host,
        port:     port,
        username: username,
        password: password,
        from:     from,
        fromName: fromName,
        dialer:   dialer,
    }, nil
}

// SendEmail sends a single email
func (s *SMTPEmailService) SendEmail(ctx context.Context, notification *EmailNotification) error {
    m := gomail.NewMessage()
    
    // Set headers
    m.SetHeader("From", m.FormatAddress(s.from, s.fromName))
    m.SetHeader("To", notification.To)
    m.SetHeader("Subject", notification.Subject)
    
    // Set body
    if notification.HTML != "" {
        m.SetBody("text/html", notification.HTML)
        if notification.Body != "" {
            m.AddAlternative("text/plain", notification.Body)
        }
    } else {
        m.SetBody("text/plain", notification.Body)
    }
    
    // Send email
    if err := s.dialer.DialAndSend(m); err != nil {
        log.Printf("Failed to send email to %s: %v", notification.To, err)
        return err
    }
    
    log.Printf("Successfully sent email to %s", notification.To)
    return nil
}

// SendBatchEmails sends multiple emails
func (s *SMTPEmailService) SendBatchEmails(ctx context.Context, notifications []*EmailNotification) error {
    for _, notification := range notifications {
        if err := s.SendEmail(ctx, notification); err != nil {
            log.Printf("Failed to send email in batch: %v", err)
            // Continue with other emails
        }
    }
    
    return nil
}

// SendGridEmailService implements email notifications using SendGrid
type SendGridEmailService struct {
    apiKey string
    from   string
}

// NewSendGridEmailService creates a new SendGrid email service
func NewSendGridEmailService() (EmailService, error) {
    apiKey := os.Getenv("SENDGRID_API_KEY")
    from := os.Getenv("SENDGRID_FROM_EMAIL")
    
    if apiKey == "" || from == "" {
        return nil, fmt.Errorf("incomplete SendGrid configuration")
    }
    
    return &SendGridEmailService{
        apiKey: apiKey,
        from:   from,
    }, nil
}

// SendEmail sends a single email via SendGrid
func (s *SendGridEmailService) SendEmail(ctx context.Context, notification *EmailNotification) error {
    // SendGrid implementation would go here
    // This is a placeholder
    log.Printf("SendGrid: Sending email to %s: %s", notification.To, notification.Subject)
    return nil
}

// SendBatchEmails sends multiple emails via SendGrid
func (s *SendGridEmailService) SendBatchEmails(ctx context.Context, notifications []*EmailNotification) error {
    for _, notification := range notifications {
        if err := s.SendEmail(ctx, notification); err != nil {
            log.Printf("Failed to send email via SendGrid: %v", err)
        }
    }
    return nil
}

// MockEmailService is a mock implementation for testing
type MockEmailService struct {
    SentEmails []*EmailNotification
}

func NewMockEmailService() EmailService {
    return &MockEmailService{
        SentEmails: make([]*EmailNotification, 0),
    }
}

func (m *MockEmailService) SendEmail(ctx context.Context, notification *EmailNotification) error {
    m.SentEmails = append(m.SentEmails, notification)
    log.Printf("Mock: Sending email to %s: %s", notification.To, notification.Subject)
    return nil
}

func (m *MockEmailService) SendBatchEmails(ctx context.Context, notifications []*EmailNotification) error {
    for _, n := range notifications {
        m.SendEmail(ctx, n)
    }
    return nil
}

// Email templates

const baseEmailTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 600px;
            margin: 0 auto;
            padding: 20px;
        }
        .header {
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            padding: 30px;
            text-align: center;
            border-radius: 10px 10px 0 0;
        }
        .content {
            background: white;
            padding: 30px;
            border: 1px solid #e0e0e0;
            border-radius: 0 0 10px 10px;
        }
        .button {
            display: inline-block;
            padding: 12px 30px;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            color: white;
            text-decoration: none;
            border-radius: 5px;
            margin: 20px 0;
        }
        .footer {
            text-align: center;
            padding: 20px;
            color: #666;
            font-size: 14px;
        }
    </style>
</head>
<body>
    <div class="header">
        <h1>{{.Title}}</h1>
    </div>
    <div class="content">
        {{.Content}}
    </div>
    <div class="footer">
        <p>© 2024 Kiekky. All rights reserved.</p>
        <p>
            <a href="{{.UnsubscribeURL}}">Unsubscribe</a> | 
            <a href="{{.PreferencesURL}}">Update Preferences</a>
        </p>
    </div>
</body>
</html>
`

// RenderEmailTemplate renders an email template with data
func RenderEmailTemplate(templateName string, data map[string]interface{}) (string, error) {
    tmpl, err := template.New("email").Parse(baseEmailTemplate)
    if err != nil {
        return "", err
    }
    
    var buf bytes.Buffer
    if err := tmpl.Execute(&buf, data); err != nil {
        return "", err
    }
    
    return buf.String(), nil
}