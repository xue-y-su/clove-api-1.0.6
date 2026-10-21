package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func performTokenAuthRequest(t *testing.T, userStatus int) *httptest.ResponseRecorder {
	t.Helper()

	openComplianceMiddlewareTestDB(t)
	withComplianceOptions(t, map[string]string{})
	withSourceAttributionOptions(t, false, false)

	require.NoError(t, model.DB.Create(&model.User{
		Id:              42,
		Username:        "api-key-user",
		Role:            common.RoleCommonUser,
		Status:          userStatus,
		BanReasonPublic: "quota abuse",
		Group:           "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             84,
		UserId:         42,
		Key:            "existingkey",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	router := gin.New()
	router.GET("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-existingkey")
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestTokenAuthKeepsLegacyAdminChannelSuffix(t *testing.T) {
	openComplianceMiddlewareTestDB(t)
	withComplianceOptions(t, map[string]string{})
	withSourceAttributionOptions(t, false, false)

	require.NoError(t, model.DB.Create(&model.User{
		Id:       44,
		Username: "admin-key-user",
		Role:     common.RoleAdminUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             86,
		UserId:         44,
		Key:            "legacychannelkey",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	router := gin.New()
	router.GET("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"token_id":            c.GetInt("token_id"),
			"specific_channel_id": c.GetString("specific_channel_id"),
		})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-legacychannelkey-17")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"token_id":86`)
	require.Contains(t, recorder.Body.String(), `"specific_channel_id":"17"`)
}

func performHyphenatedTokenAuthRequest(t *testing.T, authMiddleware gin.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()

	openComplianceMiddlewareTestDB(t)
	withComplianceOptions(t, map[string]string{})
	withSourceAttributionOptions(t, false, false)

	require.NoError(t, model.DB.Create(&model.User{
		Id:       43,
		Username: "custom-key-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             85,
		UserId:         43,
		Key:            "custom_key-123456",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)

	router := gin.New()
	router.GET("/v1/chat/completions", authMiddleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"token_id": c.GetInt("token_id")})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-custom_key-123456")
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestTokenAuthAllowsHyphenatedCustomKey(t *testing.T) {
	recorder := performHyphenatedTokenAuthRequest(t, TokenAuth())

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"token_id":85`)
}

func TestTokenAuthReadOnlyAllowsHyphenatedCustomKey(t *testing.T) {
	recorder := performHyphenatedTokenAuthRequest(t, TokenAuthReadOnly())

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"token_id":85`)
}

func TestTokenAuthRejectsKeyAfterUserDisabled(t *testing.T) {
	recorder := performTokenAuthRequest(t, common.UserStatusDisabled)

	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.Contains(t, recorder.Body.String(), "quota abuse")
}

func TestTokenAuthAllowsKeyForEnabledUser(t *testing.T) {
	recorder := performTokenAuthRequest(t, common.UserStatusEnabled)

	require.Equal(t, http.StatusOK, recorder.Code)
}

func TestTokenAuthAllowsAutoGroupToken(t *testing.T) {
	openComplianceMiddlewareTestDB(t)
	withComplianceOptions(t, map[string]string{})
	withSourceAttributionOptions(t, false, false)

	require.NoError(t, model.DB.Create(&model.User{
		Id:       45,
		Username: "auto-group-user",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Token{
		Id:             87,
		UserId:         45,
		Key:            "autogroupkey",
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
		Group:          "auto",
	}).Error)

	router := gin.New()
	router.GET("/v1/chat/completions", TokenAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"using_group": common.GetContextKeyString(c, constant.ContextKeyUsingGroup),
			"token_group": common.GetContextKeyString(c, constant.ContextKeyTokenGroup),
		})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	request.Header.Set("Authorization", "Bearer sk-autogroupkey")
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.JSONEq(t, `{"using_group":"auto","token_group":"auto"}`, recorder.Body.String())
}
