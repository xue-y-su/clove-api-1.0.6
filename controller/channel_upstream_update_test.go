package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildChannelModelDiffSeparatesAddedRetainedAndMissing(t *testing.T) {
	diff := buildChannelModelDiff(
		[]string{"retained", "missing", "alias"},
		[]string{"retained", "added", "upstream-target"},
		nil,
		map[string]string{"alias": "upstream-target"},
	)

	require.Equal(t, []string{"added"}, diff.Added)
	require.Equal(t, []string{"retained", "alias"}, diff.Retained)
	require.Equal(t, []string{"missing"}, diff.Missing)
}

func TestSub2APIFetchModelsUsesBearerAuthAndEightMiBLimit(t *testing.T) {
	service.InitHttpClient()
	t.Run("authorization", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/v1/models", r.URL.Path)
			require.Equal(t, "Bearer sub2-key", r.Header.Get("Authorization"))
			_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4.1"}]}`))
		}))
		t.Cleanup(server.Close)
		baseURL := server.URL

		models, err := fetchChannelUpstreamModelIDs(&model.Channel{
			Type:    constant.ChannelTypeSub2API,
			Key:     "sub2-key",
			BaseURL: &baseURL,
		})
		require.NoError(t, err)
		require.Equal(t, []string{"gpt-4.1"}, models)
	})

	t.Run("limit", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(bytes.Repeat([]byte("x"), (8<<20)+1))
		}))
		t.Cleanup(server.Close)
		baseURL := server.URL

		_, err := fetchChannelUpstreamModelIDs(&model.Channel{
			Type:    constant.ChannelTypeSub2API,
			Key:     "sub2-key",
			BaseURL: &baseURL,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "8 MiB")
	})
}

func TestSub2APIFetchModelsUsesIndependentTimeout(t *testing.T) {
	service.InitHttpClient()
	previousTimeout := sub2APIModelFetchTimeout
	sub2APIModelFetchTimeout = 20 * time.Millisecond
	t.Cleanup(func() { sub2APIModelFetchTimeout = previousTimeout })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL

	_, err := fetchChannelUpstreamModelIDs(&model.Channel{
		Type:    constant.ChannelTypeSub2API,
		Key:     "sub2-key",
		BaseURL: &baseURL,
	})
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "timeout")
}

func TestSub2APIConfirmedDiffApprovesAddsAndRetainsMissingByDefault(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ChannelModelPublication{}))
	channel := &model.Channel{
		Id:     601,
		Type:   constant.ChannelTypeSub2API,
		Key:    "sub2-key",
		Status: common.ChannelStatusEnabled,
		Name:   "sub2",
		Models: "retained,missing",
		Group:  "default",
	}
	settings := channel.GetOtherSettings()
	settings.UpstreamModelUpdateLastDetectedModels = []string{"added"}
	settings.UpstreamModelUpdateLastRemovedModels = []string{"missing"}
	channel.SetOtherSettings(settings)
	require.NoError(t, db.Create(channel).Error)

	added, removed, _, remainingMissing, changed, err := applyChannelUpstreamModelUpdates(
		channel,
		[]string{"added"},
		nil,
		nil,
	)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{"added"}, added)
	require.Empty(t, removed)
	require.Equal(t, []string{"missing"}, remainingMissing)
	require.ElementsMatch(t, []string{"retained", "missing", "added"}, channel.GetModels())
	require.True(t, model.IsChannelModelPublicationApproved(channel.Id, "added"))
	require.False(t, model.IsChannelModelPublicationApproved(channel.Id, "retained"))
	require.False(t, model.IsChannelModelPublicationApproved(channel.Id, "missing"))
}

func TestSub2APIExplicitMissingRemovalRevokesPublication(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ChannelModelPublication{}))
	channel := &model.Channel{
		Id:     602,
		Type:   constant.ChannelTypeSub2API,
		Key:    "sub2-key",
		Status: common.ChannelStatusEnabled,
		Name:   "sub2-remove",
		Models: "retained,missing",
		Group:  "default",
	}
	settings := channel.GetOtherSettings()
	settings.UpstreamModelUpdateLastRemovedModels = []string{"missing"}
	channel.SetOtherSettings(settings)
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, model.ApproveChannelModelPublications(db, channel.Id, []string{"missing"}))

	_, removed, _, _, changed, err := applyChannelUpstreamModelUpdates(channel, nil, nil, []string{"missing"})
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, []string{"missing"}, removed)
	require.Equal(t, []string{"retained"}, channel.GetModels())
	require.False(t, model.IsChannelModelPublicationApproved(channel.Id, "missing"))
}

