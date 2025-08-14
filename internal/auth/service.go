// internal/auth/service.go
// Service layer contains all business logic for authentication.
// Fully integrated with standalone OTP service

package auth

import (
    "context"
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "errors"
    "fmt"
    "math/big"
    "regexp"
    "strings"
    "time"
    
    "github.com/go-redis/redis/v8"
    "golang.org/x/crypto/bcrypt"
    "google.golang.org/api/oauth2/v2"
    "google.golang.org/api/option"
    
    "github.com/imadgeboyega/kiekky-backend/internal/otp"
    "github.com/imadgeboyega/kiekky-backend/internal/common/utils"
)

// Common errors
var (
    ErrUserNotFound          = errors.New("user not found")
    ErrInvalidCredentials    = errors.New("invalid credentials")
    ErrUserNotVerified       = errors.New("user not verified")
    ErrEmailAlreadyExists    = errors.New("email already exists")
    ErrUsernameAlreadyExists = errors.New("username already exists")
    ErrPhoneAlreadyExists    = errors.New("phone number already registered")
    ErrInvalidToken          = errors.New("invalid token")
    ErrTooManyAttempts       = errors.New("too many attempts")
    ErrInvalidOTP = errors.New("invalid OTP")
)

// Service interface
type Service interface {
    // Registration and authentication
    Signup(ctx context.Context, req *SignupRequest) (*SignupResponse, error)
    VerifySignupOTP(ctx context.Context, req *OTPVerificationRequest) (*AuthResponse, error)
    Signin(ctx context.Context, req *SigninRequest) (*SigninResponse, error)
    VerifySigninOTP(ctx context.Context, pendingToken string, otp string) (*AuthResponse, error)
    GoogleAuth(ctx context.Context, req *GoogleAuthRequest) (*AuthResponse, error)
    ResendOTP(ctx context.Context, req *ResendOTPRequest) error
    
    // Token management
    RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error)
    ValidateToken(ctx context.Context, token string) (*utils.JWTClaims, error)
    
    // Session management
    Logout(ctx context.Context, token string) error
    LogoutAllDevices(ctx context.Context, userID int64) error
    
    // Password management
    InitiatePasswordReset(ctx context.Context, email string) error
    VerifyPasswordResetOTP(ctx context.Context, email string, otpCode string) (string, error)
    ResetPassword(ctx context.Context, resetToken string, newPassword string) error
    
    // User queries
    GetUserByID(ctx context.Context, userID int64) (*User, error)
}

// service implementation
type service struct {
    repo       Repository
    redis      *redis.Client
    otpService otp.Service
    config     *Config
}

// Config holds service configuration
type Config struct {
    JWTSecret           string
    AccessTokenExpiry   time.Duration
    RefreshTokenExpiry  time.Duration
    BCryptCost          int
    Enable2FA           bool  // Global 2FA setting
}

// NewService creates a new auth service
func NewService(repo Repository, redis *redis.Client, otpService otp.Service, config *Config) Service {
    return &service{
        repo:       repo,
        redis:      redis,
        otpService: otpService,
        config:     config,
    }
}

