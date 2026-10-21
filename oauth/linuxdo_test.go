package oauth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLinuxDOExchangeTokenUsesConfiguredServerAddress(t *testing.T) {
	gin.SetMode(gin.TestMode)

	originalServerAddress := system_setting.ServerAddress
	originalClientID := common.LinuxDOClientId
	originalClientSecret := common.LinuxDOClientSecret
	t.Cleanup(func() {
		system_setting.ServerAddress = originalServerAddress
		common.LinuxDOClientId = originalClientID
		common.LinuxDOClientSecret = originalClientSecret
	})

	system_setting.ServerAddress = "https://safe.example"
	common.LinuxDOClientId = "client-id"
	common.LinuxDOClientSecret = "client-secret"

	var redirectURI string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		redirectURI = r.PostForm.Get("redirect_uri")
		_, _ = w.Write([]byte(`{"access_token":"token"}`))
	}))
	t.Cleanup(server.Close)
	t.Setenv("LINUX_DO_TOKEN_ENDPOINT", server.URL)

	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request = httptest.NewRequest(http.MethodGet, "http://evil.example/api/oauth/linuxdo", nil)

	token, err := (&LinuxDOProvider{}).ExchangeToken(context.Background(), "auth-code", ginContext)

	require.NoError(t, err)
	require.Equal(t, "token", token.AccessToken)
	require.Equal(t, "https://safe.example/api/oauth/linuxdo", redirectURI)
}
