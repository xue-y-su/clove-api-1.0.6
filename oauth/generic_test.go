package oauth

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOAuthLogPayloadSummaryDoesNotExposeTokensOrPII(t *testing.T) {
	body := []byte(`{"access_token":"secret-access-token","refresh_token":"secret-refresh-token","id_token":"secret-id-token","email":"user@example.com"}`)

	summary := oauthLogPayloadSummary(body)

	require.Contains(t, summary, "bytes=")
	require.Contains(t, summary, "sha256=")
	require.NotContains(t, summary, "secret-access-token")
	require.NotContains(t, summary, "secret-refresh-token")
	require.NotContains(t, summary, "secret-id-token")
	require.NotContains(t, summary, "user@example.com")
	require.NotContains(t, summary, string(body))
	require.Equal(t, summary, strings.TrimSpace(summary))
}