// Signup creates a new user account and sends verification OTP
func (s *service) Signup(ctx context.Context, req *SignupRequest) (*SignupResponse, error) {
    // 1. Validate passwords match
    if req.Password != req.ConfirmPassword {
        return nil, errors.New("passwords do not match")
    }
    
    // 2. Normalize inputs
    req.Username = strings.ToLower(strings.TrimSpace(req.Username))
    
    var normalizedEmail *string
    if req.Email != nil && *req.Email != "" {
        email := strings.ToLower(strings.TrimSpace(*req.Email))
        normalizedEmail = &email
        
        // Check if email exists
        if taken, err := s.repo.IsEmailTaken(ctx, email); err != nil {
            return nil, fmt.Errorf("failed to check email: %w", err)
        } else if taken {
            return nil, ErrEmailAlreadyExists
        }
    }
    
    var normalizedPhone *string
    if req.Phone != nil && *req.Phone != "" {
        normalizedPhone = req.Phone
        
        // Check if phone exists
        if taken, err := s.repo.IsPhoneTaken(ctx, *req.Phone); err != nil {
            return nil, fmt.Errorf("failed to check phone: %w", err)
        } else if taken {
            return nil, ErrPhoneAlreadyExists
        }
    }
    
    // 3. Ensure at least email or phone is provided
    if normalizedEmail == nil && normalizedPhone == nil {
        return nil, errors.New("either email or phone number is required")
    }
    
    // 4. Check username availability
    if taken, err := s.repo.IsUsernameTaken(ctx, req.Username); err != nil {
        return nil, fmt.Errorf("failed to check username: %w", err)
    } else if taken {
        return nil, ErrUsernameAlreadyExists
    }
    
    // 5. Hash password
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), s.config.BCryptCost)
    if err != nil {
        return nil, fmt.Errorf("failed to hash password: %w", err)
    }
    hashedPasswordStr := string(hashedPassword)
    
    // 6. Create user
    user := &User{
        Email:             normalizedEmail,
        Username:          req.Username,
        PasswordHash:      &hashedPasswordStr,
        Phone:             normalizedPhone,
        Provider:          "local",
        IsVerified:        false,
        IsProfileComplete: false,
        CreatedAt:         time.Now(),
        UpdatedAt:         time.Now(),
    }
    
    // 7. Save to database
    if err := s.repo.CreateUser(ctx, user); err != nil {
        return nil, fmt.Errorf("failed to create user: %w", err)
    }
    
    // 8. Send verification OTP using OTP service
    var otpSent bool
    var otpMessage string
    
    if normalizedEmail != nil {
        otpReq := &otp.SendOTPRequest{
            UserID: user.ID,
            Email:  *normalizedEmail,
            Type:   otp.OTPTypeSignup,
            Method: otp.DeliveryMethodEmail,
        }
        
        if _, err := s.otpService.GenerateOTP(ctx, otpReq); err != nil {
            // Log error but don't fail signup
            fmt.Printf("Failed to send OTP email: %v\n", err)
        } else {
            otpSent = true
            otpMessage = fmt.Sprintf("Verification code sent to %s", *normalizedEmail)
        }
    }
    
    if normalizedPhone != nil && !otpSent {
        otpReq := &otp.SendOTPRequest{
            UserID: user.ID,
            Phone:  *normalizedPhone,
            Type:   otp.OTPTypeSignup,
            Method: otp.DeliveryMethodSMS,
        }
        
        if _, err := s.otpService.GenerateOTP(ctx, otpReq); err != nil {
            fmt.Printf("Failed to send OTP SMS: %v\n", err)
        } else {
            otpSent = true
            otpMessage = fmt.Sprintf("Verification code sent to %s", *normalizedPhone)
        }
    }
    
    if !otpSent {
        otpMessage = "Failed to send verification code. Please use resend OTP."
    }
    
    return &SignupResponse{
        User:                 user,
        Message:              otpMessage,
        RequiresVerification: true,
    }, nil
}

// VerifySignupOTP verifies the OTP sent during signup
func (s *service) VerifySignupOTP(ctx context.Context, req *OTPVerificationRequest) (*AuthResponse, error) {
    // 1. Create OTP verification request
    otpReq := &otp.VerifyOTPRequest{
        Code: req.OTP,
        Type: otp.OTPTypeSignup,
    }
    
    // Set email or phone
    if isEmail(req.EmailOrPhone) {
        otpReq.Email = req.EmailOrPhone
    } else if isPhone(req.EmailOrPhone) {
        otpReq.Phone = req.EmailOrPhone
    } else {
        return nil, errors.New("invalid email or phone format")
    }
    
    // 2. Verify OTP using OTP service
    if err := s.otpService.VerifyOTP(ctx, otpReq); err != nil {
        return nil, fmt.Errorf("OTP verification failed: %w", err)
    }
    
    // 3. Get user
    var user *User
    var err error
    
    if otpReq.Email != "" {
        user, err = s.repo.GetUserByEmail(ctx, otpReq.Email)
    } else {
        user, err = s.repo.GetUserByPhone(ctx, otpReq.Phone)
    }
    
    if err != nil {
        return nil, err
    }
    
    // 4. Mark user as verified
    if err := s.repo.VerifyUser(ctx, user.ID); err != nil {
        return nil, fmt.Errorf("failed to verify user: %w", err)
    }
    
    user.IsVerified = true
    
    // 5. Create auth session
    return s.createAuthSession(ctx, user)
}

