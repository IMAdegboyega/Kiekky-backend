// internal/posts/routes.go
package posts

import (
	"github.com/gorilla/mux"
	"github.com/imadgeboyega/kiekky-backend/internal/auth"
)

func RegisterRoutes(router *mux.Router, handler *Handler, authMiddleware *auth.Middleware) {
	// Protected routes
	api := router.PathPrefix("/api/v1").Subrouter()
	api.Use(authMiddleware.Authenticate)
	
	// Feed operations - MUST COME BEFORE {id} routes!
	api.HandleFunc("/posts/feed", handler.GetFeed).Methods("GET")
	api.HandleFunc("/posts/explore", handler.GetExplorePosts).Methods("GET")
	
	// Post CRUD operations
	api.HandleFunc("/posts", handler.CreatePost).Methods("POST")
	api.HandleFunc("/posts/{id}", handler.GetPost).Methods("GET")
	api.HandleFunc("/posts/{id}", handler.UpdatePost).Methods("PUT")
	api.HandleFunc("/posts/{id}", handler.DeletePost).Methods("DELETE")
	
	// Like operations
	api.HandleFunc("/posts/{id}/like", handler.LikePost).Methods("POST")
	api.HandleFunc("/posts/{id}/like", handler.UnlikePost).Methods("DELETE")
	api.HandleFunc("/posts/{id}/likes", handler.GetPostLikes).Methods("GET")
	
	// Comment operations
	api.HandleFunc("/posts/{id}/comment", handler.AddComment).Methods("POST")
	api.HandleFunc("/posts/{id}/comments", handler.GetPostComments).Methods("GET")
	
	// User posts
	api.HandleFunc("/users/{id}/posts", handler.GetUserPosts).Methods("GET")
}