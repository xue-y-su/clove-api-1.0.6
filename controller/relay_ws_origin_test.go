package controller

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebSocketOriginDefaultsToSameHost(t *testing.T) {
	t.Setenv("CORS_ALLOW_ORIGINS", "")

	require.True(t, isAllowedWebSocketOrigin(&http.Request{
		Host:   "api.example",
		Header: http.Header{"Origin": {"https://api.example"}},
	}))
	require.False(t, isAllowedWebSocketOrigin(&http.Request{
		Host:   "api.example",
		Header: http.Header{"Origin": {"https://evil.example"}},
	}))
}

func TestWebSocketOriginHonorsConfiguredAllowlist(t *testing.T) {
	t.Setenv("CORS_ALLOW_ORIGINS", "https://app.example, https://admin.example")

	require.True(t, isAllowedWebSocketOrigin(&http.Request{
		Host:   "api.example",
		Header: http.Header{"Origin": {"https://app.example"}},
	}))
	require.False(t, isAllowedWebSocketOrigin(&http.Request{
		Host:   "api.example",
		Header: http.Header{"Origin": {"https://evil.example"}},
	}))
}
