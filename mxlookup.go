package mailcop

import (
	"fmt"
	"net"
	"time"
)

// dnsResult holds the result of a DNS lookup and the time it was cached. Used in the DNS cache.
type dnsResult struct {
	err      error
	cachedAt time.Time
	lastUsed time.Time // Track when this entry was last accessed
}

// validateMX performs a DNS lookup for the MX records of a domain. It caches the result for future lookups.
func (v *Validator) validateMX(domain string) error {
	if !v.options.CheckDNS {
		return nil
	}

	// Try cache first
	v.mu.RLock()
	if result, ok := v.dnsCache[domain]; ok {
		if time.Since(result.cachedAt) < v.options.DNSCacheTTL {
			// Update last used time under write lock
			v.mu.RUnlock()
			v.mu.Lock()
			if result, stillExists := v.dnsCache[domain]; stillExists {
				result.lastUsed = time.Now()
			}
			v.mu.Unlock()
			return result.err
		}
	}
	v.mu.RUnlock()

	// Perform actual lookup with timeout
	done := make(chan error, 1)
	go func() {
		_, err := net.LookupMX(domain)
		done <- err
	}()

	var lookupErr error
	select {
	case err := <-done:
		lookupErr = err
	case <-time.After(v.options.DNSTimeout):
		lookupErr = fmt.Errorf("DNS lookup timeout after %v", v.options.DNSTimeout)
	}

	// Cache the result
	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now()

	// If we're at capacity, remove LRU entry
	if len(v.dnsCache) >= v.options.DNSCacheSize {
		var (
			lruKey     string
			lruTime    time.Time
			firstEntry = true
		)

		// First remove any expired entries
		for domain, entry := range v.dnsCache {
			if now.Sub(entry.cachedAt) >= v.options.DNSCacheTTL {
				delete(v.dnsCache, domain)
				continue
			}
			// Track LRU among non-expired entries
			if firstEntry || entry.lastUsed.Before(lruTime) {
				lruKey = domain
				lruTime = entry.lastUsed
				firstEntry = false
			}
		}

		// If still at capacity, remove LRU entry
		if len(v.dnsCache) >= v.options.DNSCacheSize {
			delete(v.dnsCache, lruKey)
		}
	}

	v.dnsCache[domain] = dnsResult{
		err:      lookupErr,
		cachedAt: now,
		lastUsed: now,
	}

	return lookupErr
}
