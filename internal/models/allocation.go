package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type AllocationRequest struct {
	Region       string   `json:"region" validate:"required"`
	Zone         string   `json:"zone" validate:"required"`
	SubZone      string   `json:"sub_zone" validate:"required"`
	PreferredIPs []string `json:"preferred_ips,omitempty"`
	IPVersion    string   `json:"ip_version" validate:"required,oneof=ipv4 ipv6 both"`
	Count        int      `json:"count" validate:"min=1,max=10"`
}

type AllocationResponse struct {
	Success      bool      `json:"success"`
	AllocatedIPs []string  `json:"allocated_ips,omitempty"`
	Message      string    `json:"message,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

type IPAllocation struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Region    string             `bson:"region" json:"region"`
	Zone      string             `bson:"zone" json:"zone"`
	SubZone   string             `bson:"sub_zone" json:"sub_zone"`
	IPAddress string             `bson:"ip_address" json:"ip_address"`
	IPVersion string             `bson:"ip_version" json:"ip_version"`
	Status    string             `bson:"status" json:"status"` // allocated, reserved, available
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// Collection names
const (
	IPAllocationCollection = "ip_allocations"
)
