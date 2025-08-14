// internal/utils/response.go
// Standardized API responses ensure consistency across all endpoints

package utils

import (
	"encoding/json"
	"net/http"
)

// Response is the standard API response structure
type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SuccessResponse sends a successful response
func SuccessResponse(w http.ResponseWriter, data interface{}, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := Response{
		Success: true,
		Data:    data,
	}

	json.NewEncoder(w).Encode(response)
}

// ErrorResponse sends an error response
func ErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := Response{
		Success: false,
		Error:   message,
	}

	json.NewEncoder(w).Encode(response)
}

// MessageResponse sends a simple message response
func MessageResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := Response{
		Success: true,
		Message: message,
	}

	json.NewEncoder(w).Encode(response)
}
