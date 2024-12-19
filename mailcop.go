package mailcop

import (
	"fmt"
	"net/mail"
	"strings"
	"sync"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
)

// Options contains configuration options for email validation
type Options struct {
	CheckDNS           bool          // Whether to perform DNS MX lookup
	CheckDisposable    bool          // Whether to check for disposable domains
	CheckFreeProvider  bool          // Whether to check for free email providers
	DNSCacheTTL        time.Duration // TTL for DNS cache
	DNSCacheSize       int           // Maximum number of DNS cache entries
	DNSTimeout         time.Duration // Timeout for DNS lookups
	DisposableListURL  string        // URL for disposable domains list
	FreeProvidersURL   string        // URL for free email providers list
	MaxEmailLength     int           // Maximum email length
	MinDomainLength    int           // Minimum domain length
	RejectDisposable   bool          // Whether to invalidate disposable domains
	RejectFreeProvider bool          // Whether to invalidate free email providers
	RejectIPDomains    bool          // Whether to reject IP address domains
	RejectNamedEmails  bool          // Whether to reject named email addresses (e.g. "First Last <first.last@example.com>")
	RejectReserved     bool          // Whether to invalidate reserved example domains
}

// DefaultOptions returns the default validator options
func DefaultOptions() Options {
	return Options{
		CheckDNS:           false,
		CheckDisposable:    false,
		CheckFreeProvider:  false,
		DNSCacheTTL:        1 * time.Hour,
		DNSCacheSize:       1000,
		DNSTimeout:         3 * time.Second,
		DisposableListURL:  "https://disposable.github.io/disposable-email-domains/domains.json",
		FreeProvidersURL:   "",
		MaxEmailLength:     254,
		MinDomainLength:    1,
		RejectDisposable:   false,
		RejectFreeProvider: false,
		RejectIPDomains:    false,
		RejectNamedEmails:  false,
		RejectReserved:     false,
	}
}

// DefaultFreeProviders returns the default free email providers
func DefaultFreeProviders() map[string]struct{} {
	return map[string]struct{}{
		"gmail.com":   {},
		"yahoo.com":   {},
		"hotmail.com": {},
		"outlook.com": {},
		"aol.com":     {},
	}
}

type ValidationResult struct {
	Address        string        // Normalized email address
	IsDisposable   bool          // Whether the domain is disposable
	IsFreeProvider bool          // Whether the domain is a free provider
	IsIPDomain     bool          // Whether the domain is an IP address
	IsReserved     bool          // Whether the domain is reserved
	IsValid        bool          // Whether the email is valid
	LastError      error         // Validation error
	Name           string        // Parsed name from email
	Original       string        // Original email address input
	ValidationTime time.Duration // Time taken to validate
}

// ErrorMessage returns the last validation error as a string if present, otherwise an empty string
func (vr ValidationResult) ErrorMessage() string {
	if vr.LastError != nil {
		return vr.LastError.Error()
	}
	return ""
}

type Validator struct {
	options           Options              // Validator options
	disposableDomains map[string]struct{}  // Disposable domains
	bloomFilter       *bloom.BloomFilter   // Bloom filter for disposable domains (optional)
	bloomOptions      BloomOptions         // Bloom filter options
	freeProviders     map[string]struct{}  // Free email providers
	dnsCache          map[string]dnsResult // LRUCache for DNS lookups
	mu                sync.RWMutex
}

func New(options Options) (*Validator, error) {
	options = mergeWithDefaults(options)

	v := &Validator{
		options:           options,
		disposableDomains: make(map[string]struct{}),
		freeProviders:     DefaultFreeProviders(),
		dnsCache:          make(map[string]dnsResult),
	}

	// Load disposable domains if enabled
	if options.CheckDisposable {
		if err := v.LoadDisposableDomains(options.DisposableListURL); err != nil {
			return nil, fmt.Errorf("failed to load disposable domains: %v", err)
		}
	}

	// Load free email providers if enabled
	if options.CheckFreeProvider {
		if err := v.LoadFreeProviders(options.FreeProvidersURL); err != nil {
			return nil, fmt.Errorf("failed to load free email providers: %v", err)
		}
	}

	return v, nil
}

