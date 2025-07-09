package middleware

import (
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ZapLogger returns a gin.HandlerFunc that logs requests using Zap
func ZapLogger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery

		// Process request
		c.Next()

		// Calculate request processing time
		latency := time.Since(start)

		// Get client IP
		clientIP := getClientIP(c)

		// Build log fields
		fields := []zapcore.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("raw_query", raw),
			zap.Int("status", c.Writer.Status()),
			zap.String("client_ip", clientIP),
			zap.Duration("latency", latency),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int("body_size", c.Writer.Size()),
		}

		// Add error information if present
		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()))
		}

		// Log based on status code
		switch {
		case c.Writer.Status() >= 400 && c.Writer.Status() < 500:
			logger.Warn("Client error", fields...)
		case c.Writer.Status() >= 500:
			logger.Error("Server error", fields...)
		default:
			logger.Info("Request processed", fields...)
		}
	}
}

// ZapRecovery returns a gin.HandlerFunc that recovers from panics and logs using Zap
func ZapRecovery(logger *zap.Logger, stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Check for a broken connection
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") ||
							strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				// Build log fields
				fields := []zapcore.Field{
					zap.String("method", c.Request.Method),
					zap.String("path", c.Request.URL.Path),
					zap.String("client_ip", getClientIP(c)),
					zap.String("user_agent", c.Request.UserAgent()),
					zap.Any("error", err),
				}

				// Add stack trace if requested and not a broken pipe
				if stack && !brokenPipe {
					fields = append(fields, zap.String("stack", string(debug.Stack())))
				}

				// Add request dump for debugging (excluding sensitive data)
				if !brokenPipe {
					httpRequest, _ := httputil.DumpRequest(c.Request, false)
					fields = append(fields, zap.String("request", string(httpRequest)))
				}

				// Log the panic
				logger.Error("Panic recovered", fields...)

				// If the connection is dead, we can't write a status to it
				if brokenPipe {
					c.Error(err.(error))
					c.Abort()
					return
				}

				// Return structured error response
				c.JSON(http.StatusInternalServerError, gin.H{
					"success":   false,
					"message":   "Internal server error occurred",
					"timestamp": time.Now().Format(time.RFC3339),
				})
			}
		}()
		c.Next()
	}
}

// getClientIP gets the real client IP address
func getClientIP(c *gin.Context) string {
	// Check X-Forwarded-For header
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Check X-Real-IP header
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return ip
}

// LoggerConfig defines configuration for the logger middleware
type LoggerConfig struct {
	Logger         *zap.Logger
	SkipPaths      []string
	SkipPathRegexp []string
}

// ZapLoggerWithConfig returns a gin.HandlerFunc with custom configuration
func ZapLoggerWithConfig(config LoggerConfig) gin.HandlerFunc {
	skipPaths := make(map[string]bool, len(config.SkipPaths))
	for _, path := range config.SkipPaths {
		skipPaths[path] = true
	}

	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		// Skip logging for specified paths
		if skipPaths[path] {
			c.Next()
			return
		}

		raw := c.Request.URL.RawQuery
		c.Next()

		latency := time.Since(start)
		clientIP := getClientIP(c)

		fields := []zapcore.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("raw_query", raw),
			zap.Int("status", c.Writer.Status()),
			zap.String("client_ip", clientIP),
			zap.Duration("latency", latency),
			zap.String("user_agent", c.Request.UserAgent()),
			zap.Int("body_size", c.Writer.Size()),
		}

		if len(c.Errors) > 0 {
			fields = append(fields, zap.String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()))
		}

		switch {
		case c.Writer.Status() >= 400 && c.Writer.Status() < 500:
			config.Logger.Warn("Client error", fields...)
		case c.Writer.Status() >= 500:
			config.Logger.Error("Server error", fields...)
		default:
			config.Logger.Info("Request processed", fields...)
		}
	}
}
