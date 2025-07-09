package api

import (
	"time"

	"ip-allocator-api/internal/handlers"
	"ip-allocator-api/internal/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

func SetupRoutes(db *mongo.Database, logger *zap.Logger) *gin.Engine {
	// Set Gin mode based on environment
	gin.SetMode(gin.ReleaseMode) // Use gin.DebugMode for development

	// Create Gin router
	router := gin.New()

	// Add custom Zap logging middleware
	router.Use(middleware.ZapLogger(logger))
	router.Use(middleware.ZapRecovery(logger, true))

	// CORS configuration for production
	config := cors.Config{
		AllowOrigins:     []string{"*"}, // Configure specific origins for production
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}
	router.Use(cors.New(config))

	// Initialize handlers with Zap logger
	allocationHandler := handlers.NewAllocationHandler(db, logger)

	// Root-level health checks
	router.GET("/health", allocationHandler.HealthCheck)
	router.GET("/healthz", allocationHandler.HealthCheck)

	// API version group
	v1 := router.Group("/api/v1")
	{
		// Health check endpoints
		v1.GET("/health", allocationHandler.HealthCheck)

		// Region CRUD endpoints
		regions := v1.Group("/regions")
		{
			regions.GET("", allocationHandler.GetAllRegions)
			regions.POST("", allocationHandler.CreateRegion)
			regions.GET("/:region", allocationHandler.GetRegionHierarchy)
			regions.PUT("/:region", allocationHandler.UpdateRegion)
			regions.DELETE("/:region", allocationHandler.DeleteRegion)

			// Zone CRUD endpoints with enhanced CIDR support
			zones := regions.Group("/:region/zones")
			{
				zones.POST("", allocationHandler.CreateZone)
				zones.GET("/:zone", allocationHandler.GetZone)
				zones.PUT("/:zone", allocationHandler.UpdateZone)
				zones.DELETE("/:zone", allocationHandler.DeleteZone)

				// SubZone CRUD endpoints
				subzones := zones.Group("/:zone/subzones")
				{
					subzones.POST("", allocationHandler.CreateSubZone)
					subzones.GET("/:subzone", allocationHandler.GetSubZoneInfo)
					subzones.PUT("/:subzone", allocationHandler.UpdateSubZone)
					subzones.DELETE("/:subzone", allocationHandler.DeleteSubZone)

					// Utility endpoints
					subzones.GET("/:subzone/available", allocationHandler.GetAvailableIPs)
					subzones.GET("/:subzone/stats", allocationHandler.GetIPStats)
				}
			}
		}

		// IP management endpoints (grouped for better organization)
		ip := v1.Group("/ip")
		{
			ip.POST("/allocate", allocationHandler.AllocateIPs)
			ip.POST("/deallocate", allocationHandler.DeallocateIPs)
			ip.POST("/reserve", allocationHandler.ReserveIPs)
			ip.POST("/unreserve", allocationHandler.UnreserveIPs)
		}

		// Legacy endpoints for backward compatibility
		v1.POST("/allocate", allocationHandler.AllocateIPs)
		v1.POST("/deallocate", allocationHandler.DeallocateIPs)
		v1.POST("/reserve", allocationHandler.ReserveIPs)
		v1.POST("/unreserve", allocationHandler.UnreserveIPs)
	}

	return router
}
