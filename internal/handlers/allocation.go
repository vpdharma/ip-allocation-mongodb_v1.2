package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"ip-allocator-api/internal/models"
	"ip-allocator-api/internal/services"
	"ip-allocator-api/internal/utils"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
)

type AllocationHandler struct {
	service     *services.AllocationService
	crudService *services.CRUDService
	validator   *validator.Validate
}

func NewAllocationHandler(db *mongo.Database) *AllocationHandler {
	return &AllocationHandler{
		service:     services.NewAllocationService(db),
		crudService: services.NewCRUDService(db),
		validator:   validator.New(),
	}
}

// ===============================
// IP ALLOCATION METHODS
// ===============================

// AllocateIPs handles IP allocation requests
func (h *AllocationHandler) AllocateIPs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var req models.AllocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteBadRequestError(w, "Invalid JSON payload")
		return
	}

	// Set default count if not specified
	if req.Count == 0 {
		req.Count = 1
	}

	// Validate request
	if err := h.validator.Struct(&req); err != nil {
		utils.WriteValidationError(w, err.Error())
		return
	}

	// Additional validation for IP version
	if !utils.ValidateIPVersion(req.IPVersion) {
		utils.WriteBadRequestError(w, "Invalid IP version. Must be 'ipv4', 'ipv6', or 'both'")
		return
	}

	// Validate preferred IPs if provided
	for _, ip := range req.PreferredIPs {
		if utils.NormalizeIP(ip) == "" {
			utils.WriteBadRequestError(w, "Invalid IP address in preferred IPs: "+ip)
			return
		}
	}

	// Call service to allocate IPs
	response, err := h.service.AllocateIPs(ctx, &req)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to allocate IPs: "+err.Error())
		return
	}

	// Return response based on success
	if response.Success {
		utils.WriteSuccessResponse(w, http.StatusOK, response, "")
	} else {
		utils.WriteBadRequestError(w, response.Message)
	}
}

// DeallocateIPs handles IP deallocation requests
func (h *AllocationHandler) DeallocateIPs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var req models.DeallocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteBadRequestError(w, "Invalid JSON payload")
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		utils.WriteValidationError(w, err.Error())
		return
	}

	// Validate IP addresses
	for _, ip := range req.IPAddresses {
		if utils.NormalizeIP(ip) == "" {
			utils.WriteBadRequestError(w, "Invalid IP address: "+ip)
			return
		}
	}

	response, err := h.service.DeallocateIPs(ctx, &req)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to deallocate IPs: "+err.Error())
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, response, "")
}

// ReserveIPs handles IP reservation requests
func (h *AllocationHandler) ReserveIPs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var req models.ReservationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteBadRequestError(w, "Invalid JSON payload")
		return
	}

	// Set reservation type to reserve
	req.ReservationType = "reserve"

	if err := h.validator.Struct(&req); err != nil {
		utils.WriteValidationError(w, err.Error())
		return
	}

	// Validate IP addresses
	for _, ip := range req.IPAddresses {
		if utils.NormalizeIP(ip) == "" {
			utils.WriteBadRequestError(w, "Invalid IP address: "+ip)
			return
		}
	}

	response, err := h.service.ManageReservations(ctx, &req)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to reserve IPs: "+err.Error())
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, response, "")
}

// UnreserveIPs handles IP unreservation requests
func (h *AllocationHandler) UnreserveIPs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var req models.ReservationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteBadRequestError(w, "Invalid JSON payload")
		return
	}

	// Set reservation type to unreserve
	req.ReservationType = "unreserve"

	if err := h.validator.Struct(&req); err != nil {
		utils.WriteValidationError(w, err.Error())
		return
	}

	// Validate IP addresses
	for _, ip := range req.IPAddresses {
		if utils.NormalizeIP(ip) == "" {
			utils.WriteBadRequestError(w, "Invalid IP address: "+ip)
			return
		}
	}

	response, err := h.service.ManageReservations(ctx, &req)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to unreserve IPs: "+err.Error())
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, response, "")
}

