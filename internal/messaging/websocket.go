// internal/messaging/websocket.go

package messaging

import (
    "encoding/json"
    "log"
    "net/http"
    "time"
    
    "github.com/gorilla/websocket"
)

// WebSocket configuration constants
const (
    // Time allowed to write a message to the peer
    writeWait = 10 * time.Second
    
    // Time allowed to read the next pong message from the peer
    pongWait = 60 * time.Second
    
    // Send pings to peer with this period (must be less than pongWait)
    pingPeriod = (pongWait * 9) / 10
    
    // Maximum message size allowed from peer
    maxMessageSize = 512 * 1024 // 512KB
    
    // Maximum number of queued messages per client
    maxQueuedMessages = 256
)

// Upgrader for WebSocket connections
var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
    CheckOrigin: func(r *http.Request) bool {
        // In production, implement proper CORS checking
        // For now, accept all origins
        return true
    },
}

// WSError represents a WebSocket error message
type WSError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

// WSResponse wraps a WebSocket response
type WSResponse struct {
    Type      string          `json:"type"`
    Success   bool            `json:"success"`
    Data      json.RawMessage `json:"data,omitempty"`
    Error     *WSError        `json:"error,omitempty"`
    Timestamp time.Time       `json:"timestamp"`
}

// CreateWSResponse creates a WebSocket response
func CreateWSResponse(msgType string, data interface{}, err error) WSResponse {
    response := WSResponse{
        Type:      msgType,
        Success:   err == nil,
        Timestamp: time.Now(),
    }
    
    if err != nil {
        response.Error = &WSError{
            Code:    "ERROR",
            Message: err.Error(),
        }
    } else if data != nil {
        response.Data = mustMarshal(data)
    }
    
    return response
}

// SendWSMessage sends a message through WebSocket
func SendWSMessage(conn *websocket.Conn, msgType string, data interface{}) error {
    response := CreateWSResponse(msgType, data, nil)
    
    conn.SetWriteDeadline(time.Now().Add(writeWait))
    return conn.WriteJSON(response)
}

// SendWSError sends an error message through WebSocket
func SendWSError(conn *websocket.Conn, msgType string, err error) error {
    response := CreateWSResponse(msgType, nil, err)
    
    conn.SetWriteDeadline(time.Now().Add(writeWait))
    return conn.WriteJSON(response)
}

// mustMarshal marshals data to JSON, panics on error
func mustMarshal(v interface{}) json.RawMessage {
    data, err := json.Marshal(v)
    if err != nil {
        log.Printf("Failed to marshal data: %v", err)
        return json.RawMessage(`{}`)
    }
    return data
}

// WSAuth represents WebSocket authentication data
type WSAuth struct {
    Token string `json:"token"`
}

// WSStats tracks WebSocket statistics
type WSStats struct {
    TotalConnections   int64     `json:"total_connections"`
    ActiveConnections  int       `json:"active_connections"`
    MessagesReceived   int64     `json:"messages_received"`
    MessagesSent       int64     `json:"messages_sent"`
    LastConnectionTime time.Time `json:"last_connection_time"`
}

// RateLimiter for WebSocket connections
type RateLimiter struct {
    requests map[string][]time.Time
    limit    int
    window   time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
    return &RateLimiter{
        requests: make(map[string][]time.Time),
        limit:    limit,
        window:   window,
    }
}

// Allow checks if a request is allowed
func (r *RateLimiter) Allow(key string) bool {
    now := time.Now()
    
    // Clean old requests
    if requests, exists := r.requests[key]; exists {
        validRequests := []time.Time{}
        for _, t := range requests {
            if now.Sub(t) < r.window {
                validRequests = append(validRequests, t)
            }
        }
        r.requests[key] = validRequests
    }
    
    // Check limit
    if len(r.requests[key]) >= r.limit {
        return false
    }
    
    // Add new request
    r.requests[key] = append(r.requests[key], now)
    return true
}

// Cleanup removes old entries from rate limiter
func (r *RateLimiter) Cleanup() {
    now := time.Now()
    for key, requests := range r.requests {
        validRequests := []time.Time{}
        for _, t := range requests {
            if now.Sub(t) < r.window {
                validRequests = append(validRequests, t)
            }
        }
        if len(validRequests) == 0 {
            delete(r.requests, key)
        } else {
            r.requests[key] = validRequests
        }
    }
}