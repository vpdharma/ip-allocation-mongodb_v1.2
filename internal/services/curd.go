package services

import (
	"context"
	"fmt"
	"time"

	"ip-allocator-api/internal/models"
	"ip-allocator-api/internal/utils"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

type CRUDService struct {
	collection *mongo.Collection
	logger     *zap.Logger
}

func NewCRUDService(db *mongo.Database, logger *zap.Logger) *CRUDService {
	return &CRUDService{
		collection: db.Collection(models.RegionCollection),
		logger:     logger,
	}
}

// CreateRegion creates a new region with enhanced validation
func (s *CRUDService) CreateRegion(ctx context.Context, req *models.CreateRegionRequest) (*models.CRUDResponse, error) {
	s.logger.Info("Creating new region",
		zap.String("name", req.Name),
		zap.String("ipv4_cidr", req.IPv4CIDR),
		zap.String("ipv6_cidr", req.IPv6CIDR))

	// Check if region already exists
	filter := bson.M{"name": req.Name}
	count, err := s.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return &models.CRUDResponse{
			Success:   false,
			Message:   "Region with this name already exists",
			Timestamp: time.Now(),
		}, nil
	}

	// Validate CIDR blocks if provided
	if req.IPv4CIDR != "" {
		if _, err := utils.ParseCIDR(req.IPv4CIDR); err != nil {
			return &models.CRUDResponse{
				Success:   false,
				Message:   "Invalid IPv4 CIDR: " + err.Error(),
				Timestamp: time.Now(),
			}, nil
		}
	}
	if req.IPv6CIDR != "" {
		if _, err := utils.ParseCIDR(req.IPv6CIDR); err != nil {
			return &models.CRUDResponse{
				Success:   false,
				Message:   "Invalid IPv6 CIDR: " + err.Error(),
				Timestamp: time.Now(),
			}, nil
		}
	}

	// Create region
	region := models.Region{
		ID:        primitive.NewObjectID(),
		Name:      req.Name,
		IPv4CIDR:  req.IPv4CIDR,
		IPv6CIDR:  req.IPv6CIDR,
		Zones:     []models.Zone{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	_, err = s.collection.InsertOne(ctx, region)
	if err != nil {
		s.logger.Error("Failed to create region",
			zap.Error(err),
			zap.String("name", req.Name))
		return nil, err
	}

	s.logger.Info("Region created successfully",
		zap.String("name", req.Name),
		zap.String("id", region.ID.Hex()))

	return &models.CRUDResponse{
		Success:   true,
		Data:      region,
		Message:   "Region created successfully",
		Timestamp: time.Now(),
	}, nil
}

// UpdateRegion updates an existing region
func (s *CRUDService) UpdateRegion(ctx context.Context, regionName string, req *models.UpdateRegionRequest) (*models.CRUDResponse, error) {
	s.logger.Info("Updating region",
		zap.String("name", regionName),
		zap.Any("update", req))

	update := bson.M{
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}

	if req.Name != "" {
		update["$set"].(bson.M)["name"] = req.Name
	}
	if req.IPv4CIDR != "" {
		// Validate CIDR
		if _, err := utils.ParseCIDR(req.IPv4CIDR); err != nil {
			return &models.CRUDResponse{
				Success:   false,
				Message:   "Invalid IPv4 CIDR: " + err.Error(),
				Timestamp: time.Now(),
			}, nil
		}
		update["$set"].(bson.M)["ipv4_cidr"] = req.IPv4CIDR
	}
	if req.IPv6CIDR != "" {
		// Validate CIDR
		if _, err := utils.ParseCIDR(req.IPv6CIDR); err != nil {
			return &models.CRUDResponse{
				Success:   false,
				Message:   "Invalid IPv6 CIDR: " + err.Error(),
				Timestamp: time.Now(),
			}, nil
		}
		update["$set"].(bson.M)["ipv6_cidr"] = req.IPv6CIDR
	}

	filter := bson.M{"name": regionName}
	result, err := s.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		s.logger.Error("Failed to update region",
			zap.Error(err),
			zap.String("name", regionName))
		return nil, err
	}

	if result.MatchedCount == 0 {
		return &models.CRUDResponse{
			Success:   false,
			Message:   "Region not found",
			Timestamp: time.Now(),
		}, nil
	}

	s.logger.Info("Region updated successfully",
		zap.String("name", regionName))

	return &models.CRUDResponse{
		Success:   true,
		Message:   "Region updated successfully",
		Timestamp: time.Now(),
	}, nil
}

// DeleteRegion deletes a region
func (s *CRUDService) DeleteRegion(ctx context.Context, regionName string) (*models.CRUDResponse, error) {
	s.logger.Info("Deleting region", zap.String("name", regionName))

	filter := bson.M{"name": regionName}
	result, err := s.collection.DeleteOne(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to delete region",
			zap.Error(err),
			zap.String("name", regionName))
		return nil, err
	}

	if result.DeletedCount == 0 {
		return &models.CRUDResponse{
			Success:   false,
			Message:   "Region not found",
			Timestamp: time.Now(),
		}, nil
	}

	s.logger.Info("Region deleted successfully",
		zap.String("name", regionName))

	return &models.CRUDResponse{
		Success:   true,
		Message:   "Region deleted successfully",
		Timestamp: time.Now(),
	}, nil
}