// Signin authenticates a user
func (s *service) Signin(ctx context.Context, req *SigninRequest) (*SigninResponse, error) {
    // 1. Find user
    var user *User
    var err error
    
    if isEmail(req.EmailOrPhone) {
        user, err = s.repo.GetUserByEmail(ctx, req.EmailOrPhone)
    } else if isPhone(req.EmailOrPhone) {
        user, err = s.repo.GetUserByPhone(ctx, req.EmailOrPhone)
    } else {
        // Try username
        user, err = s.repo.GetUserByUsername(ctx, req.EmailOrPhone)
    }
    
    if err != nil {
        return nil, ErrInvalidCredentials
    }
    
    // 2. Check if it's a password-based account
    if user.PasswordHash == nil {
        return nil, errors.New("this account uses social login")
    }
    
    // 3. Verify password
    if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(req.Password)); err != nil {
        s.recordFailedAttempt(ctx, req.EmailOrPhone)
        return nil, ErrInvalidCredentials
    }
    
    // 4. Clear failed attempts
    s.clearFailedAttempts(ctx, req.EmailOrPhone)
    
    // 5. Check if user is verified
    if !user.IsVerified {
        // Send new OTP for verification
        if user.Email != nil {
            otpReq := &otp.SendOTPRequest{
                UserID: user.ID,
                Email:  *user.Email,
                Type:   otp.OTPTypeSignup,
                Method: otp.DeliveryMethodEmail,
            }
            s.otpService.GenerateOTP(ctx, otpReq)
        }
        
        return &SigninResponse{
            RequiresOTP:  true,
            Message:      "Account not verified. Verification code sent.",
        }, nil
    }
    
    // 6. Check if 2FA is enabled
    if s.config.Enable2FA {
        // Generate and send 2FA OTP
        var otpReq *otp.SendOTPRequest
        
        if user.Email != nil {
            otpReq = &otp.SendOTPRequest{
                UserID: user.ID,
                Email:  *user.Email,
                Type:   otp.OTPTypeSignin,
                Method: otp.DeliveryMethodEmail,
            }
        } else if user.Phone != nil {
            otpReq = &otp.SendOTPRequest{
                UserID: user.ID,
                Phone:  *user.Phone,
                Type:   otp.OTPTypeSignin,
                Method: otp.DeliveryMethodSMS,
            }
        }
        
        if _, err := s.otpService.GenerateOTP(ctx, otpReq); err != nil {
            fmt.Printf("Failed to send 2FA OTP: %v\n", err)
        }
        
        // Generate pending token
        pendingToken := s.generateSecureToken()
        
        // Store pending auth info
        if err := s.storePendingAuth(ctx, pendingToken, user.ID); err != nil {
            return nil, fmt.Errorf("failed to store pending auth: %w", err)
        }
        
        return &SigninResponse{
            RequiresOTP:  true,
            PendingToken: pendingToken,
            Message:      "2FA code sent to your registered email/phone",
            OTPType:      "2fa",
        }, nil
    }
    
    // 7. No 2FA, create session directly
    authResp, err := s.createAuthSession(ctx, user)
    if err != nil {
        return nil, err
    }
    
    return &SigninResponse{
        RequiresOTP:  false,
        Message:      "Login successful",
        AuthResponse: authResp,
    }, nil
}

// VerifySigninOTP verifies 2FA OTP during signin
func (s *service) VerifySigninOTP(ctx context.Context, pendingToken string, otpCode string) (*AuthResponse, error) {
    // 1. Get pending auth info
    userID, err := s.getPendingAuth(ctx, pendingToken)
    if err != nil {
        return nil, errors.New("invalid or expired session")
    }
    
    // 2. Get user
    user, err := s.repo.GetUserByID(ctx, userID)
    if err != nil {
        return nil, err
    }
    
    // 3. Verify OTP
    otpReq := &otp.VerifyOTPRequest{
        UserID: userID,
        Code:   otpCode,
        Type:   otp.OTPTypeSignin,
    }
    
    if user.Email != nil {
        otpReq.Email = *user.Email
    } else if user.Phone != nil {
        otpReq.Phone = *user.Phone
    }
    
    if err := s.otpService.VerifyOTP(ctx, otpReq); err != nil {
        return nil, fmt.Errorf("invalid OTP: %w", err)
    }
    
    // 4. Clear pending auth
    s.clearPendingAuth(ctx, pendingToken)
    
    // 5. Create session
    return s.createAuthSession(ctx, user)
}

