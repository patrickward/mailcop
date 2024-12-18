package mailcop_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/patrickward/mailcop"
)

// Helper function to create test data files
func setupTestData(t *testing.T) (string, func()) {
	tmpDir := t.TempDir()

	// Create disposable domains file
	disposableData := `[
        "disposable-example.com",
        "temp-mail.org",
        "throwaway.com"
    ]`
	err := os.WriteFile(filepath.Join(tmpDir, "disposable.json"), []byte(disposableData), 0644)
	require.NoError(t, err)

	// Create free providers file
	freeProvidersData := `[
        "free-example.com",
        "gmail.com",
        "yahoo.com",
        "hotmail.com"
    ]`
	err = os.WriteFile(filepath.Join(tmpDir, "free_providers.json"), []byte(freeProvidersData), 0644)
	require.NoError(t, err)

	return tmpDir, func() {
		// Cleanup is handled automatically by t.TempDir()
	}
}

func TestValidate(t *testing.T) {
	// Create validator with default options
	opts := mailcop.DefaultOptions()
	opts.CheckDNS = false // Disable DNS checks for unit tests
	opts.MinDomainLength = 3

	v, err := mailcop.New(opts)
	require.NoError(t, err)
	require.NotNil(t, v)

	tests := []struct {
		name     string
		email    string
		expected mailcop.ValidationResult
	}{
		{
			name:  "valid email simple",
			email: "user@example.com",
			expected: mailcop.ValidationResult{
				Name:     "",
				Address:  "user@example.com",
				Original: "user@example.com",
				IsValid:  true,
			},
		},
		{
			name:  "valid email with display name",
			email: `"John Doe" <john.doe@example.com>`,
			expected: mailcop.ValidationResult{
				Name:     "John Doe",
				Address:  "john.doe@example.com",
				Original: `"John Doe" <john.doe@example.com>`,
				IsValid:  true,
			},
		},
		{
			name:  "invalid email - no @",
			email: "invalid.email",
			expected: mailcop.ValidationResult{
				Original: "invalid.email",
				IsValid:  false,
				Error:    assert.AnError,
			},
		},
		{
			name:  "invalid email - multiple @",
			email: "user@host@domain.com",
			expected: mailcop.ValidationResult{
				Original: "user@host@domain.com",
				IsValid:  false,
				Error:    assert.AnError,
			},
		},
		{
			name:  "invalid email - domain too short",
			email: "user@ex",
			expected: mailcop.ValidationResult{
				Name:     "",
				Address:  "user@ex",
				Original: "user@ex",
				IsValid:  false,
				Error:    assert.AnError,
			},
		},
		{
			name:  "email exceeding max length",
			email: createLongEmail(300),
			expected: mailcop.ValidationResult{
				Original: createLongEmail(300),
				IsValid:  false,
				Error:    assert.AnError,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(tt.email)

			// Check if error expectation matches
			if tt.expected.Error != nil {
				assert.Error(t, result.Error)
			} else {
				assert.NoError(t, result.Error)
			}

			// Check other fields
			assert.Equal(t, tt.expected.IsValid, result.IsValid)
			assert.Equal(t, tt.expected.Name, result.Name)
			assert.Equal(t, tt.expected.Address, result.Address)
			assert.Equal(t, tt.expected.Original, result.Original)
		})
	}
}

