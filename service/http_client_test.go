package service

import (
	"errors"
	"math"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type closeTrackingRoundTripper struct {
	closed atomic.Bool
}

func (transport *closeTrackingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("not implemented")
}

func (transport *closeTrackingRoundTripper) CloseIdleConnections() {
	transport.closed.Store(true)
}

func TestRelayHTTPTransportsBoundConnectionAndResponseHeaders(t *testing.T) {
	originalClient := httpClient
	originalProtectedClient := ssrfProtectedHTTPClient
	originalRelayTimeout := common.RelayTimeout
	originalIdleTimeout := common.RelayIdleConnTimeout
	originalHeaderTimeout := common.RelayResponseHeaderTimeout
	originalMaxIdle := common.RelayMaxIdleConns
	originalMaxIdlePerHost := common.RelayMaxIdleConnsPerHost
	t.Cleanup(func() {
		httpClient = originalClient
		ssrfProtectedHTTPClient = originalProtectedClient
		common.RelayTimeout = originalRelayTimeout
		common.RelayIdleConnTimeout = originalIdleTimeout
		common.RelayResponseHeaderTimeout = originalHeaderTimeout
		common.RelayMaxIdleConns = originalMaxIdle
		common.RelayMaxIdleConnsPerHost = originalMaxIdlePerHost
	})

	common.RelayTimeout = 0
	common.RelayIdleConnTimeout = 90
	common.RelayResponseHeaderTimeout = 300
	common.RelayMaxIdleConns = 100
	common.RelayMaxIdleConnsPerHost = 20
	InitHttpClient()

	transport, ok := httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.NotNil(t, transport.DialContext)
	assert.Equal(t, relayTLSHandshakeTimeout, transport.TLSHandshakeTimeout)
	assert.Equal(t, 300*time.Second, transport.ResponseHeaderTimeout)
	assert.Zero(t, httpClient.Timeout, "streaming responses must not inherit a total timeout")

	protected := newProtectedFetchHTTPClientWithProxy(nil, nil, nil, nil)
	roundTripper, ok := protected.Transport.(*ssrfProtectedRoundTripper)
	require.True(t, ok)
	protectedTransport := roundTripper.newTransport(nil)
	assert.NotNil(t, protectedTransport.DialContext)
	assert.Equal(t, relayTLSHandshakeTimeout, protectedTransport.TLSHandshakeTimeout)
	assert.Equal(t, 300*time.Second, protectedTransport.ResponseHeaderTimeout)
	assert.Less(t, relayTLSHandshakeTimeout, 30*time.Second)
}

func TestRelayResponseHeaderTimeoutConfiguration(t *testing.T) {
	original := common.RelayResponseHeaderTimeout
	t.Cleanup(func() { common.RelayResponseHeaderTimeout = original })

	maxSeconds := int(math.MaxInt64 / int64(time.Second))
	for _, test := range []struct {
		name    string
		seconds int
		want    time.Duration
	}{
		{name: "disabled", seconds: 0, want: 0},
		{name: "custom", seconds: 73, want: 73 * time.Second},
		{name: "overflow clamped", seconds: math.MaxInt, want: time.Duration(maxSeconds) * time.Second},
	} {
		t.Run(test.name, func(t *testing.T) {
			common.RelayResponseHeaderTimeout = test.seconds
			assert.Equal(t, test.want, relayResponseHeaderTimeoutDuration())
		})
	}
}

func TestProxyClientCacheNormalizesEquivalentURLsAndSeparatesCredentials(t *testing.T) {
	ResetProxyClientCache()
	t.Cleanup(ResetProxyClientCache)

	first, err := GetHttpClientWithProxy(" socks5://user:secret@proxy.example ")
	require.NoError(t, err)
	equivalent, err := GetHttpClientWithProxy("socks5://user:secret@proxy.example:1080/")
	require.NoError(t, err)
	differentCredentials, err := GetHttpClientWithProxy("socks5://user:other@proxy.example:1080")
	require.NoError(t, err)

	assert.Same(t, first, equivalent)
	assert.NotSame(t, first, differentCredentials)
	identity, err := NormalizeProxyURL("socks5://user:secret@proxy.example")
	require.NoError(t, err)
	assert.NotContains(t, identity, "user")
	assert.NotContains(t, identity, "secret")
}

func TestProxyClientCacheRejectsInvalidScheme(t *testing.T) {
	client, err := GetHttpClientWithProxy("ftp://proxy.example")
	require.Error(t, err)
	assert.Nil(t, client)
}

func TestProxyClientCacheSupportsSocks5h(t *testing.T) {
	ResetProxyClientCache()
	t.Cleanup(ResetProxyClientCache)

	client, err := GetHttpClientWithProxy("socks5h://user:secret@proxy.example")
	require.NoError(t, err)
	require.NotNil(t, client)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotNil(t, transport.DialContext)
}

func TestInvalidateProxyClientClosesOnlyMatchingTransport(t *testing.T) {
	ResetProxyClientCache()
	t.Cleanup(ResetProxyClientCache)

	parsed, _, err := common.ParseProxyURLRuntime("http://user:secret@proxy.example:8080")
	require.NoError(t, err)
	config := newProxyURLConfig(parsed)
	matchingTransport := &closeTrackingRoundTripper{}
	matchingClient := &http.Client{Transport: matchingTransport}
	otherParsed, _, err := common.ParseProxyURLRuntime("http://other.example:8080")
	require.NoError(t, err)
	otherConfig := newProxyURLConfig(otherParsed)
	otherTransport := &closeTrackingRoundTripper{}
	otherClient := &http.Client{Transport: otherTransport}
	proxyClients.clients[config.cacheKey] = matchingClient
	proxyClients.clients[otherConfig.cacheKey] = otherClient

	InvalidateProxyClient(" http://user:secret@proxy.example:8080/ ")

	assert.True(t, matchingTransport.closed.Load())
	assert.False(t, otherTransport.closed.Load())
	_, matchingExists := proxyClients.get(config.cacheKey)
	remaining, otherExists := proxyClients.get(otherConfig.cacheKey)
	assert.False(t, matchingExists)
	assert.True(t, otherExists)
	assert.Same(t, otherClient, remaining)
}

func TestInvalidateProxyClientIfChangedUsesNormalizedIdentity(t *testing.T) {
	ResetProxyClientCache()
	t.Cleanup(ResetProxyClientCache)

	parsed, _, err := common.ParseProxyURLRuntime("socks5://user:secret@proxy.example")
	require.NoError(t, err)
	config := newProxyURLConfig(parsed)
	transport := &closeTrackingRoundTripper{}
	client := &http.Client{Transport: transport}
	proxyClients.clients[config.cacheKey] = client

	InvalidateProxyClientIfChanged(
		"socks5://user:secret@proxy.example",
		"socks5://user:secret@proxy.example:1080/",
	)
	assert.False(t, transport.closed.Load())
	remaining, exists := proxyClients.get(config.cacheKey)
	assert.True(t, exists)
	assert.Same(t, client, remaining)

	InvalidateProxyClientIfChanged(
		"socks5://user:secret@proxy.example",
		"socks5://user:other@proxy.example:1080",
	)
	assert.True(t, transport.closed.Load())
	_, exists = proxyClients.get(config.cacheKey)
	assert.False(t, exists)
}

func TestProxyClientCacheConcurrentGetReturnsOneClient(t *testing.T) {
	ResetProxyClientCache()
	t.Cleanup(ResetProxyClientCache)

	const workerCount = 32
	clients := make(chan *http.Client, workerCount)
	errorsFound := make(chan error, workerCount)
	var waitGroup sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			client, err := GetHttpClientWithProxy("http://proxy.example:8080")
			clients <- client
			errorsFound <- err
		}()
	}
	waitGroup.Wait()
	close(clients)
	close(errorsFound)

	for err := range errorsFound {
		require.NoError(t, err)
	}
	var first *http.Client
	for client := range clients {
		if first == nil {
			first = client
			continue
		}
		assert.Same(t, first, client)
	}
}

func TestNewProxyHTTPClientKeepsRelayTransportPolicy(t *testing.T) {
	originalTLSSetting := common.TLSInsecureSkipVerify
	t.Cleanup(func() { common.TLSInsecureSkipVerify = originalTLSSetting })
	common.TLSInsecureSkipVerify = true
	parsed, err := url.Parse("http://proxy.example:8080")
	require.NoError(t, err)

	client, err := newProxyHTTPClient(parsed)
	require.NoError(t, err)
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, relayTLSHandshakeTimeout, transport.TLSHandshakeTimeout)
	assert.Equal(t, relayResponseHeaderTimeoutDuration(), transport.ResponseHeaderTimeout)
	require.NotNil(t, transport.TLSClientConfig)
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
}
