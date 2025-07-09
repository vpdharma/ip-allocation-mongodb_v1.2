package utils

import (
	"fmt"
	"math/big"
	"net"
)

// ValidateCIDRHierarchy validates that child CIDRs are within parent CIDR
// ENHANCED: Now checks both first and last IP addresses
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

	// ENHANCED: Check both first and last IP in child CIDR
	if !ValidateIPRangeInCIDR(childNet, parentNet) {
		return fmt.Errorf("child CIDR %s is not entirely within parent CIDR %s", childCIDR, parentCIDR)
	}

	// Check if child network size is smaller than or equal to parent
	parentSize, _ := parentNet.Mask.Size()
	childSize, _ := childNet.Mask.Size()

	if childSize < parentSize {
		return fmt.Errorf("child CIDR %s cannot be larger than parent CIDR %s", childCIDR, parentCIDR)
	}

	return nil
}

// ValidateIPRangeInCIDR checks if entire IP range of childNet is within parentNet
func ValidateIPRangeInCIDR(childNet, parentNet *net.IPNet) bool {
	// Get first IP (network address) of child
	firstIP := childNet.IP

	// Get last IP (broadcast address) of child
	lastIP := getLastIPInNetwork(childNet)

	// Check if both first and last IPs are within parent network
	return parentNet.Contains(firstIP) && parentNet.Contains(lastIP)
}

// getLastIPInNetwork calculates the last IP address in a network
func getLastIPInNetwork(network *net.IPNet) net.IP {
	// Get network address
	ip := network.IP
	mask := network.Mask

	// Create a copy of the IP
	lastIP := make(net.IP, len(ip))
	copy(lastIP, ip)

	// Apply inverse mask to get broadcast address
	for i := 0; i < len(mask); i++ {
		lastIP[i] |= ^mask[i]
	}

	return lastIP
}

// ValidateIPRangeInCIDRString validates if a range of IPs is within CIDR
func ValidateIPRangeInCIDRString(startIP, endIP, cidr string) error {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR: %v", err)
	}

	start := net.ParseIP(startIP)
	if start == nil {
		return fmt.Errorf("invalid start IP: %s", startIP)
	}

	end := net.ParseIP(endIP)
	if end == nil {
		return fmt.Errorf("invalid end IP: %s", endIP)
	}

	// Check if both start and end IPs are in the network
	if !network.Contains(start) {
		return fmt.Errorf("start IP %s is not in CIDR %s", startIP, cidr)
	}

	if !network.Contains(end) {
		return fmt.Errorf("end IP %s is not in CIDR %s", endIP, cidr)
	}

	// Validate that start <= end
	if !isIPLessOrEqual(start, end) {
		return fmt.Errorf("start IP %s must be less than or equal to end IP %s", startIP, endIP)
	}

	return nil
}

// isIPLessOrEqual checks if ip1 <= ip2
func isIPLessOrEqual(ip1, ip2 net.IP) bool {
	// Convert to big integers for comparison
	ip1Int := new(big.Int).SetBytes(ip1)
	ip2Int := new(big.Int).SetBytes(ip2)

	return ip1Int.Cmp(ip2Int) <= 0
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

	// Check if either network contains the other's network address or broadcast address
	net1Last := getLastIPInNetwork(net1)
	net2Last := getLastIPInNetwork(net2)

	// Networks overlap if any of these conditions are true:
	// 1. net1 contains net2's first or last IP
	// 2. net2 contains net1's first or last IP
	return net1.Contains(net2.IP) || net1.Contains(net2Last) ||
		net2.Contains(net1.IP) || net2.Contains(net1Last), nil
}

// ValidateMultipleCIDRRanges validates multiple IP ranges against a CIDR
func ValidateMultipleCIDRRanges(ipRanges []string, cidr string) error {
	for i := 0; i < len(ipRanges); i += 2 {
		if i+1 >= len(ipRanges) {
			// Single IP, validate it's in CIDR
			if err := ValidateIPRangeInCIDRString(ipRanges[i], ipRanges[i], cidr); err != nil {
				return err
			}
		} else {
			// IP range, validate both start and end
			if err := ValidateIPRangeInCIDRString(ipRanges[i], ipRanges[i+1], cidr); err != nil {
				return err
			}
		}
	}
	return nil
}

// ValidateZoneCIDRHierarchy validates zone CIDR against region CIDR
func ValidateZoneCIDRHierarchy(regionIPv4, regionIPv6, zoneIPv4, zoneIPv6 string) error {
	// Validate IPv4 hierarchy
	if regionIPv4 != "" && zoneIPv4 != "" {
		if err := ValidateCIDRHierarchy(regionIPv4, zoneIPv4); err != nil {
			return fmt.Errorf("IPv4 zone CIDR validation failed: %v", err)
		}
	}

	// Validate IPv6 hierarchy
	if regionIPv6 != "" && zoneIPv6 != "" {
		if err := ValidateCIDRHierarchy(regionIPv6, zoneIPv6); err != nil {
			return fmt.Errorf("IPv6 zone CIDR validation failed: %v", err)
		}
	}

	return nil
}

// ValidateSubZoneCIDRHierarchy validates sub-zone CIDR against zone CIDR
func ValidateSubZoneCIDRHierarchy(zoneIPv4, zoneIPv6, subZoneIPv4, subZoneIPv6 string) error {
	// Validate IPv4 hierarchy
	if zoneIPv4 != "" && subZoneIPv4 != "" {
		if err := ValidateCIDRHierarchy(zoneIPv4, subZoneIPv4); err != nil {
			return fmt.Errorf("IPv4 sub-zone CIDR validation failed: %v", err)
		}
	}

	// Validate IPv6 hierarchy
	if zoneIPv6 != "" && subZoneIPv6 != "" {
		if err := ValidateCIDRHierarchy(zoneIPv6, subZoneIPv6); err != nil {
			return fmt.Errorf("IPv6 sub-zone CIDR validation failed: %v", err)
		}
	}

	return nil
}
