package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedisLogValueSummaryDoesNotExposeStringValue(t *testing.T) {
	summary := redisLogValueSummary("sk-secret-value")

	require.NotContains(t, summary, "sk-secret-value")
	require.Contains(t, summary, "type=string")
	require.Contains(t, summary, "bytes=15")
	require.Contains(t, summary, "sha256=")
}

func TestRedisLogValueSummaryDoesNotExposeObjectFields(t *testing.T) {
	summary := redisLogValueSummary(struct {
		Token string
	}{
		Token: "secret-token",
	})

	require.NotContains(t, strings.ToLower(summary), "secret-token")
	require.Contains(t, summary, "type=struct")
}
