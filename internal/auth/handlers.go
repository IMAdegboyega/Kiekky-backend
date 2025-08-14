// internal/auth/handlers.go

package auth

import (
    "encoding/json"
    "net/http"
    "strings"
    
    "github.com/gorilla/mux"
    "github.com/imadgeboyega/kiekky-backend/internal/common/utils"
)

// Handler holds dependencies for auth endpoints
type Handler struct {
    service Service
}

// NewHandler creates a new auth handler
func NewHandler(service Service) *Handler {
    return &Handler{
        service: service,
    }
}

// RegisterRoutes registers all auth routes with the router
func (h *Handler) RegisterRoutes(router *mux.Router) {
    auth := router.PathPrefix("/api/auth").Subrouter()
    
    // Public routes
    auth.HandleFunc("/signup", h.Signup).Methods("POST")
    auth.HandleFunc("/signin", h.Signin).Methods("POST")
    auth.HandleFunc("/signin/verify-otp", h.VerifySigninOTP).Methods("POST")
    auth.HandleFunc("/google", h.GoogleAuth).Methods("POST")
    auth.HandleFunc("/verify-otp", h.VerifyOTP).Methods("POST")
    auth.HandleFunc("/resend-otp", h.ResendOTP).Methods("POST")
    auth.HandleFunc("/refresh", h.RefreshToken).Methods("POST")
    auth.HandleFunc("/forgot-password", h.ForgotPassword).Methods("POST")
    auth.HandleFunc("/reset-password", h.ResetPassword).Methods("POST")
    
    // Protected routes
    auth.HandleFunc("/logout", h.Logout).Methods("POST")
    auth.HandleFunc("/logout-all", h.LogoutAllDevices).Methods("POST")
}

// Signup handles user registration 
func (h *Handler) Signup(w http.ResponseWriter, r *http.Request) {
    var req SignupRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := utils.ValidateStruct(req); err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Call service - it should return SignupResponse
    response, err := h.service.Signup(r.Context(), &req)
    if err != nil {
        switch err {
        case ErrEmailAlreadyExists:
            utils.ErrorResponse(w, "Email already registered", http.StatusConflict)
        case ErrUsernameAlreadyExists:
            utils.ErrorResponse(w, "Username already taken", http.StatusConflict)
        default:
            utils.ErrorResponse(w, "Failed to create account", http.StatusInternalServerError)
        }
        return
    }
    
    // Build response based on SignupResponse structure
    var responseData map[string]interface{}
    
    if response.User != nil {
        responseData = map[string]interface{}{
            "message": response.Message,
            "user": map[string]interface{}{
                "id":       response.User.ID,
                "email":    response.User.Email,
                "username": response.User.Username,
            },
            "requires_verification": response.RequiresVerification,
        }
    } else {
        responseData = map[string]interface{}{
            "message":               response.Message,
            "requires_verification": response.RequiresVerification,
        }
    }
    
    utils.SuccessResponse(w, responseData, http.StatusCreated)
}

// Signin handles user login
func (h *Handler) Signin(w http.ResponseWriter, r *http.Request) {
    var req SigninRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := utils.ValidateStruct(req); err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    response, err := h.service.Signin(r.Context(), &req)
    if err != nil {
        switch err {
        case ErrInvalidCredentials:
            utils.ErrorResponse(w, "Invalid email/phone or password", http.StatusUnauthorized)
        case ErrTooManyAttempts:
            utils.ErrorResponse(w, "Too many login attempts. Please try again later.", http.StatusTooManyRequests)
        default:
            utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        }
        return
    }
    
    utils.SuccessResponse(w, response, http.StatusOK)
}

// VerifySigninOTP handles OTP verification during signin

