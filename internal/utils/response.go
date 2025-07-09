package utils

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// StandardResponse represents a standard API response
type StandardResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Message   string      `json:"message"`
	Timestamp string      `json:"timestamp"`
	Error     string      `json:"error,omitempty"`
}

// WriteSuccessResponse writes a successful JSON response using Gin
func WriteSuccessResponse(c *gin.Context, statusCode int, data interface{}, message string) {
	response := StandardResponse{
		Success:   true,
		Data:      data,
		Message:   message,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	c.JSON(statusCode, response)
}

// WriteErrorResponse writes an error JSON response using Gin
func WriteErrorResponse(c *gin.Context, statusCode int, message string) {
	response := StandardResponse{
		Success:   false,
		Message:   message,
		Error:     message,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	c.JSON(statusCode, response)
}

// WriteBadRequestError writes a 400 Bad Request error
func WriteBadRequestError(c *gin.Context, message string) {
	WriteErrorResponse(c, http.StatusBadRequest, message)
}

// WriteValidationError writes a 400 Bad Request validation error
func WriteValidationError(c *gin.Context, message string) {
	WriteErrorResponse(c, http.StatusBadRequest, "Validation error: "+message)
}

// WriteNotFoundError writes a 404 Not Found error
func WriteNotFoundError(c *gin.Context, message string) {
	WriteErrorResponse(c, http.StatusNotFound, message)
}

// WriteConflictError writes a 409 Conflict error
func WriteConflictError(c *gin.Context, message string) {
	WriteErrorResponse(c, http.StatusConflict, message)
}

// WriteInternalServerError writes a 500 Internal Server Error
func WriteInternalServerError(c *gin.Context, message string) {
	WriteErrorResponse(c, http.StatusInternalServerError, message)
}

// WriteJSONResponse writes a generic JSON response
func WriteJSONResponse(c *gin.Context, statusCode int, data interface{}) {
	c.JSON(statusCode, data)
}