// ===============================
// REGION CRUD METHODS
// ===============================

// GetAllRegions returns all regions
func (h *AllocationHandler) GetAllRegions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	regions, err := h.service.GetAllRegions(ctx)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to get regions: "+err.Error())
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, regions, "Regions retrieved successfully")
}

// GetRegionHierarchy returns the complete hierarchy for a region
func (h *AllocationHandler) GetRegionHierarchy(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	regionName := vars["region"]

	if regionName == "" {
		utils.WriteBadRequestError(w, "Region name is required")
		return
	}

	region, err := h.service.GetRegionHierarchy(ctx, regionName)
	if err != nil {
		if err.Error() == "region '"+regionName+"' not found" {
			utils.WriteNotFoundError(w, err.Error())
		} else {
			utils.WriteInternalServerError(w, "Failed to get region hierarchy: "+err.Error())
		}
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, region, "Region hierarchy retrieved successfully")
}

// CreateRegion creates a new region
func (h *AllocationHandler) CreateRegion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var region models.Region
	if err := json.NewDecoder(r.Body).Decode(&region); err != nil {
		utils.WriteBadRequestError(w, "Invalid JSON payload")
		return
	}

	// Validate request
	if err := h.validator.Struct(&region); err != nil {
		utils.WriteValidationError(w, err.Error())
		return
	}

	// Validate CIDR blocks in sub-zones
	for _, zone := range region.Zones {
		for _, subZone := range zone.SubZones {
			// Validate IPv4 CIDR
			if subZone.IPv4CIDR != "" {
				if _, err := utils.ParseCIDR(subZone.IPv4CIDR); err != nil {
					utils.WriteBadRequestError(w, "Invalid IPv4 CIDR in sub-zone "+subZone.Name+": "+err.Error())
					return
				}
			}

			// Validate IPv6 CIDR
			if subZone.IPv6CIDR != "" {
				if _, err := utils.ParseCIDR(subZone.IPv6CIDR); err != nil {
					utils.WriteBadRequestError(w, "Invalid IPv6 CIDR in sub-zone "+subZone.Name+": "+err.Error())
					return
				}
			}
		}
	}

	// Create region
	if err := h.service.CreateRegion(ctx, &region); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			utils.WriteConflictError(w, "Region with this name already exists")
		} else {
			utils.WriteInternalServerError(w, "Failed to create region: "+err.Error())
		}
		return
	}

	utils.WriteSuccessResponse(w, http.StatusCreated, region, "Region created successfully")
}

// UpdateRegion updates an existing region
func (h *AllocationHandler) UpdateRegion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	regionName := vars["region"]

	var req models.UpdateRegionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteBadRequestError(w, "Invalid JSON payload")
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		utils.WriteValidationError(w, err.Error())
		return
	}

	response, err := h.crudService.UpdateRegion(ctx, regionName, &req)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to update region: "+err.Error())
		return
	}

	if response.Success {
		utils.WriteSuccessResponse(w, http.StatusOK, response, "")
	} else {
		utils.WriteBadRequestError(w, response.Message)
	}
}

// DeleteRegion deletes a region
func (h *AllocationHandler) DeleteRegion(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	regionName := vars["region"]

	response, err := h.crudService.DeleteRegion(ctx, regionName)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to delete region: "+err.Error())
		return
	}

	if response.Success {
		utils.WriteSuccessResponse(w, http.StatusOK, response, "")
	} else {
		utils.WriteNotFoundError(w, response.Message)
	}
}

// ===============================
// ZONE CRUD METHODS
// ===============================

