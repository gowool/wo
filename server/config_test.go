package server

import (
	"crypto/tls"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_SetDefaults(t *testing.T) {
	tests := []struct {
		name        string
		input       *Config
		expected    *Config
		description string
	}{
		{
			name: ":http address should default to :80",
			input: &Config{
				Address: ":http",
			},
			expected: &Config{
				Address: ":80",
				HTTP2:   &HTTP2Config{MaxConcurrentStreams: 250},
			},
			description: "Should convert :http to :80",
		},
		{
			name: ":https address should default to :443",
			input: &Config{
				Address: ":https",
			},
			expected: &Config{
				Address: ":443",
				HTTP2:   &HTTP2Config{MaxConcurrentStreams: 250},
			},
			description: "Should convert :https to :443",
		},
		{
			name: "empty address without TLS should default to :8080",
			input: &Config{
				Address: "",
			},
			expected: &Config{
				Address: ":8080",
				HTTP2:   &HTTP2Config{MaxConcurrentStreams: 250},
			},
			description: "Should use :8080 for empty address without TLS",
		},
		{
			name: "empty address with TLS should default to :8443",
			input: &Config{
				Address: "",
				TLS:     &TLSConfig{},
			},
			expected: &Config{
				Address: ":8443",
				HTTP2:   &HTTP2Config{MaxConcurrentStreams: 250},
				TLS:     &TLSConfig{},
			},
			description: "Should use :8443 for empty address with TLS",
		},
		{
			name: "custom address should remain unchanged",
			input: &Config{
				Address: ":9000",
			},
			expected: &Config{
				Address: ":9000",
				HTTP2:   &HTTP2Config{MaxConcurrentStreams: 250},
			},
			description: "Should keep custom address unchanged",
		},
		{
			name: "nil HTTP2 config should be initialized with defaults",
			input: &Config{
				Address: ":8080",
				HTTP2:   nil,
			},
			expected: &Config{
				Address: ":8080",
				HTTP2:   &HTTP2Config{MaxConcurrentStreams: 250},
			},
			description: "Should initialize nil HTTP2 config with defaults",
		},
		{
			name: "existing HTTP2 config should have defaults applied",
			input: &Config{
				Address: ":8080",
				HTTP2:   &HTTP2Config{MaxConcurrentStreams: 100},
			},
			expected: &Config{
				Address: ":8080",
				HTTP2:   &HTTP2Config{MaxConcurrentStreams: 100},
			},
			description: "Should keep existing HTTP2 config values when SetDefaults is called",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of input to avoid modifying test data
			config := &Config{
				Address:   tt.input.Address,
				HTTP2:     tt.input.HTTP2,
				HTTP3:     tt.input.HTTP3,
				Transport: tt.input.Transport,
				TLS:       tt.input.TLS,
			}

			config.SetDefaults()

			assert.Equal(t, tt.expected.Address, config.Address, tt.description)

			if tt.expected.HTTP2 != nil {
				require.NotNil(t, config.HTTP2, "HTTP2 config should not be nil")
				assert.Equal(t, tt.expected.HTTP2.MaxConcurrentStreams, config.HTTP2.MaxConcurrentStreams, "HTTP2 MaxConcurrentStreams should match")
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		description string
	}{
		{
			name: "valid config should not return error",
			config: &Config{
				Address: ":8080",
				HTTP2:   &HTTP2Config{MaxConcurrentStreams: 250},
			},
			expectError: false,
			description: "Valid config should pass validation",
		},
		{
			name: "config with nil fields should not return error",
			config: &Config{
				Address: ":8080",
				HTTP2:   nil,
				HTTP3:   nil,
				TLS:     nil,
			},
			expectError: false,
			description: "Config with nil optional fields should be valid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

func TestHTTP2Config_SetDefaults(t *testing.T) {
	tests := []struct {
		name        string
		input       *HTTP2Config
		expected    *HTTP2Config
		description string
	}{
		{
			name:        "zero MaxConcurrentStreams should default to 250",
			input:       &HTTP2Config{MaxConcurrentStreams: 0},
			expected:    &HTTP2Config{MaxConcurrentStreams: 250},
			description: "Should set default MaxConcurrentStreams when zero",
		},
		{
			name:        "non-zero MaxConcurrentStreams should remain unchanged",
			input:       &HTTP2Config{MaxConcurrentStreams: 500},
			expected:    &HTTP2Config{MaxConcurrentStreams: 500},
			description: "Should keep existing non-zero MaxConcurrentStreams",
		},
		{
			name:        "default value should remain unchanged",
			input:       &HTTP2Config{MaxConcurrentStreams: 250},
			expected:    &HTTP2Config{MaxConcurrentStreams: 250},
			description: "Should keep default value unchanged",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &HTTP2Config{MaxConcurrentStreams: tt.input.MaxConcurrentStreams}
			config.SetDefaults()
			assert.Equal(t, tt.expected.MaxConcurrentStreams, config.MaxConcurrentStreams, tt.description)
		})
	}
}

func TestTLSConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *TLSConfig
		expectError bool
		description string
	}{
		{
			name: "TLS config with certificates should be valid",
			config: &TLSConfig{
				Certificates: []CertificateConfig{
					{CertFile: "cert.pem", KeyFile: "key.pem"},
				},
			},
			expectError: false,
			description: "Valid TLS config with certificates should pass",
		},
		{
			name: "TLS config without certificates should fail validation",
			config: &TLSConfig{
				Certificates: []CertificateConfig{},
			},
			expectError: true,
			description: "TLS config without certificates should fail validation",
		},
		{
			name: "TLS config with nil certificates should fail validation",
			config: &TLSConfig{
				Certificates: nil,
			},
			expectError: true,
			description: "TLS config with nil certificates should fail validation",
		},
		{
			name: "TLS config with InsecureSkipVerify should be valid if certificates exist",
			config: &TLSConfig{
				InsecureSkipVerify: true,
				Certificates: []CertificateConfig{
					{CertFile: "cert.pem", KeyFile: "key.pem"},
				},
			},
			expectError: false,
			description: "TLS config with InsecureSkipVerify should be valid if certificates exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

func TestTLSConfig_tls(t *testing.T) {
	tests := []struct {
		name        string
		config      *TLSConfig
		expectError bool
		description string
	}{
		{
			name: "TLS config with invalid certificate should fail",
			config: &TLSConfig{
				Certificates: []CertificateConfig{
					{CertFile: "nonexistent.pem", KeyFile: "nonexistent.key"},
				},
			},
			expectError: true,
			description: "Should fail with invalid certificate files",
		},
		{
			name: "empty TLS config should create valid tls.Config without certificates",
			config: &TLSConfig{
				Certificates:       []CertificateConfig{},
				InsecureSkipVerify: true,
			},
			expectError: false,
			description: "Should create tls.Config even without certificates",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tlsConfig, err := tt.config.tls()

			if tt.expectError {
				assert.Error(t, err, tt.description)
				assert.Nil(t, tlsConfig, "tlsConfig should be nil on error")
			} else {
				assert.NoError(t, err, tt.description)
				require.NotNil(t, tlsConfig, "tlsConfig should not be nil")

				// Verify default values
				assert.Equal(t, uint16(tls.VersionTLS12), tlsConfig.MinVersion, "MinVersion should be TLS 1.2")
				assert.Equal(t, tt.config.InsecureSkipVerify, tlsConfig.InsecureSkipVerify, "InsecureSkipVerify should match")
			}
		})
	}
}

func TestCertificateConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *CertificateConfig
		expectError bool
		description string
	}{
		{
			name: "certificate config with both files should be valid",
			config: &CertificateConfig{
				CertFile: "cert.pem",
				KeyFile:  "key.pem",
			},
			expectError: false,
			description: "Valid certificate config should pass",
		},
		{
			name: "certificate config with empty cert file should fail",
			config: &CertificateConfig{
				CertFile: "",
				KeyFile:  "key.pem",
			},
			expectError: true,
			description: "Empty cert file should fail validation",
		},
		{
			name: "certificate config with empty key file should fail",
			config: &CertificateConfig{
				CertFile: "cert.pem",
				KeyFile:  "",
			},
			expectError: true,
			description: "Empty key file should fail validation",
		},
		{
			name: "certificate config with both empty files should fail",
			config: &CertificateConfig{
				CertFile: "",
				KeyFile:  "",
			},
			expectError: true,
			description: "Both empty files should fail validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
			}
		})
	}
}

func TestCertificateConfig_Certificate(t *testing.T) {
	tests := []struct {
		name        string
		config      *CertificateConfig
		expectError bool
		description string
	}{
		{
			name: "empty cert file should return error",
			config: &CertificateConfig{
				CertFile: "",
				KeyFile:  "key.pem",
			},
			expectError: true,
			description: "Empty cert file should return error",
		},
		{
			name: "empty key file should return error",
			config: &CertificateConfig{
				CertFile: "cert.pem",
				KeyFile:  "",
			},
			expectError: true,
			description: "Empty key file should return error",
		},
		{
			name: "nonexistent cert file should return error",
			config: &CertificateConfig{
				CertFile: "nonexistent.pem",
				KeyFile:  "key.pem",
			},
			expectError: true,
			description: "Nonexistent cert file should return error",
		},
		{
			name: "invalid certificate content should return error",
			config: &CertificateConfig{
				CertFile: "invalid-cert",
				KeyFile:  "invalid-key",
			},
			expectError: true,
			description: "Invalid certificate content should return error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert, err := tt.config.Certificate()

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)
				// For successful cases, verify certificate has reasonable properties
				if err == nil {
					assert.NotZero(t, cert.Certificate, "Certificate should not be zero")
				}
			}
		})
	}
}