func TestSub2APIFetchParseFailureLeavesModelsAndApprovalsUnchanged(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ChannelModelPublication{}))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":`))
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL
	channel := &model.Channel{
		Id:        603,
		Type:      constant.ChannelTypeSub2API,
		Key:       "sub2-key",
		Status:    common.ChannelStatusEnabled,
		Name:      "sub2-invalid",
		Models:    "existing",
		Group:     "default",
		BaseURL:   &baseURL,
		TestModel: nil,
	}
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, model.ApproveChannelModelPublications(db, channel.Id, []string{"existing"}))

	settings := channel.GetOtherSettings()
	_, _, err := collectPendingUpstreamModelChanges(channel, settings)
	require.Error(t, err)

	var stored model.Channel
	require.NoError(t, db.First(&stored, channel.Id).Error)
	require.Equal(t, "existing", stored.Models)
	require.True(t, model.IsChannelModelPublicationApproved(channel.Id, "existing"))
}

func TestSub2APIAutoSyncStagesCandidatesForExplicitApproval(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ChannelModelPublication{}))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"existing"},{"id":"candidate"}]}`))
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL
	channel := &model.Channel{
		Id:      604,
		Type:    constant.ChannelTypeSub2API,
		Key:     "sub2-key",
		Status:  common.ChannelStatusEnabled,
		Name:    "sub2-auto-sync",
		Models:  "existing",
		Group:   "default",
		BaseURL: &baseURL,
	}
	settings := channel.GetOtherSettings()
	settings.UpstreamModelUpdateCheckEnabled = true
	settings.UpstreamModelUpdateAutoSyncEnabled = true
	channel.SetOtherSettings(settings)
	require.NoError(t, db.Create(channel).Error)

	modelsChanged, autoAdded, err := checkAndPersistChannelUpstreamModelUpdates(
		channel,
		&settings,
		true,
		true,
	)
	require.NoError(t, err)
	require.False(t, modelsChanged)
	require.Zero(t, autoAdded)
	require.Equal(t, "existing", channel.Models)
	require.ElementsMatch(t, []string{"candidate", "existing"}, settings.UpstreamModelUpdateLastDetectedModels)
	require.False(t, model.IsChannelModelPublicationApproved(channel.Id, "candidate"))
	require.False(t, model.IsChannelModelPublicationApproved(channel.Id, "existing"))

	var stored model.Channel
	require.NoError(t, db.First(&stored, channel.Id).Error)
	require.Equal(t, "existing", stored.Models)
}

func TestSub2APIExistingUnapprovedModelRemainsConfirmationCandidate(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.ChannelModelPublication{}))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"existing"}]}`))
	}))
	t.Cleanup(server.Close)
	baseURL := server.URL
	channel := &model.Channel{
		Id:      605,
		Type:    constant.ChannelTypeSub2API,
		Key:     "sub2-key",
		Status:  common.ChannelStatusEnabled,
		Name:    "sub2-existing-unapproved",
		Models:  "existing",
		Group:   "default",
		BaseURL: &baseURL,
	}
	require.NoError(t, db.Create(channel).Error)

	addModels, removeModels, err := collectPendingUpstreamModelChanges(channel, channel.GetOtherSettings())
	require.NoError(t, err)
	require.Equal(t, []string{"existing"}, addModels)
	require.Empty(t, removeModels)
	require.False(t, model.IsChannelModelPublicationApproved(channel.Id, "existing"))

	settings := channel.GetOtherSettings()
	settings.UpstreamModelUpdateLastDetectedModels = addModels
	channel.SetOtherSettings(settings)
	require.NoError(t, db.Model(channel).Update("settings", channel.OtherSettings).Error)

	added, _, _, _, modelsChanged, err := applyChannelUpstreamModelUpdates(channel, addModels, nil, nil)
	require.NoError(t, err)
	require.False(t, modelsChanged)
	require.Equal(t, []string{"existing"}, added)
	require.True(t, model.IsChannelModelPublicationApproved(channel.Id, "existing"))
}

func TestNormalizeModelNames(t *testing.T) {
	result := normalizeModelNames([]string{
		" gpt-4o ",
		"",
		"gpt-4o",
		"gpt-4.1",
		"   ",
	})

	require.Equal(t, []string{"gpt-4o", "gpt-4.1"}, result)
}

func TestMergeModelNames(t *testing.T) {
	result := mergeModelNames(
		[]string{"gpt-4o", "gpt-4.1"},
		[]string{"gpt-4.1", " gpt-4.1-mini ", "gpt-4o"},
	)

	require.Equal(t, []string{"gpt-4o", "gpt-4.1", "gpt-4.1-mini"}, result)
}

func TestSubtractModelNames(t *testing.T) {
	result := subtractModelNames(
		[]string{"gpt-4o", "gpt-4.1", "gpt-4.1-mini"},
		[]string{"gpt-4.1", "not-exists"},
	)

	require.Equal(t, []string{"gpt-4o", "gpt-4.1-mini"}, result)
}

