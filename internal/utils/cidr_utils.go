package utils

import (
	"fmt"
	"net"
)

// ValidateCIDRHierarchy validates that child CIDRs are within parent CIDR
func ValidateCIDRHierarchy(parentCIDR, childCIDR string) error {
	if parentCIDR == "" || childCIDR == "" {
		return nil // Skip validation if either is empty
	}

	_, parentNet, err := net.ParseCIDR(parentCIDR)
	if err != nil {
		return fmt.Errorf("invalid parent CIDR: %v", err)
	}

	_, childNet, err := net.ParseCIDR(childCIDR)
	if err != nil {
		return fmt.Errorf("invalid child CIDR: %v", err)
	}

	// Check if child network is within parent network
	if !parentNet.Contains(childNet.IP) {
		return fmt.Errorf("child CIDR %s is not within parent CIDR %s", childCIDR, parentCIDR)
	}

	// Check if child network size is smaller than or equal to parent
	parentSize, _ := parentNet.Mask.Size()
	childSize, _ := childNet.Mask.Size()

	if childSize < parentSize {
		return fmt.Errorf("child CIDR %s cannot be larger than parent CIDR %s", childCIDR, parentCIDR)
	}

	return nil
}

// CheckCIDROverlap checks if two CIDR ranges overlap
func CheckCIDROverlap(cidr1, cidr2 string) (bool, error) {
	if cidr1 == "" || cidr2 == "" {
		return false, nil
	}

	_, net1, err := net.ParseCIDR(cidr1)
	if err != nil {
		return false, err
	}

	_, net2, err := net.ParseCIDR(cidr2)
	if err != nil {
		return false, err
	}

	// Check if either network contains the other's network address
	return net1.Contains(net2.IP) || net2.Contains(net1.IP), nil
}
