# mailcop

A Go package for validating email addresses according to RFC 5322 and additional rules.
It uses the Go standard library for syntax validation and provides options for checking DNS records,
disposable email domains, free email providers, and reserved domains.

[![Go Reference](https://pkg.go.dev/badge/github.com/patrickward/mailcop.svg)](https://pkg.go.dev/github.com/patrickward/mailcop)
[![Go Report Card](https://goreportcard.com/badge/github.com/patrickward/mailcop)](https://goreportcard.com/report/github.com/patrickward/mailcop)

## Table of Contents
- [Features](#features)
- [Installation](#installation)
- [Basic Usage](#basic-usage)
- [Configuration Options](#configuration-options)
- [Validation Results](#validation-results)
- [Advanced Features](#advanced-features)
   - [Free Email Providers](#free-email-providers)
   - [Disposable Email Domains](#disposable-email-domains)
   - [Bloom Filter Support](#bloom-filter-support)
- [Domain Lists](#domain-lists)
- [API Reference](#api-reference)
- [Performance Considerations](#performance-considerations)

## Features

- RFC 5322 email syntax validation using the Go standard library
- Recognizes IANA reserved domains (example.com, .test TLD, etc.)
- Identifies disposable email domains with configurable accuracy
- Detects free email providers
- Optional DNS MX record validation
- Concurrent validation for multiple addresses
- Memory-efficient Bloom filter option for large domain lists

## Installation

```bash
go get github.com/patrickward/mailcop
```

## Basic Usage

```go
// Create validator with default options
validator, err := mailcop.New(mailcop.DefaultOptions())
if err != nil {
    log.Fatal(err)
}

// Validate a single email
result := validator.Validate("user@example.com")
if !result.IsValid {
    log.Printf("Invalid email: %s", result.ErrorMessage())
}

// Validate multiple emails concurrently
emails := []string{
    "user1@example.com",
    "user2@gmail.com",
    "invalid@",
}
results := validator.ValidateMany(emails)
```

## Configuration Options

```go
opts := mailcop.Options{
    CheckDNS:           true,
    CheckDisposable:    true,
    CheckFreeProvider:  true,
    DNSCacheTTL:        1 * time.Hour,
    DNSCacheSize:       1000,
    DNSTimeout:         3 * time.Second,
    DisposableListURL:  "file:///path/to/disposable-domains.json",
    FreeProvidersURL:   "file:///path/to/free-providers.json",
    MaxEmailLength:     254,
    MinDomainLength:    3,
    RejectDisposable:   true,
    RejectFreeProvider: true,
    RejectIPDomains:    true,
    RejectNamedEmails:  true,
    RejectReserved:     true,
}
```

## Validation Results

The `ValidationResult` struct provides detailed information:

```go
type ValidationResult struct {
    Name           string        // Parsed name from email
    Address        string        // Normalized email address
    Original       string        // Original email address input
    IsValid        bool          // Whether the email is valid
    IsDisposable   bool          // Whether the domain is disposable
    IsFreeProvider bool          // Whether the domain is a free provider
    IsReserved     bool          // Whether the domain is reserved
    IsIPDomain     bool          // Whether the domain is an IP address
    ValidationTime time.Duration // Time taken to validate
    LastError      error         // Validation error
}

// Get error message as string
errMsg := result.ErrorMessage() // Returns empty string if no error
```

## Advanced Features

### Free Email Providers

Detection of free email providers (like Gmail, Yahoo, etc.). Multiple methods are available:

```go
// 1. Load from URL or file during initialization
opts := mailcop.DefaultOptions()
opts.CheckFreeProvider = true
opts.RejectFreeProvider = true // Optional: reject free providers
opts.FreeProvidersURL = "file:///path/to/providers.json"
v, err := mailcop.New(opts)

// 2. Load after initialization
err = v.LoadFreeProviders("file:///path/to/providers.json")

// 3. Register providers manually
v.RegisterFreeProviders([]string{
    "gmail.com",
    "yahoo.com",
    "hotmail.com",
})
```

### Disposable Email Domains

Detection of disposable/temporary email domains. Multiple methods are available:

```go
// 1. Load from URL or file during initialization
opts := mailcop.DefaultOptions()
opts.CheckDisposable = true
opts.RejectDisposable = true // Optional: reject disposable domains
opts.DisposableListURL = "file:///path/to/disposable.json"
v, err := mailcop.New(opts)

// 2. Load after initialization
err = v.LoadDisposableDomains("file:///path/to/disposable.json")

// 3. Register domains manually
v.RegisterDisposableDomains([]string{
    "tempmail.com",
    "throwaway.com",
})
```

### Bloom Filter Support

#### What is a Bloom Filter?

A Bloom filter is a space-efficient probabilistic data structure used to test whether an element is a member of a set. It can tell us either:
- "This element is definitely not in the set" (100% accurate)
- "This element is probably in the set" (configurable accuracy)

This makes it perfect for disposable domain checking where:
- We want to be certain about legitimate domains (no false negatives)
- We can tolerate occasionally marking a legitimate domain as disposable (false positives)
- Memory efficiency is important

#### Basic Setup

```go
// Create validator with disposable checking enabled
opts := mailcop.DefaultOptions()
opts.CheckDisposable = true
v, err := mailcop.New(opts)

// Configure and enable Bloom filter
bloomOpts := mailcop.DefaultBloomOptions()
err = v.UseBloomFilter("file:///path/to/domains.json", bloomOpts)
if err != nil {
    log.Fatal(err)
}

// Optional: Save filter to file for later use
f, err := os.Create("filter.bloom")
if err != nil {
    log.Fatal(err)
}
defer f.Close()
err = v.SaveBloomFilter(f)

// Optional: Load existing filter
f, err := os.Open("filter.bloom")
if err != nil {
    log.Fatal(err)
}
defer f.Close()
err = v.LoadBloomFilter(f)
```

#### Configuration Options

```go
type BloomOptions struct {
    // FalsePositiveRate sets the desired false positive rate (0.0 to 1.0)
    // Lower values use more memory but have fewer false positives
    FalsePositiveRate float64

    // TrustedDomains are never marked as disposable
    TrustedDomains map[string]struct{}

    // VerificationAttempts reduces false positives exponentially
    // - 1 check: FalsePositiveRate
    // - 2 checks: FalsePositiveRate^2
    // - 3 checks: FalsePositiveRate^3
    VerificationAttempts int
}
```

#### Configuring False Positives

Balance memory usage and accuracy:

```go
// High accuracy, more memory
bloomOpts := mailcop.DefaultBloomOptions()
bloomOpts.FalsePositiveRate = 0.001     // 0.1%
bloomOpts.VerificationAttempts = 1

// Less memory, multiple verifications
bloomOpts := mailcop.DefaultBloomOptions()
bloomOpts.FalsePositiveRate = 0.01      // 1%
bloomOpts.VerificationAttempts = 2       // Reduces to 0.01%

// With trusted domains
bloomOpts.TrustedDomains = map[string]struct{}{
    "gmail.com": {},
    "outlook.com": {},
}
```

## Domain Lists

### Disposable Email Domains
Several community-maintained lists are available:
- [disposable/disposable-email-domains](https://github.com/disposable/disposable-email-domains) - Comprehensive list updated regularly
- [FGRibreau/mailchecker](https://github.com/FGRibreau/mailchecker) - Multi-language with regular updates
- [martenson/disposable-email-domains](https://github.com/martenson/disposable-email-domains) - Simple list

### Free Email Providers
Some resources for free email provider lists:
- [willwhite/freemail](https://github.com/willwhite/freemail) - Maintained list of free email providers
- [goware/emailproviders](https://github.com/goware/emailproviders) - Go package with provider lists

## API Reference

### Validator Methods

```go
// Create new validator
New(options Options) (*Validator, error)

// Validation
Validate(email string) ValidationResult
ValidateMany(emails []string) []ValidationResult

// Domain Management
LoadDisposableDomains(url string) error
LoadFreeProviders(url string) error
RegisterDisposableDomains(domains []string)
RegisterFreeProviders(providers []string)

// Bloom Filter
UseBloomFilter(url string, opts BloomOptions) error
SaveBloomFilter(w io.Writer) error
LoadBloomFilter(r io.Reader) error
```

### ValidationResult Methods

```go
// Get error message as string
ErrorMessage() string
```

#### Configuration Options

```go
type BloomOptions struct {
    // FalsePositiveRate sets the desired false positive rate (0.0 to 1.0)
    // Lower values use more memory but have fewer false positives
    FalsePositiveRate float64

    // TrustedDomains are never marked as disposable
    TrustedDomains map[string]struct{}

    // VerificationAttempts reduces false positives exponentially
    // - 1 check: FalsePositiveRate
    // - 2 checks: FalsePositiveRate^2
    // - 3 checks: FalsePositiveRate^3
    VerificationAttempts int
}
```

#### Configuring False Positives

Balance memory usage and accuracy:

```go
// High accuracy, more memory
bloomOpts := mailcop.DefaultBloomOptions()
bloomOpts.FalsePositiveRate = 0.001     // 0.1%
bloomOpts.VerificationAttempts = 1

// Less memory, multiple verifications
bloomOpts := mailcop.DefaultBloomOptions()
bloomOpts.FalsePositiveRate = 0.01      // 1%
bloomOpts.VerificationAttempts = 2       // Reduces to 0.01%

// With trusted domains
bloomOpts.TrustedDomains = map[string]struct{}{
    "gmail.com": {},
    "outlook.com": {},
}
```

## Performance Considerations

### Memory Usage

Based on benchmarks with ~240k domains:

| Implementation | Memory | False Positives | Use Case |
|---------------|--------|-----------------|----------|
| Map           | ~5 MB  | None           | Perfect accuracy needed |
| Bloom (0.1%)  | ~300KB | 1 in 1,000     | Memory-constrained systems |
| Bloom (1%)    | ~150KB | 1 in 100       | Extreme memory constraints |

### Lookup Performance

- Map: Constant time O(1)
- Bloom Filter:
   - O(k) where k is VerificationAttempts
   - Additional hash calculations
   - Still microsecond-range performance

### Choosing an Implementation

Use the Bloom filter when:
- Memory is constrained
- You have many domains (>10,000)
- Occasional false positives are acceptable
- You can maintain a trusted domains list

Use the map implementation when:
- Perfect accuracy is required
- Memory isn't constrained
- You have fewer domains (<10,000)
- You need frequent domain updates

### File Format

Both implementations accept JSON files containing an array of domains:

```json
[
    "disposable1.com",
    "disposable2.com",
    "tempmail.org"
]
```

Files can be loaded from local filesystem or URLs:
```go
// Local file
v.UseBloomFilter("file:///path/to/domains.json", bloomOpts)

// Remote URL
v.UseBloomFilter("https://example.com/domains.json", bloomOpts)
```
