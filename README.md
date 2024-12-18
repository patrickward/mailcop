# mailcop

A Go package for validating email addresses according to RFC 5322 and additional rules. 
It uses the Go standard library for syntax validation and provides options for checking DNS records,
disposable email domains, free email providers, and reserved domains.

## Features

- RFC 5322 email syntax validation using the Go standard library
- Recognizes IANA reserved domains (example.com, .test TLD, etc.)
- Identifies disposable email domains (optional)
- Detects free email providers (optional)
- Optional DNS MX record validation (optional, requires network access)
- Concurrent validation for multiple addresses

## Installation

```bash
go get github.com/patrickward/mailcop
```

## Usage

Basic validation:
```go
validator, err := mailcop.New(mailcop.DefaultOptions())
if err != nil {
    log.Fatal(err)
}

result := validator.Validate("user@example.com")
if !result.IsValid {
    log.Printf("Invalid email: %v", result.Error)
}
```

Custom validation options:
```go
opts := mailcop.Options{
    CheckDNS:           true,
    CheckDisposable:    false,
    CheckFreeProvider:  false,
    DNSTimeout:         3 * time.Second,
    DisposableListURL:  "https://disposable.github.io/disposable-email-domains/domains.json",
    FreeProvidersURL:   "",
    MaxEmailLength:     254,
    MinDomainLength:    1,
    RejectDisposable:   true,
    RejectFreeProvider: true,
    RejectIPDomains:    true,
    RejectReserved:     true,
}

validator, err := mailcop.New(opts)
if err != nil {
    log.Fatal(err)
}
```

Validate multiple addresses concurrently:
```go
emails := []string{
    "user1@example.com",
    "user2@gmail.com",
    "invalid@",
}

results := validator.ValidateMany(emails)
for _, result := range results {
    fmt.Printf("Email: %s, Valid: %v\n", result.Original, result.IsValid)
}
```

## Validation Result

The `ValidationResult` struct provides detailed information about the validation:
```go
type ValidationResult struct {
    Name           string        // Parsed name from email
    Address        string        // Normalized email address
    Original       string        // Original email address input
    IsValid        bool          // Whether the email is valid
    IsDisposable   bool          // Whether the domain is disposable
    IsFreeProvider bool          // Whether the domain is a free provider
    IsReserved     bool          // Whether the domain is reserved
    ValidationTime time.Duration // Time taken to validate
    Error          error         // Validation error
}
```

## Free Email Providers

The `CheckFreeProvider` option can be used to detect free email providers. Disabled by default.

You can provide a list of free email domains by setting the `FreeProvidersURL` option to a URL that returns a JSON array of strings:
```json
[
    "gmail.com",
    "yahoo.com",
    "hotmail.com"
]
```

You can add providers via the `RegisterFreeProviders` method:
```go
    validator.RegisterFreeProviders([]string{"gmail.com", "yahoo.com"})
``` 

## Disposable Email Domains

The `CheckDisposable` option can be used to detect disposable email domains. Disabled by default. 

You can provide a list of disposable domains by setting the `DisposableListURL` option to a URL that returns a JSON array of strings:
```json
[
    "mailinator.com",
    "guerrillamail.com",
    "10minutemail.com"
]
```

You can add domains via the `RegisterDisposableDomains` method:
```go
    validator.RegisterDisposableDomains([]string{"mailinator.com", "guerrillamail.com"})
```
