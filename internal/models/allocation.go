package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Core Allocation Models
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

// Fixed DeallocationRequest with correct field name
type DeallocationRequest struct {
	Region      string   `json:"region" validate:"required"`
	Zone        string   `json:"zone" validate:"required"`
	SubZone     string   `json:"sub_zone" validate:"required"`
	IPAddresses []string `json:"ip_addresses" validate:"required,min=1"`
}

type ReservationRequest struct {
	Region          string   `json:"region" validate:"required"`
	Zone            string   `json:"zone" validate:"required"`
	SubZone         string   `json:"sub_zone" validate:"required"`
	IPAddresses     []string `json:"ip_addresses" validate:"required,min=1"`
	ReservationType string   `json:"reservation_type" validate:"required,oneof=reserve unreserve"`
}

type IPOperationResponse struct {
	Success      bool      `json:"success"`
	ProcessedIPs []string  `json:"processed_ips,omitempty"`
	FailedIPs    []string  `json:"failed_ips,omitempty"`
	Message      string    `json:"message"`
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

// CRUD Models - Keep only these, remove duplicates from other files
type CreateRegionRequest struct {
	Name     string `json:"name" validate:"required"`
	IPv4CIDR string `json:"ipv4_cidr,omitempty" validate:"omitempty,cidr"`
	IPv6CIDR string `json:"ipv6_cidr,omitempty" validate:"omitempty,cidr"`
}

type UpdateRegionRequest struct {
	Name     string `json:"name,omitempty"`
	IPv4CIDR string `json:"ipv4_cidr,omitempty" validate:"omitempty,cidr"`
	IPv6CIDR string `json:"ipv6_cidr,omitempty" validate:"omitempty,cidr"`
}

type CreateZoneRequest struct {
	Name     string `json:"name" validate:"required"`
	IPv4CIDR string `json:"ipv4_cidr,omitempty" validate:"omitempty,cidr"`
	IPv6CIDR string `json:"ipv6_cidr,omitempty" validate:"omitempty,cidr"`
}

type UpdateZoneRequest struct {
	Name     string `json:"name,omitempty"`
	IPv4CIDR string `json:"ipv4_cidr,omitempty" validate:"omitempty,cidr"`
	IPv6CIDR string `json:"ipv6_cidr,omitempty" validate:"omitempty,cidr"`
}

type CreateSubZoneRequest struct {
	Name     string `json:"name" validate:"required"`
	IPv4CIDR string `json:"ipv4_cidr,omitempty" validate:"omitempty,cidr"`
	IPv6CIDR string `json:"ipv6_cidr,omitempty" validate:"omitempty,cidr"`
}

type UpdateSubZoneRequest struct {
	Name     string `json:"name,omitempty"`
	IPv4CIDR string `json:"ipv4_cidr,omitempty" validate:"omitempty,cidr"`
	IPv6CIDR string `json:"ipv6_cidr,omitempty" validate:"omitempty,cidr"`
}

type CRUDResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Message   string      `json:"message"`
	Timestamp time.Time   `json:"timestamp"`
}
