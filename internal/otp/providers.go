// internal/otp/providers.go

package otp

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"net/smtp"
	"path/filepath"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/twilio/twilio-go"
	twilioApi "github.com/twilio/twilio-go/rest/api/v2010"
)

// EmailProvider defines the email provider interface
type EmailProvider interface {
	SendEmail(ctx context.Context, template *EmailTemplate) error
}

// SMSProvider defines the SMS provider interface
type SMSProvider interface {
	SendSMS(ctx context.Context, message *SMSMessage) error
}

// SMTPEmailProvider implements EmailProvider using SMTP
type SMTPEmailProvider struct {
	host     string
	port     string
	username string
	password string
	from     string
}

// NewSMTPEmailProvider creates a new SMTP email provider
func NewSMTPEmailProvider(host, port, username, password, from string) EmailProvider {
	return &SMTPEmailProvider{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// SendEmail sends an email using SMTP
func (p *SMTPEmailProvider) SendEmail(ctx context.Context, emailData *EmailTemplate) error {
	// Load and parse template
	templatePath := filepath.Join("templates", emailData.TemplateName+".html")
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		// Fallback to plain text if template not found
		return p.sendPlainTextEmail(emailData)
	}

	// Execute template
	var body bytes.Buffer
	if err := tmpl.Execute(&body, emailData.Data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Compose message
	message := fmt.Sprintf("From: %s\r\n", p.from)
	message += fmt.Sprintf("To: %s\r\n", emailData.To)
	message += fmt.Sprintf("Subject: %s\r\n", emailData.Subject)
	message += "MIME-version: 1.0;\r\n"
	message += "Content-Type: text/html; charset=\"UTF-8\";\r\n"
	message += "\r\n"
	message += body.String()

	// Set up authentication
	auth := smtp.PlainAuth("", p.username, p.password, p.host)

	// Send email
	addr := fmt.Sprintf("%s:%s", p.host, p.port)
	err = smtp.SendMail(addr, auth, p.from, []string{emailData.To}, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// sendPlainTextEmail sends a plain text email (fallback)
func (p *SMTPEmailProvider) sendPlainTextEmail(emailData *EmailTemplate) error {
	code := emailData.Data["code"].(string)
	expiresIn := emailData.Data["expiresIn"].(int)
	
	body := fmt.Sprintf("Your verification code is: %s\n\nThis code will expire in %d minutes.", code, expiresIn)
	
	message := fmt.Sprintf("From: %s\r\n", p.from)
	message += fmt.Sprintf("To: %s\r\n", emailData.To)
	message += fmt.Sprintf("Subject: %s\r\n", emailData.Subject)
	message += "\r\n"
	message += body

	auth := smtp.PlainAuth("", p.username, p.password, p.host)
	addr := fmt.Sprintf("%s:%s", p.host, p.port)
	
	return smtp.SendMail(addr, auth, p.from, []string{emailData.To}, []byte(message))
}

// SendGridEmailProvider implements EmailProvider using SendGrid
type SendGridEmailProvider struct {
	apiKey string
	from   string
}

// NewSendGridEmailProvider creates a new SendGrid email provider
func NewSendGridEmailProvider(apiKey, from string) EmailProvider {
	return &SendGridEmailProvider{
		apiKey: apiKey,
		from:   from,
	}
}

// SendEmail sends an email using SendGrid
func (p *SendGridEmailProvider) SendEmail(ctx context.Context, emailData *EmailTemplate) error {
	from := mail.NewEmail("Kiekky", p.from)
	to := mail.NewEmail("", emailData.To)
	
	// Load and parse template
	templatePath := filepath.Join("templates", emailData.TemplateName+".html")
	tmpl, err := template.ParseFiles(templatePath)
	
	var htmlContent string
	if err == nil {
		var body bytes.Buffer
		if err := tmpl.Execute(&body, emailData.Data); err == nil {
			htmlContent = body.String()
		}
	}
	
	// Fallback to plain text
	code := emailData.Data["code"].(string)
	expiresIn := emailData.Data["expiresIn"].(int)
	plainTextContent := fmt.Sprintf("Your verification code is: %s\n\nThis code will expire in %d minutes.", code, expiresIn)
	
	message := mail.NewSingleEmail(from, emailData.Subject, to, plainTextContent, htmlContent)
	client := sendgrid.NewSendClient(p.apiKey)
	
	response, err := client.Send(message)
	if err != nil {
		return fmt.Errorf("failed to send email via SendGrid: %w", err)
	}
	
	if response.StatusCode >= 400 {
		return fmt.Errorf("SendGrid returned error status: %d", response.StatusCode)
	}
	
	return nil
}

// TwilioSMSProvider implements SMSProvider using Twilio
type TwilioSMSProvider struct {
	client      *twilio.RestClient
	phoneNumber string
}

// NewTwilioSMSProvider creates a new Twilio SMS provider
func NewTwilioSMSProvider(accountSID, authToken, phoneNumber string) SMSProvider {
	client := twilio.NewRestClientWithParams(twilio.ClientParams{
		Username: accountSID,
		Password: authToken,
	})
	
	return &TwilioSMSProvider{
		client:      client,
		phoneNumber: phoneNumber,
	}
}

// SendSMS sends an SMS using Twilio
func (p *TwilioSMSProvider) SendSMS(ctx context.Context, message *SMSMessage) error {
	params := &twilioApi.CreateMessageParams{}
	params.SetTo(message.To)
	params.SetFrom(p.phoneNumber)
	params.SetBody(message.Message)

	_, err := p.client.Api.CreateMessage(params)
	if err != nil {
		return fmt.Errorf("failed to send SMS via Twilio: %w", err)
	}

	return nil
}

// MockEmailProvider implements EmailProvider for testing
type MockEmailProvider struct {
	SentEmails []EmailTemplate
}

// NewMockEmailProvider creates a new mock email provider
func NewMockEmailProvider() *MockEmailProvider {
	return &MockEmailProvider{
		SentEmails: make([]EmailTemplate, 0),
	}
}

// SendEmail mocks sending an email
func (p *MockEmailProvider) SendEmail(ctx context.Context, template *EmailTemplate) error {
	p.SentEmails = append(p.SentEmails, *template)
	return nil
}

// MockSMSProvider implements SMSProvider for testing
type MockSMSProvider struct {
	SentMessages []SMSMessage
}

// NewMockSMSProvider creates a new mock SMS provider
func NewMockSMSProvider() *MockSMSProvider {
	return &MockSMSProvider{
		SentMessages: make([]SMSMessage, 0),
	}
}

// SendSMS mocks sending an SMS
func (p *MockSMSProvider) SendSMS(ctx context.Context, message *SMSMessage) error {
	p.SentMessages = append(p.SentMessages, *message)
	return nil
}