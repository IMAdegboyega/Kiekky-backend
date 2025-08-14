// internal/otp/service.go

package otp

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"time"
)

var (
	ErrOTPExpired       = errors.New("OTP has expired")
	ErrOTPInvalid       = errors.New("invalid OTP code")
	ErrOTPMaxAttempts   = errors.New("maximum verification attempts exceeded")
	ErrOTPAlreadyUsed   = errors.New("OTP has already been used")
	ErrRateLimitExceeded = errors.New("rate limit exceeded, please try again later")
)

// Service defines the OTP service interface
type Service interface {
	GenerateOTP(ctx context.Context, req *SendOTPRequest) (*OTPResponse, error)
	VerifyOTP(ctx context.Context, req *VerifyOTPRequest) error
	ResendOTP(ctx context.Context, req *ResendOTPRequest) (*OTPResponse, error)
	CleanupExpiredOTPs(ctx context.Context) error
}

// service implements the OTP service
type service struct {
	repo          Repository
	emailProvider EmailProvider
	smsProvider   SMSProvider
	config        *OTPConfig
}

// NewService creates a new OTP service
func NewService(
	repo Repository,
	emailProvider EmailProvider,
	smsProvider SMSProvider,
	config *OTPConfig,
) Service {
	// Set default config if not provided
	if config == nil {
		config = &OTPConfig{
			Length:      6,
			Expiry:      10 * time.Minute,
			MaxAttempts: 3,
			RateLimit: RateLimitConfig{
				MaxRequests: 3,
				Window:      time.Hour,
			},
		}
	}

	return &service{
		repo:          repo,
		emailProvider: emailProvider,
		smsProvider:   smsProvider,
		config:        config,
	}
}

// GenerateOTP generates and sends a new OTP
func (s *service) GenerateOTP(ctx context.Context, req *SendOTPRequest) (*OTPResponse, error) {
	// Check rate limit
	count, err := s.repo.CountRecentOTPs(ctx, req.UserID, s.config.RateLimit.Window)
	if err != nil {
		return nil, fmt.Errorf("failed to check rate limit: %w", err)
	}

	if count >= s.config.RateLimit.MaxRequests {
		return nil, ErrRateLimitExceeded
	}

	// Invalidate any existing OTPs of the same type
	if err := s.repo.InvalidateOTPs(ctx, req.UserID, req.Type); err != nil {
		log.Printf("Failed to invalidate existing OTPs: %v", err)
	}

	// Generate OTP code
	code, err := s.generateCode(s.config.Length)
	if err != nil {
		return nil, fmt.Errorf("failed to generate OTP code: %w", err)
	}

	// Determine recipient
	recipient := req.Email
	if req.Method == DeliveryMethodSMS {
		recipient = req.Phone
	}

	// Create OTP record
	otp := &OTP{
		UserID:    req.UserID,
		Code:      code,
		Type:      req.Type,
		Method:    req.Method,
		Recipient: recipient,
		Attempts:  0,
		Verified:  false,
		ExpiresAt: time.Now().Add(s.config.Expiry),
		CreatedAt: time.Now(),
	}

	// Save OTP to database
	if err := s.repo.CreateOTP(ctx, otp); err != nil {
		return nil, fmt.Errorf("failed to save OTP: %w", err)
	}

	// Send OTP
	if err := s.sendOTP(ctx, otp); err != nil {
		return nil, fmt.Errorf("failed to send OTP: %w", err)
	}

	return &OTPResponse{
		Success:   true,
		Message:   fmt.Sprintf("OTP sent successfully to %s", recipient),
		ExpiresAt: otp.ExpiresAt,
	}, nil
}

