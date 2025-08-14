package stories

import (
    "github.com/gorilla/mux"
    "github.com/imadgeboyega/kiekky-backend/internal/auth"
)

func RegisterRoutes(router *mux.Router, handler *Handler, authMiddleware *auth.Middleware) {
    // Protected routes
    api := router.PathPrefix("/api/v1/stories").Subrouter()
    api.Use(authMiddleware.Authenticate)
    
    // Story management
    api.HandleFunc("", handler.CreateStory).Methods("POST")
    api.HandleFunc("", handler.GetActiveStories).Methods("GET")
    api.HandleFunc("/{id}", handler.GetStory).Methods("GET")
    api.HandleFunc("/{id}", handler.DeleteStory).Methods("DELETE")
    
    // Story interactions
    api.HandleFunc("/{id}/view", handler.ViewStory).Methods("POST")
    api.HandleFunc("/{id}/reply", handler.ReplyToStory).Methods("POST")
    api.HandleFunc("/{id}/views", handler.GetStoryViews).Methods("GET")
    api.HandleFunc("/{id}/replies", handler.GetStoryReplies).Methods("GET")
    api.HandleFunc("/replies/{replyId}/read", handler.MarkReplyAsRead).Methods("PUT")
    
    // User stories
    api.HandleFunc("/user/{userId}", handler.GetUserStories).Methods("GET")
    
    // Highlights
    api.HandleFunc("/highlights", handler.CreateHighlight).Methods("POST")
    api.HandleFunc("/highlights", handler.GetHighlights).Methods("GET")
    api.HandleFunc("/highlights/{id}", handler.DeleteHighlight).Methods("DELETE")
    
    // Media upload
    api.HandleFunc("/upload", handler.UploadMedia).Methods("POST")
}