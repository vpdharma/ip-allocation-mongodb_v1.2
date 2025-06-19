package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"ip-allocator-api/internal/models"
	"ip-allocator-api/internal/services"
	"ip-allocator-api/internal/utils"

	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/mongo"
)

type AllocationHandler struct {
	service   *services.AllocationService
	validator *validator.Validate
}

func NewAllocationHandler(db *mongo.Database) *AllocationHandler {
	return &AllocationHandler{
		service:   services.NewAllocationService(db),
		validator: validator.New(),
	}
}

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
			if _, err := utils.ParseCIDR(subZone.IPv4CIDR); err != nil {
				utils.WriteBadRequestError(w, "Invalid IPv4 CIDR in sub-zone "+subZone.Name+": "+err.Error())
				return
			}

			// Validate IPv6 CIDR
			if _, err := utils.ParseCIDR(subZone.IPv6CIDR); err != nil {
				utils.WriteBadRequestError(w, "Invalid IPv6 CIDR in sub-zone "+subZone.Name+": "+err.Error())
				return
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

// HealthCheck returns the health status of the API
func (h *AllocationHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now(),
		"service":   "IP Allocator API",
		"version":   "1.0.0",
	}

	utils.WriteSuccessResponse(w, http.StatusOK, health, "Service is healthy")
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
		"sub_zone":            targetSubZone,
		"ipv4_total_count":    ipv4Count.String(),
		"ipv6_total_count":    ipv6Count.String(),
		"ipv4_allocated_count": len(targetSubZone.AllocatedIPv4),
		"ipv6_allocated_count": len(targetSubZone.AllocatedIPv6),
		"ipv4_reserved_count":  len(targetSubZone.ReservedIPv4),
		"ipv6_reserved_count":  len(targetSubZone.ReservedIPv6),
	}

	utils.WriteSuccessResponse(w, http.StatusOK, info, "Sub-zone information retrieved successfully")
}