// CreateZone creates a new zone within a region
func (h *AllocationHandler) CreateZone(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	regionName := vars["region"]

	var req models.CreateZoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteBadRequestError(w, "Invalid JSON payload")
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		utils.WriteValidationError(w, err.Error())
		return
	}

	response, err := h.crudService.CreateZone(ctx, regionName, &req)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to create zone: "+err.Error())
		return
	}

	if response.Success {
		utils.WriteSuccessResponse(w, http.StatusCreated, response, "")
	} else {
		utils.WriteBadRequestError(w, response.Message)
	}
}

// GetZone returns information about a specific zone
func (h *AllocationHandler) GetZone(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	regionName := vars["region"]
	zoneName := vars["zone"]

	response, err := h.crudService.GetZone(ctx, regionName, zoneName)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to get zone: "+err.Error())
		return
	}

	if response.Success {
		utils.WriteSuccessResponse(w, http.StatusOK, response, "")
	} else {
		utils.WriteNotFoundError(w, response.Message)
	}
}

// UpdateZone updates an existing zone
func (h *AllocationHandler) UpdateZone(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	regionName := vars["region"]
	zoneName := vars["zone"]

	var req models.UpdateZoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteBadRequestError(w, "Invalid JSON payload")
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		utils.WriteValidationError(w, err.Error())
		return
	}

	response, err := h.crudService.UpdateZone(ctx, regionName, zoneName, &req)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to update zone: "+err.Error())
		return
	}

	if response.Success {
		utils.WriteSuccessResponse(w, http.StatusOK, response, "")
	} else {
		utils.WriteBadRequestError(w, response.Message)
	}
}

// DeleteZone deletes a zone
func (h *AllocationHandler) DeleteZone(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	regionName := vars["region"]
	zoneName := vars["zone"]

	response, err := h.crudService.DeleteZone(ctx, regionName, zoneName)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to delete zone: "+err.Error())
		return
	}

	if response.Success {
		utils.WriteSuccessResponse(w, http.StatusOK, response, "")
	} else {
		utils.WriteNotFoundError(w, response.Message)
	}
}

// ===============================
// SUBZONE CRUD METHODS
// ===============================

// CreateSubZone creates a new sub-zone within a zone
func (h *AllocationHandler) CreateSubZone(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	regionName := vars["region"]
	zoneName := vars["zone"]

	var req models.CreateSubZoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteBadRequestError(w, "Invalid JSON payload")
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		utils.WriteValidationError(w, err.Error())
		return
	}

	response, err := h.crudService.CreateSubZone(ctx, regionName, zoneName, &req)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to create sub-zone: "+err.Error())
		return
	}

	if response.Success {
		utils.WriteSuccessResponse(w, http.StatusCreated, response, "")
	} else {
		utils.WriteBadRequestError(w, response.Message)
	}
}

// GetSubZoneInfo returns detailed information about a specific sub-zone
func (h *AllocationHandler) GetSubZoneInfo(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	regionName := vars["region"]
	zoneName := vars["zone"]
	subZoneName := vars["subzone"]

	if regionName == "" || zoneName == "" || subZoneName == "" {
		utils.WriteBadRequestError(w, "Region, zone, and sub-zone names are required")
		return
	}

	region, err := h.service.GetRegionHierarchy(ctx, regionName)
	if err != nil {
		if err.Error() == "region '"+regionName+"' not found" {
			utils.WriteNotFoundError(w, err.Error())
		} else {
			utils.WriteInternalServerError(w, "Failed to get region hierarchy: "+err.Error())
		}
		return
	}

	// Find the specific sub-zone
	var targetSubZone *models.SubZone
	for _, zone := range region.Zones {
		if zone.Name == zoneName {
			for _, subZone := range zone.SubZones {
				if subZone.Name == subZoneName {
					targetSubZone = &subZone
					break
				}
			}
			break
		}
	}

	if targetSubZone == nil {
		utils.WriteNotFoundError(w, "Sub-zone not found")
		return
	}

	// Calculate additional information
	ipv4Count, _ := utils.CountIPsInCIDR(targetSubZone.IPv4CIDR)
	ipv6Count, _ := utils.CountIPsInCIDR(targetSubZone.IPv6CIDR)

	info := map[string]interface{}{
		"sub_zone":             targetSubZone,
		"ipv4_total_count":     ipv4Count.String(),
		"ipv6_total_count":     ipv6Count.String(),
		"ipv4_allocated_count": len(targetSubZone.AllocatedIPv4),
		"ipv6_allocated_count": len(targetSubZone.AllocatedIPv6),
		"ipv4_reserved_count":  len(targetSubZone.ReservedIPv4),
		"ipv6_reserved_count":  len(targetSubZone.ReservedIPv6),
	}

	utils.WriteSuccessResponse(w, http.StatusOK, info, "Sub-zone information retrieved successfully")
}

