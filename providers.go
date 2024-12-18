package mailcop

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

// RegisterFreeProviders manually adds domains to the free providers list
func (v *Validator) RegisterFreeProviders(providers []string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, provider := range providers {
		v.freeProviders[provider] = struct{}{}
	}
}

// RegisterDisposableDomains manually adds domains to the disposable domains list
func (v *Validator) RegisterDisposableDomains(domains []string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	for _, domain := range domains {
		v.disposableDomains[domain] = struct{}{}
	}
}

// LoadDisposableDomains loads a list of disposable domains from a JSON file or URL
func (v *Validator) LoadDisposableDomains(urlStr string) error {
	if !v.options.CheckDisposable || urlStr == "" {
		return nil
	}

	providers, err := v.loadProviderList(urlStr)
	if err != nil {
		return fmt.Errorf("failed to load disposable domains: %v", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	for _, provider := range providers {
		v.disposableDomains[provider] = struct{}{}
	}

	return nil
}

// LoadFreeProviders loads a list of free email providers from a JSON file or URL
func (v *Validator) LoadFreeProviders(urlStr string) error {
	if !v.options.CheckFreeProvider || urlStr == "" {
		return nil
	}

	providers, err := v.loadProviderList(urlStr)
	if err != nil {
		return fmt.Errorf("failed to load free providers: %v", err)
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	for _, provider := range providers {
		v.freeProviders[provider] = struct{}{}
	}

	return nil
}

// loadProviderList loads a list of email providers from a JSON file or URL
func (v *Validator) loadProviderList(urlStr string) ([]string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %v", err)
	}

	var data []byte
	if parsedURL.Scheme == "file" {
		// Load from file
		data, err = os.ReadFile(strings.TrimPrefix(urlStr, "file://"))
		if err != nil {
			return nil, fmt.Errorf("failed to read file: %v", err)
		}
	} else {
		// Load from URL
		resp, err := http.Get(urlStr)
		if err != nil {
			return nil, err
		}
		defer func(Body io.ReadCloser) {
			_ = Body.Close()
		}(resp.Body)

		decoder := json.NewDecoder(resp.Body)
		var providers []string
		if err := decoder.Decode(&providers); err != nil {
			return nil, err
		}
		return providers, nil
	}

	var providers []string
	if err := json.Unmarshal(data, &providers); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}

	return providers, nil
}

// isDisposable checks if the given domain is in the disposable domains list
func (v *Validator) isDisposable(domain string) bool {
	if !v.options.CheckDisposable {
		return false
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	_, exists := v.disposableDomains[domain]
	return exists
}

// Add helper method for free provider detection
func (v *Validator) isFreeProvider(domain string) bool {
	if !v.options.CheckFreeProvider {
		return false
	}

	v.mu.RLock()
	defer v.mu.RUnlock()

	_, isFree := v.freeProviders[domain]
	return isFree
}
