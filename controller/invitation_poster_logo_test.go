package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setInvitationPosterLogoTestDependencies(
	t *testing.T,
	logoURL string,
	fetch func(context.Context, string) (service.PosterLogo, error),
) {
	t.Helper()
	common.OptionMapRWMutex.Lock()
	previousLogo := common.Logo
	common.Logo = logoURL
	common.OptionMapRWMutex.Unlock()
	previousFetch := fetchInvitationPosterLogo
	fetchInvitationPosterLogo = fetch
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.Logo = previousLogo
		common.OptionMapRWMutex.Unlock()
		fetchInvitationPosterLogo = previousFetch
	})
}

func newInvitationPosterLogoContext(t *testing.T, target string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, target, nil)
	return ctx, recorder
}

func TestGetInvitationPosterLogoUsesConfiguredURLAndImageHeaders(t *testing.T) {
	const configuredURL = "https://configured.test/logo.webp"
	body := []byte("validated-webp")
	var fetchedURL string
	setInvitationPosterLogoTestDependencies(t, configuredURL, func(_ context.Context, logoURL string) (service.PosterLogo, error) {
		fetchedURL = logoURL
		return service.PosterLogo{
			Body:        body,
			ContentType: "image/webp",
			ETag:        `"poster-etag"`,
		}, nil
	})
	ctx, recorder := newInvitationPosterLogoContext(t, "/api/invitation/poster-logo?url=https://ignored.test/private.png")

	GetInvitationPosterLogo(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, configuredURL, fetchedURL)
	require.Equal(t, "image/webp", recorder.Header().Get("Content-Type"))
	require.Equal(t, `"poster-etag"`, recorder.Header().Get("ETag"))
	require.Equal(t, "public, max-age=300", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
	require.Equal(t, body, recorder.Body.Bytes())
}

func TestGetInvitationPosterLogoReturnsNotModifiedForMatchingETag(t *testing.T) {
	setInvitationPosterLogoTestDependencies(t, "https://configured.test/logo.png", func(_ context.Context, _ string) (service.PosterLogo, error) {
		return service.PosterLogo{
			Body:        []byte("validated-png"),
			ContentType: "image/png",
			ETag:        `"poster-etag"`,
		}, nil
	})
	ctx, recorder := newInvitationPosterLogoContext(t, "/api/invitation/poster-logo")
	ctx.Request.Header.Set("If-None-Match", `"poster-etag"`)
	require.Equal(t, `"poster-etag"`, ctx.GetHeader("If-None-Match"))

	GetInvitationPosterLogo(ctx)
	ctx.Writer.WriteHeaderNow()

	require.Equal(t, http.StatusNotModified, recorder.Code)
	require.Equal(t, `"poster-etag"`, recorder.Header().Get("ETag"))
	require.Empty(t, recorder.Body.Bytes())
}

func TestGetInvitationPosterLogoReturnsNonImageError(t *testing.T) {
	setInvitationPosterLogoTestDependencies(t, "", func(_ context.Context, logoURL string) (service.PosterLogo, error) {
		require.Empty(t, logoURL)
		return service.PosterLogo{}, service.ErrPosterLogoUnavailable
	})
	ctx, recorder := newInvitationPosterLogoContext(t, "/api/invitation/poster-logo?url=https://ignored.test/logo.png")

	GetInvitationPosterLogo(ctx)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.Contains(t, recorder.Header().Get("Content-Type"), "application/json")
	require.NotContains(t, recorder.Header().Get("Content-Type"), "image/")
	var response struct {
		Success bool `json:"success"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
}

func TestGetInvitationPosterLogoDoesNotExposeFetcherErrors(t *testing.T) {
	setInvitationPosterLogoTestDependencies(t, "https://configured.test/logo.png", func(_ context.Context, _ string) (service.PosterLogo, error) {
		return service.PosterLogo{}, errors.New("upstream secret response body")
	})
	ctx, recorder := newInvitationPosterLogoContext(t, "/api/invitation/poster-logo")

	GetInvitationPosterLogo(ctx)

	require.Equal(t, http.StatusNotFound, recorder.Code)
	require.NotContains(t, recorder.Body.String(), "secret")
}
