// internal/config/config.go
// Centralized configuration management
// Loads from environment variables with sensible defaults

package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all application configuration
type Config struct {
	// Server
	Port        string
	Environment string
	BaseURL     string // ADD THIS for profile/upload URLs
	
	// Database
	DatabaseURL string
	RedisURL    string
	
	// Security
	JWTSecret          string
	BCryptCost         int
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
	
	// OTP (EXISTING - keep as is)
	OTPExpiry      time.Duration
	OTPLength      int
	MaxOTPAttempts int
	
	// Email Configuration (ENHANCED)
	EmailProvider  string // "smtp", "sendgrid", or "mock"
	EmailFrom      string // General from address
	
	// SMTP (EXISTING with minor updates)
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPUsername string // Alias for SMTPUser
	SMTPPassword string
	SMTPFrom     string
	
	// SendGrid (ADD)
	SendGridAPIKey string
	
	// SMS Configuration (ENHANCED)
	SMSProvider string // "twilio", "mock"
	
	// Twilio (EXISTING)
	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFromNumber string
	TwilioPhoneNumber string // Alias for TwilioFromNumber
	
	// Storage Configuration (ENHANCED)
	// S3 (EXISTING)
	AWSRegion          string
	AWSAccessKeyID     string
	AWSSecretAccessKey string
	S3BucketName       string
	S3Bucket           string // Alias for S3BucketName
	S3Region           string // Alias for AWSRegion
	
	// Local Storage (ADD)
	UseS3          bool
	LocalUploadDir string
	
	// Profile Configuration (ADD)
	MaxProfilePictureSize     string
	MaxInterests              int
	ProfileCompletionRequired bool
	MinAge                    int
	MaxAge                    int
	
	// Feature Flags (ADD)
	Enable2FA                 bool
	EnableOAuth               bool
	EnableProfileVerification bool
	EnableLocationFeatures    bool
	
	// Rate Limiting (EXISTING)
	LoginAttemptsMax    int
	LoginAttemptsWindow time.Duration
	OTPResendMax        int
	OTPResendWindow     time.Duration
	
	// Notification Settings (ADD)
	EnableEmailNotifications bool
	EnablePushNotifications  bool
	EnableSMSNotifications   bool
}

