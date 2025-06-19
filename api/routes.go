package api

import (
	"ip-allocator-api/internal/handlers"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"go.mongodb.org/mongo-driver/mongo"
)

func SetupRoutes(db *mongo.Database) http.Handler {
	// Initialize handlers
	allocationHandler := handlers.NewAllocationHandler(db)

	// Create router
	router := mux.NewRouter()

	// API version prefix
	api := router.PathPrefix("/api/v1").Subrouter()

	// Health check endpoint
	api.HandleFunc("/health", allocationHandler.HealthCheck).Methods("GET")

	// Region management endpoints
	api.HandleFunc("/regions", allocationHandler.GetAllRegions).Methods("GET")
	api.HandleFunc("/regions", allocationHandler.CreateRegion).Methods("POST")
	api.HandleFunc("/regions/{region}", allocationHandler.GetRegionHierarchy).Methods("GET")

	// Sub-zone information endpoint
	api.HandleFunc("/regions/{region}/zones/{zone}/subzones/{subzone}", 
		allocationHandler.GetSubZoneInfo).Methods("GET")

	// IP allocation endpoint
	api.HandleFunc("/allocate", allocationHandler.AllocateIPs).Methods("POST")

	// Add logging middleware
	router.Use(loggingMiddleware)

	// Add recovery middleware
	router.Use(recoveryMiddleware)

	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"}, // In production, specify actual origins
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	})

	return c.Handler(router)
}

// loggingMiddleware logs HTTP requests
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Call the next handler
		next.ServeHTTP(w, r)

		// Log the request
		log.Printf(
			"%s %s %s %v",
			r.Method,
			r.RequestURI,
			r.RemoteAddr,
			time.Since(start),
		)
	})
}

// recoveryMiddleware recovers from panics
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
