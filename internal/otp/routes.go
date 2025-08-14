// internal/otp/routes.go
package otp

import (
	"github.com/gorilla/mux"
)

// RegisterRoutes registers all OTP routes
func RegisterRoutes(router *mux.Router, handler *Handler) {
	// Public OTP routes (no auth required)
	otp := router.PathPrefix("/api/otp").Subrouter()
	
	// These endpoints are public because they're used during signup/password reset
	otp.HandleFunc("/send", handler.SendOTP).Methods("POST")
	otp.HandleFunc("/verify", handler.VerifyOTP).Methods("POST")
	otp.HandleFunc("/resend", handler.ResendOTP).Methods("POST")
}