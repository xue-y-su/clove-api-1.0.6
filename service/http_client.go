package service

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"golang.org/x/net/proxy"
)

var (
	httpClient              *http.Client
	ssrfProtectedHTTPClient *http.Client
	proxyClients            = proxyHTTPClientCache{clients: make(map[string]*http.Client)}
	legacyProxyURLWarnings  sync.Map
)

const (
	relayDialTimeout         = 30 * time.Second
	relayKeepAlive           = 30 * time.Second
	relayTLSHandshakeTimeout = 10 * time.Second
	maxTimeoutSeconds        = int(math.MaxInt64 / int64(time.Second))
)

func relayResponseHeaderTimeoutDuration() time.Duration {
	seconds := common.RelayResponseHeaderTimeout
	if seconds <= 0 {
		return 0
	}
	if seconds > maxTimeoutSeconds {
		seconds = maxTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

func newRelayNetDialer() *net.Dialer {
	return &net.Dialer{Timeout: relayDialTimeout, KeepAlive: relayKeepAlive}
}

func newRelayHTTPTransport() *http.Transport {
	transport := &http.Transport{
		MaxIdleConns:          common.RelayMaxIdleConns,
		MaxIdleConnsPerHost:   common.RelayMaxIdleConnsPerHost,
		IdleConnTimeout:       time.Duration(common.RelayIdleConnTimeout) * time.Second,
		TLSHandshakeTimeout:   relayTLSHandshakeTimeout,
		ResponseHeaderTimeout: relayResponseHeaderTimeoutDuration(),
		ExpectContinueTimeout: time.Second,
		ForceAttemptHTTP2:     true,
		DialContext:           newRelayNetDialer().DialContext,
	}
	if tlsConfig := common.RelayTLSClientConfig(); tlsConfig != nil {
		transport.TLSClientConfig = tlsConfig
	}
	return transport
}

func newRelayHTTPClient(transport http.RoundTripper) *http.Client {
	client := &http.Client{
		Transport:     transport,
		CheckRedirect: checkRedirect,
	}
	if common.RelayTimeout != 0 {
		client.Timeout = time.Duration(common.RelayTimeout) * time.Second
	}
	return client
}

func checkRedirect(req *http.Request, via []*http.Request) error {
	urlStr := req.URL.String()
	if err := validateURLWithCurrentFetchSetting(urlStr, true); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", urlStr, err)
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}

func checkProtectedFetchRedirect(req *http.Request, via []*http.Request) error {
	urlStr := req.URL.String()
	if err := ValidateSSRFProtectedFetchURLContext(req.Context(), urlStr); err != nil {
		return fmt.Errorf("redirect to %s blocked: %v", urlStr, err)
	}
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	return nil
}

func validateURLWithCurrentFetchSetting(urlStr string, applyDomainIPFilter bool) error {
	fetchSetting := system_setting.GetFetchSetting()
	return common.ValidateURLWithFetchSetting(urlStr, fetchSetting.EnableSSRFProtection, fetchSetting.AllowPrivateIp, fetchSetting.DomainFilterMode, fetchSetting.IpFilterMode, fetchSetting.DomainList, fetchSetting.IpList, fetchSetting.AllowedPorts, applyDomainIPFilter && fetchSetting.ApplyIPFilterForDomain)
}

func ValidateSSRFProtectedFetchURL(urlStr string) error {
	return ValidateSSRFProtectedFetchURLContext(context.Background(), urlStr)
}

func ValidateSSRFProtectedFetchURLContext(ctx context.Context, urlStr string) error {
	protection, enabled, err := currentFetchProtection()
	if err != nil || !enabled {
		return err
	}
	return protection.ValidateURLWithResolver(ctx, urlStr, net.DefaultResolver)
}

func InitHttpClient() {
	httpClient = newDirectHTTPClient(defaultHTTPTransportPolicy(), nil)
	ssrfProtectedHTTPClient = newProtectedFetchHTTPClient()
}

// GetHttpClient returns the general outbound client used by relay/provider
// integrations. Do not attach the SSRF-protected dialer here: provider base URLs
// are root/operator-managed deployment targets, not arbitrary user-controlled
// input, and may legitimately point at private networks, private-link endpoints,
// self-hosted services, or local proxies. Code paths that fetch arbitrary
// user-controlled URLs must use GetSSRFProtectedHTTPClient or
// ValidateSSRFProtectedFetchURL instead.
func GetHttpClient() *http.Client {
	return httpClient
}

// GetSSRFProtectedHTTPClient 返回带拨号时 SSRF 校验的客户端。
// ssrfProtectedHTTPClient 由 InitHttpClient 在启动时初始化，运行期只读。
func GetSSRFProtectedHTTPClient() *http.Client {
	if fetchSetting := system_setting.GetFetchSetting(); fetchSetting != nil && !fetchSetting.EnableSSRFProtection {
		return GetHttpClient()
	}
	return ssrfProtectedHTTPClient
}

type proxyHTTPClientCache struct {
	mutex   sync.RWMutex
	clients map[string]*http.Client
}

type proxyURLConfig struct {
	parsedURL *url.URL
	cacheKey  string
}

func newProxyURLConfig(parsedURL *url.URL) *proxyURLConfig {
	digest := sha256.Sum256([]byte(parsedURL.String()))
	return &proxyURLConfig{
		parsedURL: parsedURL,
		cacheKey:  fmt.Sprintf("proxy:v1:%x", digest[:]),
	}
}

func warnLegacyProxyURLOnce(config *proxyURLConfig) {
	if _, loaded := legacyProxyURLWarnings.LoadOrStore(config.cacheKey, struct{}{}); loaded {
		return
	}
	logger.LogWarn(
		context.Background(),
		fmt.Sprintf(
			"legacy proxy URL suffix ignored at runtime: scheme=%s host=%s port=%s; update the channel proxy setting",
			config.parsedURL.Scheme,
			config.parsedURL.Hostname(),
			config.parsedURL.Port(),
		),
	)
}

// NormalizeProxyURL returns an opaque, credential-safe identity for comparison.
func NormalizeProxyURL(rawProxyURL string) (string, error) {
	parsedURL, legacySuffixStripped, err := common.ParseProxyURLRuntime(rawProxyURL)
	if err != nil {
		return "", err
	}
	if parsedURL == nil {
		return "", nil
	}
	config := newProxyURLConfig(parsedURL)
	if legacySuffixStripped {
		warnLegacyProxyURLOnce(config)
	}
	return config.cacheKey, nil
}

// ValidateProxyURL validates a channel proxy URL without connecting to it.
func ValidateProxyURL(rawProxyURL string) error {
	_, err := common.ParseProxyURLStrict(rawProxyURL)
	return err
}

func (cache *proxyHTTPClientCache) get(cacheKey string) (*http.Client, bool) {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()
	client, ok := cache.clients[cacheKey]
	return client, ok
}

func policyClientCacheKey(proxyCacheKey string, policy HTTPTransportPolicy) string {
	if policy == defaultHTTPTransportPolicy() {
		return proxyCacheKey
	}
	return proxyCacheKey + "\x00transport:" + policy.cacheKeyPart()
}

func (cache *proxyHTTPClientCache) getOrCreate(cacheKey string, factory func() (*http.Client, error)) (*http.Client, error) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	if client, ok := cache.clients[cacheKey]; ok {
		return client, nil
	}
	client, err := factory()
	if err != nil {
		return nil, err
	}
	cache.clients[cacheKey] = client
	return client, nil
}

func (cache *proxyHTTPClientCache) removeProxy(proxyCacheKey string) []*http.Client {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	clients := make([]*http.Client, 0)
	policyPrefix := proxyCacheKey + "\x00transport:"
	for cacheKey, client := range cache.clients {
		if cacheKey == proxyCacheKey || strings.HasPrefix(cacheKey, policyPrefix) {
			clients = append(clients, client)
			delete(cache.clients, cacheKey)
		}
	}
	return clients
}

func (cache *proxyHTTPClientCache) reset() map[string]*http.Client {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	oldClients := cache.clients
	cache.clients = make(map[string]*http.Client)
	return oldClients
}

func configureProxyTransport(transport *http.Transport, proxyURL *url.URL) error {
	switch proxyURL.Scheme {
	case "http", "https":
		transport.Proxy = http.ProxyURL(proxyURL)
		return nil

	case "socks5", "socks5h":
		transport.Proxy = nil
		dialer, err := proxy.FromURL(proxyURL, newRelayNetDialer())
		if err != nil {
			return err
		}
		contextDialer, ok := dialer.(proxy.ContextDialer)
		if !ok {
			return fmt.Errorf("SOCKS proxy dialer does not support context cancellation")
		}
		transport.DialContext = contextDialer.DialContext
		return nil

	default:
		return fmt.Errorf("unsupported proxy scheme")
	}
}

func newTransportFactory(proxyURL *url.URL, tlsConfig *tls.Config) (func() *http.Transport, error) {
	if proxyURL != nil {
		probe := newRelayHTTPTransport()
		if err := configureProxyTransport(probe, proxyURL); err != nil {
			return nil, err
		}
	}
	return func() *http.Transport {
		transport := newRelayHTTPTransport()
		if proxyURL != nil {
			_ = configureProxyTransport(transport, proxyURL)
		} else {
			transport.Proxy = http.ProxyFromEnvironment
		}
		if tlsConfig != nil {
			transport.TLSClientConfig = tlsConfig.Clone()
		}
		return transport
	}, nil
}

func newHTTPClientFromPolicy(policy HTTPTransportPolicy, proxyURL *url.URL, tlsConfig *tls.Config) (*http.Client, error) {
	factory, err := newTransportFactory(proxyURL, tlsConfig)
	if err != nil {
		return nil, err
	}
	if policy.Protocol == dto.HTTPProtocolHTTP1 || policy.Shards == 1 {
		transport := factory()
		applyHTTPTransportPolicy(transport, policy)
		return newRelayHTTPClient(transport), nil
	}
	shardedFactory := func() *http.Transport {
		transport := factory()
		applyHTTPTransportPolicy(transport, policy)
		return transport
	}
	return newRelayHTTPClient(newShardedRoundTripper(policy, shardedFactory)), nil
}

func newDirectHTTPClient(policy HTTPTransportPolicy, tlsConfig *tls.Config) *http.Client {
	client, err := newHTTPClientFromPolicy(policy, nil, tlsConfig)
	if err == nil {
		return client
	}
	transport := newRelayHTTPTransport()
	applyHTTPTransportPolicy(transport, policy)
	return newRelayHTTPClient(transport)
}

func newHTTPClientWithPolicyAndTLS(policy HTTPTransportPolicy, tlsConfig *tls.Config) *http.Client {
	return newDirectHTTPClient(policy, tlsConfig)
}

func newProxyHTTPClient(proxyURL *url.URL) (*http.Client, error) {
	return newHTTPClientFromPolicy(defaultHTTPTransportPolicy(), proxyURL, nil)
}

// GetHttpClientWithProxy returns the default client or a cached proxy client.
func GetHttpClientWithProxy(rawProxyURL string) (*http.Client, error) {
	return GetHttpClientWithProxySettings(rawProxyURL, dto.ChannelSettings{})
}

// GetHttpClientWithProxySettings returns a cached client for the channel proxy
// and normalized HTTP transport settings.
func GetHttpClientWithProxySettings(rawProxyURL string, settings dto.ChannelSettings) (*http.Client, error) {
	policy := NormalizeHTTPTransportPolicy(settings)
	if strings.TrimSpace(rawProxyURL) == "" {
		if policy != defaultHTTPTransportPolicy() {
			cacheKey := policyClientCacheKey("", policy)
			if client, ok := proxyClients.get(cacheKey); ok {
				return client, nil
			}
			return proxyClients.getOrCreate(cacheKey, func() (*http.Client, error) {
				return newDirectHTTPClient(policy, nil), nil
			})
		}
		if client := GetHttpClient(); client != nil {
			return client, nil
		}
		return http.DefaultClient, nil
	}
	parsedURL, legacySuffixStripped, err := common.ParseProxyURLRuntime(rawProxyURL)
	if err != nil {
		return nil, err
	}
	config := newProxyURLConfig(parsedURL)
	if legacySuffixStripped {
		warnLegacyProxyURLOnce(config)
	}
	cacheKey := policyClientCacheKey(config.cacheKey, policy)
	if client, ok := proxyClients.get(cacheKey); ok {
		return client, nil
	}
	return proxyClients.getOrCreate(cacheKey, func() (*http.Client, error) {
		return newHTTPClientFromPolicy(policy, config.parsedURL, nil)
	})
}

// InvalidateProxyClient removes one proxy client and closes idle connections.
func InvalidateProxyClient(rawProxyURL string) {
	parsedURL, legacySuffixStripped, err := common.ParseProxyURLRuntime(rawProxyURL)
	if err != nil || parsedURL == nil {
		return
	}
	config := newProxyURLConfig(parsedURL)
	if legacySuffixStripped {
		warnLegacyProxyURLOnce(config)
	}
	for _, client := range proxyClients.removeProxy(config.cacheKey) {
		client.CloseIdleConnections()
	}
}

// InvalidateProxyClientIfChanged retires the old transport only when the
// effective proxy identity changed. Equivalent legacy spellings share a cache.
func InvalidateProxyClientIfChanged(oldProxyURL, newProxyURL string) {
	oldIdentity, oldErr := NormalizeProxyURL(oldProxyURL)
	newIdentity, newErr := NormalizeProxyURL(newProxyURL)
	if oldErr != nil || newErr != nil || oldIdentity == newIdentity {
		return
	}
	InvalidateProxyClient(oldProxyURL)
}

// ResetProxyClientCache clears all cached proxy clients.
func ResetProxyClientCache() {
	for _, client := range proxyClients.reset() {
		client.CloseIdleConnections()
	}
}

// NewProxyHttpClient is kept for compatibility.
// Deprecated: use GetHttpClientWithProxy.
func NewProxyHttpClient(proxyURL string) (*http.Client, error) {
	return GetHttpClientWithProxy(proxyURL)
}
