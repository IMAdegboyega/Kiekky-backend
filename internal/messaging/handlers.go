// internal/messaging/handlers.go

package messaging

import (
    "encoding/json"
    "net/http"
    "strconv"
    
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
func (h *Handler) CreateConversation(w http.ResponseWriter, r *http.Request) {
    userID := r.Context().Value("userID").(int64)
    
    var req CreateConversationRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        utils.ErrorResponse(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    conversation, err := h.service.CreateConversation(r.Context(), userID, &req)
    if err != nil {
        utils.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    utils.SuccessResponse(w, conversation, http.StatusCreated)
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

func mustMarshal(v interface{}) json.RawMessage {
    data, _ := json.Marshal(v)
    return data
}
internal/messaging/
├── models.go           # Message data structures
├── service.go          # Messaging business logic
├── repository.go       # Database operations
├── postgres.go         # PostgreSQL implementation
├── handlers.go         # HTTP/WebSocket endpoints
├── routes.go           # Route registration
├── websocket.go        # WebSocket management
├── hub.go              # WebSocket connection manager
├── client.go           # WebSocket client handler
├── notifications.go    # Push notification service
└── storage.go          # Media message storage