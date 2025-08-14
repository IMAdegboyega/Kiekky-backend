//internal/profile/handlers.go

package profile

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/imadgeboyega/kiekky-backend/internal/common/utils"
)

// Handler handles profile-related HTTP requests
type Handler struct {
	service   Service
	validator *validator.Validate
}

// NewHandler creates a new profile handler
func NewHandler(service Service) *Handler {
	return &Handler{
		service:   service,
		validator: validator.New(),
	}
}

// GetMyProfile handles getting current user's profile
func (h *Handler) GetMyProfile(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	profile, err := h.service.GetMyProfile(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			utils.ErrorResponse(w, "Profile not found", http.StatusNotFound)
			return
		}
		utils.ErrorResponse(w, "Failed to get profile", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, profile, http.StatusOK)
}

// GetUserProfile handles getting another user's profile
func (h *Handler) GetUserProfile(w http.ResponseWriter, r *http.Request) {
	viewerID := r.Context().Value("user_id").(int64)
	
	// Get user ID from URL parameter
	userIDStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		utils.ErrorResponse(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	profile, err := h.service.GetProfile(r.Context(), userID, viewerID)
	if err != nil {
		if errors.Is(err, ErrProfileNotFound) {
			utils.ErrorResponse(w, "Profile not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, ErrUserBlocked) {
			utils.ErrorResponse(w, "User is blocked", http.StatusForbidden)
			return
		}
		utils.ErrorResponse(w, "Failed to get profile", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, profile, http.StatusOK)
}

// UpdateProfile handles profile updates
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	var req UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	profile, err := h.service.UpdateProfile(r.Context(), userID, &req)
	if err != nil {
		utils.ErrorResponse(w, "Failed to update profile", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, profile, http.StatusOK)
}

// SetupProfile handles initial profile setup
func (h *Handler) SetupProfile(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	var req ProfileSetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	profile, err := h.service.SetupProfile(r.Context(), userID, &req)
	if err != nil {
		utils.ErrorResponse(w, "Failed to setup profile", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, profile, http.StatusOK)
}

// UploadProfilePicture handles profile picture upload
func (h *Handler) UploadProfilePicture(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	// Parse multipart form (max 10MB)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		utils.ErrorResponse(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		utils.ErrorResponse(w, "No image file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	url, err := h.service.UploadProfilePicture(r.Context(), userID, file, header)
	if err != nil {
		if errors.Is(err, ErrImageTooLarge) {
			utils.ErrorResponse(w, "Image size exceeds 5MB limit", http.StatusBadRequest)
			return
		}
		if errors.Is(err, ErrInvalidImageFormat) {
			utils.ErrorResponse(w, "Invalid image format. Supported: JPG, PNG, GIF, WebP", http.StatusBadRequest)
			return
		}
		utils.ErrorResponse(w, "Failed to upload profile picture", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, map[string]string{
		"url": url,
		"message": "Profile picture uploaded successfully",
	}, http.StatusOK)
}

// UploadCoverPhoto handles cover photo upload
func (h *Handler) UploadCoverPhoto(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	// Parse multipart form (max 10MB)
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		utils.ErrorResponse(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		utils.ErrorResponse(w, "No image file provided", http.StatusBadRequest)
		return
	}
	defer file.Close()

	url, err := h.service.UploadCoverPhoto(r.Context(), userID, file, header)
	if err != nil {
		if errors.Is(err, ErrImageTooLarge) {
			utils.ErrorResponse(w, "Image size exceeds 5MB limit", http.StatusBadRequest)
			return
		}
		if errors.Is(err, ErrInvalidImageFormat) {
			utils.ErrorResponse(w, "Invalid image format. Supported: JPG, PNG, GIF, WebP", http.StatusBadRequest)
			return
		}
		utils.ErrorResponse(w, "Failed to upload cover photo", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, map[string]string{
		"url": url,
		"message": "Cover photo uploaded successfully",
	}, http.StatusOK)
}

// DeleteProfilePicture handles profile picture deletion
func (h *Handler) DeleteProfilePicture(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	err := h.service.DeleteProfilePicture(r.Context(), userID)
	if err != nil {
		utils.ErrorResponse(w, "Failed to delete profile picture", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, map[string]string{
		"message": "Profile picture deleted successfully",
	}, http.StatusOK)
}

// GetProfileCompletion handles profile completion check
func (h *Handler) GetProfileCompletion(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	completion, err := h.service.GetProfileCompletion(r.Context(), userID)
	if err != nil {
		utils.ErrorResponse(w, "Failed to get profile completion", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, completion, http.StatusOK)
}

// UpdatePrivacySettings handles privacy settings update
func (h *Handler) UpdatePrivacySettings(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	var req UpdatePrivacyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if err := h.validator.Struct(req); err != nil {
		utils.ErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.service.UpdatePrivacySettings(r.Context(), userID, &req)
	if err != nil {
		utils.ErrorResponse(w, "Failed to update privacy settings", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, map[string]string{
		"message": "Privacy settings updated successfully",
	}, http.StatusOK)
}

// UpdateNotificationSettings handles notification settings update
func (h *Handler) UpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	var req UpdateNotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.service.UpdateNotificationSettings(r.Context(), userID, &req)
	if err != nil {
		utils.ErrorResponse(w, "Failed to update notification settings", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, map[string]string{
		"message": "Notification settings updated successfully",
	}, http.StatusOK)
}

// GetBlockedUsers handles getting blocked users list
func (h *Handler) GetBlockedUsers(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	blockedIDs, err := h.service.GetBlockedUsers(r.Context(), userID)
	if err != nil {
		utils.ErrorResponse(w, "Failed to get blocked users", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, map[string]interface{}{
		"blocked_users": blockedIDs,
	}, http.StatusOK)
}

// BlockUser handles blocking a user
func (h *Handler) BlockUser(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	// Get blocked user ID from URL parameter
	blockedIDStr := chi.URLParam(r, "id")
	blockedID, err := strconv.ParseInt(blockedIDStr, 10, 64)
	if err != nil {
		utils.ErrorResponse(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	err = h.service.BlockUser(r.Context(), userID, blockedID)
	if err != nil {
		if errors.Is(err, ErrCannotBlockSelf) {
			utils.ErrorResponse(w, "Cannot block yourself", http.StatusBadRequest)
			return
		}
		if errors.Is(err, ErrAlreadyBlocked) {
			utils.ErrorResponse(w, "User is already blocked", http.StatusBadRequest)
			return
		}
		utils.ErrorResponse(w, "Failed to block user", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, map[string]string{
		"message": "User blocked successfully",
	}, http.StatusOK)
}

// UnblockUser handles unblocking a user
func (h *Handler) UnblockUser(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	// Get blocked user ID from URL parameter
	blockedIDStr := chi.URLParam(r, "id")
	blockedID, err := strconv.ParseInt(blockedIDStr, 10, 64)
	if err != nil {
		utils.ErrorResponse(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	err = h.service.UnblockUser(r.Context(), userID, blockedID)
	if err != nil {
		utils.ErrorResponse(w, "Failed to unblock user", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, map[string]string{
		"message": "User unblocked successfully",
	}, http.StatusOK)
}

// DiscoverProfiles handles profile discovery
func (h *Handler) DiscoverProfiles(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	// Parse query parameters for filters
	filter := &DiscoverFilter{
		Limit:  20,
		Offset: 0,
	}

	if gender := r.URL.Query().Get("gender"); gender != "" {
		filter.Gender = &gender
	}
	if minAge := r.URL.Query().Get("min_age"); minAge != "" {
		age, _ := strconv.Atoi(minAge)
		filter.MinAge = &age
	}
	if maxAge := r.URL.Query().Get("max_age"); maxAge != "" {
		age, _ := strconv.Atoi(maxAge)
		filter.MaxAge = &age
	}
	if location := r.URL.Query().Get("location"); location != "" {
		filter.Location = &location
	}
	if distance := r.URL.Query().Get("max_distance"); distance != "" {
		dist, _ := strconv.Atoi(distance)
		filter.MaxDistance = &dist
	}
	if limit := r.URL.Query().Get("limit"); limit != "" {
		l, _ := strconv.Atoi(limit)
		if l > 0 && l <= 100 {
			filter.Limit = l
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		o, _ := strconv.Atoi(offset)
		if o >= 0 {
			filter.Offset = o
		}
	}

	profiles, err := h.service.DiscoverProfiles(r.Context(), userID, filter)
	if err != nil {
		utils.ErrorResponse(w, "Failed to discover profiles", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, map[string]interface{}{
		"profiles": profiles,
		"count":    len(profiles),
	}, http.StatusOK)
}

// SearchUsers handles user search
func (h *Handler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value("user_id").(int64)

	query := r.URL.Query().Get("q")
	if query == "" || len(query) < 2 {
		utils.ErrorResponse(w, "Search query must be at least 2 characters", http.StatusBadRequest)
		return
	}

	filter := &SearchFilter{
		Query:  query,
		Limit:  20,
		Offset: 0,
	}

	if limit := r.URL.Query().Get("limit"); limit != "" {
		l, _ := strconv.Atoi(limit)
		if l > 0 && l <= 100 {
			filter.Limit = l
		}
	}
	if offset := r.URL.Query().Get("offset"); offset != "" {
		o, _ := strconv.Atoi(offset)
		if o >= 0 {
			filter.Offset = o
		}
	}

	profiles, err := h.service.SearchUsers(r.Context(), userID, filter)
	if err != nil {
		utils.ErrorResponse(w, "Failed to search users", http.StatusInternalServerError)
		return
	}

	utils.SuccessResponse(w, map[string]interface{}{
		"profiles": profiles,
		"count":    len(profiles),
	}, http.StatusOK)
}

// RecordProfileView handles recording a profile view
func (h *Handler) RecordProfileView(w http.ResponseWriter, r *http.Request) {
	viewerID := r.Context().Value("user_id").(int64)

	// Get profile ID from URL parameter
	profileIDStr := chi.URLParam(r, "id")
	profileID, err := strconv.ParseInt(profileIDStr, 10, 64)
	if err != nil {
		utils.ErrorResponse(w, "Invalid profile ID", http.StatusBadRequest)
		return
	}

	err = h.service.RecordProfileView(r.Context(), viewerID, profileID)
	if err != nil {
		// Don't return error, just log it
		utils.SuccessResponse(w, map[string]string{
			"message": "View recorded",
		}, http.StatusOK)
		return
	}

	utils.SuccessResponse(w, map[string]string{
		"message": "Profile view recorded",
	}, http.StatusOK)
}