package mailcop_test

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/patrickward/mailcop"
)

func TestBloomFilter(t *testing.T) {
	opts := mailcop.DefaultOptions()
	opts.CheckDisposable = true

	//// Sample domains we know exist in the testdata file
	//knownDisposableDomains := []string{
	//	"tempmail.com",
	//	"throwaway.com",
	//	"disposable.com",
	//	"fakeemail.com",
	//}

	testDataPath := "file://" + filepath.Join("testdata", "domains.json")

	tests := []struct {
		name               string
		setup              func(*testing.T, *mailcop.Validator)
		domain             string
		shouldBeDisposable bool
	}{
		{
			name: "standard map implementation - known disposable",
			setup: func(t *testing.T, v *mailcop.Validator) {
				err := v.LoadDisposableDomains(testDataPath)
				require.NoError(t, err)
			},
			domain:             "tempmail.com",
			shouldBeDisposable: true,
		},
		{
			name: "bloom filter implementation - known disposable",
			setup: func(t *testing.T, v *mailcop.Validator) {
				bloomOpts := mailcop.DefaultBloomOptions()
				err := v.UseBloomFilter(testDataPath, bloomOpts)
				require.NoError(t, err)
			},
			domain:             "tempmail.com",
			shouldBeDisposable: true,
		},
		{
			name: "bloom filter with trusted domains",
			setup: func(t *testing.T, v *mailcop.Validator) {
				bloomOpts := mailcop.DefaultBloomOptions()
				bloomOpts.TrustedDomains = map[string]struct{}{
					"gmail.com": {},
				}
				err := v.UseBloomFilter(testDataPath, bloomOpts)
				require.NoError(t, err)
			},
			domain:             "gmail.com",
			shouldBeDisposable: false, // Trusted domains should never be disposable
		},
		{
			name: "non-existent domain with map",
			setup: func(t *testing.T, v *mailcop.Validator) {
				err := v.LoadDisposableDomains(testDataPath)
				require.NoError(t, err)
			},
			domain:             "legitimatedomain.com",
			shouldBeDisposable: false,
		},
		{
			name: "non-existent domain with bloom - may have false positives",
			setup: func(t *testing.T, v *mailcop.Validator) {
				bloomOpts := mailcop.DefaultBloomOptions()
				err := v.UseBloomFilter(testDataPath, bloomOpts)
				require.NoError(t, err)
			},
			domain:             "legitimatedomain.com",
			shouldBeDisposable: false,
		},
		{
			name: "error case - invalid file path",
			setup: func(t *testing.T, v *mailcop.Validator) {
				bloomOpts := mailcop.DefaultBloomOptions()
				err := v.UseBloomFilter("file:///nonexistent/path.json", bloomOpts)
				require.Error(t, err)
			},
			domain:             "tempmail.com",
			shouldBeDisposable: false,
		},
		{
			name: "multiple verification attempts",
			setup: func(t *testing.T, v *mailcop.Validator) {
				bloomOpts := mailcop.DefaultBloomOptions()
				bloomOpts.VerificationAttempts = 3 // Multiple checks to reduce false positives
				err := v.UseBloomFilter(testDataPath, bloomOpts)
				require.NoError(t, err)
			},
			domain:             "disposable.com",
			shouldBeDisposable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := mailcop.New(opts)
			require.NoError(t, err)

			tt.setup(t, v)
			result := v.Validate("user@" + tt.domain)

			if tt.shouldBeDisposable {
				assert.True(t, result.IsDisposable, "Expected %s to be disposable", tt.domain)
			} else if !strings.Contains(tt.name, "may have false positives") {
				assert.False(t, result.IsDisposable, "Expected %s to not be disposable", tt.domain)
			}
		})
	}
}

// formatBytes returns a human-readable string of bytes
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func BenchmarkDataStructureMemory(b *testing.B) {
	testDataPath := "file://" + filepath.Join("testdata", "domains.json")

	tests := []struct {
		name  string
		setup func() (*mailcop.Validator, error)
	}{
		{
			name: "Map Implementation",
			setup: func() (*mailcop.Validator, error) {
				opts := mailcop.DefaultOptions()
				opts.CheckDisposable = true
				opts.DisposableListURL = testDataPath
				return mailcop.New(opts)
			},
		},
		{
			name: "Bloom Filter (0.1% FP)",
			setup: func() (*mailcop.Validator, error) {
				opts := mailcop.DefaultOptions()
				opts.CheckDisposable = true
				v, err := mailcop.New(opts)
				if err != nil {
					return nil, err
				}
				bloomOpts := mailcop.DefaultBloomOptions()
				bloomOpts.FalsePositiveRate = 0.001
				if err := v.UseBloomFilter(testDataPath, bloomOpts); err != nil {
					return nil, err
				}
				return v, nil
			},
		},
		{
			name: "Bloom Filter (1% FP)",
			setup: func() (*mailcop.Validator, error) {
				opts := mailcop.DefaultOptions()
				opts.CheckDisposable = true
				v, err := mailcop.New(opts)
				if err != nil {
					return nil, err
				}
				bloomOpts := mailcop.DefaultBloomOptions()
				bloomOpts.FalsePositiveRate = 0.01 // Higher false positive rate = less memory
				if err := v.UseBloomFilter(testDataPath, bloomOpts); err != nil {
					return nil, err
				}
				return v, nil
			},
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			runtime.GC()
			var m1 runtime.MemStats
			runtime.ReadMemStats(&m1)

			v, err := tt.setup()
			require.NoError(b, err)

			runtime.GC()
			var m2 runtime.MemStats
			runtime.ReadMemStats(&m2)

			dataStructureSize := m2.Alloc - m1.Alloc
			b.Logf("Data structure size: %s", formatBytes(dataStructureSize))
			b.Logf("Heap objects: %d", m2.HeapObjects-m1.HeapObjects)

			// Verify the data structure is working
			result := v.Validate("user@tempmail.com")
			require.True(b, result.IsDisposable)

			b.ReportMetric(float64(dataStructureSize), "struct_bytes")

			runtime.KeepAlive(v)
		})
	}
}