// CreateZone creates a new zone with enhanced CIDR validation
func (s *CRUDService) CreateZone(ctx context.Context, regionName string, req *models.CreateZoneRequest) (*models.CRUDResponse, error) {
	s.logger.Info("Creating new zone",
		zap.String("region", regionName),
		zap.String("zone", req.Name),
		zap.String("ipv4_cidr", req.IPv4CIDR),
		zap.String("ipv6_cidr", req.IPv6CIDR))

	// Get the region
	var region models.Region
	filter := bson.M{"name": regionName}
	err := s.collection.FindOne(ctx, filter).Decode(&region)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &models.CRUDResponse{
				Success:   false,
				Message:   "Region not found",
				Timestamp: time.Now(),
			}, nil
		}
		return nil, err
	}

	// Enhanced CIDR validation against region CIDRs
	if err := utils.ValidateZoneCIDRHierarchy(region.IPv4CIDR, region.IPv6CIDR, req.IPv4CIDR, req.IPv6CIDR); err != nil {
		s.logger.Warn("Zone CIDR validation failed",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", req.Name))
		return &models.CRUDResponse{
			Success:   false,
			Message:   "CIDR validation failed: " + err.Error(),
			Timestamp: time.Now(),
		}, nil
	}

	// Check for zone name conflicts
	for _, existingZone := range region.Zones {
		if existingZone.Name == req.Name {
			return &models.CRUDResponse{
				Success:   false,
				Message:   "Zone with this name already exists in the region",
				Timestamp: time.Now(),
			}, nil
		}

		// Check for CIDR overlaps with existing zones
		if req.IPv4CIDR != "" && existingZone.IPv4CIDR != "" {
			if overlap, err := utils.CheckCIDROverlap(req.IPv4CIDR, existingZone.IPv4CIDR); err == nil && overlap {
				return &models.CRUDResponse{
					Success:   false,
					Message:   fmt.Sprintf("IPv4 CIDR overlaps with existing zone '%s'", existingZone.Name),
					Timestamp: time.Now(),
				}, nil
			}
		}
		if req.IPv6CIDR != "" && existingZone.IPv6CIDR != "" {
			if overlap, err := utils.CheckCIDROverlap(req.IPv6CIDR, existingZone.IPv6CIDR); err == nil && overlap {
				return &models.CRUDResponse{
					Success:   false,
					Message:   fmt.Sprintf("IPv6 CIDR overlaps with existing zone '%s'", existingZone.Name),
					Timestamp: time.Now(),
				}, nil
			}
		}
	}

	// Create new zone
	newZone := models.Zone{
		ID:        primitive.NewObjectID(),
		Name:      req.Name,
		IPv4CIDR:  req.IPv4CIDR,
		IPv6CIDR:  req.IPv6CIDR,
		SubZones:  []models.SubZone{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Update region with new zone
	update := bson.M{
		"$push": bson.M{
			"zones": newZone,
		},
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}

	_, err = s.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		s.logger.Error("Failed to create zone",
			zap.Error(err),
			zap.String("region", regionName),
			zap.String("zone", req.Name))
		return nil, err
	}

	s.logger.Info("Zone created successfully",
		zap.String("region", regionName),
		zap.String("zone", req.Name),
		zap.String("id", newZone.ID.Hex()))

	return &models.CRUDResponse{
		Success:   true,
		Data:      newZone,
		Message:   "Zone created successfully",
		Timestamp: time.Now(),
	}, nil
}

// GetZone retrieves a specific zone
func (s *CRUDService) GetZone(ctx context.Context, regionName, zoneName string) (*models.CRUDResponse, error) {
	s.logger.Debug("Getting zone",
		zap.String("region", regionName),
		zap.String("zone", zoneName))

	var region models.Region
	filter := bson.M{"name": regionName}
	err := s.collection.FindOne(ctx, filter).Decode(&region)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return &models.CRUDResponse{
				Success:   false,
				Message:   "Region not found",
				Timestamp: time.Now(),
			}, nil
		}
		return nil, err
	}

	// Find the zone
	for _, zone := range region.Zones {
		if zone.Name == zoneName {
			return &models.CRUDResponse{
				Success:   true,
				Data:      zone,
				Message:   "Zone retrieved successfully",
				Timestamp: time.Now(),
			}, nil
		}
	}

	return &models.CRUDResponse{
		Success:   false,
		Message:   "Zone not found",
		Timestamp: time.Now(),
	}, nil
}

