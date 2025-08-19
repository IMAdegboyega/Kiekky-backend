// internal/messaging/client.go

package messaging

import (
    "context"
    "encoding/json"
    "log"
    "time"
    
    "github.com/gorilla/websocket"
)
// Client represents a websocket client
type Client struct {
    hub      *Hub
    conn     *websocket.Conn
    send     chan []byte
    userID   int64
    service  Service
}

func NewClient(hub *Hub, conn *websocket.Conn, userID int64, service Service) *Client {
    return &Client{
        hub:     hub,
        conn:    conn,
        send:    make(chan []byte, 256),
        userID:  userID,
        service: service,
    }
}

func (c *Client) Start() {
    go c.writePump()
    go c.readPump()
}

func (c *Client) readPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()
    
    c.conn.SetReadLimit(maxMessageSize)
    c.conn.SetReadDeadline(time.Now().Add(pongWait))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(pongWait))
        return nil
    })
    
    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
                log.Printf("WebSocket error: %v", err)
            }
            break
        }
        
        // Process incoming message
        go c.processMessage(message)
    }
}

func (c *Client) writePump() {
    ticker := time.NewTicker(pingPeriod)
    defer func() {
        ticker.Stop()
        c.conn.Close()
    }()
    
    for {
        select {
        case message, ok := <-c.send:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            
            w, err := c.conn.NextWriter(websocket.TextMessage)
            if err != nil {
                return
            }
            w.Write(message)
            
            // Add queued messages to the current websocket message
            n := len(c.send)
            for i := 0; i < n; i++ {
                w.Write([]byte{'\n'})
                w.Write(<-c.send)
            }
            
            if err := w.Close(); err != nil {
                return
            }
            
        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                return
            }
        }
    }
}

func (c *Client) processMessage(data []byte) {
    var msg WSMessage
    if err := json.Unmarshal(data, &msg); err != nil {
        log.Printf("Error unmarshaling message: %v", err)
        return
    }
    
    ctx := context.Background()
    
    switch WSMessageType(msg.Type) {
    case WSTypeMessage:
        c.handleNewMessage(ctx, msg.Data)
    case WSTypeTyping:
        c.handleTypingIndicator(ctx, msg.Data, true)
    case WSTypeStopTyping:
        c.handleTypingIndicator(ctx, msg.Data, false)
    case WSTypeRead:
        c.handleMarkRead(ctx, msg.Data)
    case WSTypeReaction:
        c.handleReaction(ctx, msg.Data)
    default:
        log.Printf("Unknown message type: %s", msg.Type)
    }
}

func (c *Client) handleNewMessage(ctx context.Context, payload json.RawMessage) {
    var messageData struct {
        ConversationID int64  `json:"conversation_id"`
        Content        string `json:"content"`
        MessageType    string `json:"message_type,omitempty"`
        TempID         string `json:"temp_id,omitempty"`
    }
    
    if err := json.Unmarshal(payload, &messageData); err != nil {
        log.Printf("Error parsing message data: %v", err)
        return
    }
    
    // Default to text if not specified
    if messageData.MessageType == "" {
        messageData.MessageType = "text"
    }
    
    // Create the message using SendMessageRequest
    req := &SendMessageRequest{
        ConversationID: messageData.ConversationID,
        Content:        messageData.Content,
        MessageType:    messageData.MessageType,
    }
    
    message, err := c.service.SendMessage(ctx, c.userID, req)
    if err != nil {
        log.Printf("Error creating message: %v", err)
        c.sendError("message_failed", err.Error(), messageData.TempID)
        return
    }
    
    // Broadcast via hub
    wsMsg := WSMessage{
        Type:      string(WSTypeMessage),
        Data:      mustMarshal(message),
        Timestamp: message.CreatedAt,
    }
    c.hub.SendToConversation(message.ConversationID, wsMsg, c.userID)
}

