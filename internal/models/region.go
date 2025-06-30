package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SubZone struct {
	ID            primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name          string             `bson:"name" json:"name" validate:"required"`
	IPv4CIDR      string             `bson:"ipv4_cidr" json:"ipv4_cidr" validate:"required,cidr"`
	IPv6CIDR      string             `bson:"ipv6_cidr" json:"ipv6_cidr" validate:"required,cidr"`
	AllocatedIPv4 []string           `bson:"allocated_ipv4" json:"allocated_ipv4"`
	AllocatedIPv6 []string           `bson:"allocated_ipv6" json:"allocated_ipv6"`
	ReservedIPv4  []string           `bson:"reserved_ipv4" json:"reserved_ipv4"`
	ReservedIPv6  []string           `bson:"reserved_ipv6" json:"reserved_ipv6"`
	CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt     time.Time          `bson:"updated_at" json:"updated_at"`
}

type Zone struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name      string             `bson:"name" json:"name" validate:"required"`
	SubZones  []SubZone          `bson:"sub_zones" json:"sub_zones"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

type Region struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name      string             `bson:"name" json:"name" validate:"required"`
	IPv4CIDR  string             `bson:"ipv4_cidr,omitempty" json:"ipv4_cidr,omitempty" validate:"omitempty,cidr"`
	IPv6CIDR  string             `bson:"ipv6_cidr,omitempty" json:"ipv6_cidr,omitempty" validate:"omitempty,cidr"`
	Zones     []Zone             `bson:"zones" json:"zones"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

// Collection names
const (
	RegionCollection = "regions"
)
