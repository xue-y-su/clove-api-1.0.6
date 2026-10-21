package common

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigureTLSInsecureSkipVerifyDoesNotMutateDefaultTransport(t *testing.T) {
	originalTransport := http.DefaultTransport
	transport := &http.Transport{}
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = originalTransport
	})

	configureTLSInsecureSkipVerify(true)

	assert.True(t, TLSInsecureSkipVerify)
	assert.Nil(t, transport.TLSClientConfig)
}

func TestRelayTLSConfigReturnsDedicatedInsecureConfigOnlyWhenEnabled(t *testing.T) {
	configureTLSInsecureSkipVerify(false)
	assert.Nil(t, RelayTLSClientConfig())

	configureTLSInsecureSkipVerify(true)
	config := RelayTLSClientConfig()

	assert.NotNil(t, config)
	assert.True(t, config.InsecureSkipVerify)
	assert.Equal(t, uint16(tls.VersionTLS12), config.MinVersion)
	assert.NotSame(t, config, RelayTLSClientConfig())
}

func TestLegacyInsecureTLSConfigRequiresTLS12OrNewer(t *testing.T) {
	assert.Equal(t, uint16(tls.VersionTLS12), InsecureTLSConfig.MinVersion)
}

func TestRelayTLSClientConfigCanBeSafelyMutatedByCaller(t *testing.T) {
	configureTLSInsecureSkipVerify(true)
	config := RelayTLSClientConfig()
	config.ServerName = "example.com"

	next := RelayTLSClientConfig()

	assert.Equal(t, "", next.ServerName)
	assert.IsType(t, &tls.Config{}, next)
}
