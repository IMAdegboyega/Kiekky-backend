// internal/messaging/hub.go

package messaging

import (
    "context"
    "encoding/json"
    "log"
    "sync"
    "time"
)

// Hub maintains active websocket connections
type Hub struct {
    // Registered clients
    clients    map[int64]*Client
    clientsMux sync.RWMutex
    
    // Message broadcast channels
    broadcast  chan BroadcastMessage
    
    // Register/unregister clients
    register   chan *Client
    unregister chan *Client
    
    // Services
    service    Service
    
    // Context for graceful shutdown
    ctx        context.Context
    cancel     context.CancelFunc
    
    // WaitGroup for pending operations
    wg         sync.WaitGroup
}

type BroadcastMessage struct {
    UserIDs []int64
    Message WSMessage
}

func NewHub(service Service) *Hub {
    ctx, cancel := context.WithCancel(context.Background())
    
    return &Hub{
        clients:    make(map[int64]*Client),
        broadcast:  make(chan BroadcastMessage, 256),
        register:   make(chan *Client),
        unregister: make(chan *Client),
        service:    service,
        ctx:        ctx,
        cancel:     cancel,
    }
}

func (h *Hub) Run() {
    defer func() {
        h.cleanup()
    }()
    
    for {
        select {
        case client := <-h.register:
            h.registerClient(client)
            
        case client := <-h.unregister:
            h.unregisterClient(client)
            
        case message := <-h.broadcast:
            h.broadcastMessage(message)
            
        case <-h.ctx.Done():
            return
        }
    }
}

func (h *Hub) registerClient(client *Client) {
    h.clientsMux.Lock()
    defer h.clientsMux.Unlock()
    
    // Remove old connection for the same user
    if oldClient, exists := h.clients[client.userID]; exists {
        oldClient.Close()
    }
    
    h.clients[client.userID] = client
    
    // Update user online status
    h.wg.Add(1)
    go func() {
        defer h.wg.Done()
        h.service.UpdateOnlineStatus(h.ctx, client.userID, true)
    }()
    
    // Send pending messages
    h.wg.Add(1)
    go func() {
        defer h.wg.Done()
        h.sendPendingMessages(client)
    }()
    
    // Notify contacts about online status
    h.wg.Add(1)
    go func() {
        defer h.wg.Done()
        h.notifyOnlineStatus(client.userID, true)
    }()
    
    log.Printf("User %d connected. Total clients: %d", client.userID, len(h.clients))
}

func (h *Hub) unregisterClient(client *Client) {
    h.clientsMux.Lock()
    defer h.clientsMux.Unlock()
    
    if _, exists := h.clients[client.userID]; exists {
        client.Close()
        delete(h.clients, client.userID)
        
        // Update user online status
        h.wg.Add(1)
        go func() {
            defer h.wg.Done()
            h.service.UpdateOnlineStatus(h.ctx, client.userID, false)
        }()
        
        // Notify contacts about offline status
        h.wg.Add(1)
        go func() {
            defer h.wg.Done()
            h.notifyOnlineStatus(client.userID, false)
        }()
        
        log.Printf("User %d disconnected. Total clients: %d", client.userID, len(h.clients))
    }
}

func (h *Hub) broadcastMessage(msg BroadcastMessage) {
    h.clientsMux.RLock()
    defer h.clientsMux.RUnlock()
    
    for _, userID := range msg.UserIDs {
        if client, exists := h.clients[userID]; exists {
            data, err := json.Marshal(msg.Message)
            if err != nil {
                log.Printf("Error marshalling message: %v", err)
                continue
            }
            
            select {
            case client.send <- data:
            default:
                // Unregister if channel is blocked
                go h.unregisterClient(client)
            }
        }
    }
}

func (h *Hub) cleanup() {
    // Close all client connections
    h.clientsMux.Lock()
    for _, client := range h.clients {
        client.Close()
    }
    h.clients = make(map[int64]*Client)
    h.clientsMux.Unlock()
    
    // Wait for all pending operations
    h.wg.Wait()
    
    // Close channels
    close(h.broadcast)
    close(h.register)
    close(h.unregister)
}

