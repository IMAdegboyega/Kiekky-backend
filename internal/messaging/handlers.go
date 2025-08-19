// internal/messaging/handlers.go

package messaging

import (
    "encoding/json"
    "net/http"
    "strconv"
    "log"
    "time"
    "fmt"
    
    "github.com/gorilla/mux"
    "github.com/gorilla/websocket"
    "github.com/imadgeboyega/kiekky-backend/internal/common/utils"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        // Configure CORS as needed
        return true
    },
}

type Handler struct {
    service Service
    hub     *Hub
}

func NewHandler(service Service, hub *Hub) *Handler {
    return &Handler{
        service: service,
        hub:     hub,
    }
}

// HandleWebSocket handles WebSocket connections
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
    // Get user ID from context (set by auth middleware)
    userID, ok := r.Context().Value("userID").(int64)
    if !ok {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    
    // Upgrade connection
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    
    // Create client
    client := NewClient(h.hub, conn, userID, h.service)
    
    // Register client
    h.hub.register <- client
    
    // Start client
    client.Start()
}

// CreateConversation creates a new conversation
func (h *Handler) GetConversation(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    conversationID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    
    conversation, err := h.service.GetConversation(r.Context(), conversationID, userID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusNotFound)
        return
    }
    
    utils.SuccessResponse(w, conversation, http.StatusOK)
}

// GetConversations gets user's conversations
func (h *Handler) GetConversations(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
    
    if limit <= 0 {
        limit = 20
    }
    
    conversations, err := h.service.GetUserConversations(r.Context(), userID, limit, offset)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, conversations, http.StatusOK)
}

// GetMessages gets conversation messages
func (h *Handler) GetMessages(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    conversationID, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    if err != nil {
        utils.ErrorResponse(w, "Invalid conversation ID", http.StatusBadRequest)
        return
    }
    
    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
    
    if limit <= 0 {
        limit = 50
    }
    
    messages, err := h.service.GetConversationMessages(r.Context(), conversationID, userID, limit, offset)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, messages, http.StatusOK)
}

// SendMessage sends a message (REST fallback)
func (h *Handler) SendMessage(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    var req SendMessageRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    message, err := h.service.SendMessage(r.Context(), userID, &req)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Broadcast via WebSocket
    wsMsg := WSMessage{
        Type:      string(WSTypeMessage),
        Data:      mustMarshal(message),
        Timestamp: message.CreatedAt,
    }
    
    h.hub.SendToConversation(req.ConversationID, wsMsg, userID)
    
    utils.SuccessResponse(w, message, http.StatusCreated)
}

// MarkRead marks messages as read
func (h *Handler) MarkRead(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    var req struct {
        MessageIDs []int64 `json:"message_ids"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    receipts, err := h.service.MarkMessagesRead(r.Context(), userID, req.MessageIDs)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, receipts, http.StatusOK)
}

// RegisterPushToken registers a push notification token
func (h *Handler) RegisterPushToken(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    var req PushTokenRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    if err := h.service.RegisterPushToken(r.Context(), userID, &req); err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "registered"}, http.StatusOK)
}

// BlockUser blocks a user from messaging
func (h *Handler) BlockUser(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    blockedUserID, err := strconv.ParseInt(mux.Vars(r)["userId"], 10, 64)
    if err != nil {
        utils.ErrorResponse(w, "Invalid user ID", http.StatusBadRequest)
        return
    }
    
    if err := h.service.BlockUser(r.Context(), userID, blockedUserID); err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "blocked"}, http.StatusOK)
}

func (h *Handler) UpdateConversation(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    conversationID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    
    // Verify user is participant
    if !h.service.IsUserInConversation(r.Context(), userID, conversationID) {
        utils.ErrorResponse(w, "Not authorized", http.StatusForbidden)
        return
    }
    
    var updates map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
        utils.ErrorResponse(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    // Update conversation (implement in service)
    utils.SuccessResponse(w, map[string]string{"status": "updated"}, http.StatusOK)
}

func (h *Handler) DeleteConversation(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    conversationID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    
    // Only creator can delete
    conv, err := h.service.GetConversation(r.Context(), conversationID, userID)
    if err != nil || conv.CreatedBy == nil || *conv.CreatedBy != userID {
        utils.ErrorResponse(w, "Not authorized", http.StatusForbidden)
        return
    }
    
    err = h.service.DeleteConversation(r.Context(), userID, conversationID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "deleted"}, http.StatusOK)
}

func (h *Handler) AddParticipants(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    conversationID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    
    var req struct {
        UserIDs []int64 `json:"user_ids"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    // Add participants (implement in service)
    for _, uid := range req.UserIDs {
        h.service.AddParticipant(r.Context(), conversationID, uid)
    }

    fmt.Printf("Added participants %v to conversation %d by user %d\n", req.UserIDs, conversationID, userID)
    
    utils.SuccessResponse(w, map[string]string{"status": "added"}, http.StatusOK)
}

func (h *Handler) RemoveParticipant(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    conversationID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    targetUserID, _ := strconv.ParseInt(mux.Vars(r)["userId"], 10, 64)
    
    // Implement authorization logic
    err := h.service.RemoveParticipant(r.Context(), userID, conversationID, targetUserID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "removed"}, http.StatusOK)
}

func (h *Handler) MuteConversation(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    conversationID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    
    err := h.service.MuteConversation(r.Context(), userID, conversationID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "muted"}, http.StatusOK)
}

func (h *Handler) UnmuteConversation(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    conversationID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    
    err := h.service.UnmuteConversation(r.Context(), userID, conversationID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "unmuted"}, http.StatusOK)
}