// mergeWithDefaults takes user options and fills in any zero values with defaults
func mergeWithDefaults(opts Options) Options {
	defaults := DefaultOptions()

	// Only override non-zero/non-default values
	if opts.DNSCacheTTL == 0 {
		opts.DNSCacheTTL = defaults.DNSCacheTTL
	}
	if opts.DNSCacheSize == 0 {
		opts.DNSCacheSize = defaults.DNSCacheSize
	}
	if opts.DNSTimeout == 0 {
		opts.DNSTimeout = defaults.DNSTimeout
	}
	if opts.MaxEmailLength == 0 {
		opts.MaxEmailLength = defaults.MaxEmailLength
	}
	if opts.MinDomainLength == 0 {
		opts.MinDomainLength = defaults.MinDomainLength
	}
	if opts.DisposableListURL == "" {
		opts.DisposableListURL = defaults.DisposableListURL
	}
	if opts.FreeProvidersURL == "" {
		opts.FreeProvidersURL = defaults.FreeProvidersURL
	}

	// Boolean flags don't need special handling as they'll have their zero value (false)
	// unless explicitly set

	return opts
}

// Validate checks a single email address
func (v *Validator) Validate(email string) ValidationResult {
	start := time.Now()
	result := ValidationResult{Original: email}

	// Quick length check before more expensive operations
	if len(email) > v.options.MaxEmailLength {
		result.LastError = fmt.Errorf("email exceeds maximum length of %d characters", v.options.MaxEmailLength)
		result.ValidationTime = time.Since(start)
		return result
	}

	// Parse email address including name component
	addr, err := mail.ParseAddress(email)
	if err != nil {
		result.LastError = fmt.Errorf("invalid email format: %v", err)
		result.ValidationTime = time.Since(start)
		return result
	}

	// Store both name and address components
	result.Name = addr.Name
	result.Address = addr.Address

	if v.options.RejectNamedEmails {
		if result.Address != email {
			result.LastError = fmt.Errorf("named email addresses are not allowed")
			result.ValidationTime = time.Since(start)
			return result
		}
	}

	parts := strings.Split(addr.Address, "@")
	domain := parts[1]

	// Check for minimum domain length
	if len(domain) < v.options.MinDomainLength {
		result.LastError = fmt.Errorf("domain must be at least %d characters", v.options.MinDomainLength)
		result.ValidationTime = time.Since(start)
		return result
	}

	// Check for IP address domains
	if v.isIPDomain(domain) {
		result.IsIPDomain = true
		if v.options.RejectIPDomains {
			result.LastError = fmt.Errorf("IP address domains are not allowed")
			result.ValidationTime = time.Since(start)
			return result
		}
	}

	// Check if domain is reserved
	if v.isReserved(domain) {
		result.IsReserved = true
		if v.options.RejectReserved {
			result.LastError = fmt.Errorf("reserved domain: %s", domain)
			result.ValidationTime = time.Since(start)
			return result
		}
	}

	// Check if domain is disposable
	if v.isDisposable(domain) {
		result.IsDisposable = true
		if v.options.RejectDisposable {
			result.LastError = fmt.Errorf("disposable domain: %s", domain)
			result.ValidationTime = time.Since(start)
			return result
		}
	}

	if v.isFreeProvider(domain) {
		result.IsFreeProvider = true
		if v.options.RejectFreeProvider {
			result.LastError = fmt.Errorf("free email provider: %s", domain)
			result.ValidationTime = time.Since(start)
			return result
		}
	}

	if err := v.validateMX(domain); err != nil {
		result.LastError = fmt.Errorf("invalid domain: %v", err)
		result.ValidationTime = time.Since(start)
		return result
	}

	result.IsValid = true
	result.ValidationTime = time.Since(start)
	return result
}

// ValidateMany validates multiple email addresses concurrently
func (v *Validator) ValidateMany(emails []string) []ValidationResult {
	if len(emails) == 0 {
		return nil
	}

	resultChan := make(chan ValidationResult, len(emails))
	var wg sync.WaitGroup

	for _, email := range emails {
		wg.Add(1)
		go func(e string) {
			defer wg.Done()
			resultChan <- v.Validate(e)
		}(email)
	}

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	results := make([]ValidationResult, 0, len(emails))
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}