func TestIntersectModelNames(t *testing.T) {
	result := intersectModelNames(
		[]string{"gpt-4o", "gpt-4.1", "gpt-4.1", "not-exists"},
		[]string{"gpt-4.1", "gpt-4o-mini", "gpt-4o"},
	)

	require.Equal(t, []string{"gpt-4o", "gpt-4.1"}, result)
}

func TestApplySelectedModelChanges(t *testing.T) {
	t.Run("add and remove together", func(t *testing.T) {
		result := applySelectedModelChanges(
			[]string{"gpt-4o", "gpt-4.1", "claude-3"},
			[]string{"gpt-4.1-mini"},
			[]string{"claude-3"},
		)

		require.Equal(t, []string{"gpt-4o", "gpt-4.1", "gpt-4.1-mini"}, result)
	})

	t.Run("add wins when conflict with remove", func(t *testing.T) {
		result := applySelectedModelChanges(
			[]string{"gpt-4o"},
			[]string{"gpt-4.1"},
			[]string{"gpt-4.1"},
		)

		require.Equal(t, []string{"gpt-4o", "gpt-4.1"}, result)
	})
}

func TestCollectPendingApplyUpstreamModelChanges(t *testing.T) {
	settings := dto.ChannelOtherSettings{
		UpstreamModelUpdateLastDetectedModels: []string{" gpt-4o ", "gpt-4o", "gpt-4.1"},
		UpstreamModelUpdateLastRemovedModels:  []string{" old-model ", "", "old-model"},
	}

	pendingAddModels, pendingRemoveModels := collectPendingApplyUpstreamModelChanges(settings)

	require.Equal(t, []string{"gpt-4o", "gpt-4.1"}, pendingAddModels)
	require.Equal(t, []string{"old-model"}, pendingRemoveModels)
}

func TestNormalizeChannelModelMapping(t *testing.T) {
	modelMapping := `{
		" alias-model ": " upstream-model ",
		"": "invalid",
		"invalid-target": ""
	}`
	channel := &model.Channel{
		ModelMapping: &modelMapping,
	}

	result := normalizeChannelModelMapping(channel)
	require.Equal(t, map[string]string{
		"alias-model": "upstream-model",
	}, result)
}

func TestCollectPendingUpstreamModelChangesFromModels_WithModelMapping(t *testing.T) {
	pendingAddModels, pendingRemoveModels := collectPendingUpstreamModelChangesFromModels(
		[]string{"alias-model", "gpt-4o", "stale-model"},
		[]string{"gpt-4o", "gpt-4.1", "mapped-target"},
		[]string{"gpt-4.1"},
		map[string]string{
			"alias-model": "mapped-target",
		},
	)

	require.Equal(t, []string{}, pendingAddModels)
	require.Equal(t, []string{"stale-model"}, pendingRemoveModels)
}

func TestCollectPendingUpstreamModelChangesFromModels_WithIgnoredRegexPatterns(t *testing.T) {
	pendingAddModels, pendingRemoveModels := collectPendingUpstreamModelChangesFromModels(
		[]string{"gpt-4o"},
		[]string{"gpt-4o", "claude-3-5-sonnet", "sora-video", "gpt-4.1"},
		[]string{"regex:^sora-.*$", "gpt-4.1"},
		nil,
	)

	require.Equal(t, []string{"claude-3-5-sonnet"}, pendingAddModels)
	require.Equal(t, []string{}, pendingRemoveModels)
}

func TestBuildUpstreamModelUpdateTaskNotificationContent_OmitOverflowDetails(t *testing.T) {
	channelSummaries := make([]upstreamModelUpdateChannelSummary, 0, 12)
	for i := 0; i < 12; i++ {
		channelSummaries = append(channelSummaries, upstreamModelUpdateChannelSummary{
			ChannelName: "channel-" + string(rune('A'+i)),
			AddCount:    i + 1,
			RemoveCount: i,
		})
	}

	content := buildUpstreamModelUpdateTaskNotificationContent(
		24,
		12,
		56,
		21,
		9,
		[]int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
		channelSummaries,
		[]string{
			"gpt-4.1", "gpt-4.1-mini", "o3", "o4-mini", "gemini-2.5-pro", "claude-3.7-sonnet",
			"qwen-max", "deepseek-r1", "llama-3.3-70b", "mistral-large", "command-r-plus", "doubao-pro-32k",
			"hunyuan-large",
		},
		[]string{
			"gpt-3.5-turbo", "claude-2.1", "gemini-1.5-pro", "mixtral-8x7b", "qwen-plus", "glm-4",
			"yi-large", "moonshot-v1", "doubao-lite",
		},
	)

	require.Contains(t, content, "其余 4 个渠道已省略")
	require.Contains(t, content, "其余 1 个已省略")
	require.Contains(t, content, "失败渠道 ID（展示 10/12）")
	require.Contains(t, content, "其余 2 个已省略")
}

