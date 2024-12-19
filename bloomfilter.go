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

	// VerificationAttempts reduces false positives exponentially by checking
	// the domain multiple times with different hash functions. Each check must
	// return "probably in set" for the domain to be considered disposable.
	// The actual false positive rate becomes FalsePositiveRate^VerificationAttempts.
	//
	// Examples with FalsePositiveRate = 0.01 (1%):
	// - 1 attempt: 1% false positives (0.01^1)
	// - 2 attempts: 0.01% false positives (0.01^2)
	// - 3 attempts: 0.0001% false positives (0.01^3)
	//
	// Higher values provide better accuracy at the cost of more CPU time.
	// Default is 1.
	VerificationAttempts int
}

// DefaultBloomOptions returns sensible defaults
func DefaultBloomOptions() BloomOptions {
	return BloomOptions{
		FalsePositiveRate:    0.001, // 0.1% false positive rate
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

	// Clear the existing map
	v.disposableDomains = make(map[string]struct{})

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
