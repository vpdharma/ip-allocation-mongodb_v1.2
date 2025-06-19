package utils

import (
	"encoding/json"
	"net/http"
	"time"
)

type ErrorResponse struct {
	Success   bool      `json:"success"`
	Error     string    `json:"error"`
	Message   string    `json:"message,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type SuccessResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Message   string      `json:"message,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// WriteJSONResponse writes a JSON response with the given status code
func WriteJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// WriteErrorResponse writes an error response
func WriteErrorResponse(w http.ResponseWriter, statusCode int, errorType, message string) {
	response := ErrorResponse{
		Success:   false,
		Error:     errorType,
		Message:   message,
		Timestamp: time.Now(),
	}
	WriteJSONResponse(w, statusCode, response)
}

// WriteSuccessResponse writes a success response
func WriteSuccessResponse(w http.ResponseWriter, statusCode int, data interface{}, message string) {
	response := SuccessResponse{
		Success:   true,
		Data:      data,
		Message:   message,
		Timestamp: time.Now(),
	}
	WriteJSONResponse(w, statusCode, response)
}

// WriteBadRequestError writes a 400 Bad Request error
func WriteBadRequestError(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusBadRequest, "bad_request", message)
}

// WriteNotFoundError writes a 404 Not Found error
func WriteNotFoundError(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusNotFound, "not_found", message)
}

// WriteInternalServerError writes a 500 Internal Server Error
func WriteInternalServerError(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusInternalServerError, "internal_server_error", message)
}

// WriteConflictError writes a 409 Conflict error
func WriteConflictError(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusConflict, "conflict", message)
}

// WriteValidationError writes a 422 Unprocessable Entity error
func WriteValidationError(w http.ResponseWriter, message string) {
	WriteErrorResponse(w, http.StatusUnprocessableEntity, "validation_error", message)
}