func TestShouldSendUpstreamModelUpdateNotification(t *testing.T) {
	channelUpstreamModelUpdateNotifyState.Lock()
	channelUpstreamModelUpdateNotifyState.lastNotifiedAt = 0
	channelUpstreamModelUpdateNotifyState.lastChangedChannels = 0
	channelUpstreamModelUpdateNotifyState.lastFailedChannels = 0
	channelUpstreamModelUpdateNotifyState.Unlock()

	baseTime := int64(2000000)

	require.True(t, shouldSendUpstreamModelUpdateNotification(baseTime, 6, 0))
	require.False(t, shouldSendUpstreamModelUpdateNotification(baseTime+3600, 6, 0))
	require.True(t, shouldSendUpstreamModelUpdateNotification(baseTime+3600, 7, 0))
	require.False(t, shouldSendUpstreamModelUpdateNotification(baseTime+7200, 7, 0))
	require.True(t, shouldSendUpstreamModelUpdateNotification(baseTime+8000, 0, 3))
	require.False(t, shouldSendUpstreamModelUpdateNotification(baseTime+9000, 0, 3))
	require.True(t, shouldSendUpstreamModelUpdateNotification(baseTime+10000, 0, 4))
	require.True(t, shouldSendUpstreamModelUpdateNotification(baseTime+90000, 7, 0))
	require.False(t, shouldSendUpstreamModelUpdateNotification(baseTime+90001, 0, 0))
}

func decodeFetchModelsResponse(t *testing.T, recorder *httptest.ResponseRecorder) (bool, []string, string) {
	t.Helper()

	var payload struct {
		Success bool     `json:"success"`
		Data    []string `json:"data"`
		Message string   `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	return payload.Success, payload.Data, payload.Message
}

func performFetchModelsRequest(t *testing.T, channelType int, baseURL string, key string) *httptest.ResponseRecorder {
	t.Helper()

	body := []byte(`{"type":` + strconv.Itoa(channelType) + `,"base_url":"` + baseURL + `","key":"` + key + `"}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/fetch_models", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")

	FetchModels(ctx)
	return recorder
}

func TestFetchModelsUsesAnthropicAuthHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		if r.Header.Get("x-api-key") != "anthropic-key" || r.Header.Get("anthropic-version") == "" || r.Header.Get("Authorization") != "" {
			http.Error(w, "wrong auth headers", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-3-5-sonnet"}]}`))
	}))
	t.Cleanup(server.Close)

	recorder := performFetchModelsRequest(t, constant.ChannelTypeAnthropic, server.URL, "anthropic-key")

	require.Equal(t, http.StatusOK, recorder.Code)
	success, models, message := decodeFetchModelsResponse(t, recorder)
	require.True(t, success, message)
	require.Equal(t, []string{"claude-3-5-sonnet"}, models)
}

func TestFetchModelsUsesChannelSpecificModelEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/compatible-mode/v1/models" {
			http.NotFound(w, r)
			return
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, "missing auth", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"qwen-plus"}]}`))
	}))
	t.Cleanup(server.Close)

	recorder := performFetchModelsRequest(t, constant.ChannelTypeAli, server.URL, "ali-key")

	require.Equal(t, http.StatusOK, recorder.Code)
	success, models, message := decodeFetchModelsResponse(t, recorder)
	require.True(t, success, message)
	require.Equal(t, []string{"qwen-plus"}, models)
}

func TestFetchModelsReturnsSuccessFalseOnUpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	recorder := performFetchModelsRequest(t, constant.ChannelTypeOpenAI, server.URL, "openai-key")

	require.Equal(t, http.StatusOK, recorder.Code)
	success, _, message := decodeFetchModelsResponse(t, recorder)
	require.False(t, success)
	require.Contains(t, message, "获取模型列表失败")
}

func TestDetectAllChannelUpstreamModelUpdatesRejectsExistingActiveTask(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.SystemTask{}, &model.SystemTaskLock{}))

	existing, err := model.CreateSystemTask(model.SystemTaskTypeModelUpdate, nil, nil)
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/upstream-models/detect-all", nil)

	DetectAllChannelUpstreamModelUpdates(ctx)

	require.Equal(t, http.StatusConflict, recorder.Code)
	require.Contains(t, recorder.Body.String(), existing.TaskID)
	require.Contains(t, recorder.Body.String(), "已有模型更新任务正在运行或等待中")
}