// ResendOTP resends an OTP
func (s *service) ResendOTP(ctx context.Context, req *ResendOTPRequest) error {
    // Determine the OTP type
    otpType := otp.OTPTypeSignup // default
    switch req.Type {
    case "signup":
        otpType = otp.OTPTypeSignup
    case "signin":
        otpType = otp.OTPTypeSignin
    case "password_reset":
        otpType = otp.OTPTypePasswordReset
    }
    
    // Determine the delivery method
    var otpReq *otp.ResendOTPRequest
    
    if req.Email != "" {
        otpReq = &otp.ResendOTPRequest{
            Email:  req.Email,
            Type:   otpType,
            Method: otp.DeliveryMethodEmail,
        }
    } else if req.Phone != "" {
        otpReq = &otp.ResendOTPRequest{
            Phone:  req.Phone,
            Type:   otpType,
            Method: otp.DeliveryMethodSMS,
        }
    } else {
        return errors.New("email or phone required")
    }
    
    _, err := s.otpService.ResendOTP(ctx, otpReq)
    return err
}

// InitiatePasswordReset starts the password reset process
func (s *service) InitiatePasswordReset(ctx context.Context, email string) error {
    // 1. Check if user exists (don't reveal this to client)
    user, err := s.repo.GetUserByEmail(ctx, email)
    if err != nil {
        // Return success to prevent email enumeration
        return nil
    }
    
    // 2. Send OTP for password reset
    otpReq := &otp.SendOTPRequest{
        UserID: user.ID,
        Email:  email,
        Type:   otp.OTPTypePasswordReset,
        Method: otp.DeliveryMethodEmail,
    }
    
    _, err = s.otpService.GenerateOTP(ctx, otpReq)
    return err
}

// VerifyPasswordResetOTP verifies the OTP and returns a reset token
func (s *service) VerifyPasswordResetOTP(ctx context.Context, email string, otpCode string) (string, error) {
    // 1. Verify OTP
    otpReq := &otp.VerifyOTPRequest{
        Email: email,
        Code:  otpCode,
        Type:  otp.OTPTypePasswordReset,
    }
    
    if err := s.otpService.VerifyOTP(ctx, otpReq); err != nil {
        return "", err
    }
    
    // 2. Get user
    user, err := s.repo.GetUserByEmail(ctx, email)
    if err != nil {
        return "", err
    }
    
    // 3. Generate reset token
    resetToken := s.generateSecureToken()
    
    // 4. Store reset token with expiry
    if s.redis != nil {
        key := fmt.Sprintf("password_reset:%s", resetToken)
        data := map[string]interface{}{
            "user_id": user.ID,
            "email":   email,
        }
        jsonData, _ := json.Marshal(data)
        s.redis.Set(ctx, key, jsonData, 30*time.Minute)
    }
    
    return resetToken, nil
}

// ResetPassword completes the password reset
func (s *service) ResetPassword(ctx context.Context, resetToken string, newPassword string) error {
    // 1. Get reset data
    var userID int64
    var email string
    
    if s.redis != nil {
        key := fmt.Sprintf("password_reset:%s", resetToken)
        data, err := s.redis.Get(ctx, key).Result()
        if err != nil {
            return ErrInvalidToken
        }
        
        var resetData map[string]interface{}
        if err := json.Unmarshal([]byte(data), &resetData); err != nil {
            return ErrInvalidToken
        }
        
        userID = int64(resetData["user_id"].(float64))
        email = resetData["email"].(string)

        if email == "" {
            return errors.New("missing email in reset token")
        }

        
        // Delete the token after use
        s.redis.Del(ctx, key)
    } else {
        return errors.New("password reset not available")
    }
    
    // 2. Get user
    user, err := s.repo.GetUserByID(ctx, userID)
    if err != nil {
        return err
    }
    
    // 3. Hash new password
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), s.config.BCryptCost)
    if err != nil {
        return fmt.Errorf("failed to hash password: %w", err)
    }
    
    // 4. Update password
    hashedPasswordStr := string(hashedPassword)
    user.PasswordHash = &hashedPasswordStr
    if err := s.repo.UpdateUser(ctx, user); err != nil {
        return fmt.Errorf("failed to update password: %w", err)
    }
    
    // 5. Logout all devices for security
    s.repo.DeleteUserSessions(ctx, userID)
    
    return nil
}

