package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"ip-allocator-api/internal/models"
	"ip-allocator-api/internal/services"
	"ip-allocator-api/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
)

type AllocationHandler struct {
	service     *services.AllocationService
	crudService *services.CRUDService
	validator   *validator.Validate
	logger      *zap.Logger
}

func NewAllocationHandler(db *mongo.Database, logger *zap.Logger) *AllocationHandler {
	return &AllocationHandler{
		service:     services.NewAllocationService(db, logger),
		crudService: services.NewCRUDService(db, logger),
		validator:   validator.New(),
		logger:      logger,
	}
}

// ===============================
// IP ALLOCATION METHODS
// ===============================

// AllocateIPs handles IP allocation requests using Gin framework with enhanced logging
func (h *AllocationHandler) AllocateIPs(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var req models.AllocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid JSON payload for IP allocation",
			zap.Error(err),
			zap.String("endpoint", "/allocate"),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.GetHeader("User-Agent")))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid JSON payload: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Set default count if not specified
	if req.Count == 0 {
		req.Count = 1
	}

	// Validate request structure
	if err := h.validator.Struct(&req); err != nil {
		h.logger.Warn("Validation error in IP allocation",
			zap.Error(err),
			zap.Any("request", req),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Validation error: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Additional validation for IP version
	if !utils.ValidateIPVersion(req.IPVersion) {
		h.logger.Warn("Invalid IP version requested",
			zap.String("ip_version", req.IPVersion),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid IP version. Must be 'ipv4', 'ipv6', or 'both'",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Enhanced validation for preferred IPs with CIDR checking
	for _, ip := range req.PreferredIPs {
		if utils.NormalizeIP(ip) == "" {
			h.logger.Warn("Invalid preferred IP in allocation request",
				zap.String("invalid_ip", ip),
				zap.Any("request", req),
				zap.String("client_ip", c.ClientIP()))
			c.JSON(http.StatusBadRequest, gin.H{
				"success":   false,
				"message":   "Invalid IP address in preferred IPs: " + ip,
				"timestamp": time.Now().Format(time.RFC3339),
			})
			return
		}
	}

	// Call service to allocate IPs
	response, err := h.service.AllocateIPs(ctx, &req)
	if err != nil {
		h.logger.Error("IP allocation service error",
			zap.Error(err),
			zap.Any("request", req),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to allocate IPs: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Log successful allocation
	if response.Success {
		h.logger.Info("IP allocation successful",
			zap.String("region", req.Region),
			zap.String("zone", req.Zone),
			zap.String("subzone", req.SubZone),
			zap.Int("allocated_count", len(response.AllocatedIPs)),
			zap.String("ip_version", req.IPVersion),
			zap.String("client_ip", c.ClientIP()))
	}

	// Return response
	if response.Success {
		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   response.Message,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// DeallocateIPs handles IP deallocation requests with enhanced validation
func (h *AllocationHandler) DeallocateIPs(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var req models.DeallocationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid JSON payload for IP deallocation",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid JSON payload: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		h.logger.Warn("Validation error in IP deallocation",
			zap.Error(err),
			zap.Any("request", req),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Validation error: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Enhanced IP address validation
	for _, ip := range req.IPAddresses {
		if utils.NormalizeIP(ip) == "" {
			h.logger.Warn("Invalid IP address in deallocation request",
				zap.String("invalid_ip", ip),
				zap.String("client_ip", c.ClientIP()))
			c.JSON(http.StatusBadRequest, gin.H{
				"success":   false,
				"message":   "Invalid IP address: " + ip,
				"timestamp": time.Now().Format(time.RFC3339),
			})
			return
		}
	}

	response, err := h.service.DeallocateIPs(ctx, &req)
	if err != nil {
		h.logger.Error("IP deallocation service error",
			zap.Error(err),
			zap.Any("request", req),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to deallocate IPs: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Log successful deallocation
	if response.Success {
		h.logger.Info("IP deallocation successful",
			zap.String("region", req.Region),
			zap.String("zone", req.Zone),
			zap.String("subzone", req.SubZone),
			zap.Int("deallocated_count", len(response.ProcessedIPs)),
			zap.String("client_ip", c.ClientIP()))
	}

	c.JSON(http.StatusOK, response)
}

// ReserveIPs handles IP reservation requests with enhanced CIDR validation
func (h *AllocationHandler) ReserveIPs(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var req models.ReservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid JSON payload for IP reservation",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid JSON payload: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Set reservation type to reserve
	req.ReservationType = "reserve"

	if err := h.validator.Struct(&req); err != nil {
		h.logger.Warn("Validation error in IP reservation",
			zap.Error(err),
			zap.Any("request", req),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Validation error: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Enhanced IP address validation
	for _, ip := range req.IPAddresses {
		if utils.NormalizeIP(ip) == "" {
			h.logger.Warn("Invalid IP address in reservation request",
				zap.String("invalid_ip", ip),
				zap.String("client_ip", c.ClientIP()))
			c.JSON(http.StatusBadRequest, gin.H{
				"success":   false,
				"message":   "Invalid IP address: " + ip,
				"timestamp": time.Now().Format(time.RFC3339),
			})
			return
		}
	}

	response, err := h.service.ManageReservations(ctx, &req)
	if err != nil {
		h.logger.Error("IP reservation service error",
			zap.Error(err),
			zap.Any("request", req),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to reserve IPs: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Log successful reservation
	if response.Success {
		h.logger.Info("IP reservation successful",
			zap.String("region", req.Region),
			zap.String("zone", req.Zone),
			zap.String("subzone", req.SubZone),
			zap.Int("reserved_count", len(response.ProcessedIPs)),
			zap.String("client_ip", c.ClientIP()))
	}

	c.JSON(http.StatusOK, response)
}

// UnreserveIPs handles IP unreservation requests
func (h *AllocationHandler) UnreserveIPs(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var req models.ReservationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid JSON payload for IP unreservation",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid JSON payload: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Set reservation type to unreserve
	req.ReservationType = "unreserve"

	if err := h.validator.Struct(&req); err != nil {
		h.logger.Warn("Validation error in IP unreservation",
			zap.Error(err),
			zap.Any("request", req),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Validation error: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Enhanced IP address validation
	for _, ip := range req.IPAddresses {
		if utils.NormalizeIP(ip) == "" {
			h.logger.Warn("Invalid IP address in unreservation request",
				zap.String("invalid_ip", ip),
				zap.String("client_ip", c.ClientIP()))
			c.JSON(http.StatusBadRequest, gin.H{
				"success":   false,
				"message":   "Invalid IP address: " + ip,
				"timestamp": time.Now().Format(time.RFC3339),
			})
			return
		}
	}

	response, err := h.service.ManageReservations(ctx, &req)
	if err != nil {
		h.logger.Error("IP unreservation service error",
			zap.Error(err),
			zap.Any("request", req),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to unreserve IPs: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Log successful unreservation
	if response.Success {
		h.logger.Info("IP unreservation successful",
			zap.String("region", req.Region),
			zap.String("zone", req.Zone),
			zap.String("subzone", req.SubZone),
			zap.Int("unreserved_count", len(response.ProcessedIPs)),
			zap.String("client_ip", c.ClientIP()))
	}

	c.JSON(http.StatusOK, response)
}

// ===============================
// REGION CRUD METHODS
// ===============================

// GetAllRegions returns all regions with enhanced logging
func (h *AllocationHandler) GetAllRegions(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	h.logger.Debug("Fetching all regions", zap.String("client_ip", c.ClientIP()))

	regions, err := h.service.GetAllRegions(ctx)
	if err != nil {
		h.logger.Error("Failed to get all regions",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to get regions: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	h.logger.Info("All regions retrieved successfully",
		zap.Int("count", len(regions)),
		zap.String("client_ip", c.ClientIP()))

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      regions,
		"count":     len(regions),
		"message":   "Regions retrieved successfully",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// GetRegionHierarchy returns the complete hierarchy for a region
func (h *AllocationHandler) GetRegionHierarchy(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	regionName := c.Param("region")
	if regionName == "" {
		h.logger.Warn("Region name missing in request", zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Region name is required",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	h.logger.Debug("Fetching region hierarchy",
		zap.String("region", regionName),
		zap.String("client_ip", c.ClientIP()))

	region, err := h.service.GetRegionHierarchy(ctx, regionName)
	if err != nil {
		if err.Error() == "region '"+regionName+"' not found" {
			h.logger.Warn("Region not found",
				zap.String("region", regionName),
				zap.String("client_ip", c.ClientIP()))
			c.JSON(http.StatusNotFound, gin.H{
				"success":   false,
				"message":   err.Error(),
				"timestamp": time.Now().Format(time.RFC3339),
			})
		} else {
			h.logger.Error("Failed to get region hierarchy",
				zap.Error(err),
				zap.String("region", regionName),
				zap.String("client_ip", c.ClientIP()))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":   false,
				"message":   "Failed to get region hierarchy: " + err.Error(),
				"timestamp": time.Now().Format(time.RFC3339),
			})
		}
		return
	}

	h.logger.Info("Region hierarchy retrieved successfully",
		zap.String("region", regionName),
		zap.Int("zones_count", len(region.Zones)),
		zap.String("client_ip", c.ClientIP()))

	c.JSON(http.StatusOK, gin.H{
		"success":   true,
		"data":      region,
		"message":   "Region hierarchy retrieved successfully",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// CreateRegion creates a new region with enhanced Zone CIDR validation
func (h *AllocationHandler) CreateRegion(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var region models.Region
	if err := c.ShouldBindJSON(&region); err != nil {
		h.logger.Warn("Invalid JSON payload for region creation",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid JSON payload: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Validate request structure
	if err := h.validator.Struct(&region); err != nil {
		h.logger.Warn("Validation error in region creation",
			zap.Error(err),
			zap.Any("region", region),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Validation error: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Enhanced CIDR validation with Zone CIDR support
	for _, zone := range region.Zones {
		// Validate Zone CIDR against Region CIDR
		if err := utils.ValidateZoneCIDRHierarchy(region.IPv4CIDR, region.IPv6CIDR, zone.IPv4CIDR, zone.IPv6CIDR); err != nil {
			h.logger.Error("Zone CIDR validation failed",
				zap.Error(err),
				zap.String("region", region.Name),
				zap.String("zone", zone.Name),
				zap.String("client_ip", c.ClientIP()))
			c.JSON(http.StatusBadRequest, gin.H{
				"success":   false,
				"message":   "Zone CIDR validation failed for zone " + zone.Name + ": " + err.Error(),
				"timestamp": time.Now().Format(time.RFC3339),
			})
			return
		}

		// Validate Sub-zone CIDRs
		for _, subZone := range zone.SubZones {
			// Validate Sub-zone CIDR against Zone CIDR
			if err := utils.ValidateSubZoneCIDRHierarchy(zone.IPv4CIDR, zone.IPv6CIDR, subZone.IPv4CIDR, subZone.IPv6CIDR); err != nil {
				h.logger.Error("Sub-zone CIDR validation failed",
					zap.Error(err),
					zap.String("region", region.Name),
					zap.String("zone", zone.Name),
					zap.String("subzone", subZone.Name),
					zap.String("client_ip", c.ClientIP()))
				c.JSON(http.StatusBadRequest, gin.H{
					"success":   false,
					"message":   "Sub-zone CIDR validation failed for sub-zone " + subZone.Name + ": " + err.Error(),
					"timestamp": time.Now().Format(time.RFC3339),
				})
				return
			}

			// Validate individual IPv4 and IPv6 CIDRs
			if subZone.IPv4CIDR != "" {
				if _, err := utils.ParseCIDR(subZone.IPv4CIDR); err != nil {
					h.logger.Error("Invalid IPv4 CIDR in sub-zone",
						zap.Error(err),
						zap.String("subzone", subZone.Name),
						zap.String("cidr", subZone.IPv4CIDR),
						zap.String("client_ip", c.ClientIP()))
					c.JSON(http.StatusBadRequest, gin.H{
						"success":   false,
						"message":   "Invalid IPv4 CIDR in sub-zone " + subZone.Name + ": " + err.Error(),
						"timestamp": time.Now().Format(time.RFC3339),
					})
					return
				}
			}

			if subZone.IPv6CIDR != "" {
				if _, err := utils.ParseCIDR(subZone.IPv6CIDR); err != nil {
					h.logger.Error("Invalid IPv6 CIDR in sub-zone",
						zap.Error(err),
						zap.String("subzone", subZone.Name),
						zap.String("cidr", subZone.IPv6CIDR),
						zap.String("client_ip", c.ClientIP()))
					c.JSON(http.StatusBadRequest, gin.H{
						"success":   false,
						"message":   "Invalid IPv6 CIDR in sub-zone " + subZone.Name + ": " + err.Error(),
						"timestamp": time.Now().Format(time.RFC3339),
					})
					return
				}
			}
		}
	}

	// Create region
	if err := h.service.CreateRegion(ctx, &region); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			h.logger.Warn("Duplicate region creation attempted",
				zap.String("region", region.Name),
				zap.String("client_ip", c.ClientIP()))
			c.JSON(http.StatusConflict, gin.H{
				"success":   false,
				"message":   "Region with this name already exists",
				"timestamp": time.Now().Format(time.RFC3339),
			})
		} else {
			h.logger.Error("Failed to create region",
				zap.Error(err),
				zap.String("region", region.Name),
				zap.String("client_ip", c.ClientIP()))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":   false,
				"message":   "Failed to create region: " + err.Error(),
				"timestamp": time.Now().Format(time.RFC3339),
			})
		}
		return
	}

	h.logger.Info("Region created successfully",
		zap.String("region", region.Name),
		zap.Int("zones_count", len(region.Zones)),
		zap.String("client_ip", c.ClientIP()))

	c.JSON(http.StatusCreated, gin.H{
		"success":   true,
		"data":      region,
		"message":   "Region created successfully",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// UpdateRegion updates an existing region with enhanced validation
func (h *AllocationHandler) UpdateRegion(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	regionName := c.Param("region")
	if regionName == "" {
		h.logger.Warn("Region name missing in update request", zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Region name is required",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	var req models.UpdateRegionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid JSON payload for region update",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid JSON payload: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		h.logger.Warn("Validation error in region update",
			zap.Error(err),
			zap.String("region", regionName),
			zap.Any("request", req),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Validation error: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	response, err := h.crudService.UpdateRegion(ctx, regionName, &req)
	if err != nil {
		h.logger.Error("Failed to update region",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to update region: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if response.Success {
		h.logger.Info("Region updated successfully",
			zap.String("region", regionName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusOK, response)
	} else {
		h.logger.Warn("Region update failed",
			zap.String("region", regionName),
			zap.String("message", response.Message),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   response.Message,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// DeleteRegion deletes a region with enhanced logging
func (h *AllocationHandler) DeleteRegion(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	regionName := c.Param("region")
	if regionName == "" {
		h.logger.Warn("Region name missing in delete request", zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Region name is required",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	h.logger.Info("Attempting to delete region",
		zap.String("region", regionName),
		zap.String("client_ip", c.ClientIP()))

	response, err := h.crudService.DeleteRegion(ctx, regionName)
	if err != nil {
		h.logger.Error("Failed to delete region",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to delete region: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if response.Success {
		h.logger.Info("Region deleted successfully",
			zap.String("region", regionName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusOK, response)
	} else {
		h.logger.Warn("Region deletion failed - not found",
			zap.String("region", regionName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   response.Message,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// ===============================
// ZONE CRUD METHODS (Enhanced with Zone CIDR Support)
// ===============================

// CreateZone creates a new zone within a region with enhanced CIDR validation
func (h *AllocationHandler) CreateZone(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	regionName := c.Param("region")
	if regionName == "" {
		h.logger.Warn("Region name missing in zone creation", zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Region name is required",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	var req models.CreateZoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid JSON payload for zone creation",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid JSON payload: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		h.logger.Warn("Validation error in zone creation",
			zap.Error(err),
			zap.String("region", regionName),
			zap.Any("request", req),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Validation error: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	h.logger.Info("Creating zone with CIDR validation",
		zap.String("region", regionName),
		zap.String("zone", req.Name),
		zap.String("ipv4_cidr", req.IPv4CIDR),
		zap.String("ipv6_cidr", req.IPv6CIDR),
		zap.String("client_ip", c.ClientIP()))

	response, err := h.crudService.CreateZone(ctx, regionName, &req)
	if err != nil {
		h.logger.Error("Failed to create zone",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", req.Name),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to create zone: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if response.Success {
		h.logger.Info("Zone created successfully",
			zap.String("region", regionName),
			zap.String("zone", req.Name),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusCreated, response)
	} else {
		h.logger.Warn("Zone creation failed",
			zap.String("region", regionName),
			zap.String("zone", req.Name),
			zap.String("message", response.Message),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   response.Message,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// GetZone returns information about a specific zone
func (h *AllocationHandler) GetZone(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	regionName := c.Param("region")
	zoneName := c.Param("zone")

	if regionName == "" || zoneName == "" {
		h.logger.Warn("Missing parameters in zone retrieval",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Region and zone names are required",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	h.logger.Debug("Fetching zone information",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("client_ip", c.ClientIP()))

	response, err := h.crudService.GetZone(ctx, regionName, zoneName)
	if err != nil {
		h.logger.Error("Failed to get zone",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to get zone: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if response.Success {
		h.logger.Info("Zone retrieved successfully",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusOK, response)
	} else {
		h.logger.Warn("Zone not found",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   response.Message,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// UpdateZone updates an existing zone with enhanced CIDR validation
func (h *AllocationHandler) UpdateZone(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	regionName := c.Param("region")
	zoneName := c.Param("zone")

	if regionName == "" || zoneName == "" {
		h.logger.Warn("Missing parameters in zone update",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Region and zone names are required",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	var req models.UpdateZoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid JSON payload for zone update",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid JSON payload: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		h.logger.Warn("Validation error in zone update",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.Any("request", req),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Validation error: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	response, err := h.crudService.UpdateZone(ctx, regionName, zoneName, &req)
	if err != nil {
		h.logger.Error("Failed to update zone",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to update zone: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if response.Success {
		h.logger.Info("Zone updated successfully",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusOK, response)
	} else {
		h.logger.Warn("Zone update failed",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("message", response.Message),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   response.Message,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// DeleteZone deletes a zone with enhanced logging
func (h *AllocationHandler) DeleteZone(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	regionName := c.Param("region")
	zoneName := c.Param("zone")

	if regionName == "" || zoneName == "" {
		h.logger.Warn("Missing parameters in zone deletion",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Region and zone names are required",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	h.logger.Info("Attempting to delete zone",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("client_ip", c.ClientIP()))

	response, err := h.crudService.DeleteZone(ctx, regionName, zoneName)
	if err != nil {
		h.logger.Error("Failed to delete zone",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to delete zone: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if response.Success {
		h.logger.Info("Zone deleted successfully",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusOK, response)
	} else {
		h.logger.Warn("Zone deletion failed - not found",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   response.Message,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// ===============================
// SUBZONE CRUD METHODS
// ===============================

// CreateSubZone creates a new sub-zone within a zone with enhanced validation
func (h *AllocationHandler) CreateSubZone(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	regionName := c.Param("region")
	zoneName := c.Param("zone")

	if regionName == "" || zoneName == "" {
		h.logger.Warn("Missing parameters in sub-zone creation",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Region and zone names are required",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	var req models.CreateSubZoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid JSON payload for sub-zone creation",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid JSON payload: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		h.logger.Warn("Validation error in sub-zone creation",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.Any("request", req),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Validation error: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	h.logger.Info("Creating sub-zone with enhanced validation",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", req.Name),
		zap.String("ipv4_cidr", req.IPv4CIDR),
		zap.String("ipv6_cidr", req.IPv6CIDR),
		zap.String("client_ip", c.ClientIP()))

	response, err := h.crudService.CreateSubZone(ctx, regionName, zoneName, &req)
	if err != nil {
		h.logger.Error("Failed to create sub-zone",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", req.Name),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to create sub-zone: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if response.Success {
		h.logger.Info("Sub-zone created successfully",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", req.Name),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusCreated, response)
	} else {
		h.logger.Warn("Sub-zone creation failed",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", req.Name),
			zap.String("message", response.Message),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   response.Message,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// GetSubZoneInfo returns detailed information about a specific sub-zone with enhanced statistics
func (h *AllocationHandler) GetSubZoneInfo(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	regionName := c.Param("region")
	zoneName := c.Param("zone")
	subZoneName := c.Param("subzone")

	if regionName == "" || zoneName == "" || subZoneName == "" {
		h.logger.Warn("Missing parameters in sub-zone info request",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Region, zone, and sub-zone names are required",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	h.logger.Debug("Fetching sub-zone information",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName),
		zap.String("client_ip", c.ClientIP()))

	region, err := h.service.GetRegionHierarchy(ctx, regionName)
	if err != nil {
		if err.Error() == "region '"+regionName+"' not found" {
			h.logger.Warn("Region not found for sub-zone info",
				zap.String("region", regionName),
				zap.String("client_ip", c.ClientIP()))
			c.JSON(http.StatusNotFound, gin.H{
				"success":   false,
				"message":   err.Error(),
				"timestamp": time.Now().Format(time.RFC3339),
			})
		} else {
			h.logger.Error("Failed to get region hierarchy for sub-zone info",
				zap.Error(err),
				zap.String("region", regionName),
				zap.String("client_ip", c.ClientIP()))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success":   false,
				"message":   "Failed to get region hierarchy: " + err.Error(),
				"timestamp": time.Now().Format(time.RFC3339),
			})
		}
		return
	}

	// Find the specific sub-zone
	var targetSubZone *models.SubZone
	var parentZone *models.Zone
	for _, zone := range region.Zones {
		if zone.Name == zoneName {
			parentZone = &zone
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
		h.logger.Warn("Sub-zone not found",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   "Sub-zone not found",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Enhanced statistics calculation
	ipv4Count, _ := utils.CountIPsInCIDR(targetSubZone.IPv4CIDR)
	ipv6Count, _ := utils.CountIPsInCIDR(targetSubZone.IPv6CIDR)

	// Calculate available counts
	ipv4Available := int64(0)
	ipv6Available := int64(0)
	if ipv4Count.Int64() > 0 {
		ipv4Available = ipv4Count.Int64() - int64(len(targetSubZone.AllocatedIPv4)) - int64(len(targetSubZone.ReservedIPv4))
	}
	if ipv6Count.Int64() > 0 {
		ipv6Available = ipv6Count.Int64() - int64(len(targetSubZone.AllocatedIPv6)) - int64(len(targetSubZone.ReservedIPv6))
	}

	info := gin.H{
		"success": true,
		"data": gin.H{
			"sub_zone":             targetSubZone,
			"parent_zone":          parentZone,
			"parent_region":        region,
			"ipv4_total_count":     ipv4Count.String(),
			"ipv6_total_count":     ipv6Count.String(),
			"ipv4_allocated_count": len(targetSubZone.AllocatedIPv4),
			"ipv6_allocated_count": len(targetSubZone.AllocatedIPv6),
			"ipv4_reserved_count":  len(targetSubZone.ReservedIPv4),
			"ipv6_reserved_count":  len(targetSubZone.ReservedIPv6),
			"ipv4_available_count": ipv4Available,
			"ipv6_available_count": ipv6Available,
		},
		"message":   "Sub-zone information retrieved successfully",
		"timestamp": time.Now().Format(time.RFC3339),
	}

	h.logger.Info("Sub-zone information retrieved successfully",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName),
		zap.Int("ipv4_allocated", len(targetSubZone.AllocatedIPv4)),
		zap.Int("ipv6_allocated", len(targetSubZone.AllocatedIPv6)),
		zap.String("client_ip", c.ClientIP()))

	c.JSON(http.StatusOK, info)
}

// UpdateSubZone updates an existing sub-zone with enhanced validation
func (h *AllocationHandler) UpdateSubZone(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	regionName := c.Param("region")
	zoneName := c.Param("zone")
	subZoneName := c.Param("subzone")

	if regionName == "" || zoneName == "" || subZoneName == "" {
		h.logger.Warn("Missing parameters in sub-zone update",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Region, zone, and sub-zone names are required",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	var req models.UpdateSubZoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warn("Invalid JSON payload for sub-zone update",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid JSON payload: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if err := h.validator.Struct(&req); err != nil {
		h.logger.Warn("Validation error in sub-zone update",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.Any("request", req),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Validation error: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	response, err := h.crudService.UpdateSubZone(ctx, regionName, zoneName, subZoneName, &req)
	if err != nil {
		h.logger.Error("Failed to update sub-zone",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to update sub-zone: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if response.Success {
		h.logger.Info("Sub-zone updated successfully",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusOK, response)
	} else {
		h.logger.Warn("Sub-zone update failed",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("message", response.Message),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   response.Message,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// DeleteSubZone deletes a sub-zone with enhanced logging
func (h *AllocationHandler) DeleteSubZone(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	regionName := c.Param("region")
	zoneName := c.Param("zone")
	subZoneName := c.Param("subzone")

	if regionName == "" || zoneName == "" || subZoneName == "" {
		h.logger.Warn("Missing parameters in sub-zone deletion",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Region, zone, and sub-zone names are required",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	h.logger.Info("Attempting to delete sub-zone",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName),
		zap.String("client_ip", c.ClientIP()))

	response, err := h.crudService.DeleteSubZone(ctx, regionName, zoneName, subZoneName)
	if err != nil {
		h.logger.Error("Failed to delete sub-zone",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to delete sub-zone: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	if response.Success {
		h.logger.Info("Sub-zone deleted successfully",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusOK, response)
	} else {
		h.logger.Warn("Sub-zone deletion failed - not found",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusNotFound, gin.H{
			"success":   false,
			"message":   response.Message,
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// ===============================
// UTILITY METHODS
// ===============================

// GetAvailableIPs returns available IP addresses with enhanced query parameter handling
func (h *AllocationHandler) GetAvailableIPs(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	regionName := c.Param("region")
	zoneName := c.Param("zone")
	subZoneName := c.Param("subzone")

	if regionName == "" || zoneName == "" || subZoneName == "" {
		h.logger.Warn("Missing parameters in available IPs request",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Region, zone, and sub-zone names are required",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	// Parse query parameters with enhanced defaults
	ipVersion := c.DefaultQuery("ip_version", "ipv4")
	limitStr := c.DefaultQuery("limit", "10")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100 // Cap at 100 for performance
	}

	// Validate IP version
	if !utils.ValidateIPVersion(ipVersion) || ipVersion == "both" {
		h.logger.Warn("Invalid IP version for available IPs",
			zap.String("ip_version", ipVersion),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Invalid IP version. Must be 'ipv4' or 'ipv6'",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	h.logger.Debug("Fetching available IPs",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName),
		zap.String("ip_version", ipVersion),
		zap.Int("limit", limit),
		zap.String("client_ip", c.ClientIP()))

	response, err := h.service.GetAvailableIPs(ctx, regionName, zoneName, subZoneName, ipVersion, limit)
	if err != nil {
		h.logger.Error("Failed to get available IPs",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("ip_version", ipVersion),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to get available IPs: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	h.logger.Info("Available IPs retrieved successfully",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName),
		zap.String("ip_version", ipVersion),
		zap.Int("available_count", len(response["available_ips"].([]string))),
		zap.String("client_ip", c.ClientIP()))

	c.JSON(http.StatusOK, response)
}

// GetIPStats returns comprehensive IP statistics with enhanced metrics
func (h *AllocationHandler) GetIPStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	regionName := c.Param("region")
	zoneName := c.Param("zone")
	subZoneName := c.Param("subzone")

	if regionName == "" || zoneName == "" || subZoneName == "" {
		h.logger.Warn("Missing parameters in IP stats request",
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusBadRequest, gin.H{
			"success":   false,
			"message":   "Region, zone, and sub-zone names are required",
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	h.logger.Debug("Fetching IP statistics",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName),
		zap.String("client_ip", c.ClientIP()))

	response, err := h.service.GetIPStats(ctx, regionName, zoneName, subZoneName)
	if err != nil {
		h.logger.Error("Failed to get IP statistics",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", zoneName),
			zap.String("subzone", subZoneName),
			zap.String("client_ip", c.ClientIP()))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success":   false,
			"message":   "Failed to get IP statistics: " + err.Error(),
			"timestamp": time.Now().Format(time.RFC3339),
		})
		return
	}

	h.logger.Info("IP statistics retrieved successfully",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName),
		zap.String("client_ip", c.ClientIP()))

	c.JSON(http.StatusOK, response)
}

// HealthCheck with enhanced Gin support and comprehensive Zap logging
func (h *AllocationHandler) HealthCheck(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h.logger.Debug("Health check requested", zap.String("client_ip", c.ClientIP()))

	health := gin.H{
		"status":     "healthy",
		"timestamp":  time.Now().Format(time.RFC3339),
		"service":    "IP Allocator API",
		"version":    "2.0.0",
		"framework":  "Gin",
		"go_version": "1.24",
		"features": gin.H{
			"zone_cidrs":          true,
			"enhanced_validation": true,
			"zap_logging":         true,
			"gin_framework":       true,
			"first_last_ip_check": true,
		},
	}

	// Test database connectivity
	if err := h.service.TestConnection(ctx); err != nil {
		h.logger.Error("Database health check failed",
			zap.Error(err),
			zap.String("client_ip", c.ClientIP()))
		health["status"] = "unhealthy"
		health["database"] = "disconnected"
		health["error"] = err.Error()
		c.JSON(http.StatusServiceUnavailable, health)
		return
	}

	health["database"] = "connected"
	h.logger.Info("Health check passed", zap.String("client_ip", c.ClientIP()))
	c.JSON(http.StatusOK, health)
}
