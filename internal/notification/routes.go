package notifications

import (
    "github.com/gorilla/mux"
    "github.com/imadgeboyega/kiekky-backend/internal/auth"
)

func RegisterRoutes(router *mux.Router, handler *Handler, authMiddleware *auth.Middleware) {
    // Protected routes
    api := router.PathPrefix("/api/v1/notifications").Subrouter()
    api.Use(authMiddleware.Authenticate)
    
    // User notifications
    api.HandleFunc("", handler.GetNotifications).Methods("GET")
    api.HandleFunc("/{id}", handler.GetNotification).Methods("GET")
    api.HandleFunc("/{id}/read", handler.MarkAsRead).Methods("PUT")
    api.HandleFunc("/read-all", handler.MarkAllAsRead).Methods("PUT")
    api.HandleFunc("/{id}", handler.DeleteNotification).Methods("DELETE")
    
    // Push tokens
    api.HandleFunc("/push-token", handler.RegisterPushToken).Methods("POST")
    api.HandleFunc("/push-token", handler.UnregisterPushToken).Methods("DELETE")
    
    // Preferences
    api.HandleFunc("/preferences", handler.GetPreferences).Methods("GET")
    api.HandleFunc("/preferences", handler.UpdatePreferences).Methods("PUT")
    
    // Test notification
    api.HandleFunc("/test", handler.TestPushNotification).Methods("POST")
    
    // Admin routes (should add admin middleware)
    admin := router.PathPrefix("/api/v1/admin/notifications").Subrouter()
    admin.Use(authMiddleware.Authenticate)
    // TODO: Add admin authorization middleware
    
    admin.HandleFunc("/send", handler.SendNotification).Methods("POST")
    admin.HandleFunc("/broadcast", handler.BroadcastNotification).Methods("POST")
    admin.HandleFunc("/schedule", handler.ScheduleNotification).Methods("POST")
    admin.HandleFunc("/schedule/{id}/cancel", handler.CancelScheduledNotification).Methods("PUT")
}