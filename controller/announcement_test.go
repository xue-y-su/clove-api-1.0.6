package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	appi18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/announcement_setting"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnnouncementAPIErrorReturnsStableSafeReasons(t *testing.T) {
	require.NoError(t, appi18n.Init())
	tests := []struct {
		name string
		err  error
		code string
	}{
		{name: "missing media", err: errAnnouncementMediaRequired, code: "announcement_media_required"},
		{name: "unsupported media", err: service.ErrAnnouncementMediaUnsupported, code: "announcement_media_unsupported"},
		{name: "oversized media", err: service.ErrAnnouncementMediaTooLarge, code: "announcement_media_too_large"},
		{name: "invalid dimensions", err: service.ErrAnnouncementMediaDimensions, code: "announcement_media_dimensions"},
		{name: "storage failure", err: fmt.Errorf("%w: open C:\\private\\announcement.png", service.ErrAnnouncementMediaStorage), code: "announcement_media_storage"},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodPost, "/api/announcement/media", nil)
			context.Request.Header.Set("Accept-Language", "zh-CN")
			context.Set(common.RequestIdKey, "announcement-request")

			announcementAPIError(context, testCase.err)

			var response struct {
				Success   bool   `json:"success"`
				Code      string `json:"code"`
				Message   string `json:"message"`
				RequestID string `json:"request_id"`
			}
			require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
			require.False(t, response.Success)
			require.Equal(t, testCase.code, response.Code)
			require.NotEmpty(t, response.Message)
			require.NotContains(t, response.Message, "C:\\private")
			require.NotContains(t, response.Message, "announcement media")
			require.Equal(t, "announcement-request", response.RequestID)
		})
	}
}

func setupAnnouncementControllerTest(t *testing.T) {
	t.Helper()
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.AnnouncementAcknowledgement{},
		&model.IPAuditRollup{},
		&model.IPSecurityEvent{},
		&model.Log{},
		&model.Option{},
		&model.User{},
	))
	previousConsole := *console_setting.GetConsoleSetting()
	common.OptionMapRWMutex.Lock()
	previousOptions := common.OptionMap
	common.OptionMap = map[string]string{announcement_setting.PopupEnabledKey: "true"}
	common.OptionMapRWMutex.Unlock()
	console_setting.GetConsoleSetting().AnnouncementsEnabled = true
	payload, err := common.Marshal([]service.Announcement{{
		ID: 9, Content: "Controller announcement", PublishDate: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339), Type: "default", Popup: true,
	}})
	require.NoError(t, err)
	console_setting.GetConsoleSetting().Announcements = string(payload)
	t.Cleanup(func() {
		*console_setting.GetConsoleSetting() = previousConsole
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptions
		common.OptionMapRWMutex.Unlock()
	})
}

func TestGetAnnouncementPopupStateRejectsGuests(t *testing.T) {
	setupAnnouncementControllerTest(t)
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/announcement/popup-state", nil, 0)

	GetAnnouncementPopupState(ctx)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	response := decodeAPIResponse(t, recorder)
	assert.False(t, response.Success)
	assert.NotEmpty(t, response.Message)
}

func TestAcknowledgeAnnouncementPersistsAndRejectsUnknownSignatures(t *testing.T) {
	setupAnnouncementControllerTest(t)
	state, err := service.GetAnnouncementPopupState(51, time.Now())
	require.NoError(t, err)

	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/announcement/acknowledge", map[string]any{
		"signature": state.Signature,
	}, 51)
	AcknowledgeAnnouncement(ctx)
	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	acknowledged, err := model.HasAnnouncementAcknowledgement(51, state.Signature)
	require.NoError(t, err)
	assert.True(t, acknowledged)

	ctx, recorder = newAuthenticatedContext(t, http.MethodPost, "/api/announcement/acknowledge", map[string]any{
		"signature": strings.Repeat("f", 64),
	}, 51)
	AcknowledgeAnnouncement(ctx)
	response = decodeAPIResponse(t, recorder)
	assert.False(t, response.Success)
}