func (c *Client) handleTypingIndicator(ctx context.Context, payload json.RawMessage, isTyping bool) {
    var typingData struct {
        ConversationID int64 `json:"conversation_id"`
    }
    
    if err := json.Unmarshal(payload, &typingData); err != nil {
        log.Printf("Error parsing typing data: %v", err)
        return
    }
    
    // Verify user is part of conversation
    if !c.service.IsUserInConversation(ctx, c.userID, typingData.ConversationID) {
        return
    }
    
    // Update typing status
    c.service.UpdateTypingStatus(ctx, c.userID, typingData.ConversationID, isTyping)
    
    // Broadcast typing status
    eventType := WSTypeTyping
    if !isTyping {
        eventType = WSTypeStopTyping
    }
    
    wsMsg := WSMessage{
        Type: string(eventType),
        Data: mustMarshal(map[string]interface{}{
            "user_id":         c.userID,
            "conversation_id": typingData.ConversationID,
        }),
        Timestamp: time.Now(),
    }
    c.hub.SendToConversation(typingData.ConversationID, wsMsg, c.userID)
}

func (c *Client) handleMarkRead(ctx context.Context, payload json.RawMessage) {
    var readData struct {
        MessageIDs []int64 `json:"message_ids"`
    }
    
    if err := json.Unmarshal(payload, &readData); err != nil {
        log.Printf("Error parsing read data: %v", err)
        return
    }
    
    // Mark messages as read
    receipts, err := c.service.MarkMessagesRead(ctx, c.userID, readData.MessageIDs)
    if err != nil {
        log.Printf("Error marking as read: %v", err)
        return
    }
    
    // Broadcast read receipts
    for _, receipt := range receipts {
        // Get message to find conversation
        message, err := c.service.GetMessage(ctx, receipt.MessageID)
        if err != nil {
            continue
        }
        
        wsMsg := WSMessage{
            Type: string(WSTypeRead),
            Data: mustMarshal(map[string]interface{}{
                "user_id":    c.userID,
                "message_id": receipt.MessageID,
                "read_at":    receipt.ReadAt,
            }),
            Timestamp: time.Now(),
        }
        c.hub.SendToConversation(message.ConversationID, wsMsg, c.userID)
    }
}

func (c *Client) handleReaction(ctx context.Context, payload json.RawMessage) {
    var reactionData struct {
        MessageID int64  `json:"message_id"`
        Emoji     string `json:"emoji"`
        Action    string `json:"action"` // "add" or "remove"
    }
    
    if err := json.Unmarshal(payload, &reactionData); err != nil {
        log.Printf("Error parsing reaction data: %v", err)
        return
    }
    
    // Get message first to verify access
    message, err := c.service.GetMessage(ctx, reactionData.MessageID)
    if err != nil {
        return
    }
    
    // Verify user is in conversation
    if !c.service.IsUserInConversation(ctx, c.userID, message.ConversationID) {
        return
    }
    
    // Add or remove reaction
    var reaction *Reaction
    if reactionData.Action == "add" {
        reaction, err = c.service.AddReaction(ctx, c.userID, reactionData.MessageID, reactionData.Emoji)
    } else {
        err = c.service.RemoveReaction(ctx, c.userID, reactionData.MessageID, reactionData.Emoji)
    }
    
    if err != nil {
        log.Printf("Error handling reaction: %v", err)
        return
    }
    
    // Broadcast reaction update
    wsMsg := WSMessage{
        Type: string(WSTypeReaction),
        Data: mustMarshal(map[string]interface{}{
            "message_id": reactionData.MessageID,
            "user_id":    c.userID,
            "emoji":      reactionData.Emoji,
            "action":     reactionData.Action,
            "reaction":   reaction,
        }),
        Timestamp: time.Now(),
    }
    c.hub.SendToConversation(message.ConversationID, wsMsg, c.userID)
}

// Helper method to send error messages
func (c *Client) sendError(errorType, message string, tempID string) {
    errorData := map[string]interface{}{
        "error_type": errorType,
        "message":    message,
        "temp_id":    tempID,
    }
    
    errorMsg := &WSMessage{
        Type:      "error",
        Data:      mustMarshal(errorData),
        Timestamp: time.Now(),
    }
    
    data, err := json.Marshal(errorMsg)
    if err != nil {
        return
    }
    
    select {
    case c.send <- data:
    default:
        // Client send channel is full
    }
}

func (c *Client) Close() {
    close(c.send)
}