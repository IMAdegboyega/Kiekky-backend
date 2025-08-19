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

// RespondWithError sends an error response with the specified status code and message
func RespondWithError(w http.ResponseWriter, code int, message string) {
    RespondWithJSON(w, code, map[string]string{"error": message})
}

// RespondWithJSON sends a JSON response with the specified status code and payload
func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
    response, err := json.Marshal(payload)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        w.Write([]byte(`{"error":"Error marshaling JSON"}`))
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    w.Write(response)
}

// RespondWithDetailedError sends a detailed error response
func RespondWithDetailedError(w http.ResponseWriter, statusCode int, err error, message string) {
    errResp := Response{
        Error:   err.Error(),
        Message: message,
    }
    RespondWithJSON(w, statusCode, errResp)
}

// RespondWithData sends a success response with data wrapped in a standard format
func RespondWithData(w http.ResponseWriter, code int, data interface{}) {
    response := map[string]interface{}{
        "success": true,
        "data":    data,
    }
    RespondWithJSON(w, code, response)
}
