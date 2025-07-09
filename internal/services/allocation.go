package services

import (
	"context"
	"fmt"
	"net"
	"time"

	"ip-allocator-api/internal/models"
	"ip-allocator-api/internal/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type AllocationService struct {
	collection *mongo.Collection
	logger     *zap.Logger
}

func NewAllocationService(db *mongo.Database, logger *zap.Logger) *AllocationService {
	return &AllocationService{
		collection: db.Collection(models.RegionCollection),
		logger:     logger,
	}
}

// TestConnection tests the database connection with enhanced logging
func (s *AllocationService) TestConnection(ctx context.Context) error {
	s.logger.Debug("Testing database connection")
	err := s.collection.Database().Client().Ping(ctx, nil)
	if err != nil {
		s.logger.Error("Database connection test failed", zap.Error(err))
		return err
	}
	s.logger.Debug("Database connection test successful")
	return nil
}

// AllocateIPs allocates IP addresses with enhanced CIDR validation and logging
func (s *AllocationService) AllocateIPs(ctx context.Context, req *models.AllocationRequest) (*models.AllocationResponse, error) {
	s.logger.Info("Starting IP allocation process",
		zap.String("region", req.Region),
		zap.String("zone", req.Zone),
		zap.String("subzone", req.SubZone),
		zap.String("ip_version", req.IPVersion),
		zap.Int("count", req.Count),
		zap.Int("preferred_ips_count", len(req.PreferredIPs)))

	// Find the target sub-zone with enhanced validation
	subZone, regionData, zoneData, err := s.findSubZoneWithHierarchy(ctx, req.Region, req.Zone, req.SubZone)
	if err != nil {
		s.logger.Error("Failed to find sub-zone in hierarchy",
			zap.Error(err),
			zap.String("region", req.Region),
			zap.String("zone", req.Zone),
			zap.String("subzone", req.SubZone))
		return &models.AllocationResponse{
			Success:   false,
			Message:   fmt.Sprintf("Failed to find sub-zone: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	// Enhanced CIDR hierarchy validation
	if err := s.validateCIDRHierarchy(regionData, zoneData, subZone); err != nil {
		s.logger.Warn("CIDR hierarchy validation warning",
			zap.Error(err),
			zap.String("region", req.Region),
			zap.String("zone", req.Zone),
			zap.String("subzone", req.SubZone))
		// Continue with warning logged
	}

	var allocatedIPs []string
	var errors []string

	// Handle different IP version requirements with enhanced validation
	switch req.IPVersion {
	case "ipv4":
		ips, err := s.allocateIPsForVersionEnhanced(ctx, subZone, req.PreferredIPs, req.Count, "ipv4")
		if err != nil {
			s.logger.Error("IPv4 allocation failed", zap.Error(err))
			errors = append(errors, fmt.Sprintf("IPv4 allocation failed: %v", err))
		} else {
			allocatedIPs = append(allocatedIPs, ips...)
			s.logger.Info("IPv4 allocation successful",
				zap.Int("allocated_count", len(ips)),
				zap.Strings("allocated_ips", ips))
		}
	case "ipv6":
		ips, err := s.allocateIPsForVersionEnhanced(ctx, subZone, req.PreferredIPs, req.Count, "ipv6")
		if err != nil {
			s.logger.Error("IPv6 allocation failed", zap.Error(err))
			errors = append(errors, fmt.Sprintf("IPv6 allocation failed: %v", err))
		} else {
			allocatedIPs = append(allocatedIPs, ips...)
			s.logger.Info("IPv6 allocation successful",
				zap.Int("allocated_count", len(ips)),
				zap.Strings("allocated_ips", ips))
		}
	case "both":
		// Enhanced dual-stack allocation
		ipv4Count := req.Count / 2
		ipv6Count := req.Count - ipv4Count

		s.logger.Debug("Dual-stack allocation requested",
			zap.Int("ipv4_count", ipv4Count),
			zap.Int("ipv6_count", ipv6Count))

		if ipv4Count > 0 {
			ipv4Preferred, _, err := utils.SplitIPsByVersion(req.PreferredIPs)
			if err != nil {
				s.logger.Error("Failed to split preferred IPs by version", zap.Error(err))
				errors = append(errors, fmt.Sprintf("Failed to split preferred IPs: %v", err))
			} else {
				ips, err := s.allocateIPsForVersionEnhanced(ctx, subZone, ipv4Preferred, ipv4Count, "ipv4")
				if err != nil {
					s.logger.Error("IPv4 allocation in dual-stack failed", zap.Error(err))
					errors = append(errors, fmt.Sprintf("IPv4 allocation failed: %v", err))
				} else {
					allocatedIPs = append(allocatedIPs, ips...)
					s.logger.Info("IPv4 allocation in dual-stack successful",
						zap.Int("allocated_count", len(ips)))
				}
			}
		}

		if ipv6Count > 0 {
			_, ipv6Preferred, err := utils.SplitIPsByVersion(req.PreferredIPs)
			if err != nil {
				s.logger.Error("Failed to split preferred IPs by version for IPv6", zap.Error(err))
				errors = append(errors, fmt.Sprintf("Failed to split preferred IPs: %v", err))
			} else {
				ips, err := s.allocateIPsForVersionEnhanced(ctx, subZone, ipv6Preferred, ipv6Count, "ipv6")
				if err != nil {
					s.logger.Error("IPv6 allocation in dual-stack failed", zap.Error(err))
					errors = append(errors, fmt.Sprintf("IPv6 allocation failed: %v", err))
				} else {
					allocatedIPs = append(allocatedIPs, ips...)
					s.logger.Info("IPv6 allocation in dual-stack successful",
						zap.Int("allocated_count", len(ips)))
				}
			}
		}
	}

	// Update the database with allocated IPs
	if len(allocatedIPs) > 0 {
		s.logger.Debug("Updating database with allocated IPs",
			zap.Int("total_allocated", len(allocatedIPs)))
		err = s.updateAllocatedIPs(ctx, req.Region, req.Zone, req.SubZone, allocatedIPs)
		if err != nil {
			s.logger.Error("Failed to update allocated IPs in database",
				zap.Error(err),
				zap.Strings("allocated_ips", allocatedIPs))
			return &models.AllocationResponse{
				Success:   false,
				Message:   fmt.Sprintf("Failed to update database: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
		s.logger.Info("Database updated successfully with allocated IPs")
	}

	// Prepare response
	success := len(allocatedIPs) > 0
	message := "IPs allocated successfully"
	if len(errors) > 0 {
		if !success {
			message = fmt.Sprintf("Allocation failed: %v", errors)
		} else {
			message = fmt.Sprintf("Partial allocation completed with warnings: %v", errors)
		}
	}

	s.logger.Info("IP allocation process completed",
		zap.Bool("success", success),
		zap.Int("total_allocated", len(allocatedIPs)),
		zap.Int("error_count", len(errors)))

	return &models.AllocationResponse{
		Success:      success,
		AllocatedIPs: allocatedIPs,
		Message:      message,
		Timestamp:    time.Now(),
	}, nil
}

// DeallocateIPs removes IPs from allocated lists with enhanced validation and logging
func (s *AllocationService) DeallocateIPs(ctx context.Context, req *models.DeallocationRequest) (*models.IPOperationResponse, error) {
	s.logger.Info("Starting IP deallocation process",
		zap.String("region", req.Region),
		zap.String("zone", req.Zone),
		zap.String("subzone", req.SubZone),
		zap.Int("ip_count", len(req.IPAddresses)))

	// Find the target sub-zone with enhanced validation
	subZone, _, _, err := s.findSubZoneWithHierarchy(ctx, req.Region, req.Zone, req.SubZone)
	if err != nil {
		s.logger.Error("Failed to find sub-zone for deallocation",
			zap.Error(err),
			zap.String("region", req.Region),
			zap.String("zone", req.Zone),
			zap.String("subzone", req.SubZone))
		return &models.IPOperationResponse{
			Success:   false,
			Message:   fmt.Sprintf("Failed to find sub-zone: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	var processedIPs, failedIPs []string
	ipv4sToRemove := []string{}
	ipv6sToRemove := []string{}

	// Process each IP address with enhanced validation
	for _, ip := range req.IPAddresses {
		s.logger.Debug("Processing IP for deallocation", zap.String("ip", ip))

		normalizedIP := utils.NormalizeIP(ip)
		if normalizedIP == "" {
			s.logger.Warn("Invalid IP address format", zap.String("ip", ip))
			failedIPs = append(failedIPs, ip)
			continue
		}

		// Enhanced CIDR validation - check if IP is in valid range
		if err := s.validateIPInSubZoneCIDR(normalizedIP, subZone); err != nil {
			s.logger.Warn("IP not in valid CIDR range for deallocation",
				zap.String("ip", normalizedIP),
				zap.Error(err))
			failedIPs = append(failedIPs, normalizedIP)
			continue
		}

		// Check if IP is actually allocated
		var found bool
		if utils.IsIPv4(net.ParseIP(normalizedIP)) {
			for _, allocatedIP := range subZone.AllocatedIPv4 {
				if allocatedIP == normalizedIP {
					ipv4sToRemove = append(ipv4sToRemove, normalizedIP)
					processedIPs = append(processedIPs, normalizedIP)
					found = true
					s.logger.Debug("IPv4 found in allocated list", zap.String("ip", normalizedIP))
					break
				}
			}
		} else if utils.IsIPv6(net.ParseIP(normalizedIP)) {
			for _, allocatedIP := range subZone.AllocatedIPv6 {
				if allocatedIP == normalizedIP {
					ipv6sToRemove = append(ipv6sToRemove, normalizedIP)
					processedIPs = append(processedIPs, normalizedIP)
					found = true
					s.logger.Debug("IPv6 found in allocated list", zap.String("ip", normalizedIP))
					break
				}
			}
		}

		if !found {
			s.logger.Warn("IP not found in allocated list", zap.String("ip", normalizedIP))
			failedIPs = append(failedIPs, normalizedIP)
		}
	}

	// Update database to remove IPs
	if len(processedIPs) > 0 {
		s.logger.Debug("Updating database to remove allocated IPs",
			zap.Int("ipv4_count", len(ipv4sToRemove)),
			zap.Int("ipv6_count", len(ipv6sToRemove)))
		err = s.removeAllocatedIPs(ctx, req.Region, req.Zone, req.SubZone, ipv4sToRemove, ipv6sToRemove)
		if err != nil {
			s.logger.Error("Failed to update database for deallocation",
				zap.Error(err),
				zap.Strings("processed_ips", processedIPs))
			return &models.IPOperationResponse{
				Success:   false,
				Message:   fmt.Sprintf("Failed to update database: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
		s.logger.Info("Database updated successfully for deallocation")
	}

	success := len(processedIPs) > 0
	message := "IPs deallocated successfully"
	if len(failedIPs) > 0 {
		if !success {
			message = "No IPs were deallocated (not found in allocated list)"
		} else {
			message = fmt.Sprintf("Partial deallocation: %d successful, %d failed", len(processedIPs), len(failedIPs))
		}
	}

	s.logger.Info("IP deallocation process completed",
		zap.Bool("success", success),
		zap.Int("processed_count", len(processedIPs)),
		zap.Int("failed_count", len(failedIPs)))

	return &models.IPOperationResponse{
		Success:      success,
		ProcessedIPs: processedIPs,
		FailedIPs:    failedIPs,
		Message:      message,
		Timestamp:    time.Now(),
	}, nil
}

// ManageReservations handles IP reservation and unreservation with enhanced validation
func (s *AllocationService) ManageReservations(ctx context.Context, req *models.ReservationRequest) (*models.IPOperationResponse, error) {
	s.logger.Info("Starting IP reservation management",
		zap.String("region", req.Region),
		zap.String("zone", req.Zone),
		zap.String("subzone", req.SubZone),
		zap.String("operation", req.ReservationType),
		zap.Int("ip_count", len(req.IPAddresses)))

	// Find the target sub-zone with enhanced validation
	subZone, _, _, err := s.findSubZoneWithHierarchy(ctx, req.Region, req.Zone, req.SubZone)
	if err != nil {
		s.logger.Error("Failed to find sub-zone for reservation management",
			zap.Error(err),
			zap.String("region", req.Region),
			zap.String("zone", req.Zone),
			zap.String("subzone", req.SubZone))
		return &models.IPOperationResponse{
			Success:   false,
			Message:   fmt.Sprintf("Failed to find sub-zone: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	var processedIPs, failedIPs []string

	for _, ip := range req.IPAddresses {
		s.logger.Debug("Processing IP for reservation management",
			zap.String("ip", ip),
			zap.String("operation", req.ReservationType))

		normalizedIP := utils.NormalizeIP(ip)
		if normalizedIP == "" {
			s.logger.Warn("Invalid IP address format", zap.String("ip", ip))
			failedIPs = append(failedIPs, ip)
			continue
		}

		// Enhanced CIDR validation with both first and last IP checking
		if err := s.validateIPInSubZoneCIDR(normalizedIP, subZone); err != nil {
			s.logger.Warn("IP not in valid CIDR range",
				zap.String("ip", normalizedIP),
				zap.Error(err))
			failedIPs = append(failedIPs, normalizedIP)
			continue
		}

		if req.ReservationType == "reserve" {
			// Check if IP is not already allocated or reserved
			if !s.isIPUsed(normalizedIP, subZone.AllocatedIPv4, subZone.ReservedIPv4) &&
				!s.isIPUsed(normalizedIP, subZone.AllocatedIPv6, subZone.ReservedIPv6) {
				processedIPs = append(processedIPs, normalizedIP)
				s.logger.Debug("IP available for reservation", zap.String("ip", normalizedIP))
			} else {
				s.logger.Warn("IP already in use, cannot reserve", zap.String("ip", normalizedIP))
				failedIPs = append(failedIPs, normalizedIP)
			}
		} else { // unreserve
			// Check if IP is actually reserved
			var isReserved bool
			if utils.IsIPv4(net.ParseIP(normalizedIP)) {
				for _, reservedIP := range subZone.ReservedIPv4 {
					if reservedIP == normalizedIP {
						isReserved = true
						break
					}
				}
			} else if utils.IsIPv6(net.ParseIP(normalizedIP)) {
				for _, reservedIP := range subZone.ReservedIPv6 {
					if reservedIP == normalizedIP {
						isReserved = true
						break
					}
				}
			}

			if isReserved {
				processedIPs = append(processedIPs, normalizedIP)
				s.logger.Debug("IP found in reserved list for unreservation", zap.String("ip", normalizedIP))
			} else {
				s.logger.Warn("IP not found in reserved list", zap.String("ip", normalizedIP))
				failedIPs = append(failedIPs, normalizedIP)
			}
		}
	}

	// Update database
	if len(processedIPs) > 0 {
		s.logger.Debug("Updating database for reservation management",
			zap.String("operation", req.ReservationType),
			zap.Int("processed_count", len(processedIPs)))
		if req.ReservationType == "reserve" {
			err = s.addReservedIPs(ctx, req.Region, req.Zone, req.SubZone, processedIPs)
		} else {
			err = s.removeReservedIPs(ctx, req.Region, req.Zone, req.SubZone, processedIPs)
		}

		if err != nil {
			s.logger.Error("Failed to update database for reservation management",
				zap.Error(err),
				zap.String("operation", req.ReservationType),
				zap.Strings("processed_ips", processedIPs))
			return &models.IPOperationResponse{
				Success:   false,
				Message:   fmt.Sprintf("Failed to update database: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
		s.logger.Info("Database updated successfully for reservation management")
	}

	success := len(processedIPs) > 0
	operation := "reserved"
	if req.ReservationType == "unreserve" {
		operation = "unreserved"
	}

	message := fmt.Sprintf("IPs %s successfully", operation)
	if len(failedIPs) > 0 {
		if !success {
			message = fmt.Sprintf("No IPs were %s", operation)
		} else {
			message = fmt.Sprintf("Partial operation: %d %s, %d failed", len(processedIPs), operation, len(failedIPs))
		}
	}

	s.logger.Info("IP reservation management completed",
		zap.Bool("success", success),
		zap.String("operation", operation),
		zap.Int("processed_count", len(processedIPs)),
		zap.Int("failed_count", len(failedIPs)))

	return &models.IPOperationResponse{
		Success:      success,
		ProcessedIPs: processedIPs,
		FailedIPs:    failedIPs,
		Message:      message,
		Timestamp:    time.Now(),
	}, nil
}

// GetAvailableIPs returns available IP addresses with enhanced CIDR validation
func (s *AllocationService) GetAvailableIPs(ctx context.Context, regionName, zoneName, subZoneName, ipVersion string, limit int) (map[string]interface{}, error) {
	s.logger.Debug("Getting available IPs",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName),
		zap.String("ip_version", ipVersion),
		zap.Int("limit", limit))

	subZone, _, _, err := s.findSubZoneWithHierarchy(ctx, regionName, zoneName, subZoneName)
	if err != nil {
		s.logger.Error("Failed to find sub-zone for available IPs", zap.Error(err))
		return nil, err
	}

	var cidr string
	var allocated, reserved []string

	// Select appropriate CIDR and lists based on IP version
	switch ipVersion {
	case "ipv4":
		cidr = subZone.IPv4CIDR
		allocated = subZone.AllocatedIPv4
		reserved = subZone.ReservedIPv4
	case "ipv6":
		cidr = subZone.IPv6CIDR
		allocated = subZone.AllocatedIPv6
		reserved = subZone.ReservedIPv6
	default:
		s.logger.Warn("Invalid IP version requested", zap.String("ip_version", ipVersion))
		return map[string]interface{}{
			"success":   false,
			"message":   "Invalid IP version. Must be 'ipv4' or 'ipv6'",
			"timestamp": time.Now().Format(time.RFC3339),
		}, nil
	}

	var availableIPs []string
	if cidr != "" {
		availableIPs, err = utils.GetAvailableIPsInRange(cidr, allocated, reserved, limit)
		if err != nil {
			s.logger.Error("Failed to get available IPs in range",
				zap.Error(err),
				zap.String("cidr", cidr))
			return nil, err
		}
	}

	s.logger.Debug("Available IPs retrieved",
		zap.Int("available_count", len(availableIPs)),
		zap.String("cidr", cidr))

	return map[string]interface{}{
		"success":       true,
		"available_ips": availableIPs,
		"count":         len(availableIPs),
		"ip_version":    ipVersion,
		"limit":         limit,
		"cidr":          cidr,
		"timestamp":     time.Now().Format(time.RFC3339),
	}, nil
}

// GetIPStats returns comprehensive IP statistics with enhanced information
func (s *AllocationService) GetIPStats(ctx context.Context, regionName, zoneName, subZoneName string) (map[string]interface{}, error) {
	s.logger.Debug("Getting IP statistics",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName))

	subZone, _, _, err := s.findSubZoneWithHierarchy(ctx, regionName, zoneName, subZoneName)
	if err != nil {
		s.logger.Error("Failed to find sub-zone for IP stats", zap.Error(err))
		return nil, err
	}

	// Calculate comprehensive statistics
	ipv4Total, _ := utils.CountIPsInCIDR(subZone.IPv4CIDR)
	ipv6Total, _ := utils.CountIPsInCIDR(subZone.IPv6CIDR)

	stats := map[string]interface{}{
		"success":              true,
		"ipv4_cidr":            subZone.IPv4CIDR,
		"ipv6_cidr":            subZone.IPv6CIDR,
		"ipv4_total_count":     ipv4Total.String(),
		"ipv6_total_count":     ipv6Total.String(),
		"ipv4_allocated_count": len(subZone.AllocatedIPv4),
		"ipv6_allocated_count": len(subZone.AllocatedIPv6),
		"ipv4_reserved_count":  len(subZone.ReservedIPv4),
		"ipv6_reserved_count":  len(subZone.ReservedIPv6),
		"timestamp":            time.Now().Format(time.RFC3339),
	}

	// Calculate available counts
	if ipv4Total.Int64() > 0 {
		stats["ipv4_available_count"] = ipv4Total.Int64() - int64(len(subZone.AllocatedIPv4)) - int64(len(subZone.ReservedIPv4))
	}
	if ipv6Total.Int64() > 0 {
		stats["ipv6_available_count"] = ipv6Total.Int64() - int64(len(subZone.AllocatedIPv6)) - int64(len(subZone.ReservedIPv6))
	}

	s.logger.Debug("IP statistics calculated",
		zap.Int("ipv4_allocated", len(subZone.AllocatedIPv4)),
		zap.Int("ipv6_allocated", len(subZone.AllocatedIPv6)),
		zap.Int("ipv4_reserved", len(subZone.ReservedIPv4)),
		zap.Int("ipv6_reserved", len(subZone.ReservedIPv6)))

	return stats, nil
}

// Enhanced helper methods

// findSubZoneWithHierarchy finds sub-zone and returns full hierarchy for validation
func (s *AllocationService) findSubZoneWithHierarchy(ctx context.Context, regionName, zoneName, subZoneName string) (*models.SubZone, *models.Region, *models.Zone, error) {
	var region models.Region

	filter := bson.M{"name": regionName}
	err := s.collection.FindOne(ctx, filter).Decode(&region)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil, nil, fmt.Errorf("region '%s' not found", regionName)
		}
		return nil, nil, nil, err
	}

	// Find zone
	var targetZone *models.Zone
	for i := range region.Zones {
		if region.Zones[i].Name == zoneName {
			targetZone = &region.Zones[i]
			break
		}
	}
	if targetZone == nil {
		return nil, nil, nil, fmt.Errorf("zone '%s' not found in region '%s'", zoneName, regionName)
	}

	// Find sub-zone
	for i := range targetZone.SubZones {
		if targetZone.SubZones[i].Name == subZoneName {
			return &targetZone.SubZones[i], &region, targetZone, nil
		}
	}

	return nil, nil, nil, fmt.Errorf("sub-zone '%s' not found in zone '%s'", subZoneName, zoneName)
}

// validateCIDRHierarchy validates CIDR hierarchy across Region -> Zone -> SubZone
func (s *AllocationService) validateCIDRHierarchy(region *models.Region, zone *models.Zone, subZone *models.SubZone) error {
	// Validate Zone CIDR against Region CIDR
	if err := utils.ValidateZoneCIDRHierarchy(region.IPv4CIDR, region.IPv6CIDR, zone.IPv4CIDR, zone.IPv6CIDR); err != nil {
		return fmt.Errorf("zone CIDR hierarchy validation failed: %v", err)
	}

	// Validate SubZone CIDR against Zone CIDR
	if err := utils.ValidateSubZoneCIDRHierarchy(zone.IPv4CIDR, zone.IPv6CIDR, subZone.IPv4CIDR, subZone.IPv6CIDR); err != nil {
		return fmt.Errorf("sub-zone CIDR hierarchy validation failed: %v", err)
	}

	return nil
}

// validateIPInSubZoneCIDR validates if IP is in the sub-zone's CIDR range
func (s *AllocationService) validateIPInSubZoneCIDR(ip string, subZone *models.SubZone) error {
	var cidr string
	var err error

	if utils.IsIPv4(net.ParseIP(ip)) {
		cidr = subZone.IPv4CIDR
	} else if utils.IsIPv6(net.ParseIP(ip)) {
		cidr = subZone.IPv6CIDR
	} else {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	if cidr == "" {
		return fmt.Errorf("no CIDR configured for IP version")
	}

	// Enhanced validation: check if IP is in CIDR range
	inRange, err := utils.IsIPInCIDR(ip, cidr)
	if err != nil {
		return fmt.Errorf("CIDR validation error: %v", err)
	}
	if !inRange {
		return fmt.Errorf("IP %s is not in CIDR range %s", ip, cidr)
	}

	return nil
}

// allocateIPsForVersionEnhanced allocates IPs with enhanced CIDR validation
func (s *AllocationService) allocateIPsForVersionEnhanced(ctx context.Context, subZone *models.SubZone, preferredIPs []string, count int, version string) ([]string, error) {
	var cidr string
	var allocatedList, reservedList []string

	if version == "ipv4" {
		cidr = subZone.IPv4CIDR
		allocatedList = subZone.AllocatedIPv4
		reservedList = subZone.ReservedIPv4
	} else {
		cidr = subZone.IPv6CIDR
		allocatedList = subZone.AllocatedIPv6
		reservedList = subZone.ReservedIPv6
	}

	if cidr == "" {
		return nil, fmt.Errorf("no %s CIDR configured for sub-zone", version)
	}

	s.logger.Debug("Starting IP allocation for version",
		zap.String("version", version),
		zap.String("cidr", cidr),
		zap.Int("requested_count", count),
		zap.Int("preferred_count", len(preferredIPs)))

	var allocatedIPs []string

	// Enhanced preferred IP processing with CIDR validation
	for _, ip := range preferredIPs {
		if len(allocatedIPs) >= count {
			break
		}

		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			s.logger.Warn("Invalid IP in preferred list", zap.String("ip", ip))
			continue
		}

		normalizedIP := utils.NormalizeIP(ip)

		// Validate IP version matches
		if (version == "ipv4" && !utils.IsIPv4(parsedIP)) || (version == "ipv6" && !utils.IsIPv6(parsedIP)) {
			s.logger.Warn("IP version mismatch in preferred list",
				zap.String("ip", ip),
				zap.String("expected_version", version))
			continue
		}

		// Enhanced CIDR validation: check if IP is in CIDR range
		inRange, err := utils.IsIPInCIDR(normalizedIP, cidr)
		if err != nil || !inRange {
			s.logger.Warn("Preferred IP not in CIDR range",
				zap.String("ip", normalizedIP),
				zap.String("cidr", cidr),
				zap.Error(err))
			continue
		}

		// Check if IP is already allocated or reserved
		if s.isIPUsed(normalizedIP, allocatedList, reservedList) {
			s.logger.Debug("Preferred IP already in use", zap.String("ip", normalizedIP))
			continue
		}

		allocatedIPs = append(allocatedIPs, normalizedIP)
		s.logger.Debug("Preferred IP allocated", zap.String("ip", normalizedIP))
	}

	// If we need more IPs, allocate from available range
	remaining := count - len(allocatedIPs)
	if remaining > 0 {
		s.logger.Debug("Allocating additional IPs from available range",
			zap.Int("remaining", remaining))

		for i := 0; i < remaining; i++ {
			nextIP, err := utils.GetNextAvailableIP(cidr, append(allocatedList, allocatedIPs...), reservedList)
			if err != nil {
				s.logger.Warn("No more available IPs in range",
					zap.String("cidr", cidr),
					zap.Error(err))
				break
			}
			allocatedIPs = append(allocatedIPs, nextIP)
			s.logger.Debug("Auto-allocated IP", zap.String("ip", nextIP))
		}
	}

	s.logger.Info("IP allocation for version completed",
		zap.String("version", version),
		zap.Int("allocated_count", len(allocatedIPs)),
		zap.Int("requested_count", count))

	return allocatedIPs, nil
}

// updateAllocatedIPs updates the allocated IPs in the database
func (s *AllocationService) updateAllocatedIPs(ctx context.Context, regionName, zoneName, subZoneName string, newIPs []string) error {
	// Split IPs by version
	ipv4s, ipv6s, err := utils.SplitIPsByVersion(newIPs)
	if err != nil {
		return err
	}

	s.logger.Debug("Updating allocated IPs in database",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName),
		zap.Int("ipv4_count", len(ipv4s)),
		zap.Int("ipv6_count", len(ipv6s)))

	// Prepare update operations
	update := bson.M{}
	if len(ipv4s) > 0 {
		update["$push"] = bson.M{
			"zones.$[zone].sub_zones.$[subzone].allocated_ipv4": bson.M{"$each": ipv4s},
		}
	}
	if len(ipv6s) > 0 {
		if update["$push"] == nil {
			update["$push"] = bson.M{}
		}
		update["$push"].(bson.M)["zones.$[zone].sub_zones.$[subzone].allocated_ipv6"] = bson.M{"$each": ipv6s}
	}

	// Set updated timestamp
	update["$set"] = bson.M{
		"zones.$[zone].sub_zones.$[subzone].updated_at": time.Now(),
		"updated_at": time.Now(),
	}

	// Array filters for nested update
	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{
			bson.M{"zone.name": zoneName},
			bson.M{"subzone.name": subZoneName},
		},
	}

	opts := options.Update().SetArrayFilters(arrayFilters)
	filter := bson.M{"name": regionName}
	result, err := s.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("no matching document found for region %s", regionName)
	}

	return nil
}

// removeAllocatedIPs removes IPs from allocated lists
func (s *AllocationService) removeAllocatedIPs(ctx context.Context, regionName, zoneName, subZoneName string, ipv4s, ipv6s []string) error {
	s.logger.Debug("Removing allocated IPs from database",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName),
		zap.Int("ipv4_count", len(ipv4s)),
		zap.Int("ipv6_count", len(ipv6s)))

	update := bson.M{}

	if len(ipv4s) > 0 {
		update["$pullAll"] = bson.M{
			"zones.$[zone].sub_zones.$[subzone].allocated_ipv4": ipv4s,
		}
	}

	if len(ipv6s) > 0 {
		if update["$pullAll"] == nil {
			update["$pullAll"] = bson.M{}
		}
		update["$pullAll"].(bson.M)["zones.$[zone].sub_zones.$[subzone].allocated_ipv6"] = ipv6s
	}

	update["$set"] = bson.M{
		"zones.$[zone].sub_zones.$[subzone].updated_at": time.Now(),
		"updated_at": time.Now(),
	}

	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{
			bson.M{"zone.name": zoneName},
			bson.M{"subzone.name": subZoneName},
		},
	}

	opts := options.Update().SetArrayFilters(arrayFilters)
	filter := bson.M{"name": regionName}
	result, err := s.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("no matching document found for region %s", regionName)
	}

	return nil
}

// addReservedIPs adds IPs to reserved lists
func (s *AllocationService) addReservedIPs(ctx context.Context, regionName, zoneName, subZoneName string, ips []string) error {
	ipv4s, ipv6s, err := utils.SplitIPsByVersion(ips)
	if err != nil {
		return err
	}

	s.logger.Debug("Adding reserved IPs to database",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName),
		zap.Int("ipv4_count", len(ipv4s)),
		zap.Int("ipv6_count", len(ipv6s)))

	update := bson.M{}
	if len(ipv4s) > 0 {
		update["$push"] = bson.M{
			"zones.$[zone].sub_zones.$[subzone].reserved_ipv4": bson.M{"$each": ipv4s},
		}
	}
	if len(ipv6s) > 0 {
		if update["$push"] == nil {
			update["$push"] = bson.M{}
		}
		update["$push"].(bson.M)["zones.$[zone].sub_zones.$[subzone].reserved_ipv6"] = bson.M{"$each": ipv6s}
	}

	update["$set"] = bson.M{
		"zones.$[zone].sub_zones.$[subzone].updated_at": time.Now(),
		"updated_at": time.Now(),
	}

	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{
			bson.M{"zone.name": zoneName},
			bson.M{"subzone.name": subZoneName},
		},
	}

	opts := options.Update().SetArrayFilters(arrayFilters)
	filter := bson.M{"name": regionName}
	result, err := s.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("no matching document found for region %s", regionName)
	}

	return nil
}

// removeReservedIPs removes IPs from reserved lists
func (s *AllocationService) removeReservedIPs(ctx context.Context, regionName, zoneName, subZoneName string, ips []string) error {
	ipv4s, ipv6s, err := utils.SplitIPsByVersion(ips)
	if err != nil {
		return err
	}

	s.logger.Debug("Removing reserved IPs from database",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName),
		zap.Int("ipv4_count", len(ipv4s)),
		zap.Int("ipv6_count", len(ipv6s)))

	update := bson.M{}
	if len(ipv4s) > 0 {
		update["$pullAll"] = bson.M{
			"zones.$[zone].sub_zones.$[subzone].reserved_ipv4": ipv4s,
		}
	}
	if len(ipv6s) > 0 {
		if update["$pullAll"] == nil {
			update["$pullAll"] = bson.M{}
		}
		update["$pullAll"].(bson.M)["zones.$[zone].sub_zones.$[subzone].reserved_ipv6"] = ipv6s
	}

	update["$set"] = bson.M{
		"zones.$[zone].sub_zones.$[subzone].updated_at": time.Now(),
		"updated_at": time.Now(),
	}

	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{
			bson.M{"zone.name": zoneName},
			bson.M{"subzone.name": subZoneName},
		},
	}

	opts := options.Update().SetArrayFilters(arrayFilters)
	filter := bson.M{"name": regionName}
	result, err := s.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return err
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("no matching document found for region %s", regionName)
	}

	return nil
}

// isIPUsed checks if an IP is already in use (allocated or reserved)
func (s *AllocationService) isIPUsed(ip string, allocated, reserved []string) bool {
	for _, allocatedIP := range allocated {
		if allocatedIP == ip {
			return true
		}
	}
	for _, reservedIP := range reserved {
		if reservedIP == ip {
			return true
		}
	}
	return false
}

// Existing methods maintained for backward compatibility

// GetRegionHierarchy returns the complete hierarchy for a region
func (s *AllocationService) GetRegionHierarchy(ctx context.Context, regionName string) (*models.Region, error) {
	s.logger.Debug("Getting region hierarchy", zap.String("region", regionName))

	var region models.Region
	filter := bson.M{"name": regionName}
	err := s.collection.FindOne(ctx, filter).Decode(&region)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			s.logger.Warn("Region not found", zap.String("region", regionName))
			return nil, fmt.Errorf("region '%s' not found", regionName)
		}
		s.logger.Error("Error retrieving region", zap.Error(err), zap.String("region", regionName))
		return nil, err
	}

	s.logger.Debug("Region hierarchy retrieved successfully",
		zap.String("region", regionName),
		zap.Int("zones_count", len(region.Zones)))

	return &region, nil
}

