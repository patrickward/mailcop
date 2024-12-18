package mailcop

import (
	"fmt"
	"net"
	"strings"
)

// isIPDomain checks if the domain is an IP address
func (v *Validator) isIPDomainOLD(domain string) bool {
	// Remove brackets if present
	domain = strings.TrimPrefix(domain, "[")
	domain = strings.TrimSuffix(domain, "]")

	// Remove IPv6: prefix if present
	domain = strings.TrimPrefix(domain, "IPv6:")

	// Check if domain is an IPv4 address
	ip := net.ParseIP(domain)
	fmt.Println("IP: ", ip)
	if ip != nil && ip.To4() != nil {
		return true
	}

	// Check if domain is an IPv6 address
	if ip != nil && ip.To4() != nil {
		return true
	}

	return false
}

func (v *Validator) isIPDomain(domain string) bool {
	// Only handle bracketed IP addresses
	if strings.HasPrefix(domain, "[") && strings.HasSuffix(domain, "]") {
		// Remove brackets
		ipStr := domain[1 : len(domain)-1]
		// Handle IPv6 format with prefix
		ipStr = strings.TrimPrefix(ipStr, "IPv6:")

		if ip := net.ParseIP(ipStr); ip != nil {
			return true // Valid IPv4 or IPv6
		}
	}
	return false
}
