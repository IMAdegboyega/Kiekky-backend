// internal/profile/routes.go

package profile

import (
	"github.com/go-chi/chi/v5"
	"github.com/imadgeboyega/kiekky-backend/internal/auth"
)

// RegisterRoutes registers all profile routes
func RegisterRoutes(r chi.Router, handler *Handler, authMiddleware auth.Middleware) {
	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)

		// Profile viewing
		r.Get("/api/v1/profile", handler.GetMyProfile)
		r.Get("/api/v1/users/{id}/profile", handler.GetUserProfile)
		r.Put("/api/v1/profile", handler.UpdateProfile)
		
		// Profile setup
		r.Post("/api/v1/profile/setup", handler.SetupProfile)
		
		// Profile pictures
		r.Post("/api/v1/profile/picture", handler.UploadProfilePicture)
		r.Post("/api/v1/profile/cover", handler.UploadCoverPhoto)
		r.Delete("/api/v1/profile/picture", handler.DeleteProfilePicture)
		
		// Profile completion
		r.Get("/api/v1/profile/completion", handler.GetProfileCompletion)
		
		// Privacy & Settings
		r.Put("/api/v1/profile/privacy", handler.UpdatePrivacySettings)
		r.Put("/api/v1/profile/notifications", handler.UpdateNotificationSettings)
		
		// Blocking
		r.Get("/api/v1/profile/blocked", handler.GetBlockedUsers)
		r.Post("/api/v1/users/{id}/block", handler.BlockUser)
		r.Delete("/api/v1/users/{id}/block", handler.UnblockUser)
		
		// Discovery & Search
		r.Get("/api/v1/discover", handler.DiscoverProfiles)
		r.Get("/api/v1/search/users", handler.SearchUsers)
		
		// Profile views
		r.Post("/api/v1/profile/views/{id}", handler.RecordProfileView)
	})
}