func (h *Hub) sendPendingMessages(client *Client) {
    // Get pending messages from service
    messages, err := h.service.GetPendingMessages(h.ctx, client.userID)
    if err != nil {
        log.Printf("Error getting pending messages: %v", err)
        return
    }
    
    // Send each pending message
    for _, msg := range messages {
        // Create WSMessage wrapper for consistency
        wsMsg := WSMessage{
            Type:      string(WSTypeMessage),
            Data:      mustMarshalJSON(msg),
            Timestamp: msg.CreatedAt,
        }
        
        data, err := json.Marshal(wsMsg)
        if err != nil {
            log.Printf("Error marshalling pending message: %v", err)
            continue
        }
        
        select {
        case client.send <- data:
            // Mark message as delivered for this specific user
            go h.service.MarkMessageDelivered(h.ctx, msg.ID, client.userID)
        default:
            // Channel blocked, skip
        }
    }
}

func (h *Hub) notifyOnlineStatus(userID int64, online bool) {
    // Get user's contacts
    contacts, err := h.service.GetUserContacts(h.ctx, userID)
    if err != nil {
        log.Printf("Error getting contacts: %v", err)
        return
    }
    
    // Prepare status message
    status := "offline"
    if online {
        status = "online"
    }
    
    msg := WSMessage{
        Type: "presence",
        Data: mustMarshalJSON(map[string]interface{}{
            "user_id": userID,
            "status":  status,
        }),
        Timestamp: time.Now(),
    }
    
    // Notify each contact
    for _, contactID := range contacts {
        h.SendToUser(contactID, msg)
    }
}

func (h *Hub) sendPushNotification(userID int64, message WSMessage) {
    // Get user's push tokens
    tokens, err := h.service.GetPushTokens(h.ctx, userID)
    if err != nil {
        log.Printf("Error getting push tokens: %v", err)
        return
    }
    
    // Send notification via service
    go h.service.SendPushNotification(h.ctx, tokens, message)
}

func (h *Hub) SendToUser(userID int64, message WSMessage) {
    h.clientsMux.RLock()
    client, exists := h.clients[userID]
    h.clientsMux.RUnlock()
    
    if !exists {
        // User offline, send push notification
        go h.sendPushNotification(userID, message)
        return
    }
    
    data, err := json.Marshal(message)
    if err != nil {
        return
    }
    
    select {
    case client.send <- data:
    default:
        go h.unregisterClient(client)
    }
}

func (h *Hub) SendToConversation(conversationID int64, message WSMessage, excludeUserID int64) {
    // Get conversation participants
    participants, err := h.service.GetConversationParticipants(h.ctx, conversationID)
    if err != nil {
        return
    }
    
    userIDs := make([]int64, 0, len(participants))
    for _, p := range participants {
        if p.UserID != excludeUserID {
            userIDs = append(userIDs, p.UserID)
        }
    }
    
    h.broadcastMessage(BroadcastMessage{
        UserIDs: userIDs,
        Message: message,
    })
}

func (h *Hub) IsUserOnline(userID int64) bool {
    h.clientsMux.RLock()
    defer h.clientsMux.RUnlock()
    
    _, exists := h.clients[userID]
    return exists
}

func (h *Hub) BroadcastToConversation(conversationID int64, message *WSMessage) {
    // Get all participants
    participants, err := h.service.GetConversationParticipants(context.Background(), conversationID)
    if err != nil {
        return
    }
    
    // Send to all participants
    for _, p := range participants {
        h.SendToUser(p.UserID, *message)
    }
}

func (h *Hub) BroadcastToConversationExcept(conversationID int64, exceptUserID int64, message *WSMessage) {
    // Get all participants
    participants, err := h.service.GetConversationParticipants(context.Background(), conversationID)
    if err != nil {
        return
    }
    
    // Send to all except specified user
    for _, p := range participants {
        if p.UserID != exceptUserID {
            h.SendToUser(p.UserID, *message)
        }
    }
}

func (h *Hub) Shutdown() {
    h.cancel()
    h.wg.Wait() // Wait for Run() to exit
}

func (h *Hub) GetActiveConnections() int {
    h.clientsMux.RLock()
    defer h.clientsMux.RUnlock()
    return len(h.clients)
}

func mustMarshalJSON(v interface{}) json.RawMessage {
    data, err := json.Marshal(v)
    if err != nil {
        log.Printf("Error marshaling: %v", err)
        return json.RawMessage(`{}`)
    }
    return json.RawMessage(data)
}