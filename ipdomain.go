package mailcop

import (
	"net"
	"strings"
)

// isIPDomain checks if a domain is an IP address
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
