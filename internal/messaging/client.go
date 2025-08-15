// internal/messaging/client.go

package messaging

import (
    "context"
    "encoding/json"
    "log"
    "time"
    
    "github.com/gorilla/websocket"
)

const (
    // Time allowed to write a message to the peer
    writeWait = 10 * time.Second
    
    // Time allowed to read the next pong message from the peer
    pongWait = 60 * time.Second
    
    // Send pings to peer with this period
    pingPeriod = (pongWait * 9) / 10
    
    // Maximum message size allowed from peer
    maxMessageSize = 512 * 1024 // 512KB
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

func (c *Client) close() {
    close(c.send)
}