// Load reads configuration from environment variables
func Load() *Config {
	cfg := &Config{
		// Server
		Port:        getEnv("PORT", "8080"),
		Environment: getEnv("ENVIRONMENT", "development"),
		BaseURL:     getEnv("BASE_URL", ""), // Will be set after Port is loaded
		
		// Database
		DatabaseURL: getEnv("DATABASE_URL", "postgresql://postgres:Adegboyega5685@db.bfgjgoeslizbvrzvyiut.supabase.co:5432/postgres"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379/0"),
		
		// Security
		JWTSecret:          getEnv("JWT_SECRET", "your-super-secret-key-change-this-in-production"),
		BCryptCost:         getEnvInt("BCRYPT_COST", 10),
		AccessTokenExpiry:  getEnvDuration("ACCESS_TOKEN_EXPIRY", "1h"),
		RefreshTokenExpiry: getEnvDuration("REFRESH_TOKEN_EXPIRY", "720h"), // 30 days
		
		// OTP
		OTPExpiry:      getEnvDuration("OTP_EXPIRY", "10m"),
		OTPLength:      getEnvInt("OTP_LENGTH", 6),
		MaxOTPAttempts: getEnvInt("MAX_OTP_ATTEMPTS", 5),
		
		// Email Configuration
		EmailProvider: getEnv("EMAIL_PROVIDER", "smtp"), // smtp, sendgrid, or mock
		EmailFrom:     getEnv("EMAIL_FROM", "noreply@kiekky.com"),
		
		// SMTP
		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     getEnvInt("SMTP_PORT", 587),
		SMTPUser:     getEnv("SMTP_USER", ""),
		SMTPPassword: getEnv("SMTP_PASSWORD", ""),
		SMTPFrom:     getEnv("SMTP_FROM", "noreply@socialdating.com"),
		
		// SendGrid
		SendGridAPIKey: getEnv("SENDGRID_API_KEY", ""),
		
		// SMS Configuration
		SMSProvider: getEnv("SMS_PROVIDER", "mock"), // twilio or mock
		
		// Twilio
		TwilioAccountSID: getEnv("TWILIO_ACCOUNT_SID", ""),
		TwilioAuthToken:  getEnv("TWILIO_AUTH_TOKEN", ""),
		TwilioFromNumber: getEnv("TWILIO_FROM_NUMBER", ""),
		
		// Storage
		UseS3:              getEnvBool("USE_S3", false),
		LocalUploadDir:     getEnv("LOCAL_UPLOAD_DIR", "./uploads"),
		AWSRegion:          getEnv("AWS_REGION", "us-east-1"),
		AWSAccessKeyID:     getEnv("AWS_ACCESS_KEY_ID", ""),
		AWSSecretAccessKey: getEnv("AWS_SECRET_ACCESS_KEY", ""),
		S3BucketName:       getEnv("S3_BUCKET_NAME", "social-dating-uploads"),
		
		// Profile Configuration
		MaxProfilePictureSize:     getEnv("MAX_PROFILE_PICTURE_SIZE", "5MB"),
		MaxInterests:              getEnvInt("MAX_INTERESTS", 10),
		ProfileCompletionRequired: getEnvBool("PROFILE_COMPLETION_REQUIRED", false),
		MinAge:                    getEnvInt("MIN_AGE", 18),
		MaxAge:                    getEnvInt("MAX_AGE", 100),
		
		// Feature Flags
		Enable2FA:                 getEnvBool("ENABLE_2FA", false),
		EnableOAuth:               getEnvBool("ENABLE_OAUTH", true),
		EnableProfileVerification: getEnvBool("ENABLE_PROFILE_VERIFICATION", true),
		EnableLocationFeatures:    getEnvBool("ENABLE_LOCATION_FEATURES", true),
		
		// Rate Limiting
		LoginAttemptsMax:    getEnvInt("LOGIN_ATTEMPTS_MAX", 5),
		LoginAttemptsWindow: getEnvDuration("LOGIN_ATTEMPTS_WINDOW", "15m"),
		OTPResendMax:        getEnvInt("OTP_RESEND_MAX", 3),
		OTPResendWindow:     getEnvDuration("OTP_RESEND_WINDOW", "1h"),
		
		// Notifications
		EnableEmailNotifications: getEnvBool("ENABLE_EMAIL_NOTIFICATIONS", true),
		EnablePushNotifications:  getEnvBool("ENABLE_PUSH_NOTIFICATIONS", false),
		EnableSMSNotifications:   getEnvBool("ENABLE_SMS_NOTIFICATIONS", false),
	}
	
	// Set aliases for compatibility
	cfg.SMTPUsername = cfg.SMTPUser
	cfg.TwilioPhoneNumber = cfg.TwilioFromNumber
	cfg.S3Bucket = cfg.S3BucketName
	cfg.S3Region = cfg.AWSRegion
	
	// Set BaseURL if not provided
	if cfg.BaseURL == "" {
		if cfg.Environment == "production" {
			cfg.BaseURL = "https://api.kiekky.com" // Update with your domain
		} else {
			cfg.BaseURL = fmt.Sprintf("http://localhost:%s", cfg.Port)
		}
	}
	
	return cfg
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Required fields
	if c.JWTSecret == "your-super-secret-key-change-this-in-production" && c.Environment == "production" {
		return fmt.Errorf("JWT secret must be changed for production")
	}
	
	if c.DatabaseURL == "" {
		return fmt.Errorf("database URL is required")
	}
	
	// OTP validation
	if c.OTPLength < 4 || c.OTPLength > 8 {
		return fmt.Errorf("OTP length must be between 4 and 8")
	}
	
	if c.MaxOTPAttempts < 1 || c.MaxOTPAttempts > 10 {
		return fmt.Errorf("max OTP attempts must be between 1 and 10")
	}
	
	// Email validation
	switch c.EmailProvider {
	case "smtp":
		if c.SMTPHost == "" || c.SMTPUser == "" || c.SMTPPassword == "" {
			if c.Environment == "production" {
				return fmt.Errorf("SMTP configuration incomplete for production")
			}
		}
	case "sendgrid":
		if c.SendGridAPIKey == "" {
			if c.Environment == "production" {
				return fmt.Errorf("SendGrid API key is required for production")
			}
		}
	case "mock":
		if c.Environment == "production" {
			return fmt.Errorf("mock email provider cannot be used in production")
		}
	default:
		return fmt.Errorf("invalid email provider: %s", c.EmailProvider)
	}
	
	// SMS validation
	switch c.SMSProvider {
	case "twilio":
		if c.TwilioAccountSID == "" || c.TwilioAuthToken == "" || c.TwilioFromNumber == "" {
			if c.EnableSMSNotifications || c.Enable2FA {
				return fmt.Errorf("Twilio configuration incomplete but SMS features are enabled")
			}
		}
	case "mock":
		if c.Environment == "production" && c.EnableSMSNotifications {
			return fmt.Errorf("mock SMS provider cannot be used in production with SMS notifications enabled")
		}
	default:
		if c.SMSProvider != "" {
			return fmt.Errorf("invalid SMS provider: %s", c.SMSProvider)
		}
	}
	
	// Storage validation
	if c.UseS3 {
		if c.AWSAccessKeyID == "" || c.AWSSecretAccessKey == "" || c.S3BucketName == "" {
			return fmt.Errorf("S3 configuration incomplete")
		}
	} else {
		// Check if local upload directory exists or can be created
		if c.LocalUploadDir == "" {
			return fmt.Errorf("local upload directory not specified")
		}
	}
	
	// Profile validation
	if c.MinAge < 13 || c.MinAge > c.MaxAge {
		return fmt.Errorf("invalid age range configuration")
	}
	
	if c.MaxInterests < 1 || c.MaxInterests > 50 {
		return fmt.Errorf("max interests must be between 1 and 50")
	}
	
	// Rate limiting validation
	if c.LoginAttemptsMax < 1 || c.OTPResendMax < 1 {
		return fmt.Errorf("rate limiting values must be positive")
	}
	
	return nil
}

// IsProduction returns true if running in production
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// IsDevelopment returns true if running in development
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// Helper functions

// getEnv gets a string value from environment with a default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets an integer value from environment with a default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

// getEnvDuration gets a duration value from environment with a default
func getEnvDuration(key string, defaultValue string) time.Duration {
	value := getEnv(key, defaultValue)
	duration, err := time.ParseDuration(value)
	if err != nil {
		// If parsing fails, try to parse the default
		duration, _ = time.ParseDuration(defaultValue)
	}
	return duration
}

// getEnvBool gets a boolean value from environment with a default
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}