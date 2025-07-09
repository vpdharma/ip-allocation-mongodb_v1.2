package utils

import (
	"fmt"
	"math/big"
	"net"
)

// ValidateIPVersion checks if IP version is valid
func ValidateIPVersion(version string) bool {
	return version == "ipv4" || version == "ipv6" || version == "both"
}

// IsIPv4 checks if the IP is IPv4
func IsIPv4(ip net.IP) bool {
	return ip != nil && ip.To4() != nil
}

// IsIPv6 checks if the IP is IPv6
func IsIPv6(ip net.IP) bool {
	return ip != nil && ip.To4() == nil && ip.To16() != nil
}

// NormalizeIP normalizes an IP address string
func NormalizeIP(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}

	if IsIPv4(ip) {
		return ip.To4().String()
	}
	return ip.To16().String()
}

// ParseCIDR parses a CIDR string and returns the network
func ParseCIDR(cidr string) (*net.IPNet, error) {
	_, network, err := net.ParseCIDR(cidr)
	return network, err
}

// IsIPInCIDR checks if an IP address is within a CIDR range
func IsIPInCIDR(ipStr, cidrStr string) (bool, error) {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	_, network, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return false, fmt.Errorf("invalid CIDR: %s", cidrStr)
	}

	return network.Contains(ip), nil
}

// GetNextAvailableIP finds the next available IP in a CIDR range
func GetNextAvailableIP(cidrStr string, allocated, reserved []string) (string, error) {
	_, network, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return "", err
	}

	// Create a map for faster lookup
	usedIPs := make(map[string]bool)
	for _, ip := range allocated {
		usedIPs[ip] = true
	}
	for _, ip := range reserved {
		usedIPs[ip] = true
	}

	// Start from the first usable IP in the network
	ip := network.IP
	for network.Contains(ip) {
		ipStr := ip.String()
		if !usedIPs[ipStr] && !isNetworkOrBroadcast(ip, network) {
			return ipStr, nil
		}
		ip = incrementIP(ip)
	}

	return "", fmt.Errorf("no available IPs in CIDR range %s", cidrStr)
}

// incrementIP increments an IP address by 1
func incrementIP(ip net.IP) net.IP {
	// Make a copy of the IP
	result := make(net.IP, len(ip))
	copy(result, ip)

	// Increment from the rightmost byte
	for i := len(result) - 1; i >= 0; i-- {
		result[i]++
		if result[i] != 0 {
			break
		}
	}

	return result
}

// isNetworkOrBroadcast checks if IP is network or broadcast address
func isNetworkOrBroadcast(ip net.IP, network *net.IPNet) bool {
	// Network address
	if ip.Equal(network.IP) {
		return true
	}

	// For IPv4, check broadcast address
	if IsIPv4(ip) {
		broadcast := make(net.IP, len(network.IP))
		copy(broadcast, network.IP)

		mask := network.Mask
		for i := 0; i < len(mask); i++ {
			broadcast[i] |= ^mask[i]
		}

		return ip.Equal(broadcast)
	}

	return false
}

// CountIPsInCIDR counts the number of usable IPs in a CIDR range
func CountIPsInCIDR(cidrStr string) (*big.Int, error) {
	if cidrStr == "" {
		return big.NewInt(0), nil
	}

	_, network, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return nil, err
	}

	ones, bits := network.Mask.Size()
	hostBits := bits - ones

	// Calculate 2^hostBits
	total := new(big.Int).Exp(big.NewInt(2), big.NewInt(int64(hostBits)), nil)

	// For IPv4, subtract network and broadcast addresses
	if IsIPv4(network.IP) && hostBits > 1 {
		total.Sub(total, big.NewInt(2))
	}

	return total, nil
}

// SplitIPsByVersion splits a slice of IP addresses by version
func SplitIPsByVersion(ips []string) ([]string, []string, error) {
	var ipv4s, ipv6s []string

	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return nil, nil, fmt.Errorf("invalid IP address: %s", ipStr)
		}

		if IsIPv4(ip) {
			ipv4s = append(ipv4s, NormalizeIP(ipStr))
		} else if IsIPv6(ip) {
			ipv6s = append(ipv6s, NormalizeIP(ipStr))
		}
	}

	return ipv4s, ipv6s, nil
}

// GetIPRange returns the first and last usable IPs in a CIDR range
func GetIPRange(cidrStr string) (string, string, error) {
	_, network, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return "", "", err
	}

	firstIP := network.IP
	lastIP := make(net.IP, len(network.IP))
	copy(lastIP, network.IP)

	// Calculate last IP in range
	mask := network.Mask
	for i := 0; i < len(mask); i++ {
		lastIP[i] |= ^mask[i]
	}

	// For IPv4, adjust for network and broadcast addresses
	if IsIPv4(network.IP) {
		firstIP = incrementIP(firstIP) // Skip network address
		lastIP = decrementIP(lastIP)   // Skip broadcast address
	}

	return firstIP.String(), lastIP.String(), nil
}

// decrementIP decrements an IP address by 1
func decrementIP(ip net.IP) net.IP {
	result := make(net.IP, len(ip))
	copy(result, ip)

	for i := len(result) - 1; i >= 0; i-- {
		if result[i] > 0 {
			result[i]--
			break
		}
		result[i] = 255
	}

	return result
}

// ValidateIPList validates a list of IP addresses
func ValidateIPList(ips []string) error {
	for _, ipStr := range ips {
		ip := net.ParseIP(ipStr)
		if ip == nil {
			return fmt.Errorf("invalid IP address: %s", ipStr)
		}
	}
	return nil
}

// GetAvailableIPsInRange returns available IPs in a CIDR range
func GetAvailableIPsInRange(cidrStr string, allocated, reserved []string, limit int) ([]string, error) {
	_, network, err := net.ParseCIDR(cidrStr)
	if err != nil {
		return nil, err
	}

	// Create map for faster lookup
	usedIPs := make(map[string]bool)
	for _, ip := range allocated {
		usedIPs[ip] = true
	}
	for _, ip := range reserved {
		usedIPs[ip] = true
	}

	var available []string
	ip := network.IP
	count := 0

	for network.Contains(ip) && count < limit {
		ipStr := ip.String()
		if !usedIPs[ipStr] && !isNetworkOrBroadcast(ip, network) {
			available = append(available, ipStr)
			count++
		}
		ip = incrementIP(ip)
	}

	return available, nil
}