// VerifyOTP verifies an OTP code
func (s *service) VerifyOTP(ctx context.Context, req *VerifyOTPRequest) error {
	// Find the OTP
	var otp *OTP
	var err error

	if req.UserID > 0 {
		otp, err = s.repo.GetLatestOTP(ctx, req.UserID, req.Type)
	} else if req.Email != "" {
		otp, err = s.repo.GetLatestOTPByRecipient(ctx, req.Email, req.Type)
	} else if req.Phone != "" {
		otp, err = s.repo.GetLatestOTPByRecipient(ctx, req.Phone, req.Type)
	} else {
		return errors.New("user_id, email, or phone is required")
	}

	if err != nil {
		return fmt.Errorf("failed to get OTP: %w", err)
	}

	// Check if OTP is already verified
	if otp.Verified {
		return ErrOTPAlreadyUsed
	}

	// Check if OTP has expired
	if time.Now().After(otp.ExpiresAt) {
		return ErrOTPExpired
	}

	// Check max attempts
	if otp.Attempts >= s.config.MaxAttempts {
		return ErrOTPMaxAttempts
	}

	// Increment attempts
	otp.Attempts++
	if err := s.repo.UpdateOTPAttempts(ctx, otp.ID, otp.Attempts); err != nil {
		log.Printf("Failed to update OTP attempts: %v", err)
	}

	// Verify code
	if otp.Code != req.Code {
		return ErrOTPInvalid
	}

	// Mark as verified
	now := time.Now()
	otp.Verified = true
	otp.VerifiedAt = &now

	if err := s.repo.MarkOTPAsVerified(ctx, otp.ID); err != nil {
		return fmt.Errorf("failed to mark OTP as verified: %w", err)
	}

	return nil
}

// ResendOTP resends an OTP
func (s *service) ResendOTP(ctx context.Context, req *ResendOTPRequest) (*OTPResponse, error) {
	// Convert ResendOTPRequest to SendOTPRequest
	sendReq := &SendOTPRequest{
		UserID: req.UserID,
		Email:  req.Email,
		Phone:  req.Phone,
		Type:   req.Type,
		Method: req.Method,
	}

	return s.GenerateOTP(ctx, sendReq)
}

// CleanupExpiredOTPs removes expired OTPs from the database
func (s *service) CleanupExpiredOTPs(ctx context.Context) error {
	return s.repo.DeleteExpiredOTPs(ctx, time.Now())
}

// generateCode generates a random numeric code
func (s *service) generateCode(length int) (string, error) {
	const digits = "0123456789"
	code := make([]byte, length)

	for i := 0; i < length; i++ {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", err
		}
		code[i] = digits[num.Int64()]
	}

	return string(code), nil
}

// sendOTP sends the OTP via email or SMS
func (s *service) sendOTP(ctx context.Context, otp *OTP) error {
	switch otp.Method {
	case DeliveryMethodEmail:
		return s.sendOTPEmail(ctx, otp)
	case DeliveryMethodSMS:
		return s.sendOTPSMS(ctx, otp)
	default:
		return fmt.Errorf("unsupported delivery method: %s", otp.Method)
	}
}

// sendOTPEmail sends OTP via email
func (s *service) sendOTPEmail(ctx context.Context, otp *OTP) error {
	if s.emailProvider == nil {
		return errors.New("email provider not configured")
	}

	subject := s.getEmailSubject(otp.Type)
	
	template := &EmailTemplate{
		To:           otp.Recipient,
		Subject:      subject,
		TemplateName: "otp_verification",
		Data: map[string]interface{}{
			"code":      otp.Code,
			"type":      string(otp.Type),
			"expiresIn": int(s.config.Expiry.Minutes()),
		},
	}

	return s.emailProvider.SendEmail(ctx, template)
}

// sendOTPSMS sends OTP via SMS
func (s *service) sendOTPSMS(ctx context.Context, otp *OTP) error {
	if s.smsProvider == nil {
		return errors.New("SMS provider not configured")
	}

	message := fmt.Sprintf("Your verification code is: %s. It will expire in %d minutes.",
		otp.Code, int(s.config.Expiry.Minutes()))

	sms := &SMSMessage{
		To:      otp.Recipient,
		Message: message,
	}

	return s.smsProvider.SendSMS(ctx, sms)
}

// getEmailSubject returns the email subject based on OTP type
func (s *service) getEmailSubject(otpType OTPType) string {
	subjects := map[OTPType]string{
		OTPTypeSignup:        "Verify Your Account",
		OTPTypeSignin:        "Two-Factor Authentication Code",
		OTPTypePasswordReset: "Password Reset Code",
		OTPTypePhoneVerify:   "Verify Your Phone Number",
		OTPTypeEmailVerify:   "Verify Your Email Address",
	}

	if subject, ok := subjects[otpType]; ok {
		return subject
	}
	return "Verification Code"
}