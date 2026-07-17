//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateEndpointAllowsLocalMonitorEndpoints(t *testing.T) {
	for _, endpoint := range []string{
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"http://[::1]:8080",
		"https://localhost:8443",
		"https://127.0.0.1:8443",
	} {
		t.Run(endpoint, func(t *testing.T) {
			require.NoError(t, validateEndpoint(endpoint))
		})
	}
}

func TestValidateEndpointRejectsUnsafeNonLocalEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		wantErr  error
	}{
		{
			name:     "public_http",
			endpoint: "http://example.com",
			wantErr:  ErrChannelMonitorEndpointScheme,
		},
		{
			name:     "private_http",
			endpoint: "http://192.168.1.10:8080",
			wantErr:  ErrChannelMonitorEndpointScheme,
		},
		{
			name:     "private_https",
			endpoint: "https://10.0.0.1:8443",
			wantErr:  ErrChannelMonitorEndpointPrivate,
		},
		{
			name:     "metadata_https",
			endpoint: "https://169.254.169.254",
			wantErr:  ErrChannelMonitorEndpointPrivate,
		},
		{
			name:     "path_not_origin",
			endpoint: "https://api.openai.com/v1",
			wantErr:  ErrChannelMonitorEndpointPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ErrorIs(t, validateEndpoint(tt.endpoint), tt.wantErr)
		})
	}
}
