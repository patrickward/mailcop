//go:build integration

package mailcop

import (
	"testing"

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
