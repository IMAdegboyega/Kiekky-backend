package dating

import (
    "github.com/gorilla/mux"
    "github.com/imadgeboyega/kiekky-backend/internal/auth"
)

func RegisterRoutes(router *mux.Router, handler *Handler, authMiddleware *auth.Middleware) {
    api := router.PathPrefix("/api/v1/dating").Subrouter()
    api.Use(authMiddleware.Authenticate)
    
    // Date requests
    api.HandleFunc("/requests", handler.CreateDateRequest).Methods("POST")
    api.HandleFunc("/requests", handler.GetDateRequests).Methods("GET")
    api.HandleFunc("/requests/{id}/respond", handler.RespondToRequest).Methods("POST")
    api.HandleFunc("/requests/{id}/cancel", handler.CancelRequest).Methods("POST")
    api.HandleFunc("/upcoming", handler.GetUpcomingDates).Methods("GET")
    
    // Matches
    api.HandleFunc("/matches", handler.GetMatches).Methods("GET")
    api.HandleFunc("/matches/{id}/unmatch", handler.Unmatch).Methods("POST")
    api.HandleFunc("/matches/check/{userId}", handler.CheckMatch).Methods("GET")
    
    // Hotpicks
    api.HandleFunc("/hotpicks", handler.GetHotpicks).Methods("GET")
    api.HandleFunc("/hotpicks/{id}/action", handler.RecordAction).Methods("POST")
    api.HandleFunc("/hotpicks/generate", handler.GenerateHotpicks).Methods("POST")
    
    // Compatibility
    api.HandleFunc("/compatibility/{userId}", handler.GetCompatibility).Methods("GET")
    api.HandleFunc("/discover", handler.DiscoverMatches).Methods("GET")
}