package controller

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/stretchr/testify/require"
)

func TestFetchCustomOAuthDiscoveryRejectsPrivateNetworkTargets(t *testing.T) {
	fetchSetting := system_setting.GetFetchSetting()
	originalFetchSetting := *fetchSetting
	t.Cleanup(func() {
		*fetchSetting = originalFetchSetting
	})

	var hitCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hitCount, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issuer":"http://127.0.0.1"}`))
	}))
	t.Cleanup(server.Close)

	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	port := parsed.Port()
	require.NotEmpty(t, port)

	fetchSetting.EnableSSRFProtection = true
	fetchSetting.AllowPrivateIp = false
	fetchSetting.DomainFilterMode = false
	fetchSetting.IpFilterMode = false
	fetchSetting.DomainList = nil
	fetchSetting.IpList = nil
	fetchSetting.AllowedPorts = []string{port}
	fetchSetting.ApplyIPFilterForDomain = true

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/custom-oauth-provider/discovery", map[string]string{
		"well_known_url": server.URL,
	}, 1)

	FetchCustomOAuthDiscovery(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)
	require.Contains(t, response.Message, "private IP")
	require.Equal(t, int32(0), atomic.LoadInt32(&hitCount))
	portNumber, err := strconv.Atoi(port)
	require.NoError(t, err)
	require.Greater(t, portNumber, 0)
}
