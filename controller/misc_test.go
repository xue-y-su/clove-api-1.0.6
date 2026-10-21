package controller

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/registration_setting"
	"github.com/QuantumNous/new-api/setting/user_redemption_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGetStatusIncludesDisplayVersionAndAuthor(t *testing.T) {
	previousOptionMap := common.OptionMap
	previousVersion := common.Version
	common.OptionMap = map[string]string{}
	common.Version = "v1.0.0-rc.21.clove-enhanced"
	t.Cleanup(func() {
		common.OptionMap = previousOptionMap
		common.Version = previousVersion
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)

	GetStatus(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.Contains(t, string(response.Data), `"version":"Clove API v1.0.5-beta"`)
	require.NotContains(t, string(response.Data), common.Version)
	require.Contains(t, string(response.Data), `"version_author":"Clove. , xue-y-su"`)
}

func TestGetStatusIncludesCustomKeyCreationPolicy(t *testing.T) {
	previousOptionMap := common.OptionMap
	previous := operation_setting.GetTokenSetting().AllowCustomKeys
	common.OptionMap = map[string]string{}
	operation_setting.GetTokenSetting().AllowCustomKeys = false
	t.Cleanup(func() {
		common.OptionMap = previousOptionMap
		operation_setting.GetTokenSetting().AllowCustomKeys = previous
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)

	GetStatus(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.Contains(t, string(response.Data), `"allow_custom_keys":false`)
}

func TestGetStatusIncludesInvitationPosterCopy(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{
		registration_setting.InvitationPosterTitleKey:    "Join {站点名称}",
		registration_setting.InvitationPosterSubtitleKey: "Scan now",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)

	GetStatus(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.Contains(t, string(response.Data), `"invitation_poster_title":"Join {站点名称}"`)
	require.Contains(t, string(response.Data), `"invitation_poster_subtitle":"Scan now"`)
}

func TestGetStatusIncludesPublicUserRedemptionPolicy(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{
		user_redemption_setting.EnabledKey:               "false",
		user_redemption_setting.SingleEnabledKey:         "true",
		user_redemption_setting.RandomEnabledKey:         "false",
		user_redemption_setting.MaxActiveCodesPerUserKey: "4",
		user_redemption_setting.MaxClaimsPerCodeKey:      "20",
		user_redemption_setting.MaxExpiryHoursKey:        "72",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)

	GetStatus(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.Contains(t, string(response.Data), `"user_redemption":{"enabled":false,"single_enabled":true,"random_enabled":false,"subscription_enabled":true,"max_active_codes_per_user":4,"max_claims_per_code":20,"max_expiry_hours":72}`)
}

func TestGetStatusIncludesPublicPlaygroundSearchCapability(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{
		service.PlaygroundSearchOptionEnabled:        "true",
		service.PlaygroundSearchOptionAPIKey:         "tvly-status-secret",
		service.PlaygroundSearchOptionNativeModels:   "gpt-4.1*",
		service.PlaygroundSearchOptionNativeChannels: "7",
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)

	GetStatus(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	require.Contains(t, string(response.Data), `"playground_web_search":{"available":true,"native":true,"tavily":true,"reason":""}`)
	require.NotContains(t, string(response.Data), "tvly-status-secret")
}

func TestBuildStatusDataLockedDoesNotReenterOptionReadLock(t *testing.T) {
	common.OptionMapRWMutex.Lock()
	optionMapWasNil := common.OptionMap == nil
	if optionMapWasNil {
		common.OptionMap = map[string]string{}
	}
	previousAffRequired, hadAffRequired := common.OptionMap[registration_setting.AffRequiredKey]
	common.OptionMap[registration_setting.AffRequiredKey] = "true"
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		defer common.OptionMapRWMutex.Unlock()
		if optionMapWasNil {
			common.OptionMap = nil
		} else if hadAffRequired {
			common.OptionMap[registration_setting.AffRequiredKey] = previousAffRequired
		} else {
			delete(common.OptionMap, registration_setting.AffRequiredKey)
		}
	})

	consoleSettings := console_setting.GetConsoleSetting()
	common.OptionMapRWMutex.RLock()
	readerHeld := true
	defer func() {
		if readerHeld {
			common.OptionMapRWMutex.RUnlock()
		}
	}()

	writerStarted := make(chan struct{})
	writerDone := make(chan string, 1)
	go func() {
		close(writerStarted)
		common.OptionMapRWMutex.Lock()
		affRequired := common.OptionMap[registration_setting.AffRequiredKey]
		common.OptionMapRWMutex.Unlock()
		writerDone <- affRequired
	}()
	<-writerStarted

	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		if !common.OptionMapRWMutex.TryRLock() {
			break
		}
		common.OptionMapRWMutex.RUnlock()

		select {
		case <-deadline.C:
			t.Fatal("option writer did not enter the wait queue")
		default:
			runtime.Gosched()
		}
	}

	dataDone := make(chan gin.H, 1)
	go func() {
		dataDone <- buildStatusDataLocked(consoleSettings)
	}()

	var data gin.H
	select {
	case data = <-dataDone:
	case <-deadline.C:
		t.Fatal("status data builder re-entered the option read lock")
	}

	common.OptionMapRWMutex.RUnlock()
	readerHeld = false
	require.Equal(t, "true", <-writerDone)
	require.Equal(t, true, data["registration_aff_required"])
}

func TestRenderEmailVerificationContentUsesDefaultWhenTemplateIsEmpty(t *testing.T) {
	previousSystemName := common.SystemName
	previousTemplate := common.EmailVerificationTemplate
	common.SystemName = "Clove API"
	common.EmailVerificationTemplate = ""
	t.Cleanup(func() {
		common.SystemName = previousSystemName
		common.EmailVerificationTemplate = previousTemplate
	})

	content := renderEmailVerificationContent("123456")

	require.Contains(t, content, "Clove API邮箱验证")
	require.Contains(t, content, "<strong>123456</strong>")
	require.Contains(t, content, "验证码 10 分钟内有效")
}

func TestRenderEmailVerificationContentReplacesEscapedPlaceholders(t *testing.T) {
	previousSystemName := common.SystemName
	previousTemplate := common.EmailVerificationTemplate
	common.SystemName = `<Clove "API">`
	common.EmailVerificationTemplate = "<p>{{site_name}}</p><p>{{code}}</p><p>{{expiration_minutes}}</p><p>{{unknown}}</p>"
	t.Cleanup(func() {
		common.SystemName = previousSystemName
		common.EmailVerificationTemplate = previousTemplate
	})

	content := renderEmailVerificationContent("A&B<123>")

	require.Equal(t, "<p>&lt;Clove &#34;API&#34;&gt;</p><p>A&amp;B&lt;123&gt;</p><p>10</p><p>{{unknown}}</p>", content)
}
