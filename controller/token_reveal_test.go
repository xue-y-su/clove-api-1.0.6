package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func tokenRevealTestRouter(t *testing.T, userID int) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("token-reveal-test-secret"))))
	router.Use(func(c *gin.Context) {
		c.Set("id", userID)
		c.Next()
	})
	router.POST("/verify", VerifyTokenRevealPassword)
	router.POST("/token/:id/key", middleware.TokenRevealRequired(), GetTokenKey)
	router.POST("/token/batch/keys", middleware.TokenRevealRequired(), GetTokenKeysBatch)
	return router
}

func performTokenRevealRequest(router http.Handler, method string, target string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	var payload []byte
	if body != nil {
		payload, _ = json.Marshal(body)
	}
	request := httptest.NewRequest(method, target, bytes.NewReader(payload))
	request.Header.Set("Content-Type", "application/json")
	for _, item := range cookies {
		request.AddCookie(item)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

func TestVerifyTokenRevealPasswordCreatesThirtyMinuteWindow(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	passwordHash, err := common.Password2Hash("correct-password")
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.User{Id: 31, Username: "reveal-user", Password: passwordHash, Status: common.UserStatusEnabled, Role: common.RoleCommonUser, AffCode: "reveal-aff-31"}).Error)
	router := tokenRevealTestRouter(t, 31)

	recorder := performTokenRevealRequest(router, http.MethodPost, "/verify", map[string]string{"password": "correct-password"}, nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), "verified_until")
	require.NotContains(t, recorder.Body.String(), "correct-password")
	require.NotEmpty(t, recorder.Result().Cookies())
}

func TestVerifyTokenRevealPasswordReportsPasswordSetupRequired(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	require.NoError(t, db.Create(&model.User{Id: 32, Username: "passwordless", Password: "", Email: "owner@example.test", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, AffCode: "reveal-aff-32"}).Error)
	router := tokenRevealTestRouter(t, 32)

	recorder := performTokenRevealRequest(router, http.MethodPost, "/verify", map[string]string{"password": "anything"}, nil)
	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), "PASSWORD_SETUP_REQUIRED")
}

func TestEncryptedTokenRevealAuditsEveryReturnedKeyBeforeResponse(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	passwordHash, err := common.Password2Hash("correct-password")
	require.NoError(t, err)
	require.NoError(t, db.Create(&model.User{Id: 33, Username: "batch-owner", Password: passwordHash, Status: common.UserStatusEnabled, Role: common.RoleCommonUser, AffCode: "reveal-aff-33"}).Error)
	config := common.TokenSecretConfig{
		Phase:   common.TokenSecretPhaseDualRead,
		HMACKey: []byte(strings.Repeat("h", 32)),
		EncryptionKeys: []common.TokenEncryptionKey{
			{Version: 7, Key: []byte(strings.Repeat("a", 32))},
		},
	}
	require.NoError(t, service.ConfigureTokenSecretManager(config))
	t.Cleanup(func() {
		require.NoError(t, service.ConfigureTokenSecretManager(common.TokenSecretConfig{Phase: common.TokenSecretPhaseSchema}))
	})

	tokens := make([]model.Token, 0, 3)
	for index := 0; index < 3; index++ {
		token := model.Token{UserId: 33, Name: "owned-" + strconv.Itoa(index), Key: "owned-secret-key-000" + strconv.Itoa(index), Status: common.TokenStatusEnabled, ExpiredTime: -1, UnlimitedQuota: true}
		require.NoError(t, service.InsertProtectedToken(&token))
		tokens = append(tokens, token)
	}
	router := tokenRevealTestRouter(t, 33)
	verifyRecorder := performTokenRevealRequest(router, http.MethodPost, "/verify", map[string]string{"password": "correct-password"}, nil)
	require.Equal(t, http.StatusOK, verifyRecorder.Code)
	cookies := verifyRecorder.Result().Cookies()

	ids := []int{tokens[0].Id, tokens[1].Id, tokens[2].Id}
	recorder := performTokenRevealRequest(router, http.MethodPost, "/token/batch/keys", map[string]any{"ids": ids}, cookies)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	for index := range tokens {
		require.Contains(t, recorder.Body.String(), "owned-secret-key-000"+strconv.Itoa(index))
	}

	var revealCount int64
	require.NoError(t, db.Model(&model.AuditOutboxEvent{}).Where("event_type = ?", "token.secret_reveal").Count(&revealCount).Error)
	require.EqualValues(t, 3, revealCount)
	var batchCount int64
	require.NoError(t, db.Model(&model.AuditOutboxEvent{}).Where("event_type = ?", "token.secret_reveal_batch").Count(&batchCount).Error)
	require.EqualValues(t, 1, batchCount)
	var events []model.AuditOutboxEvent
	require.NoError(t, db.Find(&events).Error)
	for _, event := range events {
		for index := range tokens {
			require.NotContains(t, event.Payload, "owned-secret-key-000"+strconv.Itoa(index))
		}
	}
}

func TestPasswordlessUserCanSetPasswordWithBoundEmailCode(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	user := model.User{Id: 34, Username: "setup-password", Password: "", Email: "bound@example.test", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, AffCode: "reveal-aff-34"}
	require.NoError(t, db.Create(&user).Error)
	common.RegisterVerificationCodeWithKey(user.Email, "123456", common.EmailVerificationPurpose)
	t.Cleanup(func() { common.DeleteKey(user.Email, common.EmailVerificationPurpose) })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(sessions.Sessions("session", cookie.NewStore([]byte("password-setup-test-secret"))))
	router.Use(func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Next()
	})
	router.POST("/password/setup", SetupOwnPassword)
	recorder := performTokenRevealRequest(router, http.MethodPost, "/password/setup", map[string]string{
		"password":          "new-password-123",
		"verification_code": "123456",
	}, nil)

	require.Equal(t, http.StatusOK, recorder.Code)
	updated, err := model.GetUserById(user.Id, true)
	require.NoError(t, err)
	require.True(t, common.ValidatePasswordAndHash("new-password-123", updated.Password))
	common.DeleteKey(user.Email, common.EmailVerificationPurpose)
}

func TestPasswordSetupCodeIsSentOnlyToCurrentBoundEmail(t *testing.T) {
	db := setupTokenControllerTestDB(t)
	user := model.User{Id: 35, Username: "send-password-code", Password: "", Email: "bound-code@example.test", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, AffCode: "reveal-aff-35"}
	require.NoError(t, db.Create(&user).Error)
	var receiver string
	var content string
	previousSender := tokenPasswordSetupEmailSender
	tokenPasswordSetupEmailSender = func(subject string, target string, body string) error {
		receiver = target
		content = body
		return nil
	}
	t.Cleanup(func() {
		tokenPasswordSetupEmailSender = previousSender
		common.DeleteKey(user.Email, common.EmailVerificationPurpose)
	})
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/self/password/setup/code", nil, user.Id)

	SendOwnPasswordSetupCode(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, user.Email, receiver)
	code := regexp.MustCompile(`\b[0-9a-f]{6}\b`).FindString(content)
	require.NotEmpty(t, code)
	require.True(t, common.VerifyCodeWithKey(user.Email, code, common.EmailVerificationPurpose))
}

func TestTokenRevealWindowCookieRemainsValidBeforeExpiry(t *testing.T) {
	verifiedAt := time.Now().Add(-29*time.Minute - 59*time.Second)
	require.True(t, middleware.TokenRevealWindowValid(verifiedAt, time.Now()))
}
