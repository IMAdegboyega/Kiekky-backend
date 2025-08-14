// internal/otp/models.go

package otp

import (
	"time"
)

// OTPType represents different OTP use cases
type OTPType string

const (
	OTPTypeSignup        OTPType = "signup"
	OTPTypeSignin        OTPType = "signin"
	OTPTypePasswordReset OTPType = "password_reset"
	OTPTypePhoneVerify   OTPType = "phone_verify"
	OTPTypeEmailVerify   OTPType = "email_verify"
)

// DeliveryMethod represents how OTP is sent
type DeliveryMethod string

const (
	DeliveryMethodEmail DeliveryMethod = "email"
	DeliveryMethodSMS   DeliveryMethod = "sms"
)

// OTP represents an OTP record
type OTP struct {
	ID         int64          `json:"id" db:"id"`
	UserID     int64          `json:"user_id" db:"user_id"`
	Code       string         `json:"-" db:"code"`
	Type       OTPType        `json:"type" db:"type"`
	Method     DeliveryMethod `json:"method" db:"method"`
	Recipient  string         `json:"recipient" db:"recipient"` // email or phone
	Attempts   int            `json:"attempts" db:"attempts"`
	Verified   bool           `json:"verified" db:"verified"`
	ExpiresAt  time.Time      `json:"expires_at" db:"expires_at"`
	VerifiedAt *time.Time     `json:"verified_at,omitempty" db:"verified_at"`
	CreatedAt  time.Time      `json:"created_at" db:"created_at"`
}

// SendOTPRequest represents request to send OTP
type SendOTPRequest struct {
	UserID    int64          `json:"user_id,omitempty"`
	Email     string         `json:"email,omitempty" validate:"omitempty,email"`
	Phone     string         `json:"phone,omitempty" validate:"omitempty,e164"`
	Type      OTPType        `json:"type" validate:"required,oneof=signup signin password_reset phone_verify email_verify"`
	Method    DeliveryMethod `json:"method" validate:"required,oneof=email sms"`
}

// VerifyOTPRequest represents request to verify OTP
type VerifyOTPRequest struct {
	UserID    int64   `json:"user_id,omitempty"`
	Email     string  `json:"email,omitempty" validate:"omitempty,email"`
	Phone     string  `json:"phone,omitempty" validate:"omitempty,e164"`
	Code      string  `json:"code" validate:"required,len=6,numeric"`
	Type      OTPType `json:"type" validate:"required"`
}

// ResendOTPRequest represents request to resend OTP
type ResendOTPRequest struct {
	UserID    int64          `json:"user_id,omitempty"`
	Email     string         `json:"email,omitempty" validate:"omitempty,email"`
	Phone     string         `json:"phone,omitempty" validate:"omitempty,e164"`
	Type      OTPType        `json:"type" validate:"required"`
	Method    DeliveryMethod `json:"method" validate:"required,oneof=email sms"`
}

// OTPResponse represents OTP operation response
type OTPResponse struct {
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// OTPConfig holds OTP configuration
type OTPConfig struct {
	Length      int           `json:"length"`
	Expiry      time.Duration `json:"expiry"`
	MaxAttempts int           `json:"max_attempts"`
	RateLimit   RateLimitConfig
}

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	MaxRequests int           `json:"max_requests"`
	Window      time.Duration `json:"window"`
}

// EmailTemplate represents email template data
type EmailTemplate struct {
	To           string
	Subject      string
	TemplateName string
	Data         map[string]interface{}
}

// SMSMessage represents SMS message data
type SMSMessage struct {
	To      string
	Message string
}