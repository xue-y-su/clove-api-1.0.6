package logger

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactSensitiveFieldsRemovesQuotedSecrets(t *testing.T) {
	message := `webhook signature="t=123,v1=secret-signature" body="{\"email\":\"buyer@example.com\",\"token\":\"secret-token\"}" status=400`

	redacted := redactSensitiveFields(message)

	assert.Contains(t, redacted, `signature=<redacted bytes=25 sha256=`)
	assert.Contains(t, redacted, `body=<redacted bytes=52 sha256=`)
	assert.Contains(t, redacted, "status=400")
	assert.NotContains(t, redacted, "secret-signature")
	assert.NotContains(t, redacted, "buyer@example.com")
	assert.NotContains(t, redacted, "secret-token")
}

func TestRedactSensitiveFieldsCoversSupportedKeysCaseInsensitively(t *testing.T) {
	message := `Authorization=Bearer-secret token=plain-token SECRET="quoted secret" payload="private payload"`

	redacted := redactSensitiveFields(message)

	assert.Equal(t, 4, strings.Count(redacted, "<redacted bytes="))
	assert.NotContains(t, redacted, "Bearer-secret")
	assert.NotContains(t, redacted, "plain-token")
	assert.NotContains(t, redacted, "quoted secret")
	assert.NotContains(t, redacted, "private payload")
}

func TestRedactSensitiveFieldsDoesNotAlterSimilarOrNarrativeFields(t *testing.T) {
	message := `token_quota=100 access_token_count=2 response body: unavailable`

	assert.Equal(t, message, redactSensitiveFields(message))
}