// TestTransportConfig tests that TransportConfig has the expected fields
func TestTransportConfig(t *testing.T) {
	config := TransportConfig{
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    8192,
	}

	// This test verifies the struct has the expected fields
	assert.Equal(t, 30*time.Second, config.ReadTimeout, "ReadTimeout should be set")
	assert.Equal(t, 10*time.Second, config.ReadHeaderTimeout, "ReadHeaderTimeout should be set")
	assert.Equal(t, 30*time.Second, config.WriteTimeout, "WriteTimeout should be set")
	assert.Equal(t, 120*time.Second, config.IdleTimeout, "IdleTimeout should be set")
	assert.Equal(t, 8192, config.MaxHeaderBytes, "MaxHeaderBytes should be set")
}

// TestHTTP3Config tests that HTTP3Config has the expected fields
func TestHTTP3Config(t *testing.T) {
	config := HTTP3Config{
		AdvertisedPort: 443,
	}

	// This test verifies the struct has the expected fields
	assert.Equal(t, uint(443), config.AdvertisedPort, "AdvertisedPort should be set")
}

// Benchmark tests for performance critical functions
func BenchmarkConfig_SetDefaults(b *testing.B) {
	config := &Config{
		Address: "",
		HTTP2:   nil,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.SetDefaults()
	}
}

func BenchmarkHTTP2Config_SetDefaults(b *testing.B) {
	config := &HTTP2Config{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.SetDefaults()
	}
}

func BenchmarkTLSConfig_Validate(b *testing.B) {
	config := &TLSConfig{
		Certificates: []CertificateConfig{
			{CertFile: "cert.pem", KeyFile: "key.pem"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Validate()
	}
}
