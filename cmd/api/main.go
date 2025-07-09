package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ip-allocator-api/api"
	"ip-allocator-api/internal/config"
	"ip-allocator-api/internal/database"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func main() {
	// Initialize Zap logger
	logger, err := initZapLogger()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting IP Allocator API",
		zap.String("version", "2.0.0"),
		zap.String("go_version", "1.24"),
		zap.String("framework", "Gin"))

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	logger.Info("Configuration loaded",
		zap.String("host", cfg.Server.Host),
		zap.String("port", cfg.Server.Port),
		zap.String("database", cfg.MongoDB.Database))

	// Connect to MongoDB
	client, err := database.ConnectDB(cfg.MongoDB.URI, cfg.MongoDB.Database)
	if err != nil {
		logger.Fatal("Failed to connect to MongoDB", zap.Error(err))
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.Disconnect(ctx); err != nil {
			logger.Error("Failed to disconnect from MongoDB", zap.Error(err))
		}
	}()

	logger.Info("Successfully connected to MongoDB")

	// Setup routes with Gin framework
	router := api.SetupRoutes(client.Database(cfg.MongoDB.Database), logger)

	// Create HTTP server with production-ready settings
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		logger.Info("Starting HTTP server",
			zap.String("address", server.Addr))

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited")
}

// initZapLogger initializes a production-ready Zap logger with custom configuration
func initZapLogger() (*zap.Logger, error) {
	config := zap.NewProductionConfig()

	// Customize log level and encoding
	config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
	config.Encoding = "json"

	// Customize time encoding for better readability
	config.EncoderConfig.TimeKey = "timestamp"
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Add caller information for debugging
	config.EncoderConfig.CallerKey = "caller"
	config.EncoderConfig.EncodeCaller = zapcore.ShortCallerEncoder

	// Customize level encoding
	config.EncoderConfig.LevelKey = "level"
	config.EncoderConfig.EncodeLevel = zapcore.LowercaseLevelEncoder

	// Build logger with caller and stack trace
	logger, err := config.Build(
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
		zap.Fields(zap.String("service", "ip-allocator-api")))
	if err != nil {
		return nil, err
	}

	return logger, nil
}