func TestValidatorOptions(t *testing.T) {
	tests := []struct {
		name           string
		options        mailcop.Options
		email          string
		expectError    bool
		expectValid    bool
		isFreeProvider bool
	}{
		{
			name: "with IP domain not allowed",
			options: mailcop.Options{
				CheckDNS:        false,
				MaxEmailLength:  254,
				MinDomainLength: 3,
				RejectIPDomains: true,
			},
			email:       "user@[127.0.0.1]",
			expectError: true,
			expectValid: false,
		},
		{
			name: "with IP domain allowed",
			options: mailcop.Options{
				CheckDNS:        false,
				MaxEmailLength:  254,
				MinDomainLength: 3,
				RejectIPDomains: false,
			},
			email:       "user@[127.0.0.1]",
			expectError: false,
			expectValid: true,
		},
		{
			name: "with IPv6 domain",
			options: mailcop.Options{
				CheckDNS:        false,
				MaxEmailLength:  254,
				MinDomainLength: 3,
				RejectIPDomains: true,
			},
			email:       "user@[::1]",
			expectError: true,
			expectValid: false,
		},
		{
			name: "free provider detection enabled",
			options: mailcop.Options{
				CheckFreeProvider: true,
				CheckDNS:          false,
				MaxEmailLength:    254,
				MinDomainLength:   3,
			},
			email:          "user@gmail.com",
			expectError:    false,
			expectValid:    true,
			isFreeProvider: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := mailcop.New(tt.options)
			require.NoError(t, err)
			require.NotNil(t, v)

			if tt.options.CheckFreeProvider {
				v.RegisterFreeProviders([]string{"gmail.com"})
			}

			result := v.Validate(tt.email)

			if tt.expectError {
				assert.Error(t, result.Error)
			} else {
				assert.NoError(t, result.Error)
			}

			assert.Equal(t, tt.expectValid, result.IsValid)
			assert.Equal(t, tt.isFreeProvider, result.IsFreeProvider)
		})
	}
}

func TestValidateMany(t *testing.T) {
	opts := mailcop.DefaultOptions()
	opts.CheckDNS = false
	opts.MaxEmailLength = 254
	opts.MinDomainLength = 3

	v, err := mailcop.New(opts)
	require.NoError(t, err)
	require.NotNil(t, v)

	emails := []string{
		"valid@example.com",
		"invalid@",
		`"John Doe" <john@example.com>`,
	}

	results := v.ValidateMany(emails)
	assert.Len(t, results, len(emails))

	// Create maps to track expected results
	found := make(map[string]bool)
	validCount := 0
	nameFound := false

	// Check each result without assuming order
	for _, result := range results {
		found[result.Original] = true

		switch result.Original {
		case "valid@example.com":
			assert.True(t, result.IsValid)
			assert.Empty(t, result.Name)
			validCount++
		case "invalid@":
			assert.False(t, result.IsValid)
			assert.Error(t, result.Error)
		case `"John Doe" <john@example.com>`:
			assert.True(t, result.IsValid)
			assert.Equal(t, "John Doe", result.Name)
			assert.Equal(t, "john@example.com", result.Address)
			validCount++
			nameFound = true
		}
	}

	// Verify we found all expected emails
	assert.Equal(t, len(emails), len(found))
	assert.Equal(t, 2, validCount)
	assert.True(t, nameFound)
}

// Helper function to create long email addresses for testing
func createLongEmail(length int) string {
	if length < 10 {
		return "test@test.com"
	}

	username := make([]byte, length-10)
	for i := range username {
		username[i] = 'a'
	}
	return string(username) + "@test.com"
}

func TestLoadProviderLists(t *testing.T) {
	tmpDir, cleanup := setupTestData(t)
	defer cleanup()

	opts := mailcop.DefaultOptions()
	opts.CheckDNS = false
	opts.CheckDisposable = true
	opts.CheckFreeProvider = true
	opts.DisposableListURL = "file://" + filepath.Join(tmpDir, "disposable.json")
	opts.FreeProvidersURL = "file://" + filepath.Join(tmpDir, "free_providers.json")
	opts.RejectDisposable = true
	opts.RejectFreeProvider = true

	v, err := mailcop.New(opts)
	require.NoError(t, err)
	require.NotNil(t, v)

	// Test disposable domain detection
	result := v.Validate("user@disposable-example.com")
	assert.False(t, result.IsValid)
	assert.True(t, result.IsDisposable)
	assert.False(t, result.IsFreeProvider)

	// Test free provider detection
	result = v.Validate("user@free-example.com")
	assert.False(t, result.IsValid)
	assert.False(t, result.IsDisposable)
	assert.True(t, result.IsFreeProvider)
}

