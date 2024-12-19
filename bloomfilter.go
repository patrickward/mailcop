package mailcop

import (
	"fmt"
	"io"

	"github.com/bits-and-blooms/bloom/v3"
)

// BloomOptions configures the bloom filter behavior
type BloomOptions struct {
	// FalsePositiveRate sets the desired false positive rate (0.0 to 1.0)
	// Lower values use more memory but have fewer false positives
	FalsePositiveRate float64

	// TrustedDomains is an optional set of domains to whitelist
	// These domains will never be considered disposable
	TrustedDomains map[string]struct{}

	// VerificationAttempts is the number of times to check the bloom filter
	// Multiple checks reduce false positives exponentially:
	// - 1 check: FalsePositiveRate chance
	// - 2 checks: FalsePositiveRate^2 chance
	// - 3 checks: FalsePositiveRate^3 chance
	// Default is 1
	VerificationAttempts int
}

// DefaultBloomOptions returns sensible defaults
func DefaultBloomOptions() BloomOptions {
	return BloomOptions{
		FalsePositiveRate:    0.001, // 0.1% false positive rate
		TrustedDomains:       make(map[string]struct{}),
		VerificationAttempts: 1,
	}
}

// UseBloomFilter converts the validator to use a bloom filter instead of a map
// for disposable domain checking. This can significantly reduce memory usage.
// The expectedItems parameter should be set to the approximate number of
// disposable domains you expect to add to the filter.
func (v *Validator) UseBloomFilter(url string, opts BloomOptions) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if url == "" {
		return fmt.Errorf("URL is required")
	}

	// Load the list of disposable domains
	domains, err := v.loadProviderList(url)
	if err != nil {
		return fmt.Errorf("failed to load provider list: %v", err)
	}

	// Create new bloom filter with given parameters
	filter := bloom.NewWithEstimates(uint(len(domains)), opts.FalsePositiveRate)

	// If we have existing domains, add them to the bloom filter
	for domain := range v.disposableDomains {
		filter.Add([]byte(domain))
	}

	// Switch to bloom filter implementation
	v.bloomFilter = filter

	// Clear the existing map and use it for trusted domains
	v.disposableDomains = opts.TrustedDomains
	if v.disposableDomains == nil {
		v.disposableDomains = make(map[string]struct{})
	}

	v.bloomOptions = opts
	return nil
}

// SaveBloomFilter serializes the bloom filter to the provided writer
func (v *Validator) SaveBloomFilter(w io.Writer) error {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if v.bloomFilter == nil {
		return fmt.Errorf("bloom filter not initialized")
	}

	_, err := v.bloomFilter.WriteTo(w)
	return err
}

// LoadBloomFilter deserializes the bloom filter from the provided reader
func (v *Validator) LoadBloomFilter(r io.Reader) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	filter := &bloom.BloomFilter{}
	if _, err := filter.ReadFrom(r); err != nil {
		return err
	}

	v.bloomFilter = filter
	return nil
}
