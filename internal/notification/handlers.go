package notifications

import (
    "encoding/json"
    "net/http"
    "strconv"
    "time"
    
    "github.com/gorilla/mux"
    "github.com/imadgeboyega/kiekky-backend/internal/common/utils"
)

type Handler struct {
    service Service
}

func NewHandler(service Service) *Handler {
    return &Handler{service: service}
}

// GetNotifications retrieves notifications for the authenticated user
func (h *Handler) GetNotifications(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    // Parse query parameters
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
    unreadOnly := r.URL.Query().Get("unread_only") == "true"
    
    if limit == 0 {
        limit = 20
    }
    
    response, err := h.service.GetNotifications(r.Context(), userID, limit, offset, unreadOnly)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get notifications")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, response)
}

// GetNotification retrieves a specific notification
func (h *Handler) GetNotification(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    vars := mux.Vars(r)
    
    notificationID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid notification ID")
        return
    }
    
    notification, err := h.service.GetNotification(r.Context(), notificationID, userID)
    if err != nil {
        if err == ErrNotificationNotFound {
            utils.RespondWithError(w, http.StatusNotFound, "Notification not found")
        } else if err == ErrUnauthorized {
            utils.RespondWithError(w, http.StatusForbidden, "Unauthorized")
        } else {
            utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get notification")
        }
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, notification)
}

// MarkAsRead marks a notification as read
func (h *Handler) MarkAsRead(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    vars := mux.Vars(r)
    
    notificationID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid notification ID")
        return
    }
    
    if err := h.service.MarkAsRead(r.Context(), notificationID, userID); err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to mark as read")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{
        "message": "Notification marked as read",
    })
}

// MarkAllAsRead marks all notifications as read for the user
func (h *Handler) MarkAllAsRead(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    if err := h.service.MarkAllAsRead(r.Context(), userID); err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to mark all as read")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{
        "message": "All notifications marked as read",
    })
}

// DeleteNotification deletes a notification
func (h *Handler) DeleteNotification(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    vars := mux.Vars(r)
    
    notificationID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid notification ID")
        return
    }
    
    if err := h.service.DeleteNotification(r.Context(), notificationID, userID); err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to delete notification")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{
        "message": "Notification deleted successfully",
    })
}

// RegisterPushToken registers a device push token
func (h *Handler) RegisterPushToken(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    var req RegisterPushTokenRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }
    
    // Validate request
    if req.Token == "" || req.Platform == "" || req.DeviceID == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "Token, platform, and device ID are required")
        return
    }
    
    if err := h.service.RegisterPushToken(r.Context(), userID, &req); err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to register push token")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{
        "message": "Push token registered successfully",
    })
}

// UnregisterPushToken unregisters a device push token
func (h *Handler) UnregisterPushToken(w http.ResponseWriter, r *http.Request) {
    token := r.URL.Query().Get("token")
    if token == "" {
        utils.RespondWithError(w, http.StatusBadRequest, "Token is required")
        return
    }
    
    if err := h.service.UnregisterPushToken(r.Context(), token); err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to unregister push token")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{
        "message": "Push token unregistered successfully",
    })
}

// GetPreferences retrieves notification preferences
func (h *Handler) GetPreferences(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    preferences, err := h.service.GetPreferences(r.Context(), userID)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to get preferences")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, preferences)
}

// UpdatePreferences updates notification preferences
func (h *Handler) UpdatePreferences(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    var req UpdatePreferencesRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }
    
    if err := h.service.UpdatePreferences(r.Context(), userID, &req); err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to update preferences")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{
        "message": "Preferences updated successfully",
    })
}

// SendNotification sends a notification (admin only)
func (h *Handler) SendNotification(w http.ResponseWriter, r *http.Request) {
    // TODO: Add admin authorization check
    
    var req CreateNotificationRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }
    
    notification, err := h.service.SendNotification(r.Context(), &req)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to send notification")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusCreated, notification)
}

// BroadcastNotification broadcasts a notification to multiple users (admin only)
func (h *Handler) BroadcastNotification(w http.ResponseWriter, r *http.Request) {
    // TODO: Add admin authorization check
    
    var req BroadcastNotificationRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }
    
    if err := h.service.SendBatchNotifications(r.Context(), &req); err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to broadcast notification")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{
        "message": "Notification broadcast successfully",
    })
}

// ScheduleNotification schedules a notification (admin only)
func (h *Handler) ScheduleNotification(w http.ResponseWriter, r *http.Request) {
    // TODO: Add admin authorization check
    
    var req ScheduleNotificationRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }
    
    // Validate scheduled time is in the future
    if req.ScheduledFor.Before(time.Now()) {
        utils.RespondWithError(w, http.StatusBadRequest, "Scheduled time must be in the future")
        return
    }
    
    scheduled, err := h.service.ScheduleNotification(r.Context(), &req)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to schedule notification")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusCreated, scheduled)
}

// CancelScheduledNotification cancels a scheduled notification (admin only)
func (h *Handler) CancelScheduledNotification(w http.ResponseWriter, r *http.Request) {
    // TODO: Add admin authorization check
    userID := r.Context().Value("userID").(int64)
    vars := mux.Vars(r)
    
    scheduledID, err := strconv.ParseInt(vars["id"], 10, 64)
    if err != nil {
        utils.RespondWithError(w, http.StatusBadRequest, "Invalid scheduled notification ID")
        return
    }
    
    if err := h.service.CancelScheduledNotification(r.Context(), scheduledID, userID); err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to cancel scheduled notification")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]string{
        "message": "Scheduled notification cancelled",
    })
}

// TestPushNotification sends a test push notification
func (h *Handler) TestPushNotification(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    req := &CreateNotificationRequest{
        UserID:   userID,
        Type:     TypePromotion,
        Title:    "Test Push Notification 🔔",
        Message:  "This is a test notification to verify your push settings are working correctly.",
        Channels: []DeliveryChannel{ChannelPush},
    }
    
    notification, err := h.service.SendNotification(r.Context(), req)
    if err != nil {
        utils.RespondWithError(w, http.StatusInternalServerError, "Failed to send test notification")
        return
    }
    
    utils.RespondWithJSON(w, http.StatusOK, map[string]interface{}{
        "message":      "Test notification sent",
        "notification": notification,
    })
}