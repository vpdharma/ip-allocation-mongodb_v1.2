package utils

import (
	"fmt"
	"math/big"
	"net"
)

// ParseCIDR parses a CIDR string and returns network details
func ParseCIDR(cidr string) (*net.IPNet, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %v", err)
	}
	return ipnet, nil
}

// GetFirstIP returns the first usable IP address in a CIDR range
func GetFirstIP(cidr string) (net.IP, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	// For IPv4, skip network address
	// For IPv6, first address is usually usable
	ip := ipnet.IP
	if IsIPv4(ip) {
		ip = incrementIP(ip)
	}

	return ip, nil
}

// GetLastIP returns the last usable IP address in a CIDR range
func GetLastIP(cidr string) (net.IP, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	// Calculate broadcast address
	ip := ipnet.IP
	mask := ipnet.Mask

	// Create broadcast address
	broadcast := make(net.IP, len(ip))
	for i := range ip {
		broadcast[i] = ip[i] | ^mask[i]
	}

	// For IPv4, skip broadcast address
	if IsIPv4(broadcast) {
		broadcast = decrementIP(broadcast)
	}

	return broadcast, nil
}

// IsIPInCIDR checks if an IP address is within a CIDR range
func IsIPInCIDR(ip, cidr string) (bool, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false, fmt.Errorf("invalid IP address: %s", ip)
	}

	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return false, fmt.Errorf("invalid CIDR: %v", err)
	}

	return ipnet.Contains(parsedIP), nil
}

// GetNextAvailableIP finds the next available IP in a CIDR range
func GetNextAvailableIP(cidr string, allocatedIPs, reservedIPs []string) (string, error) {
	firstIP, err := GetFirstIP(cidr)
	if err != nil {
		return "", err
	}

	lastIP, err := GetLastIP(cidr)
	if err != nil {
		return "", err
	}

	// Create a map for quick lookup of used IPs
	usedIPs := make(map[string]bool)
	for _, ip := range allocatedIPs {
		usedIPs[ip] = true
	}
	for _, ip := range reservedIPs {
		usedIPs[ip] = true
	}

	// Iterate through the range to find first available IP
	currentIP := make(net.IP, len(firstIP))
	copy(currentIP, firstIP)

	for {
		if compareIPs(currentIP, lastIP) > 0 {
			return "", fmt.Errorf("no available IP addresses in range")
		}

		ipStr := currentIP.String()
		if !usedIPs[ipStr] {
			return ipStr, nil
		}

		currentIP = incrementIP(currentIP)
	}
}

// IsIPv4 checks if an IP address is IPv4
func IsIPv4(ip net.IP) bool {
	return ip.To4() != nil
}

// IsIPv6 checks if an IP address is IPv6
func IsIPv6(ip net.IP) bool {
	return ip.To4() == nil && ip.To16() != nil
}

// ValidateIPVersion validates IP version string
func ValidateIPVersion(version string) bool {
	return version == "ipv4" || version == "ipv6" || version == "both"
}

// incrementIP increments an IP address by 1
func incrementIP(ip net.IP) net.IP {
	// Make a copy to avoid modifying the original
	result := make(net.IP, len(ip))
	copy(result, ip)

	// Increment from the last byte
	for i := len(result) - 1; i >= 0; i-- {
		result[i]++
		if result[i] != 0 {
			break
		}
	}

	return result
}

// decrementIP decrements an IP address by 1
func decrementIP(ip net.IP) net.IP {
	// Make a copy to avoid modifying the original
	result := make(net.IP, len(ip))
	copy(result, ip)

	// Decrement from the last byte
	for i := len(result) - 1; i >= 0; i-- {
		if result[i] != 0 {
			result[i]--
			break
		}
		result[i] = 255
	}

	return result
}

// compareIPs compares two IP addresses
// Returns: -1 if ip1 < ip2, 0 if ip1 == ip2, 1 if ip1 > ip2
func compareIPs(ip1, ip2 net.IP) int {
	// Convert IPs to big integers for comparison
	bigInt1 := new(big.Int)
	bigInt2 := new(big.Int)

	bigInt1.SetBytes(ip1)
	bigInt2.SetBytes(ip2)

	return bigInt1.Cmp(bigInt2)
}

// CountIPsInCIDR returns the number of usable IP addresses in a CIDR range
func CountIPsInCIDR(cidr string) (*big.Int, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	ones, bits := ipnet.Mask.Size()
	hostBits := bits - ones

	// Calculate 2^hostBits
	count := new(big.Int)
	count.Exp(big.NewInt(2), big.NewInt(int64(hostBits)), nil)

	// For IPv4, subtract network and broadcast addresses
	if IsIPv4(ipnet.IP) && hostBits > 1 {
		count.Sub(count, big.NewInt(2))
	}

	return count, nil
}

// SplitIPsByVersion separates IPv4 and IPv6 addresses from a slice
func SplitIPsByVersion(ips []string) (ipv4s []string, ipv6s []string, err error) {
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return nil, nil, fmt.Errorf("invalid IP address: %s", ipStr)
		}

		if IsIPv4(ip) {
			ipv4s = append(ipv4s, ipStr)
		} else {
			ipv6s = append(ipv6s, ipStr)
		}
	}

	return ipv4s, ipv6s, nil
}

// NormalizeIP normalizes an IP address string
func NormalizeIP(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ipStr
	}
	return ip.String()
}

// GetCIDRVersion returns the IP version of a CIDR block
func GetCIDRVersion(cidr string) (string, error) {
	ip, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", err
	}

	if IsIPv4(ip) {
		return "ipv4", nil
	}
	return "ipv6", nil
}