// GoogleAuth handles Google OAuth
func (s *service) GoogleAuth(ctx context.Context, req *GoogleAuthRequest) (*AuthResponse, error) {
    // 1. Verify Google ID token
    oauth2Service, err := oauth2.NewService(ctx, option.WithoutAuthentication())
    if err != nil {
        return nil, fmt.Errorf("failed to create oauth2 service: %w", err)
    }
    
    tokenInfo, err := oauth2Service.Tokeninfo().IdToken(req.IDToken).Do()
    if err != nil {
        return nil, fmt.Errorf("invalid Google token: %w", err)
    }
    
    // 2. Check if user exists
    user, err := s.repo.GetUserByEmail(ctx, tokenInfo.Email)
    if err != nil {
        // 3. Create new user
        username := generateUsernameFromEmail(tokenInfo.Email)
        user = &User{
            Email:      &tokenInfo.Email,
            Username:   username,
            Provider:   "google",
            ProviderID: &tokenInfo.UserId,
            IsVerified: true, // Google accounts are pre-verified
            CreatedAt:  time.Now(),
            UpdatedAt:  time.Now(),
        }
        
        if err := s.repo.CreateUser(ctx, user); err != nil {
            return nil, fmt.Errorf("failed to create user: %w", err)
        }
    } else {
        // Update provider info if needed
        if user.Provider == "local" {
            user.Provider = "google"
            user.ProviderID = &tokenInfo.UserId
            s.repo.UpdateUser(ctx, user)
        }
    }
    
    // 4. Create session
    return s.createAuthSession(ctx, user)
}

// Helper functions

func (s *service) createAuthSession(ctx context.Context, user *User) (*AuthResponse, error) {
    accessToken, err := s.generateAccessToken(user)
    if err != nil {
        return nil, fmt.Errorf("failed to generate access token: %w", err)
    }
    
    refreshToken, err := s.generateRefreshToken(user)
    if err != nil {
        return nil, fmt.Errorf("failed to generate refresh token: %w", err)
    }
    
    session := &Session{
        UserID:       user.ID,
        Token:        accessToken,
        RefreshToken: refreshToken,
        ExpiresAt:    time.Now().Add(s.config.AccessTokenExpiry),
        CreatedAt:    time.Now(),
    }
    
    if err := s.repo.CreateSession(ctx, session); err != nil {
        return nil, fmt.Errorf("failed to create session: %w", err)
    }
    
    return &AuthResponse{
        User:         user,
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresIn:    int(s.config.AccessTokenExpiry.Seconds()),
        TokenType:    "Bearer",
    }, nil
}

func (s *service) storePendingAuth(ctx context.Context, token string, userID int64) error {
    if s.redis == nil {
        return errors.New("redis not available")
    }
    
    key := fmt.Sprintf("pending_auth:%s", token)
    data := map[string]interface{}{
        "user_id": userID,
        "expires": time.Now().Add(10 * time.Minute).Unix(),
    }
    jsonData, _ := json.Marshal(data)
    return s.redis.Set(ctx, key, jsonData, 10*time.Minute).Err()
}

func (s *service) getPendingAuth(ctx context.Context, token string) (int64, error) {
    if s.redis == nil {
        return 0, errors.New("redis not available")
    }
    
    key := fmt.Sprintf("pending_auth:%s", token)
    data, err := s.redis.Get(ctx, key).Result()
    if err != nil {
        return 0, err
    }
    
    var pendingData map[string]interface{}
    if err := json.Unmarshal([]byte(data), &pendingData); err != nil {
        return 0, err
    }
    
    return int64(pendingData["user_id"].(float64)), nil
}