func TestAnnouncementMediaUploadAndReadUseManagedURLAndImmutableHeaders(t *testing.T) {
	setupAnnouncementControllerTest(t)
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", t.TempDir())
	data := controllerAnnouncementTestPNG()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "../../unsafe.svg")
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/announcement/media", &body)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	ctx.Set("id", 1)
	ctx.Set("role", common.RoleRootUser)
	UploadAnnouncementMedia(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	payload, err := common.Marshal(response.Data)
	require.NoError(t, err)
	var uploaded service.AnnouncementMedia
	require.NoError(t, common.Unmarshal(payload, &uploaded))
	filename := path.Base(uploaded.URL)
	assert.Regexp(t, `^[a-f0-9]{64}\.png$`, filename)
	assert.NotContains(t, uploaded.URL, "unsafe")

	readRecorder := httptest.NewRecorder()
	readContext, _ := gin.CreateTestContext(readRecorder)
	readContext.Request = httptest.NewRequest(http.MethodGet, uploaded.URL, nil)
	readContext.Params = gin.Params{{Key: "filename", Value: filename}}
	GetAnnouncementMedia(readContext)

	assert.Equal(t, http.StatusOK, readRecorder.Code)
	assert.Equal(t, "image/png", readRecorder.Header().Get("Content-Type"))
	assert.Equal(t, "public, max-age=31536000, immutable", readRecorder.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", readRecorder.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, data, readRecorder.Body.Bytes())
}

func TestAnnouncementMediaUploadRejectsMissingFile(t *testing.T) {
	setupAnnouncementControllerTest(t)
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", t.TempDir())
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/announcement/media", nil)
	ctx.Set("id", 1)
	UploadAnnouncementMedia(ctx)

	response := decodeAPIResponse(t, recorder)
	assert.False(t, response.Success)
}

func TestAnnouncementOptionSaveCleansExpiredUnreferencedMedia(t *testing.T) {
	setupAnnouncementControllerTest(t)
	mediaDir := t.TempDir()
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", mediaDir)
	orphanData := []byte("expired announcement media")
	digest := sha256.Sum256(orphanData)
	filename := hex.EncodeToString(digest[:]) + ".png"
	pathName := filepath.Join(mediaDir, filename)
	require.NoError(t, os.WriteFile(pathName, orphanData, 0o600))
	oldTime := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(pathName, oldTime, oldTime))

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]any{
		"key":   "console_setting.announcements",
		"value": "[]",
	}, 1)
	UpdateOption(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.NoFileExists(t, pathName)
}

func TestAnnouncementOptionSaveDefersRecentMediaDeletion(t *testing.T) {
	setupAnnouncementControllerTest(t)
	mediaDir := t.TempDir()
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", mediaDir)
	filename := strings.Repeat("a", 64) + ".png"
	pathName := filepath.Join(mediaDir, filename)
	require.NoError(t, os.WriteFile(pathName, []byte("recent announcement media"), 0o600))
	console_setting.GetConsoleSetting().Announcements = `[{"id":1,"content":"old","publishDate":"2026-07-21T00:00:00Z","type":"default","imageUrl":"/api/announcement-media/` + filename + `"}]`

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]any{
		"key":   "console_setting.announcements",
		"value": "[]",
	}, 1)
	UpdateOption(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	assert.FileExists(t, pathName)
}

func TestAnnouncementOptionSavePersistsNormalizedContent(t *testing.T) {
	setupAnnouncementControllerTest(t)
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", t.TempDir())
	value := `[{"id":1,"title":" <b>Release</b> ","content":"<p>Hello<script>alert(1)</script><strong>world</strong></p>","publishDate":"2026-07-21T00:00:00Z"}]`

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]any{
		"key":   "console_setting.announcements",
		"value": value,
	}, 1)
	UpdateOption(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var stored []map[string]any
	require.NoError(t, common.Unmarshal([]byte(console_setting.GetConsoleSetting().Announcements), &stored))
	require.Len(t, stored, 1)
	assert.Equal(t, "Release", stored[0]["title"])
	assert.Equal(t, true, stored[0]["published"])
	assert.Equal(t, "default", stored[0]["type"])
	assert.Equal(t, "<p>Hello<strong>world</strong></p>", stored[0]["content"])
}

func TestAnnouncementOptionSaveRejectsMissingManagedMedia(t *testing.T) {
	setupAnnouncementControllerTest(t)
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", t.TempDir())
	previous := console_setting.GetConsoleSetting().Announcements
	missingFilename := strings.Repeat("d", 64) + ".png"
	value := `[{"id":1,"content":"missing","publishDate":"2026-07-21T00:00:00Z","type":"default","imageUrl":"/api/announcement-media/` + missingFilename + `"}]`

	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/option/", map[string]any{
		"key":   "console_setting.announcements",
		"value": value,
	}, 1)
	UpdateOption(ctx)

	response := decodeAPIResponse(t, recorder)
	assert.False(t, response.Success)
	assert.Equal(t, previous, console_setting.GetConsoleSetting().Announcements)
}

func controllerAnnouncementTestPNG() []byte {
	imageData := image.NewRGBA(image.Rect(0, 0, 2, 2))
	for y := 0; y < 2; y++ {
		for x := 0; x < 2; x++ {
			imageData.Set(x, y, color.RGBA{R: 20, G: 30, B: 40, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, imageData); err != nil {
		panic(err)
	}
	return buffer.Bytes()
}