// GetAllRegions returns all regions
func (s *AllocationService) GetAllRegions(ctx context.Context) ([]models.Region, error) {
	s.logger.Debug("Getting all regions")

	var regions []models.Region
	cursor, err := s.collection.Find(ctx, bson.M{})
	if err != nil {
		s.logger.Error("Error retrieving regions", zap.Error(err))
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &regions); err != nil {
		s.logger.Error("Error decoding regions", zap.Error(err))
		return nil, err
	}

	s.logger.Debug("All regions retrieved successfully", zap.Int("count", len(regions)))
	return regions, nil
}

// CreateRegion creates a new region with enhanced validation
func (s *AllocationService) CreateRegion(ctx context.Context, region *models.Region) error {
	s.logger.Info("Creating new region",
		zap.String("region", region.Name),
		zap.String("ipv4_cidr", region.IPv4CIDR),
		zap.String("ipv6_cidr", region.IPv6CIDR))

	// Set timestamps
	region.CreatedAt = time.Now()
	region.UpdatedAt = time.Now()

	// Set timestamps for zones and sub-zones
	for i := range region.Zones {
		region.Zones[i].CreatedAt = time.Now()
		region.Zones[i].UpdatedAt = time.Now()

		// Validate zone CIDR against region CIDR
		if err := utils.ValidateZoneCIDRHierarchy(region.IPv4CIDR, region.IPv6CIDR, region.Zones[i].IPv4CIDR, region.Zones[i].IPv6CIDR); err != nil {
			s.logger.Error("Zone CIDR validation failed",
				zap.Error(err),
				zap.String("zone", region.Zones[i].Name))
			return fmt.Errorf("zone CIDR validation failed for zone %s: %v", region.Zones[i].Name, err)
		}

		for j := range region.Zones[i].SubZones {
			region.Zones[i].SubZones[j].CreatedAt = time.Now()
			region.Zones[i].SubZones[j].UpdatedAt = time.Now()

			// Validate sub-zone CIDR against zone CIDR
			if err := utils.ValidateSubZoneCIDRHierarchy(region.Zones[i].IPv4CIDR, region.Zones[i].IPv6CIDR, region.Zones[i].SubZones[j].IPv4CIDR, region.Zones[i].SubZones[j].IPv6CIDR); err != nil {
				s.logger.Error("Sub-zone CIDR validation failed",
					zap.Error(err),
					zap.String("subzone", region.Zones[i].SubZones[j].Name))
				return fmt.Errorf("sub-zone CIDR validation failed for sub-zone %s: %v", region.Zones[i].SubZones[j].Name, err)
			}

			// Initialize empty slices for IP lists
			if region.Zones[i].SubZones[j].AllocatedIPv4 == nil {
				region.Zones[i].SubZones[j].AllocatedIPv4 = []string{}
			}
			if region.Zones[i].SubZones[j].AllocatedIPv6 == nil {
				region.Zones[i].SubZones[j].AllocatedIPv6 = []string{}
			}
			if region.Zones[i].SubZones[j].ReservedIPv4 == nil {
				region.Zones[i].SubZones[j].ReservedIPv4 = []string{}
			}
			if region.Zones[i].SubZones[j].ReservedIPv6 == nil {
				region.Zones[i].SubZones[j].ReservedIPv6 = []string{}
			}
		}
	}

	result, err := s.collection.InsertOne(ctx, region)
	if err != nil {
		s.logger.Error("Failed to create region", zap.Error(err), zap.String("region", region.Name))
		return err
	}

	s.logger.Info("Region created successfully",
		zap.String("region", region.Name),
		zap.Any("id", result.InsertedID))

	return nil
}
