package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderNotifyContentReplacesValuePlaceholders(t *testing.T) {
	content := renderNotifyContent("quota {{value}}, remaining {{value}}", []interface{}{10, 20})

	assert.Equal(t, "quota 10, remaining 20", content)
	assert.NotContains(t, content, "%!(EXTRA")
}

func TestWebhookHeadersDoNotForwardRawSecret(t *testing.T) {
	headers := webhookHeaders("top-secret", []byte(`{"ok":true}`))

	require.NotEmpty(t, headers["X-Webhook-Signature"])
	assert.NotContains(t, headers, "Authorization")
	for _, value := range headers {
		assert.False(t, strings.Contains(value, "top-secret"))
	}
}

func TestGotifyEndpointAndHeadersKeepTokenOutOfURL(t *testing.T) {
	endpoint := gotifyEndpoint("https://notify.example/base/")
	headers := gotifyHeaders("gotify-secret")

	assert.Equal(t, "https://notify.example/base/message", endpoint)
	assert.NotContains(t, endpoint, "gotify-secret")
	assert.Equal(t, "gotify-secret", headers["X-Gotify-Key"])
}
