package mailcop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMXLookup(t *testing.T) {
	tests := []struct {
		name      string
		email     string
		wantValid bool
	}{
		{
			name:      "valid gmail mx",
			email:     "test@gmail.com",
			wantValid: true,
		},
		{
			name:      "valid microsoft mx",
			email:     "test@outlook.com",
			wantValid: true,
		},
		{
			name:      "valid icloud mx",
			email:     "test@icloud.com",
			wantValid: true,
		},
		{
			name:      "invalid domain mx",
			email:     "test@invalid-domain-that-does-not-exist-123.com",
			wantValid: false,
		},
		{
			name:      "domain without mx",
			email:     "test@localhost",
			wantValid: false,
		},
		{
			name:      "ip domain with mx check",
			email:     "test@[192.168.1.1]",
			wantValid: false,
		},
	}

	opts := DefaultOptions()
	opts.CheckDNS = true
	v, err := New(opts)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(tt.email)
			assert.Equal(t, tt.wantValid, result.IsValid)
		})
	}
}

func TestDNSCache(t *testing.T) {
	opts := DefaultOptions()
	opts.CheckDNS = true
	opts.DNSCacheSize = 2
	opts.DNSCacheTTL = 2 * time.Second
	opts.DNSTimeout = 5 * time.Second

	v, err := New(opts)
	require.NoError(t, err)

	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "cache hit",
			test: func(t *testing.T) {
				err := v.validateMX("gmail.com")
				require.NoError(t, err)

				v.mu.RLock()
				initialResult, exists := v.dnsCache["gmail.com"]
				v.mu.RUnlock()
				require.True(t, exists)

				err = v.validateMX("gmail.com")
				require.NoError(t, err)

				v.mu.RLock()
				secondResult, exists := v.dnsCache["gmail.com"]
				v.mu.RUnlock()
				require.True(t, exists)
				assert.Equal(t, initialResult.cachedAt, secondResult.cachedAt,
					"cache entry should not be renewed on hit")
			},
		},
		{
			name: "cache expiration",
			test: func(t *testing.T) {
				err := v.validateMX("microsoft.com")
				require.NoError(t, err)

				v.mu.RLock()
				initialResult, exists := v.dnsCache["microsoft.com"]
				v.mu.RUnlock()
				require.True(t, exists, "entry should be in cache")

				time.Sleep(3 * time.Second)

				err = v.validateMX("microsoft.com")
				require.NoError(t, err)

				v.mu.RLock()
				newResult, exists := v.dnsCache["microsoft.com"]
				v.mu.RUnlock()
				require.True(t, exists, "entry should still be in cache")
				assert.True(t, newResult.cachedAt.After(initialResult.cachedAt),
					"cache entry should have been renewed after expiration")
			},
		},
		{
			name: "cache size limit and LRU",
			test: func(t *testing.T) {
				err := v.validateMX("gmail.com")
				require.NoError(t, err)
				time.Sleep(100 * time.Millisecond)

				err = v.validateMX("microsoft.com")
				require.NoError(t, err)
				time.Sleep(100 * time.Millisecond)

				// Access gmail.com to make it most recently used
				err = v.validateMX("gmail.com")
				require.NoError(t, err)
				time.Sleep(100 * time.Millisecond)

				// Add yahoo.com - should evict microsoft.com (LRU)
				err = v.validateMX("yahoo.com")
				require.NoError(t, err)

				v.mu.RLock()
				_, hasGmail := v.dnsCache["gmail.com"]
				_, hasMicrosoft := v.dnsCache["microsoft.com"]
				_, hasYahoo := v.dnsCache["yahoo.com"]
				cacheSize := len(v.dnsCache)
				v.mu.RUnlock()

				assert.True(t, hasGmail, "gmail.com should still be in cache as MRU")
				assert.False(t, hasMicrosoft, "microsoft.com should have been evicted as LRU")
				assert.True(t, hasYahoo, "yahoo.com should be in cache as newest entry")
				assert.Equal(t, 2, cacheSize, "cache should maintain size limit")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t)
		})
	}
}
