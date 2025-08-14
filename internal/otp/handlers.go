// internal/otp/handlers.go

package otp

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-playground/validator/v10"
	"github.com/imadgeboyega/kiekky-backend/internal/common/utils"
)

// Handler handles OTP-related HTTP requests
type Handler struct {
	service   Service
	validator *validator.Validate
}

// NewHandler creates a new OTP handler
func NewHandler(service Service) *Handler {
	return &Handler{
		service:   service,
		validator: validator.New(),
	}
}

// SendOTP handles sending OTP requests
func (h *Handler) SendOTP(w http.ResponseWriter, r *http.Request) {
	var req SendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get user ID from context if authenticated (optional)
	if userID, ok := r.Context().Value("user_id").(int64); ok && req.UserID == 0 {
		req.UserID = userID
	}

	// Generate and send OTP
	response, err := h.service.GenerateOTP(r.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrRateLimitExceeded) {
			utils.ErrorResponse(w, err.Error(), http.StatusTooManyRequests)
			return
		}
		utils.ErrorResponse(w, "Failed to send OTP", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, response, http.StatusOK)
}

// VerifyOTP handles OTP verification requests
func (h *Handler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req VerifyOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get user ID from context if authenticated (optional)
	if userID, ok := r.Context().Value("user_id").(int64); ok && req.UserID == 0 {
		req.UserID = userID
	}

	// Verify OTP
	err := h.service.VerifyOTP(r.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, ErrOTPExpired):
			utils.ErrorResponse(w, "OTP has expired", http.StatusBadRequest)
		case errors.Is(err, ErrOTPInvalid):
			utils.ErrorResponse(w, "Invalid OTP code", http.StatusBadRequest)
		case errors.Is(err, ErrOTPMaxAttempts):
			utils.ErrorResponse(w, "Maximum verification attempts exceeded", http.StatusBadRequest)
		case errors.Is(err, ErrOTPAlreadyUsed):
			utils.ErrorResponse(w, "OTP has already been used", http.StatusBadRequest)
		default:
			utils.ErrorResponse(w, "Failed to verify OTP", http.StatusInternalServerError)
		}
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "OTP verified successfully",
	}
	utils.SuccessResponse(w, response, http.StatusOK)
}

// ResendOTP handles OTP resend requests
func (h *Handler) ResendOTP(w http.ResponseWriter, r *http.Request) {
	var req ResendOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get user ID from context if authenticated (optional)
	if userID, ok := r.Context().Value("user_id").(int64); ok && req.UserID == 0 {
		req.UserID = userID
	}

	// Resend OTP
	response, err := h.service.ResendOTP(r.Context(), &req)
	if err != nil {
		if errors.Is(err, ErrRateLimitExceeded) {
			utils.ErrorResponse(w, err.Error(), http.StatusTooManyRequests)
			return
		}
		utils.ErrorResponse(w, "Failed to resend OTP", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, response, http.StatusOK)
}