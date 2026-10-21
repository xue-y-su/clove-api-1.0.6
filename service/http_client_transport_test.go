package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeHTTPTransportPolicy(t *testing.T) {
	assert.Equal(t, HTTPTransportPolicy{Protocol: dto.HTTPProtocolAuto, Shards: 1}, NormalizeHTTPTransportPolicy(dto.ChannelSettings{}))
	assert.Equal(t, HTTPTransportPolicy{Protocol: dto.HTTPProtocolAuto, Shards: 4}, NormalizeHTTPTransportPolicy(dto.ChannelSettings{HTTPProtocol: "AUTO", HTTP2ConnectionShards: 4}))
	assert.Equal(t, HTTPTransportPolicy{Protocol: dto.HTTPProtocolHTTP1, Shards: 1}, NormalizeHTTPTransportPolicy(dto.ChannelSettings{HTTPProtocol: "HTTP1", HTTP2ConnectionShards: 8}))
	assert.Equal(t, HTTPTransportPolicy{Protocol: dto.HTTPProtocolAuto, Shards: 8}, NormalizeHTTPTransportPolicy(dto.ChannelSettings{HTTPProtocol: "http3", HTTP2ConnectionShards: 99}))
}

func TestShardedRoundTripperRotatesPerOrigin(t *testing.T) {
	sharded := &shardedRoundTripper{n: 3}
	assert.Equal(t, []uint32{0, 1, 2, 0}, []uint32{
		sharded.pickShard("https://a.example"),
		sharded.pickShard("https://a.example"),
		sharded.pickShard("https://a.example"),
		sharded.pickShard("https://a.example"),
	})
	assert.Equal(t, uint32(0), sharded.pickShard("https://b.example"))
}

func TestHTTPTransportPolicyNegotiatesHTTP2AndCanForceHTTP1(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	t.Cleanup(server.Close)

	serverTransport, ok := server.Client().Transport.(*http.Transport)
	require.True(t, ok)
	tlsConfig := serverTransport.TLSClientConfig.Clone()

	autoClient := newHTTPClientWithPolicyAndTLS(defaultHTTPTransportPolicy(), tlsConfig)
	autoResponse, err := autoClient.Get(server.URL)
	require.NoError(t, err)
	require.NoError(t, autoResponse.Body.Close())
	assert.Equal(t, 2, autoResponse.ProtoMajor)

	http1Policy := HTTPTransportPolicy{Protocol: dto.HTTPProtocolHTTP1, Shards: 1}
	http1Client := newHTTPClientWithPolicyAndTLS(http1Policy, tlsConfig)
	http1Response, err := http1Client.Get(server.URL)
	require.NoError(t, err)
	require.NoError(t, http1Response.Body.Close())
	assert.Equal(t, 1, http1Response.ProtoMajor)

	transport, ok := http1Client.Transport.(*http.Transport)
	require.True(t, ok)
	assert.False(t, transport.ForceAttemptHTTP2)
	assert.Empty(t, transport.TLSNextProto)
}

func TestHTTPClientCacheSeparatesPoliciesAndInvalidatesAllProxyVariants(t *testing.T) {
	originalClient := httpClient
	originalProtectedClient := ssrfProtectedHTTPClient
	ResetProxyClientCache()
	InitHttpClient()
	t.Cleanup(func() {
		ResetProxyClientCache()
		httpClient.CloseIdleConnections()
		httpClient = originalClient
		ssrfProtectedHTTPClient = originalProtectedClient
	})

	defaultClient, err := GetHttpClientWithProxySettings("", dto.ChannelSettings{})
	require.NoError(t, err)
	assert.Same(t, GetHttpClient(), defaultClient)

	http1Client, err := GetHttpClientWithProxySettings("", dto.ChannelSettings{HTTPProtocol: dto.HTTPProtocolHTTP1})
	require.NoError(t, err)
	shardedClient, err := GetHttpClientWithProxySettings("", dto.ChannelSettings{HTTP2ConnectionShards: 3})
	require.NoError(t, err)
	assert.NotSame(t, defaultClient, http1Client)
	assert.IsType(t, &shardedRoundTripper{}, shardedClient.Transport)

	proxyURL := "http://user:secret@proxy.example:8080"
	proxyDefault, err := GetHttpClientWithProxy(proxyURL)
	require.NoError(t, err)
	proxyHTTP1, err := GetHttpClientWithProxySettings(proxyURL, dto.ChannelSettings{HTTPProtocol: dto.HTTPProtocolHTTP1})
	require.NoError(t, err)
	assert.NotSame(t, proxyDefault, proxyHTTP1)

	InvalidateProxyClient(proxyURL)
	afterDefault, err := GetHttpClientWithProxy(proxyURL)
	require.NoError(t, err)
	afterHTTP1, err := GetHttpClientWithProxySettings(proxyURL, dto.ChannelSettings{HTTPProtocol: dto.HTTPProtocolHTTP1})
	require.NoError(t, err)
	assert.NotSame(t, proxyDefault, afterDefault)
	assert.NotSame(t, proxyHTTP1, afterHTTP1)
}
