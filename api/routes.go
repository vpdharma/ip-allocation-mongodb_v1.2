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

	// Root-level health check (standard practice)
	router.HandleFunc("/health", allocationHandler.HealthCheck).Methods("GET")
	router.HandleFunc("/healthz", allocationHandler.HealthCheck).Methods("GET")

	// API version prefix
	api := router.PathPrefix("/api/v1").Subrouter()

	// Health check endpoints
	api.HandleFunc("/health", allocationHandler.HealthCheck).Methods("GET")

	// === REGION CRUD ENDPOINTS ===
	api.HandleFunc("/regions", allocationHandler.GetAllRegions).Methods("GET")
	api.HandleFunc("/regions", allocationHandler.CreateRegion).Methods("POST")
	api.HandleFunc("/regions/{region}", allocationHandler.GetRegionHierarchy).Methods("GET")
	api.HandleFunc("/regions/{region}", allocationHandler.UpdateRegion).Methods("PUT")
	api.HandleFunc("/regions/{region}", allocationHandler.DeleteRegion).Methods("DELETE")

	// === ZONE CRUD ENDPOINTS ===
	api.HandleFunc("/regions/{region}/zones", allocationHandler.CreateZone).Methods("POST")
	api.HandleFunc("/regions/{region}/zones/{zone}", allocationHandler.GetZone).Methods("GET")
	api.HandleFunc("/regions/{region}/zones/{zone}", allocationHandler.UpdateZone).Methods("PUT")
	api.HandleFunc("/regions/{region}/zones/{zone}", allocationHandler.DeleteZone).Methods("DELETE")

	// === SUBZONE CRUD ENDPOINTS ===
	api.HandleFunc("/regions/{region}/zones/{zone}/subzones", allocationHandler.CreateSubZone).Methods("POST")
	api.HandleFunc("/regions/{region}/zones/{zone}/subzones/{subzone}", allocationHandler.GetSubZoneInfo).Methods("GET")
	api.HandleFunc("/regions/{region}/zones/{zone}/subzones/{subzone}", allocationHandler.UpdateSubZone).Methods("PUT")
	api.HandleFunc("/regions/{region}/zones/{zone}/subzones/{subzone}", allocationHandler.DeleteSubZone).Methods("DELETE")

	// === IP MANAGEMENT ENDPOINTS ===
	api.HandleFunc("/allocate", allocationHandler.AllocateIPs).Methods("POST")
	api.HandleFunc("/deallocate", allocationHandler.DeallocateIPs).Methods("POST")
	api.HandleFunc("/reserve", allocationHandler.ReserveIPs).Methods("POST")
	api.HandleFunc("/unreserve", allocationHandler.UnreserveIPs).Methods("POST")

	// === UTILITY ENDPOINTS ===
	api.HandleFunc("/regions/{region}/zones/{zone}/subzones/{subzone}/available", allocationHandler.GetAvailableIPs).Methods("GET")
	api.HandleFunc("/regions/{region}/zones/{zone}/subzones/{subzone}/stats", allocationHandler.GetIPStats).Methods("GET")

	// Add middleware in correct order
	router.Use(loggingMiddleware)
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

// loggingMiddleware logs HTTP requests with enhanced information
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a custom response writer to capture status code
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call the next handler
		next.ServeHTTP(rw, r)

		// Log the request with more details
		log.Printf(
			"[%s] %s %s %s %d %v %s",
			start.Format("2006-01-02 15:04:05"),
			r.Method,
			r.RequestURI,
			r.RemoteAddr,
			rw.statusCode,
			time.Since(start),
			r.UserAgent(),
		)
	})
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// recoveryMiddleware recovers from panics and provides detailed error information
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered in %s %s: %v", r.Method, r.URL.Path, err)

				// Return JSON error response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{
					"success": false,
					"message": "Internal server error occurred",
					"timestamp": "` + time.Now().Format(time.RFC3339) + `"
				}`))
			}
		}()

		next.ServeHTTP(w, r)
	})
}