// UpdateZone updates an existing zone
func (s *CRUDService) UpdateZone(ctx context.Context, regionName, zoneName string, req *models.UpdateZoneRequest) (*models.CRUDResponse, error) {
	s.logger.Info("Updating zone",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.Any("update", req))

	update := bson.M{
		"$set": bson.M{
			"zones.$[zone].updated_at": time.Now(),
			"updated_at":               time.Now(),
		},
	}

	if req.Name != "" {
		update["$set"].(bson.M)["zones.$[zone].name"] = req.Name
	}
	if req.IPv4CIDR != "" {
		if _, err := utils.ParseCIDR(req.IPv4CIDR); err != nil {
			return &models.CRUDResponse{
				Success:   false,
				Message:   "Invalid IPv4 CIDR: " + err.Error(),
				Timestamp: time.Now(),
			}, nil
		}
		update["$set"].(bson.M)["zones.$[zone].ipv4_cidr"] = req.IPv4CIDR
	}
	if req.IPv6CIDR != "" {
		if _, err := utils.ParseCIDR(req.IPv6CIDR); err != nil {
			return &models.CRUDResponse{
				Success:   false,
				Message:   "Invalid IPv6 CIDR: " + err.Error(),
				Timestamp: time.Now(),
			}, nil
		}
		update["$set"].(bson.M)["zones.$[zone].ipv6_cidr"] = req.IPv6CIDR
	}

	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{
			bson.M{"zone.name": zoneName},
		},
	}

	opts := options.Update().SetArrayFilters(arrayFilters)
	filter := bson.M{"name": regionName}
	result, err := s.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return nil, err
	}

	if result.MatchedCount == 0 {
		return &models.CRUDResponse{
			Success:   false,
			Message:   "Zone not found",
			Timestamp: time.Now(),
		}, nil
	}

	return &models.CRUDResponse{
		Success:   true,
		Message:   "Zone updated successfully",
		Timestamp: time.Now(),
	}, nil
}

// DeleteZone deletes a zone
func (s *CRUDService) DeleteZone(ctx context.Context, regionName, zoneName string) (*models.CRUDResponse, error) {
	s.logger.Info("Deleting zone",
		zap.String("region", regionName),
		zap.String("zone", zoneName))

	update := bson.M{
		"$pull": bson.M{
			"zones": bson.M{"name": zoneName},
		},
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}

	filter := bson.M{"name": regionName}
	result, err := s.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	if result.MatchedCount == 0 {
		return &models.CRUDResponse{
			Success:   false,
			Message:   "Region not found",
			Timestamp: time.Now(),
		}, nil
	}

	return &models.CRUDResponse{
		Success:   true,
		Message:   "Zone deleted successfully",
		Timestamp: time.Now(),
	}, nil
}

