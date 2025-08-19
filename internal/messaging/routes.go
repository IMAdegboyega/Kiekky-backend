// internal/messaging/routes.go

package messaging

import (
    "github.com/gorilla/mux"
    "net/http"
)

// AuthMiddleware type for the authentication middleware function
type AuthMiddleware func(http.HandlerFunc) http.HandlerFunc

// RegisterRoutes registers all messaging routes
func RegisterRoutes(router *mux.Router, handler *Handler, authMiddleware AuthMiddleware) {
    // WebSocket endpoint - requires authentication
    router.HandleFunc("/ws", authMiddleware(handler.HandleWebSocket)).Methods("GET")
    
    // REST API endpoints
    api := router.PathPrefix("/api/v1/messages").Subrouter()
    
    // Apply auth middleware to all API routes
    api.Use(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authMiddleware(next.ServeHTTP)(w, r)
        })
    })
    
    // Conversation endpoints
    api.HandleFunc("/conversations", handler.GetConversations).Methods("GET")
    api.HandleFunc("/conversations/{id:[0-9]+}", handler.GetConversation).Methods("GET")
    api.HandleFunc("/conversations/{id:[0-9]+}", handler.UpdateConversation).Methods("PUT", "PATCH")
    api.HandleFunc("/conversations/{id:[0-9]+}", handler.DeleteConversation).Methods("DELETE")
    api.HandleFunc("/conversations/{id:[0-9]+}/participants", handler.AddParticipants).Methods("POST")
    api.HandleFunc("/conversations/{id:[0-9]+}/participants/{userId:[0-9]+}", handler.RemoveParticipant).Methods("DELETE")
    api.HandleFunc("/conversations/{id:[0-9]+}/mute", handler.MuteConversation).Methods("POST")
    api.HandleFunc("/conversations/{id:[0-9]+}/unmute", handler.UnmuteConversation).Methods("POST")
    api.HandleFunc("/conversations/{id:[0-9]+}/archive", handler.ArchiveConversation).Methods("POST")
    api.HandleFunc("/conversations/{id:[0-9]+}/unarchive", handler.UnarchiveConversation).Methods("POST")
    
    // Message endpoints
    api.HandleFunc("/conversations/{id:[0-9]+}/messages", handler.GetMessages).Methods("GET")
    api.HandleFunc("/messages", handler.SendMessage).Methods("POST")
    api.HandleFunc("/messages/{id:[0-9]+}", handler.GetMessage).Methods("GET")
    api.HandleFunc("/messages/{id:[0-9]+}", handler.EditMessage).Methods("PUT", "PATCH")
    api.HandleFunc("/messages/{id:[0-9]+}", handler.DeleteMessage).Methods("DELETE")
    
    // Message status endpoints
    api.HandleFunc("/messages/read", handler.MarkRead).Methods("POST")
    api.HandleFunc("/messages/delivered", handler.MarkDelivered).Methods("POST")
    
    // Reaction endpoints
    api.HandleFunc("/messages/{id:[0-9]+}/reactions", handler.AddReaction).Methods("POST")
    api.HandleFunc("/messages/{id:[0-9]+}/reactions/{reaction}", handler.RemoveReaction).Methods("DELETE")
    api.HandleFunc("/messages/{id:[0-9]+}/reactions", handler.GetReactions).Methods("GET")
    
    // Typing indicator endpoint
    api.HandleFunc("/typing", handler.UpdateTyping).Methods("POST")
    
    // Push token endpoints
    api.HandleFunc("/push-tokens", handler.RegisterPushToken).Methods("POST")
    api.HandleFunc("/push-tokens/{token}", handler.UnregisterPushToken).Methods("DELETE")
    api.HandleFunc("/push-tokens", handler.GetPushTokens).Methods("GET")
    
    // Search endpoint
    api.HandleFunc("/search", handler.SearchMessages).Methods("GET")
    
    // Blocking endpoints
    api.HandleFunc("/block/{userId:[0-9]+}", handler.BlockUser).Methods("POST")
    api.HandleFunc("/unblock/{userId:[0-9]+}", handler.UnblockUser).Methods("POST")
    api.HandleFunc("/blocked", handler.GetBlockedUsers).Methods("GET")
    
    // Media upload endpoint
    api.HandleFunc("/upload", handler.UploadMedia).Methods("POST")
    
    // Online status endpoint
    api.HandleFunc("/online-status", handler.GetOnlineStatus).Methods("GET")
    
    // Direct conversation helper
    api.HandleFunc("/conversations/direct/{userId:[0-9]+}", handler.GetOrCreateDirectConversation).Methods("GET", "POST")
}

// HealthCheck endpoint for the messaging service
func RegisterHealthCheck(router *mux.Router, handler *Handler) {
    router.HandleFunc("/health/messaging", handler.HealthCheck).Methods("GET")
} 