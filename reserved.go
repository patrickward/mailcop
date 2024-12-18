package mailcop

import "strings"

var (
	// Reserved full domains (exact matches)
	reservedDomains = []string{
		"example.com",
		"example.net",
		"example.org",
		"example.edu",
		"localhost",
	}

	// Reserved TLDs (with and without dots)
	reservedTLDs = []string{
		"test",
		"example",
		"invalid",
		"localhost",
	}
)

// isReserved checks if a domain is a reserved example domain
func (v *Validator) isReserved(domain string) bool {
	domain = strings.ToLower(domain)

	// Check exact matches first
	for _, reserved := range reservedDomains {
		if domain == reserved {
			return true
		}
	}

	// Check TLD matches (both with and without dots)
	for _, tld := range reservedTLDs {
		if strings.HasSuffix(domain, "."+tld) || domain == tld {
			return true
		}
	}

	return false
}