func TestReservedDomains(t *testing.T) {
	opts := mailcop.DefaultOptions()
	opts.CheckDNS = false
	opts.MaxEmailLength = 254
	opts.MinDomainLength = 3
	opts.RejectReserved = true

	v, err := mailcop.New(opts)
	require.NoError(t, err)
	require.NotNil(t, v)

	tests := []struct {
		name        string
		email       string
		expected    mailcop.ValidationResult
		wantExample bool
		wantError   bool
	}{
		{
			name:        "example.com domain",
			email:       "user@example.com",
			wantExample: true,
			wantError:   true,
		},
		{
			name:        "subdomain.example TLD",
			email:       "user@mydomain.example",
			wantExample: true,
			wantError:   true,
		},
		{
			name:        "test TLD",
			email:       "user@domain.test",
			wantExample: true,
			wantError:   true,
		},
		{
			name:        "exact match test domain",
			email:       "user@test",
			wantExample: true,
			wantError:   true,
		},
		{
			name:        "should not match substring",
			email:       "user@mytest.com",
			wantExample: false,
			wantError:   false,
		},
		{
			name:        "regular domain",
			email:       "user@gmail.com",
			wantExample: false,
			wantError:   false,
		},
		{
			name:        "localhost domain",
			email:       "user@localhost",
			wantExample: true,
			wantError:   true,
		},
		{
			name:        "localhost TLD",
			email:       "user@foo.localhost",
			wantExample: true,
			wantError:   true,
		},
		{
			name:        "localhost subdomain",
			email:       "user@localhost.foo.com",
			wantExample: false,
			wantError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(tt.email)
			if tt.wantExample {
				assert.True(t, result.IsReserved)
			} else {
				assert.False(t, result.IsReserved)
			}
			assert.Equal(t, tt.email, result.Original)
		})
	}
}

func TestIPDomains(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantIP  bool
		wantErr bool
	}{
		{
			name:    "IPv4 with brackets",
			email:   "user@[192.168.1.1]",
			wantIP:  true,
			wantErr: true,
		},
		{
			name:    "IPv4 without brackets",
			email:   "user@192.168.1.1",
			wantIP:  true,
			wantErr: true,
		},
		{
			name:    "IPv6 with brackets and prefix (ParseAddress fails on IPv6 with invalid prefix)",
			email:   "user@[IPv6:2001:db8::1]",
			wantIP:  false,
			wantErr: true,
		},
		{
			name:    "IPv6 with brackets",
			email:   "user@[2001:db8::1]",
			wantIP:  true,
			wantErr: true,
		},
		{
			name:    "IPv6 without brackets (ParseAddress fails on IPv6 without brackets)",
			email:   "user@2001:db8::1",
			wantIP:  false,
			wantErr: true,
		},
		{
			name:    "regular domain",
			email:   "user@example.com",
			wantIP:  false,
			wantErr: false,
		},
		{
			name:    "invalid IP",
			email:   "user@[300.300.300.300]",
			wantIP:  false,
			wantErr: true,
		},

		// Additional edge cases
		{
			name:    "empty brackets",
			email:   "user@[]",
			wantIP:  false,
			wantErr: true,
		},
		{
			name:    "incomplete IPv4",
			email:   "user@[192.168.1.]",
			wantIP:  false,
			wantErr: true,
		},
		{
			name:    "IPv4 with leading zeros",
			email:   "user@[192.168.001.001]",
			wantIP:  false,
			wantErr: true,
		},
		{
			name:    "IPv6 localhost",
			email:   "user@[::1]",
			wantIP:  true,
			wantErr: true,
		},
		{
			name:    "IPv6 compressed zeros",
			email:   "user@[2001:db8::0:1]",
			wantIP:  true,
			wantErr: true,
		},
		{
			name:    "malformed brackets",
			email:   "user@[192.168.1.1",
			wantIP:  false,
			wantErr: true,
		},
		{
			name:    "double brackets",
			email:   "user@[[192.168.1.1]]",
			wantIP:  false,
			wantErr: true,
		},
		{
			name:    "IP with spaces",
			email:   "user@[ 192.168.1.1 ]",
			wantIP:  false,
			wantErr: true,
		},
		{
			name:    "IP with invalid chars",
			email:   "user@[192.168.1.1a]",
			wantIP:  false,
			wantErr: true,
		},
	}

	opts := mailcop.DefaultOptions()
	opts.RejectIPDomains = true
	v, err := mailcop.New(opts)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := v.Validate(tt.email)
			assert.Equal(t, tt.wantIP, result.IsIPDomain)
			if tt.wantErr {
				assert.False(t, result.IsValid)
				assert.Error(t, result.Error)
			} else {
				assert.True(t, result.IsValid)
				assert.NoError(t, result.Error)
			}
		})
	}
}
