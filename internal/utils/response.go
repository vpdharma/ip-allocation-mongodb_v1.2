package utils

import (
	"encoding/json"
	"net/http"
	"time"
)

// StandardResponse represents a standard API response
type StandardResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Message   string      `json:"message"`
	Timestamp string      `json:"timestamp"`
	Error     string      `json:"error,omitempty"`
}

// WriteJSONResponse writes a JSON response
func WriteJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// WriteSuccessResponse writes a successful response
func WriteSuccessResponse(w http.ResponseWriter, statusCode int, data interface{}, message string) {
	response := StandardResponse{
		Success:   true,
		Data:      data,
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	WriteJSONResponse(w, statusCode, response)
}

// WriteErrorResponse writes an error response with proper signature
func WriteErrorResponse(w http.ResponseWriter, statusCode int, message string) {
	response := StandardResponse{
		Success:   false,
		Message:   message,
		Error:     message,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	WriteJSONResponse(w, statusCode, response)
}

// WriteBadRequestError writes a 400 Bad Request error
func WriteBadRequestError(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusBadRequest, message)
}

// WriteValidationError writes a 400 Bad Request validation error
func WriteValidationError(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusBadRequest, "Validation error: "+message)
}

// WriteNotFoundError writes a 404 Not Found error
func WriteNotFoundError(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusNotFound, message)
}

// WriteConflictError writes a 409 Conflict error
func WriteConflictError(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusConflict, message)
}

// WriteInternalServerError writes a 500 Internal Server Error
func WriteInternalServerError(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusInternalServerError, message)
}
