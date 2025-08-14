// internal/auth/models.go
// This file contains all data structures used in the authentication system.
// Models are the foundation - they define what data we work with.

package auth

import (
    "database/sql/driver"
    "encoding/json"
    "time"
)

// User represents a user in our system
// Using SERIAL (int64) for ID instead of UUID for better performance
// Tags like `json` control JSON serialization, `db` maps to database columns
type User struct {
    ID                int64     `json:"id" db:"id"`
    Email             *string   `json:"email" db:"email"`                      // Nullable for phone-only users
    Username          string    `json:"username" db:"username"`
    PasswordHash      *string   `json:"-" db:"password_hash"`                  // Nullable for OAuth users
    Phone             *string   `json:"phone" db:"phone"`                      // Nullable for email-only users
    Provider          string    `json:"provider" db:"provider"`                // 'local' or 'google'
    ProviderID        *string   `json:"provider_id" db:"provider_id"`          // Google user ID
    IsVerified        bool      `json:"is_verified" db:"is_verified"`
    IsProfileComplete bool      `json:"is_profile_complete" db:"is_profile_complete"`
    CreatedAt         time.Time `json:"created_at" db:"created_at"`
    UpdatedAt         time.Time `json:"updated_at" db:"updated_at"`
}

type UserAdditions struct {
    TwoFactorEnabled bool `json:"two_factor_enabled" db:"two_factor_enabled"`
}

// Additional fields for User struct (add to existing User struct)
type UserExtensions struct {
    // Add these fields to your existing User struct:
    TwoFactorEnabled bool `json:"two_factor_enabled" db:"two_factor_enabled"`
    
    // Profile completion tracking
    ProfileCompletionPercentage int `json:"profile_completion_percentage,omitempty"`
}

// Session represents an active user session
// We store sessions in the database for better security and multi-device support
type Session struct {
    ID           int64     `json:"id" db:"id"`
    UserID       int64     `json:"user_id" db:"user_id"`
    Token        string    `json:"token" db:"token"`
    RefreshToken string    `json:"refresh_token" db:"refresh_token"`
    DeviceInfo   *string   `json:"device_info" db:"device_info"`
    IPAddress    *string   `json:"ip_address" db:"ip_address"`
    ExpiresAt    time.Time `json:"expires_at" db:"expires_at"`
    CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// SignupRequest is what the client sends to create an account
// Validation tags ensure data quality at the API boundary
type SignupRequest struct {
    Email           *string `json:"email" validate:"required_without=Phone,omitempty,email"`
    Phone           *string `json:"phone" validate:"required_without=Email,omitempty,e164"`
    Username        string  `json:"username" validate:"required,min=3,max=30,alphanum"`
    Password        string  `json:"password" validate:"required,min=8,max=100"`
    ConfirmPassword string  `json:"confirm_password" validate:"required,eqfield=Password"`
    AcceptTerms     bool    `json:"accept_terms" validate:"required"`
}

// SigninRequest handles both email and username login
type SigninRequest struct {
    EmailOrPhone string `json:"email_or_phone" validate:"required"`
    Password     string `json:"password" validate:"required"`
}

// GoogleAuthRequest for OAuth signin/signup
type GoogleAuthRequest struct {
    IDToken string `json:"id_token" validate:"required"` // Google ID token from frontend
}

// OTPVerificationRequest for verifying email/phone
type OTPVerificationRequest struct {
    EmailOrPhone string `json:"email_or_phone" validate:"required"`
    OTP          string `json:"otp" validate:"required,len=6,numeric"`
}

// PendingAuth represents a signin attempt waiting for OTP
type PendingAuth struct {
    UserID    int64     `json:"user_id"`
    Token     string    `json:"token"`      // Temporary token
    ExpiresAt time.Time `json:"expires_at"`
}

// RefreshTokenRequest to get new access token
type RefreshTokenRequest struct {
    RefreshToken string `json:"refresh_token" validate:"required"`
}

// PasswordResetRequest to initiate password reset
type PasswordResetRequest struct {
    Email string `json:"email" validate:"required,email"`
}

// PasswordResetConfirmRequest to complete password reset
type PasswordResetConfirmRequest struct {
    Email       string `json:"email" validate:"required,email"`
    Token       string `json:"token" validate:"required"`
    NewPassword string `json:"new_password" validate:"required,min=8,max=100"`
}

// AuthResponse is what we send back after successful authentication
type AuthResponse struct {
    User         *User  `json:"user"`
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int    `json:"expires_in"`
    TokenType    string `json:"token_type"`
}

// JWTClaims contains the data we store in JWT tokens
// Extends standard claims with our custom fields
// type JWTClaims struct {
//     UserID   int64  `json:"user_id"`
//     Email    string `json:"email"`
//     Username string `json:"username"`
//     Type     string `json:"type"` // "access" or "refresh"
//     // Standard JWT claims
//     ExpiresAt int64  `json:"exp"`
//     IssuedAt  int64  `json:"iat"`
//     NotBefore int64  `json:"nbf"`
//     Issuer    string `json:"iss"`
//     Subject   string `json:"sub"`
// }

// OTPData stores OTP information in Redis
// We use a struct to store multiple related values
type OTPData struct {
    Code      string    `json:"code"`
    Attempts  int       `json:"attempts"`
    ExpiresAt time.Time `json:"expires_at"`
    Type      string    `json:"type"` // "signup" or "reset"
} 

// SigninResponse - returns either auth tokens or pending OTP
type SigninResponse struct {
    RequiresOTP   bool    `json:"requires_otp"`
    PendingToken  string  `json:"pending_token,omitempty"`  // Temporary token for OTP verification
    Message       string  `json:"message,omitempty"`
    *AuthResponse         // Embedded, only populated if OTP not required (OAuth)
    OTPType      string        `json:"otpType,omitempty"`
}

// Additional SigninResponse fields
type SigninResponseExtended struct {
    OTPType string `json:"otp_type,omitempty"` // "verification" or "2fa"
}

// SignupResponse for OTP flow
type SignupResponse struct {
    User                 *User  `json:"user"`
    Message              string `json:"message"`
    RequiresVerification bool   `json:"requires_verification"`
}

// ResendOTPRequest for resending OTP
type ResendOTPRequest struct {
    Email string `json:"email,omitempty" validate:"omitempty,email"`
    Phone string `json:"phone,omitempty" validate:"omitempty,e164"`
    Type  string `json:"type" validate:"required"` // Change from otp.OTPType to string
}

// Scan implements sql.Scanner for OTPData
// This allows us to read JSON from PostgreSQL JSONB columns
func (o *OTPData) Scan(value interface{}) error {
    if value == nil {
        return nil
    }
    
    switch v := value.(type) {
    case []byte:
        return json.Unmarshal(v, o)
    case string:
        return json.Unmarshal([]byte(v), o)
    default:
        return nil
    }
}

// Value implements driver.Valuer for OTPData
// This allows us to write JSON to PostgreSQL JSONB columns
func (o OTPData) Value() (driver.Value, error) {
    return json.Marshal(o)
}