func (s *service) clearPendingAuth(ctx context.Context, token string) {
    if s.redis != nil {
        key := fmt.Sprintf("pending_auth:%s", token)
        s.redis.Del(ctx, key)
    }
}

func (s *service) recordFailedAttempt(ctx context.Context, identifier string) {
    if s.redis == nil {
        return
    }
    key := fmt.Sprintf("failed:%s", identifier)
    s.redis.Incr(ctx, key)
    s.redis.Expire(ctx, key, 15*time.Minute)
}

func (s *service) clearFailedAttempts(ctx context.Context, identifier string) {
    if s.redis == nil {
        return
    }
    key := fmt.Sprintf("failed:%s", identifier)
    s.redis.Del(ctx, key)
}

// Implement remaining interface methods...
func (s *service) RefreshToken(ctx context.Context, refreshToken string) (*AuthResponse, error) {
    // Implementation as before
    claims, err := utils.ValidateJWT(refreshToken, s.config.JWTSecret)
    if err != nil {
        return nil, ErrInvalidToken
    }
    
    if claims.Type != "refresh" {
        return nil, ErrInvalidToken
    }
    
    session, err := s.repo.GetSessionByRefreshToken(ctx, refreshToken)
    if err != nil {
        return nil, err
    }
    
    user, err := s.repo.GetUserByID(ctx, session.UserID)
    if err != nil {
        return nil, err
    }
    
    return s.createAuthSession(ctx, user)
}

func (s *service) ValidateToken(ctx context.Context, token string) (*utils.JWTClaims, error) {
    return utils.ValidateJWT(token, s.config.JWTSecret)
}

func (s *service) Logout(ctx context.Context, token string) error {
    return s.repo.DeleteSessionByToken(ctx, token)
}

func (s *service) LogoutAllDevices(ctx context.Context, userID int64) error {
    return s.repo.DeleteUserSessions(ctx, userID)
}

func (s *service) GetUserByID(ctx context.Context, userID int64) (*User, error) {
    return s.repo.GetUserByID(ctx, userID)
}

func (s *service) generateAccessToken(user *User) (string, error) {
    email := ""
    if user.Email != nil {
        email = *user.Email
    }
    
    claims := &utils.JWTClaims{
        UserID:    user.ID,
        Email:     email,
        Username:  user.Username,
        Type:      "access",
        ExpiresAt: time.Now().Add(s.config.AccessTokenExpiry).Unix(),
        IssuedAt:  time.Now().Unix(),
        NotBefore: time.Now().Unix(),
        Issuer:    "kiekky-backend",
        Subject:   fmt.Sprintf("%d", user.ID),
    }
    
    return utils.GenerateJWT(claims, s.config.JWTSecret)
}

func (s *service) generateRefreshToken(user *User) (string, error) {
    claims := &utils.JWTClaims{
        UserID:    user.ID,
        Type:      "refresh",
        ExpiresAt: time.Now().Add(s.config.RefreshTokenExpiry).Unix(),
        IssuedAt:  time.Now().Unix(),
        NotBefore: time.Now().Unix(),
        Issuer:    "kiekky-backend",
        Subject:   fmt.Sprintf("%d", user.ID),
    }
    
    return utils.GenerateJWT(claims, s.config.JWTSecret)
}

func (s *service) generateSecureToken() string {
    b := make([]byte, 32)
    rand.Read(b)
    return base64.URLEncoding.EncodeToString(b)
}

// Utility functions
func isEmail(input string) bool {
    emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
    return emailRegex.MatchString(input)
}

func isPhone(input string) bool {
    phoneRegex := regexp.MustCompile(`^\+[1-9]\d{1,14}$`)
    return phoneRegex.MatchString(input)
}

func generateUsernameFromEmail(email string) string {
    parts := strings.Split(email, "@")
    base := parts[0]
    suffix := generateRandomString(4)
    return fmt.Sprintf("%s_%s", base, suffix)
}

func generateRandomString(length int) string {
    const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
    result := make([]byte, length)
    for i := range result {
        n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
        result[i] = charset[n.Int64()]
    }
    return string(result)
}