// UpdateSubZone updates an existing sub-zone
func (h *AllocationHandler) UpdateSubZone(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	regionName := vars["region"]
	zoneName := vars["zone"]
	subZoneName := vars["subzone"]

	var req models.UpdateSubZoneRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteBadRequestError(w, "Invalid JSON payload")
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		utils.WriteValidationError(w, err.Error())
		return
	}

	response, err := h.crudService.UpdateSubZone(ctx, regionName, zoneName, subZoneName, &req)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to update sub-zone: "+err.Error())
		return
	}

	if response.Success {
		utils.WriteSuccessResponse(w, http.StatusOK, response, "")
	} else {
		utils.WriteBadRequestError(w, response.Message)
	}
}

// DeleteSubZone deletes a sub-zone
func (h *AllocationHandler) DeleteSubZone(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	regionName := vars["region"]
	zoneName := vars["zone"]
	subZoneName := vars["subzone"]

	response, err := h.crudService.DeleteSubZone(ctx, regionName, zoneName, subZoneName)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to delete sub-zone: "+err.Error())
		return
	}

	if response.Success {
		utils.WriteSuccessResponse(w, http.StatusOK, response, "")
	} else {
		utils.WriteNotFoundError(w, response.Message)
	}
}

// ===============================
// UTILITY METHODS
// ===============================

// GetAvailableIPs returns available IP addresses in a sub-zone
func (h *AllocationHandler) GetAvailableIPs(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	regionName := vars["region"]
	zoneName := vars["zone"]
	subZoneName := vars["subzone"]

	// Parse query parameters
	ipVersion := r.URL.Query().Get("ip_version")
	if ipVersion == "" {
		ipVersion = "ipv4" // default to IPv4
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10 // default limit
	if limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	response, err := h.service.GetAvailableIPs(ctx, regionName, zoneName, subZoneName, ipVersion, limit)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to get available IPs: "+err.Error())
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, response, "")
}

// GetIPStats returns comprehensive IP statistics for a sub-zone
func (h *AllocationHandler) GetIPStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	vars := mux.Vars(r)
	regionName := vars["region"]
	zoneName := vars["zone"]
	subZoneName := vars["subzone"]

	response, err := h.service.GetIPStats(ctx, regionName, zoneName, subZoneName)
	if err != nil {
		utils.WriteInternalServerError(w, "Failed to get IP statistics: "+err.Error())
		return
	}

	utils.WriteSuccessResponse(w, http.StatusOK, response, "")
}

// HealthCheck returns the health status of the API
func (h *AllocationHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"service":   "IP Allocator API",
		"version":   "2.0.0",
	}

	// Test database connectivity
	if err := h.service.TestConnection(ctx); err != nil {
		health["status"] = "unhealthy"
		health["database"] = "disconnected"
		health["error"] = err.Error()
		utils.WriteErrorResponse(w, http.StatusServiceUnavailable, "Database connection failed")
		return
	}

	health["database"] = "connected"
	utils.WriteSuccessResponse(w, http.StatusOK, health, "Service is healthy")
}