// CreateSubZone creates a new sub-zone
func (s *CRUDService) CreateSubZone(ctx context.Context, regionName, zoneName string, req *models.CreateSubZoneRequest) (*models.CRUDResponse, error) {
	s.logger.Info("Creating sub-zone",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", req.Name))

	// Create new sub-zone
	newSubZone := models.SubZone{
		ID:            primitive.NewObjectID(),
		Name:          req.Name,
		IPv4CIDR:      req.IPv4CIDR,
		IPv6CIDR:      req.IPv6CIDR,
		AllocatedIPv4: []string{},
		AllocatedIPv6: []string{},
		ReservedIPv4:  []string{},
		ReservedIPv6:  []string{},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	update := bson.M{
		"$push": bson.M{
			"zones.$[zone].sub_zones": newSubZone,
		},
		"$set": bson.M{
			"zones.$[zone].updated_at": time.Now(),
			"updated_at":               time.Now(),
		},
	}

	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{
			bson.M{"zone.name": zoneName},
		},
	}

	opts := options.Update().SetArrayFilters(arrayFilters)
	filter := bson.M{"name": regionName}
	result, err := s.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return nil, err
	}

	if result.MatchedCount == 0 {
		return &models.CRUDResponse{
			Success:   false,
			Message:   "Zone not found",
			Timestamp: time.Now(),
		}, nil
	}

	return &models.CRUDResponse{
		Success:   true,
		Data:      newSubZone,
		Message:   "Sub-zone created successfully",
		Timestamp: time.Now(),
	}, nil
}

// UpdateSubZone updates an existing sub-zone
func (s *CRUDService) UpdateSubZone(ctx context.Context, regionName, zoneName, subZoneName string, req *models.UpdateSubZoneRequest) (*models.CRUDResponse, error) {
	s.logger.Info("Updating sub-zone",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName))

	update := bson.M{
		"$set": bson.M{
			"zones.$[zone].sub_zones.$[subzone].updated_at": time.Now(),
			"zones.$[zone].updated_at":                      time.Now(),
			"updated_at":                                    time.Now(),
		},
	}

	if req.Name != "" {
		update["$set"].(bson.M)["zones.$[zone].sub_zones.$[subzone].name"] = req.Name
	}
	if req.IPv4CIDR != "" {
		if _, err := utils.ParseCIDR(req.IPv4CIDR); err != nil {
			return &models.CRUDResponse{
				Success:   false,
				Message:   "Invalid IPv4 CIDR: " + err.Error(),
				Timestamp: time.Now(),
			}, nil
		}
		update["$set"].(bson.M)["zones.$[zone].sub_zones.$[subzone].ipv4_cidr"] = req.IPv4CIDR
	}
	if req.IPv6CIDR != "" {
		if _, err := utils.ParseCIDR(req.IPv6CIDR); err != nil {
			return &models.CRUDResponse{
				Success:   false,
				Message:   "Invalid IPv6 CIDR: " + err.Error(),
				Timestamp: time.Now(),
			}, nil
		}
		update["$set"].(bson.M)["zones.$[zone].sub_zones.$[subzone].ipv6_cidr"] = req.IPv6CIDR
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
		return nil, err
	}

	if result.MatchedCount == 0 {
		return &models.CRUDResponse{
			Success:   false,
			Message:   "Sub-zone not found",
			Timestamp: time.Now(),
		}, nil
	}

	return &models.CRUDResponse{
		Success:   true,
		Message:   "Sub-zone updated successfully",
		Timestamp: time.Now(),
	}, nil
}

// DeleteSubZone deletes a sub-zone
func (s *CRUDService) DeleteSubZone(ctx context.Context, regionName, zoneName, subZoneName string) (*models.CRUDResponse, error) {
	s.logger.Info("Deleting sub-zone",
		zap.String("region", regionName),
		zap.String("zone", zoneName),
		zap.String("subzone", subZoneName))

	update := bson.M{
		"$pull": bson.M{
			"zones.$[zone].sub_zones": bson.M{"name": subZoneName},
		},
		"$set": bson.M{
			"zones.$[zone].updated_at": time.Now(),
			"updated_at":               time.Now(),
		},
	}

	arrayFilters := options.ArrayFilters{
		Filters: []interface{}{
			bson.M{"zone.name": zoneName},
		},
	}

	opts := options.Update().SetArrayFilters(arrayFilters)
	filter := bson.M{"name": regionName}
	result, err := s.collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return nil, err
	}

	if result.MatchedCount == 0 {
		return &models.CRUDResponse{
			Success:   false,
			Message:   "Sub-zone not found",
			Timestamp: time.Now(),
		}, nil
	}

	return &models.CRUDResponse{
		Success:   true,
		Message:   "Sub-zone deleted successfully",
		Timestamp: time.Now(),
	}, nil
}
