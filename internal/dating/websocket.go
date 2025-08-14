package dating

import (
    "log"
    "net/http"
    
    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    CheckOrigin: func(r *http.Request) bool {
        // Configure origin checking in production
        return true
    },
}

type Hub struct {
    clients    map[int64]*Client
    broadcast  chan Message
    register   chan *Client
    unregister chan *Client
}

type Client struct {
    hub    *Hub
    conn   *websocket.Conn
    send   chan Message
    userID int64
}

type Message struct {
    Type   string      `json:"type"`
    UserID int64       `json:"user_id"`
    Data   interface{} `json:"data"`
}

func NewHub() *Hub {
    return &Hub{
        clients:    make(map[int64]*Client),
        broadcast:  make(chan Message),
        register:   make(chan *Client),
        unregister: make(chan *Client),
    }
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.clients[client.userID] = client
            log.Printf("User %d connected", client.userID)
            
        case client := <-h.unregister:
            if _, ok := h.clients[client.userID]; ok {
                delete(h.clients, client.userID)
                close(client.send)
                log.Printf("User %d disconnected", client.userID)
            }
            
        case message := <-h.broadcast:
            if client, ok := h.clients[message.UserID]; ok {
                select {
                case client.send <- message:
                default:
                    close(client.send)
                    delete(h.clients, client.userID)
                }
            }
        }
    }
}

func (h *Hub) NotifyDateRequest(receiverID int64, request *DateRequest) {
    message := Message{
        Type:   "date_request",
        UserID: receiverID,
        Data:   request,
    }
    h.broadcast <- message
}

func (h *Hub) NotifyMatch(user1ID, user2ID int64, match *Match) {
    message := Message{
        Type: "new_match",
        Data: match,
    }
    
    // Notify both users
    message.UserID = user1ID
    h.broadcast <- message
    
    message.UserID = user2ID
    h.broadcast <- message
}

func (h *Hub) NotifyDateReminder(userID int64, request *DateRequest) {
    message := Message{
        Type:   "date_reminder",
        UserID: userID,
        Data:   request,
    }
    h.broadcast <- message
}

func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
    // Get user ID from context (set by auth middleware)
    userID := r.Context().Value("userID").(int64)
    
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Println(err)
        return
    }
    
    client := &Client{
        hub:    h,
        conn:   conn,
        send:   make(chan Message, 256),
        userID: userID,
    }
    
    client.hub.register <- client
    
    go client.writePump()
    go client.readPump()
}

func (c *Client) readPump() {
    defer func() {
        c.hub.unregister <- c
        c.conn.Close()
    }()
    
    for {
        _, _, err := c.conn.ReadMessage()
        if err != nil {
            break
        }
    }
}

func (c *Client) writePump() {
    defer c.conn.Close()
    
    for {
        select {
        case message, ok := <-c.send:
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{})
                return
            }
            
            c.conn.WriteJSON(message)
        }
    }
}