func (h *Handler) ArchiveConversation(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    conversationID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    
    err := h.service.ArchiveConversation(r.Context(), userID, conversationID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "archived"}, http.StatusOK)
}

func (h *Handler) UnarchiveConversation(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    conversationID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    
    err := h.service.UnarchiveConversation(r.Context(), userID, conversationID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "unarchived"}, http.StatusOK)
}

func (h *Handler) GetMessage(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    messageID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    
    message, err := h.service.GetMessage(r.Context(), messageID)
    if err != nil {
        utils.ErrorResponse(w, "Message not found", http.StatusNotFound)
        return
    }
    
    // Verify user has access
    if !h.service.IsUserInConversation(r.Context(), userID, message.ConversationID) {
        utils.ErrorResponse(w, "Not authorized", http.StatusForbidden)
        return
    }
    
    utils.SuccessResponse(w, message, http.StatusOK)
}

func (h *Handler) EditMessage(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    messageID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    
    var req struct {
        Content string `json:"content"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    message, err := h.service.EditMessage(r.Context(), messageID, userID, req.Content)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Broadcast update
    wsMsg := WSMessage{
        Type:      string(WSTypeMessageEdited),
        Data:      mustMarshal(message),
        Timestamp: time.Now(),
    }
    h.hub.SendToConversation(message.ConversationID, wsMsg, 0)
    
    utils.SuccessResponse(w, message, http.StatusOK)
}

func (h *Handler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    messageID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    
    err := h.service.DeleteMessage(r.Context(), messageID, userID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "deleted"}, http.StatusOK)
}

func (h *Handler) MarkDelivered(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    var req struct {
        MessageIDs []int64 `json:"message_ids"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    // Mark messages as delivered
    for _, msgID := range req.MessageIDs {
        h.service.MarkMessageDelivered(r.Context(), msgID, userID)
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "marked"}, http.StatusOK)
}

func (h *Handler) AddReaction(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    messageID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    
    var req struct {
        Emoji string `json:"emoji"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    reaction, err := h.service.AddReaction(r.Context(), userID, messageID, req.Emoji)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, reaction, http.StatusOK)
}

func (h *Handler) RemoveReaction(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    messageID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    reaction := mux.Vars(r)["reaction"]
    
    err := h.service.RemoveReaction(r.Context(), userID, messageID, reaction)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "removed"}, http.StatusOK)
}

func (h *Handler) GetReactions(w http.ResponseWriter, r *http.Request) {
    messageID, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
    
    reactions, err := h.service.GetReactions(r.Context(), messageID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, reactions, http.StatusOK)
}

func (h *Handler) UpdateTyping(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    var req struct {
        ConversationID int64 `json:"conversation_id"`
        IsTyping      bool  `json:"is_typing"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    err := h.service.UpdateTypingStatus(r.Context(), userID, req.ConversationID, req.IsTyping)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "updated"}, http.StatusOK)
}

func (h *Handler) UnregisterPushToken(w http.ResponseWriter, r *http.Request) {
    token := mux.Vars(r)["token"]
    
    err := h.service.UnregisterPushToken(r.Context(), token)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "unregistered"}, http.StatusOK)
}

func (h *Handler) GetPushTokens(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    tokens, err := h.service.GetPushTokens(r.Context(), userID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, tokens, http.StatusOK)
}

func (h *Handler) SearchMessages(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    query := r.URL.Query().Get("q")
    
    if query == "" {
        utils.ErrorResponse(w, "Search query required", http.StatusBadRequest)
        return
    }
    
    messages, err := h.service.SearchMessages(r.Context(), userID, query)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, messages, http.StatusOK)
}

func (h *Handler) UnblockUser(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    blockedUserID, _ := strconv.ParseInt(mux.Vars(r)["userId"], 10, 64)
    
    err := h.service.UnblockUser(r.Context(), userID, blockedUserID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"status": "unblocked"}, http.StatusOK)
}

func (h *Handler) GetBlockedUsers(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    users, err := h.service.GetBlockedUsers(r.Context(), userID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, users, http.StatusOK)
}

func (h *Handler) UploadMedia(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    // Parse multipart form
    err := r.ParseMultipartForm(10 << 20) // 10MB
    if err != nil {
        utils.ErrorResponse(w, "File too large", http.StatusBadRequest)
        return
    }
    
    file, header, err := r.FormFile("file")
    if err != nil {
        utils.ErrorResponse(w, "Invalid file", http.StatusBadRequest)
        return
    }
    defer file.Close()
    
    // Upload file (implement in service)
    url, err := h.service.UploadMedia(r.Context(), userID, file, header)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, map[string]string{"url": url}, http.StatusOK)
}

func (h *Handler) GetOnlineStatus(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    // Get user's contacts online status
    statuses, err := h.service.GetContactsOnlineStatus(r.Context(), userID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, statuses, http.StatusOK)
}

func (h *Handler) GetOrCreateDirectConversation(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    otherUserID, _ := strconv.ParseInt(mux.Vars(r)["userId"], 10, 64)
    
    conversation, err := h.service.GetOrCreateDirectConversation(r.Context(), userID, otherUserID)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, conversation, http.StatusOK)
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
    status := map[string]interface{}{
        "status": "healthy",
        "service": "messaging",
        "websocket_clients": h.hub.GetActiveConnections(),
    }
    
    utils.SuccessResponse(w, status, http.StatusOK)
}

func mustMarshal(v interface{}) json.RawMessage {
    data, err := json.Marshal(v)
    if err != nil {
        log.Printf("Failed to marshal: %v", err)
        return json.RawMessage(`{}`)
    }
    return data
}