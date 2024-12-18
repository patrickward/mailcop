package mailcop

import (
	"fmt"
	"net"
	"net/mail"
	"strings"
	"sync"
	"time"
)

// Options contains configuration options for email validation
type Options struct {
	CheckDNS           bool          // Whether to perform DNS MX lookup
	CheckDisposable    bool          // Whether to check for disposable domains
	CheckFreeProvider  bool          // Whether to check for free email providers
	DNSTimeout         time.Duration // Timeout for DNS lookups
	DisposableListURL  string        // URL for disposable domains list
	FreeProvidersURL   string        // URL for free email providers list
	MaxEmailLength     int           // Maximum email length
	MinDomainLength    int           // Minimum domain length
	RejectDisposable   bool          // Whether to invalidate disposable domains
	RejectFreeProvider bool          // Whether to invalidate free email providers
	RejectIPDomains    bool          // Whether to reject IP address domains
	RejectReserved     bool          // Whether to invalidate reserved example domains
}

// DefaultOptions returns the default validator options
func DefaultOptions() Options {
	return Options{
		CheckDNS:           true,
		CheckDisposable:    false,
		CheckFreeProvider:  false,
		DNSTimeout:         3 * time.Second,
		DisposableListURL:  "https://disposable.github.io/disposable-email-domains/domains.json",
		FreeProvidersURL:   "",
		MaxEmailLength:     254,
		MinDomainLength:    1,
		RejectDisposable:   false,
		RejectFreeProvider: false,
		RejectIPDomains:    true,
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
	Error          error         // Validation error
	IsDisposable   bool          // Whether the domain is disposable
	IsFreeProvider bool          // Whether the domain is a free provider
	IsIPDomain     bool          // Whether the domain is an IP address
	IsReserved     bool          // Whether the domain is reserved
	IsValid        bool          // Whether the email is valid
	Name           string        // Parsed name from email
	Original       string        // Original email address input
	ValidationTime time.Duration // Time taken to validate
}

type Validator struct {
	options           Options
	disposableDomains map[string]struct{}
	freeProviders     map[string]struct{}
	mu                sync.RWMutex
}

func New(options Options) (*Validator, error) {
	v := &Validator{
		options:           options,
		disposableDomains: make(map[string]struct{}),
		freeProviders:     DefaultFreeProviders(),
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

// validateDomain validates the given domain by performing a DNS MX lookup
func (v *Validator) validateDomain(domain string) error {
	if !v.options.CheckDNS {
		return nil
	}

	done := make(chan error, 1)
	go func() {
		_, err := net.LookupMX(domain)
		done <- err
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(v.options.DNSTimeout):
		return fmt.Errorf("DNS lookup timeout after %v", v.options.DNSTimeout)
	}
}

// Validate checks a single email address
func (v *Validator) Validate(email string) ValidationResult {
	start := time.Now()
	result := ValidationResult{Original: email}

	// Quick length check before more expensive operations
	if len(email) > v.options.MaxEmailLength {
		result.Error = fmt.Errorf("email exceeds maximum length of %d characters", v.options.MaxEmailLength)
		result.ValidationTime = time.Since(start)
		return result
	}

	// Parse email address including name component
	addr, err := mail.ParseAddress(email)
	if err != nil {
		result.Error = fmt.Errorf("invalid email format: %v", err)
		result.ValidationTime = time.Since(start)
		return result
	}

	// Store both name and address components
	result.Name = addr.Name
	result.Address = addr.Address

	parts := strings.Split(addr.Address, "@")
	domain := parts[1]

	// Check for minimum domain length
	if len(domain) < v.options.MinDomainLength {
		result.Error = fmt.Errorf("domain must be at least %d characters", v.options.MinDomainLength)
		result.ValidationTime = time.Since(start)
		return result
	}

	// Check for IP address domains
	if v.isIPDomain(domain) {
		result.IsIPDomain = true
		if v.options.RejectIPDomains {
			result.Error = fmt.Errorf("IP address domains are not allowed")
			result.ValidationTime = time.Since(start)
			return result
		}
	}

	// Check if domain is reserved
	if v.isReserved(domain) {
		result.IsReserved = true
		if v.options.RejectReserved {
			result.Error = fmt.Errorf("reserved domain: %s", domain)
			result.ValidationTime = time.Since(start)
			return result
		}
	}

	// Check if domain is disposable
	if v.isDisposable(domain) {
		result.IsDisposable = true
		if v.options.RejectDisposable {
			result.Error = fmt.Errorf("disposable domain: %s", domain)
			result.ValidationTime = time.Since(start)
			return result
		}
	}

	if v.isFreeProvider(domain) {
		result.IsFreeProvider = true
		if v.options.RejectFreeProvider {
			result.Error = fmt.Errorf("free email provider: %s", domain)
			result.ValidationTime = time.Since(start)
			return result
		}
	}

	if err := v.validateDomain(domain); err != nil {
		result.Error = fmt.Errorf("invalid domain: %v", err)
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