func (h *Handler) VerifySigninOTP(w http.ResponseWriter, r *http.Request) {
    var req struct {
        PendingToken string `json:"pending_token" validate:"required"`
        OTP          string `json:"otp" validate:"required,len=6,numeric"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := utils.ValidateStruct(req); err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    authResp, err := h.service.VerifySigninOTP(r.Context(), req.PendingToken, req.OTP)
    if err != nil {
        if err == ErrInvalidOTP {
            utils.ErrorResponse(w, "Invalid OTP", http.StatusBadRequest)
            return
        }
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, authResp, http.StatusOK)
}

// GoogleAuth handles Google OAuth
func (h *Handler) GoogleAuth(w http.ResponseWriter, r *http.Request) {
    var req GoogleAuthRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := utils.ValidateStruct(req); err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    authResp, err := h.service.GoogleAuth(r.Context(), &req)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusUnauthorized)
        return
    }
    
    utils.SuccessResponse(w, authResp, http.StatusOK)
}

// VerifyOTP handles OTP verification for signup
func (h *Handler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
    var req OTPVerificationRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := utils.ValidateStruct(req); err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    authResp, err := h.service.VerifySignupOTP(r.Context(), &req)
    if err != nil {
        switch err {
        case ErrInvalidOTP:
            utils.ErrorResponse(w, "Invalid or expired OTP", http.StatusBadRequest)
        case ErrTooManyAttempts:
            utils.ErrorResponse(w, "Too many attempts. Please request a new code.", http.StatusTooManyRequests)
        default:
            utils.ErrorResponse(w, "Failed to verify OTP", http.StatusInternalServerError)
        }
        return
    }
    
    utils.SuccessResponse(w, authResp, http.StatusOK)
}

// ResendOTP handles OTP resend requests
func (h *Handler) ResendOTP(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Email string `json:"email" validate:"omitempty,email"`
        Phone string `json:"phone" validate:"omitempty,e164"`
        Type  string `json:"type" validate:"required"` // signup, signin, password_reset
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := utils.ValidateStruct(req); err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Create ResendOTPRequest
    resendReq := &ResendOTPRequest{
        Email: req.Email,
        Phone: req.Phone,
        Type:  req.Type,
    }
    
    if err := h.service.ResendOTP(r.Context(), resendReq); err != nil {
        if err == ErrTooManyAttempts {
            utils.ErrorResponse(w, "Too many requests. Please wait before trying again.", http.StatusTooManyRequests)
            return
        }
        utils.ErrorResponse(w, "Failed to send verification code", http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{
        "message": "Verification code sent successfully",
    }, http.StatusOK)
}

// RefreshToken handles token refresh
func (h *Handler) RefreshToken(w http.ResponseWriter, r *http.Request) {
    var req RefreshTokenRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := utils.ValidateStruct(req); err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    authResp, err := h.service.RefreshToken(r.Context(), req.RefreshToken)
    if err != nil {
        utils.ErrorResponse(w, "Invalid or expired refresh token", http.StatusUnauthorized)
        return
    }
    
    utils.SuccessResponse(w, authResp, http.StatusOK)
}

// ForgotPassword initiates password reset
func (h *Handler) ForgotPassword(w http.ResponseWriter, r *http.Request) {
    var req PasswordResetRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := utils.ValidateStruct(req); err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    h.service.InitiatePasswordReset(r.Context(), req.Email)
    
    utils.SuccessResponse(w, map[string]string{
        "message": "If an account exists with this email, you will receive password reset instructions.",
    }, http.StatusOK)
}

// ResetPassword completes password reset
func (h *Handler) ResetPassword(w http.ResponseWriter, r *http.Request) {
    var req PasswordResetConfirmRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
        return
    }
    
    if err := utils.ValidateStruct(req); err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    // Call service with individual parameters
    if err := h.service.ResetPassword(r.Context(), req.Token, req.NewPassword); err != nil {
        if err == ErrInvalidToken {
            utils.ErrorResponse(w, "Invalid or expired reset token", http.StatusBadRequest)
            return
        }
        utils.ErrorResponse(w, "Failed to reset password", http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{
        "message": "Password reset successfully. Please sign in with your new password.",
    }, http.StatusOK)
}

// Logout handles user logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
    authHeader := r.Header.Get("Authorization")
    if authHeader == "" {
        utils.ErrorResponse(w, "Missing authorization header", http.StatusUnauthorized)
        return
    }
    
    parts := strings.Split(authHeader, " ")
    if len(parts) != 2 || parts[0] != "Bearer" {
        utils.ErrorResponse(w, "Invalid authorization format", http.StatusUnauthorized)
        return
    }
    
    token := parts[1]
    
    if err := h.service.Logout(r.Context(), token); err != nil {
        utils.ErrorResponse(w, "Failed to logout", http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{
        "message": "Logged out successfully",
    }, http.StatusOK)
}

// LogoutAllDevices logs out from all devices
func (h *Handler) LogoutAllDevices(w http.ResponseWriter, r *http.Request) {
    userID, ok := r.Context().Value("userID").(int64)
    if !ok {
        utils.ErrorResponse(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    if err := h.service.LogoutAllDevices(r.Context(), userID); err != nil {
        utils.ErrorResponse(w, "Failed to logout from all devices", http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{
        "message": "Logged out from all devices successfully",
    }, http.StatusOK)
}