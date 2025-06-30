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
)

type AllocationService struct {
	collection *mongo.Collection
}

func NewAllocationService(db *mongo.Database) *AllocationService {
	return &AllocationService{
		collection: db.Collection(models.RegionCollection),
	}
}

// TestConnection tests the database connection
func (s *AllocationService) TestConnection(ctx context.Context) error {
	return s.collection.Database().Client().Ping(ctx, nil)
}

// AllocateIPs allocates IP addresses based on the request
func (s *AllocationService) AllocateIPs(ctx context.Context, req *models.AllocationRequest) (*models.AllocationResponse, error) {
	// Find the target sub-zone
	subZone, err := s.findSubZone(ctx, req.Region, req.Zone, req.SubZone)
	if err != nil {
		return &models.AllocationResponse{
			Success:   false,
			Message:   fmt.Sprintf("Failed to find sub-zone: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	var allocatedIPs []string
	var errors []string

	// Handle different IP version requirements
	switch req.IPVersion {
	case "ipv4":
		ips, err := s.allocateIPsForVersion(ctx, subZone, req.PreferredIPs, req.Count, "ipv4")
		if err != nil {
			errors = append(errors, fmt.Sprintf("IPv4 allocation failed: %v", err))
		} else {
			allocatedIPs = append(allocatedIPs, ips...)
		}
	case "ipv6":
		ips, err := s.allocateIPsForVersion(ctx, subZone, req.PreferredIPs, req.Count, "ipv6")
		if err != nil {
			errors = append(errors, fmt.Sprintf("IPv6 allocation failed: %v", err))
		} else {
			allocatedIPs = append(allocatedIPs, ips...)
		}
	case "both":
		// Allocate half for IPv4 and half for IPv6
		ipv4Count := req.Count / 2
		ipv6Count := req.Count - ipv4Count

		if ipv4Count > 0 {
			ipv4Preferred, _, err := utils.SplitIPsByVersion(req.PreferredIPs)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to split preferred IPs: %v", err))
			} else {
				ips, err := s.allocateIPsForVersion(ctx, subZone, ipv4Preferred, ipv4Count, "ipv4")
				if err != nil {
					errors = append(errors, fmt.Sprintf("IPv4 allocation failed: %v", err))
				} else {
					allocatedIPs = append(allocatedIPs, ips...)
				}
			}
		}

		if ipv6Count > 0 {
			_, ipv6Preferred, err := utils.SplitIPsByVersion(req.PreferredIPs)
			if err != nil {
				errors = append(errors, fmt.Sprintf("Failed to split preferred IPs: %v", err))
			} else {
				ips, err := s.allocateIPsForVersion(ctx, subZone, ipv6Preferred, ipv6Count, "ipv6")
				if err != nil {
					errors = append(errors, fmt.Sprintf("IPv6 allocation failed: %v", err))
				} else {
					allocatedIPs = append(allocatedIPs, ips...)
				}
			}
		}
	}

	// Update the database with allocated IPs
	if len(allocatedIPs) > 0 {
		err = s.updateAllocatedIPs(ctx, req.Region, req.Zone, req.SubZone, allocatedIPs)
		if err != nil {
			return &models.AllocationResponse{
				Success:   false,
				Message:   fmt.Sprintf("Failed to update database: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
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

	return &models.AllocationResponse{
		Success:      success,
		AllocatedIPs: allocatedIPs,
		Message:      message,
		Timestamp:    time.Now(),
	}, nil
}

// DeallocateIPs removes IPs from allocated lists
func (s *AllocationService) DeallocateIPs(ctx context.Context, req *models.DeallocationRequest) (*models.IPOperationResponse, error) {
	// Find the target sub-zone
	subZone, err := s.findSubZone(ctx, req.Region, req.Zone, req.SubZone)
	if err != nil {
		return &models.IPOperationResponse{
			Success:   false,
			Message:   fmt.Sprintf("Failed to find sub-zone: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	var processedIPs, failedIPs []string
	ipv4sToRemove := []string{}
	ipv6sToRemove := []string{}

	// Process each IP address
	for _, ip := range req.IPAddresses {
		normalizedIP := utils.NormalizeIP(ip)

		// Check if IP is actually allocated
		var found bool
		if utils.IsIPv4(net.ParseIP(ip)) {
			for _, allocatedIP := range subZone.AllocatedIPv4 {
				if allocatedIP == normalizedIP {
					ipv4sToRemove = append(ipv4sToRemove, normalizedIP)
					processedIPs = append(processedIPs, normalizedIP)
					found = true
					break
				}
			}
		} else if utils.IsIPv6(net.ParseIP(ip)) {
			for _, allocatedIP := range subZone.AllocatedIPv6 {
				if allocatedIP == normalizedIP {
					ipv6sToRemove = append(ipv6sToRemove, normalizedIP)
					processedIPs = append(processedIPs, normalizedIP)
					found = true
					break
				}
			}
		}

		if !found {
			failedIPs = append(failedIPs, normalizedIP)
		}
	}

	// Update database to remove IPs
	if len(processedIPs) > 0 {
		err = s.removeAllocatedIPs(ctx, req.Region, req.Zone, req.SubZone, ipv4sToRemove, ipv6sToRemove)
		if err != nil {
			return &models.IPOperationResponse{
				Success:   false,
				Message:   fmt.Sprintf("Failed to update database: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
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

	return &models.IPOperationResponse{
		Success:      success,
		ProcessedIPs: processedIPs,
		FailedIPs:    failedIPs,
		Message:      message,
		Timestamp:    time.Now(),
	}, nil
}

// ManageReservations handles IP reservation and unreservation
func (s *AllocationService) ManageReservations(ctx context.Context, req *models.ReservationRequest) (*models.IPOperationResponse, error) {
	// Find the target sub-zone
	subZone, err := s.findSubZone(ctx, req.Region, req.Zone, req.SubZone)
	if err != nil {
		return &models.IPOperationResponse{
			Success:   false,
			Message:   fmt.Sprintf("Failed to find sub-zone: %v", err),
			Timestamp: time.Now(),
		}, nil
	}

	var processedIPs, failedIPs []string

	for _, ip := range req.IPAddresses {
		normalizedIP := utils.NormalizeIP(ip)

		// Validate IP is in CIDR range
		var inRange bool
		var err error

		if utils.IsIPv4(net.ParseIP(ip)) && subZone.IPv4CIDR != "" {
			inRange, err = utils.IsIPInCIDR(normalizedIP, subZone.IPv4CIDR)
		} else if utils.IsIPv6(net.ParseIP(ip)) && subZone.IPv6CIDR != "" {
			inRange, err = utils.IsIPInCIDR(normalizedIP, subZone.IPv6CIDR)
		}

		if err != nil || !inRange {
			failedIPs = append(failedIPs, normalizedIP)
			continue
		}

		if req.ReservationType == "reserve" {
			// Check if IP is not already allocated or reserved
			if !s.isIPUsed(normalizedIP, subZone.AllocatedIPv4, subZone.ReservedIPv4) &&
				!s.isIPUsed(normalizedIP, subZone.AllocatedIPv6, subZone.ReservedIPv6) {
				processedIPs = append(processedIPs, normalizedIP)
			} else {
				failedIPs = append(failedIPs, normalizedIP)
			}
		} else { // unreserve
			// Check if IP is actually reserved
			var isReserved bool
			if utils.IsIPv4(net.ParseIP(ip)) {
				for _, reservedIP := range subZone.ReservedIPv4 {
					if reservedIP == normalizedIP {
						isReserved = true
						break
					}
				}
			} else if utils.IsIPv6(net.ParseIP(ip)) {
				for _, reservedIP := range subZone.ReservedIPv6 {
					if reservedIP == normalizedIP {
						isReserved = true
						break
					}
				}
			}

			if isReserved {
				processedIPs = append(processedIPs, normalizedIP)
			} else {
				failedIPs = append(failedIPs, normalizedIP)
			}
		}
	}

	// Update database
	if len(processedIPs) > 0 {
		if req.ReservationType == "reserve" {
			err = s.addReservedIPs(ctx, req.Region, req.Zone, req.SubZone, processedIPs)
		} else {
			err = s.removeReservedIPs(ctx, req.Region, req.Zone, req.SubZone, processedIPs)
		}

		if err != nil {
			return &models.IPOperationResponse{
				Success:   false,
				Message:   fmt.Sprintf("Failed to update database: %v", err),
				Timestamp: time.Now(),
			}, nil
		}
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

	return &models.IPOperationResponse{
		Success:      success,
		ProcessedIPs: processedIPs,
		FailedIPs:    failedIPs,
		Message:      message,
		Timestamp:    time.Now(),
	}, nil
}

// GetAvailableIPs returns available IP addresses in a sub-zone
func (s *AllocationService) GetAvailableIPs(ctx context.Context, regionName, zoneName, subZoneName, ipVersion string, limit int) (map[string]interface{}, error) {
	_, err := s.findSubZone(ctx, regionName, zoneName, subZoneName)
	if err != nil {
		return nil, err
	}

	availableIPs := []string{}
	// Logic to find available IPs would go here

	return map[string]interface{}{
		"success":       true,
		"available_ips": availableIPs,
		"count":         len(availableIPs),
		"ip_version":    ipVersion,
		"limit":         limit,
		"timestamp":     time.Now().Format(time.RFC3339),
	}, nil
}

// GetIPStats returns comprehensive IP statistics for a sub-zone
func (s *AllocationService) GetIPStats(ctx context.Context, regionName, zoneName, subZoneName string) (map[string]interface{}, error) {
	subZone, err := s.findSubZone(ctx, regionName, zoneName, subZoneName)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"success":              true,
		"ipv4_allocated_count": len(subZone.AllocatedIPv4),
		"ipv6_allocated_count": len(subZone.AllocatedIPv6),
		"ipv4_reserved_count":  len(subZone.ReservedIPv4),
		"ipv6_reserved_count":  len(subZone.ReservedIPv6),
		"timestamp":            time.Now().Format(time.RFC3339),
	}, nil
}

// allocateIPsForVersion allocates IPs for a specific version (ipv4 or ipv6)
func (s *AllocationService) allocateIPsForVersion(ctx context.Context, subZone *models.SubZone, preferredIPs []string, count int, version string) ([]string, error) {
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

	var allocatedIPs []string

	// First, try to allocate preferred IPs
	for _, ip := range preferredIPs {
		if len(allocatedIPs) >= count {
			break
		}

		// Parse and validate IP address
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			continue
		}

		// Normalize IP for consistent comparison
		normalizedIP := utils.NormalizeIP(ip)

		// Validate IP version matches
		if (version == "ipv4" && !utils.IsIPv4(parsedIP)) || (version == "ipv6" && !utils.IsIPv6(parsedIP)) {
			continue
		}

		// Check if IP is in CIDR range
		inRange, err := utils.IsIPInCIDR(normalizedIP, cidr)
		if err != nil || !inRange {
			continue
		}

		// Check if IP is already allocated or reserved
		if s.isIPUsed(normalizedIP, allocatedList, reservedList) {
			continue
		}

		allocatedIPs = append(allocatedIPs, normalizedIP)
	}

	// If we need more IPs, allocate from available range
	for len(allocatedIPs) < count {
		nextIP, err := utils.GetNextAvailableIP(cidr, append(allocatedList, allocatedIPs...), reservedList)
		if err != nil {
			return allocatedIPs, err
		}
		allocatedIPs = append(allocatedIPs, nextIP)
	}

	return allocatedIPs, nil
}

// Helper methods (keep all existing helper methods)

// findSubZone finds a sub-zone by hierarchy path
func (s *AllocationService) findSubZone(ctx context.Context, regionName, zoneName, subZoneName string) (*models.SubZone, error) {
	var region models.Region

	filter := bson.M{"name": regionName}
	err := s.collection.FindOne(ctx, filter).Decode(&region)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("region '%s' not found", regionName)
		}
		return nil, err
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
		return nil, fmt.Errorf("zone '%s' not found in region '%s'", zoneName, regionName)
	}

	// Find sub-zone
	for i := range targetZone.SubZones {
		if targetZone.SubZones[i].Name == subZoneName {
			return &targetZone.SubZones[i], nil
		}
	}

	return nil, fmt.Errorf("sub-zone '%s' not found in zone '%s'", subZoneName, zoneName)
}

// updateAllocatedIPs updates the allocated IPs in the database
func (s *AllocationService) updateAllocatedIPs(ctx context.Context, regionName, zoneName, subZoneName string, newIPs []string) error {
	// Split IPs by version
	ipv4s, ipv6s, err := utils.SplitIPsByVersion(newIPs)
	if err != nil {
		return err
	}

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

	// Array filters - Fixed implementation
	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{
			bson.M{"zone.name": zoneName},
			bson.M{"subzone.name": subZoneName},
		},
	}

	// Update options - Fixed ArrayFilters usage
	opts := options.Update().SetArrayFilters(arrayFilters)

	filter := bson.M{"name": regionName}
	_, err = s.collection.UpdateOne(ctx, filter, update, opts)
	return err
}

// removeAllocatedIPs helper method
func (s *AllocationService) removeAllocatedIPs(ctx context.Context, regionName, zoneName, subZoneName string, ipv4s, ipv6s []string) error {
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
	_, err := s.collection.UpdateOne(ctx, filter, update, opts)
	return err
}

// addReservedIPs helper method
func (s *AllocationService) addReservedIPs(ctx context.Context, regionName, zoneName, subZoneName string, ips []string) error {
	ipv4s, ipv6s, err := utils.SplitIPsByVersion(ips)
	if err != nil {
		return err
	}

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
	_, err = s.collection.UpdateOne(ctx, filter, update, opts)
	return err
}

// removeReservedIPs helper method
func (s *AllocationService) removeReservedIPs(ctx context.Context, regionName, zoneName, subZoneName string, ips []string) error {
	ipv4s, ipv6s, err := utils.SplitIPsByVersion(ips)
	if err != nil {
		return err
	}

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
	_, err = s.collection.UpdateOne(ctx, filter, update, opts)
	return err
}

// isIPUsed checks if an IP is already in use
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

// GetRegionHierarchy returns the complete hierarchy for a region
func (s *AllocationService) GetRegionHierarchy(ctx context.Context, regionName string) (*models.Region, error) {
	var region models.Region

	filter := bson.M{"name": regionName}
	err := s.collection.FindOne(ctx, filter).Decode(&region)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("region '%s' not found", regionName)
		}
		return nil, err
	}

	return &region, nil
}

// GetAllRegions returns all regions
func (s *AllocationService) GetAllRegions(ctx context.Context) ([]models.Region, error) {
	var regions []models.Region

	cursor, err := s.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	if err = cursor.All(ctx, &regions); err != nil {
		return nil, err
	}

	return regions, nil
}

// CreateRegion creates a new region with initial structure
func (s *AllocationService) CreateRegion(ctx context.Context, region *models.Region) error {
	region.CreatedAt = time.Now()
	region.UpdatedAt = time.Now()

	// Set timestamps for zones and sub-zones
	for i := range region.Zones {
		region.Zones[i].CreatedAt = time.Now()
		region.Zones[i].UpdatedAt = time.Now()
		for j := range region.Zones[i].SubZones {
			region.Zones[i].SubZones[j].CreatedAt = time.Now()
			region.Zones[i].SubZones[j].UpdatedAt = time.Now()
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

	_, err := s.collection.InsertOne(ctx, region)
	return err
}
