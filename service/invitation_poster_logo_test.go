package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	testPosterLogoMaxBytes  = int64(2 * 1024 * 1024)
	testPosterLogoMaxPixels = int64(16 * 1024 * 1024)
)

func TestPosterLogoLoaderLoadsNoCORSWebPAndCachesACopy(t *testing.T) {
	body := posterLogoWebPFixture(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer server.Close()

	loader := posterLogoLoader{
		client:    server.Client(),
		timeout:   time.Second,
		maxBytes:  testPosterLogoMaxBytes,
		maxPixels: testPosterLogoMaxPixels,
	}

	first, err := loader.load(context.Background(), server.URL+"/logo.webp")
	require.NoError(t, err)
	require.Equal(t, "image/webp", first.ContentType)
	require.Equal(t, body, first.Body)
	require.Positive(t, first.Width)
	require.Positive(t, first.Height)
	digest := sha256.Sum256(append([]byte(server.URL+"/logo.webp\x00"), body...))
	require.Equal(t, fmt.Sprintf("\"%x\"", digest), first.ETag)
	require.EqualValues(t, 1, requests.Load())

	first.Body[0] ^= 0xff
	second, err := loader.load(context.Background(), server.URL+"/logo.webp")
	require.NoError(t, err)
	require.Equal(t, body, second.Body)
	require.EqualValues(t, 1, requests.Load())
}

func TestPosterLogoLoaderDetectsSupportedImageTypesFromBytes(t *testing.T) {
	fixtures := []struct {
		name        string
		body        []byte
		contentType string
	}{
		{name: "png", body: posterLogoPNG(t, 2, 3), contentType: "image/png"},
		{name: "jpeg", body: posterLogoJPEG(t, 2, 3), contentType: "image/jpeg"},
		{name: "gif", body: posterLogoGIF(t, 2, 3), contentType: "image/gif"},
		{name: "webp", body: posterLogoWebPFixture(t), contentType: "image/webp"},
	}

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				_, _ = w.Write(fixture.body)
			}))
			defer server.Close()
			loader := posterLogoLoader{
				client: server.Client(), timeout: time.Second,
				maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
			}

			logo, err := loader.load(context.Background(), server.URL+"/logo")
			require.NoError(t, err)
			require.Equal(t, fixture.contentType, logo.ContentType)
			require.Positive(t, logo.Width)
			require.Positive(t, logo.Height)
		})
	}
}

func TestPosterLogoLoaderRejectsInvalidURLBeforeRequest(t *testing.T) {
	var requests atomic.Int32
	client := &http.Client{Transport: posterLogoRoundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, fmt.Errorf("unexpected request")
	})}
	loader := posterLogoLoader{
		client: client, timeout: time.Second,
		maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
	}

	for _, rawURL := range []string{"", "/logo.png", "data:image/png;base64,AA==", "ftp://example.com/logo.png", "https:///logo.png"} {
		t.Run(rawURL, func(t *testing.T) {
			_, err := loader.load(context.Background(), rawURL)
			require.ErrorIs(t, err, ErrPosterLogoUnavailable)
		})
	}
	require.Zero(t, requests.Load())
}

func TestPosterLogoLoaderRejectsNonSuccessResponses(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				http.Error(w, "upstream failure", status)
			}))
			defer server.Close()
			loader := posterLogoLoader{
				client: server.Client(), timeout: time.Second,
				maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
			}

			_, err := loader.load(context.Background(), server.URL+"/logo")
			require.ErrorIs(t, err, ErrPosterLogoUnavailable)
		})
	}
}

func TestPosterLogoLoaderRejectsBodyOverByteLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.(http.Flusher).Flush()
		_, _ = w.Write(bytes.Repeat([]byte{'x'}, 65))
	}))
	defer server.Close()
	loader := posterLogoLoader{
		client: server.Client(), timeout: time.Second, maxBytes: 64,
		maxPixels: testPosterLogoMaxPixels,
	}

	_, err := loader.load(context.Background(), server.URL+"/logo")
	require.ErrorIs(t, err, ErrPosterLogoUnavailable)
	require.Contains(t, err.Error(), "byte limit")
}

func TestPosterLogoLoaderRejectsInvalidImageBytes(t *testing.T) {
	truncatedPNG := posterLogoPNG(t, 1, 1)[:33]
	for _, fixture := range []struct {
		name        string
		contentType string
		body        []byte
	}{
		{name: "random", contentType: "application/octet-stream", body: []byte("not-an-image")},
		{name: "mislabeled-html", contentType: "image/png", body: []byte("<html>not an image</html>")},
		{name: "truncated-png", contentType: "image/png", body: truncatedPNG},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", fixture.contentType)
				_, _ = w.Write(fixture.body)
			}))
			defer server.Close()
			loader := posterLogoLoader{
				client: server.Client(), timeout: time.Second,
				maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
			}

			_, err := loader.load(context.Background(), server.URL+"/logo")
			require.ErrorIs(t, err, ErrPosterLogoUnavailable)
		})
	}
}

func TestPosterLogoLoaderRejectsImagesOverPixelLimit(t *testing.T) {
	body := posterLogoPNGWithDimensions(t, 4097, 4096)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	defer server.Close()
	loader := posterLogoLoader{
		client: server.Client(), timeout: time.Second,
		maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
	}

	_, err := loader.load(context.Background(), server.URL+"/logo.png")
	require.ErrorIs(t, err, ErrPosterLogoUnavailable)
	require.Contains(t, err.Error(), "pixel limit")
}

func TestPosterLogoLoaderTimesOutSlowUpstream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	loader := posterLogoLoader{
		client: server.Client(), timeout: 20 * time.Millisecond,
		maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
	}

	started := time.Now()
	_, err := loader.load(context.Background(), server.URL+"/slow-logo")
	require.ErrorIs(t, err, ErrPosterLogoUnavailable)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(started), time.Second)
}

func TestPosterLogoLoaderPropagatesClientRejectedRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Redirect(w, &http.Request{}, "http://127.0.0.1/private-logo", http.StatusFound)
	}))
	defer server.Close()
	client := server.Client()
	client.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
		if req.URL.Hostname() == "127.0.0.1" {
			return fmt.Errorf("private redirect blocked")
		}
		return nil
	}
	loader := posterLogoLoader{
		client: client, timeout: time.Second,
		maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
	}

	_, err := loader.load(context.Background(), server.URL+"/redirect")
	require.ErrorIs(t, err, ErrPosterLogoUnavailable)
	require.Contains(t, err.Error(), "private redirect blocked")
}

func TestPosterLogoLoaderBacksOffSequentialFailures(t *testing.T) {
	body := posterLogoPNG(t, 1, 1)
	var requests atomic.Int32
	now := time.Unix(1_785_000_000, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			http.Error(w, "temporary failure", http.StatusBadGateway)
			return
		}
		_, _ = w.Write(body)
	}))
	defer server.Close()
	loader := posterLogoLoader{
		client: server.Client(), timeout: time.Second,
		maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
		now: func() time.Time { return now },
	}

	_, err := loader.load(context.Background(), server.URL+"/logo")
	require.ErrorIs(t, err, ErrPosterLogoUnavailable)
	_, err = loader.load(context.Background(), server.URL+"/logo")
	require.ErrorIs(t, err, ErrPosterLogoUnavailable)
	require.EqualValues(t, 1, requests.Load())

	now = now.Add(invitationPosterLogoFailureBackoff + time.Nanosecond)
	_, err = loader.load(context.Background(), server.URL+"/logo")
	require.NoError(t, err)
	_, err = loader.load(context.Background(), server.URL+"/logo")
	require.NoError(t, err)
	require.EqualValues(t, 2, requests.Load())
}

func TestPosterLogoLoaderExpiresSuccessfulCache(t *testing.T) {
	firstBody := posterLogoPNG(t, 1, 1)
	secondBody := posterLogoPNG(t, 2, 1)
	var requests atomic.Int32
	now := time.Unix(1_785_000_000, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) == 1 {
			_, _ = w.Write(firstBody)
			return
		}
		_, _ = w.Write(secondBody)
	}))
	defer server.Close()
	loader := posterLogoLoader{
		client: server.Client(), timeout: time.Second,
		maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
		now: func() time.Time { return now },
	}

	first, err := loader.load(context.Background(), server.URL+"/logo")
	require.NoError(t, err)
	now = now.Add(invitationPosterLogoCacheTTL - time.Nanosecond)
	cached, err := loader.load(context.Background(), server.URL+"/logo")
	require.NoError(t, err)
	require.Equal(t, firstBody, first.Body)
	require.Equal(t, firstBody, cached.Body)
	require.EqualValues(t, 1, requests.Load())

	now = now.Add(2 * time.Nanosecond)
	refreshed, err := loader.load(context.Background(), server.URL+"/logo")
	require.NoError(t, err)
	require.Equal(t, secondBody, refreshed.Body)
	require.EqualValues(t, 2, requests.Load())
}

func TestPosterLogoLoaderRevalidatesPolicyBeforeReturningCache(t *testing.T) {
	body := posterLogoPNG(t, 1, 1)
	var requests atomic.Int32
	var blocked atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(body)
	}))
	defer server.Close()
	loader := posterLogoLoader{
		client: server.Client(), timeout: time.Second,
		maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
		validate: func(_ context.Context, _ string) error {
			if blocked.Load() {
				return fmt.Errorf("blocked by current policy")
			}
			return nil
		},
	}

	_, err := loader.load(context.Background(), server.URL+"/logo")
	require.NoError(t, err)
	blocked.Store(true)
	_, err = loader.load(context.Background(), server.URL+"/logo")
	require.ErrorIs(t, err, ErrPosterLogoUnavailable)
	require.EqualValues(t, 1, requests.Load())

	blocked.Store(false)
	_, err = loader.load(context.Background(), server.URL+"/logo")
	require.NoError(t, err)
	require.EqualValues(t, 2, requests.Load())
}

func TestPosterLogoLoaderBoundsAndCoalescesPolicyValidation(t *testing.T) {
	body := posterLogoPNG(t, 1, 1)
	var validations atomic.Int32
	var requests atomic.Int32
	validationStarted := make(chan struct{}, 1)
	releaseValidation := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		_, _ = w.Write(body)
	}))
	defer server.Close()
	loader := posterLogoLoader{
		client: server.Client(), timeout: time.Second,
		maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
		validate: func(ctx context.Context, _ string) error {
			validations.Add(1)
			select {
			case validationStarted <- struct{}{}:
			default:
			}
			select {
			case <-releaseValidation:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}

	const callers = 8
	start := make(chan struct{})
	errors := make(chan error, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	for range callers {
		go func() {
			ready.Done()
			<-start
			_, err := loader.load(context.Background(), server.URL+"/logo")
			errors <- err
		}()
	}
	ready.Wait()
	close(start)
	<-validationStarted
	time.Sleep(20 * time.Millisecond)
	close(releaseValidation)
	for range callers {
		require.NoError(t, <-errors)
	}
	require.EqualValues(t, 1, validations.Load())
	require.EqualValues(t, 1, requests.Load())
}

func TestPosterLogoLoaderBoundsSlowPolicyValidation(t *testing.T) {
	loader := posterLogoLoader{
		client: &http.Client{}, timeout: 20 * time.Millisecond,
		maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
		validate: func(ctx context.Context, _ string) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}

	started := time.Now()
	_, err := loader.load(context.Background(), "https://logo.example/image.webp")

	require.ErrorIs(t, err, ErrPosterLogoUnavailable)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	require.Less(t, time.Since(started), time.Second)
}

func TestPosterLogoLoaderCoalescesConcurrentFailures(t *testing.T) {
	var requests atomic.Int32
	requestStarted := make(chan struct{}, 1)
	releaseRequest := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests.Add(1)
		select {
		case requestStarted <- struct{}{}:
		default:
		}
		<-releaseRequest
		http.Error(w, "temporary failure", http.StatusBadGateway)
	}))
	defer server.Close()
	loader := posterLogoLoader{
		client: server.Client(), timeout: time.Second,
		maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
	}

	const callers = 8
	start := make(chan struct{})
	errors := make(chan error, callers)
	var ready sync.WaitGroup
	ready.Add(callers)
	for range callers {
		go func() {
			ready.Done()
			<-start
			_, err := loader.load(context.Background(), server.URL+"/logo")
			errors <- err
		}()
	}
	ready.Wait()
	close(start)
	<-requestStarted
	time.Sleep(20 * time.Millisecond)
	close(releaseRequest)

	for range callers {
		require.ErrorIs(t, <-errors, ErrPosterLogoUnavailable)
	}
	require.EqualValues(t, 1, requests.Load())
}

func TestPosterLogoLoaderCachesOnlyTheMostRecentURL(t *testing.T) {
	body := posterLogoPNG(t, 1, 1)
	var firstRequests atomic.Int32
	var secondRequests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/first":
			firstRequests.Add(1)
		case "/second":
			secondRequests.Add(1)
		default:
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	defer server.Close()
	loader := posterLogoLoader{
		client: server.Client(), timeout: time.Second,
		maxBytes: testPosterLogoMaxBytes, maxPixels: testPosterLogoMaxPixels,
	}

	_, err := loader.load(context.Background(), server.URL+"/first")
	require.NoError(t, err)
	_, err = loader.load(context.Background(), server.URL+"/second")
	require.NoError(t, err)
	_, err = loader.load(context.Background(), server.URL+"/first")
	require.NoError(t, err)
	require.EqualValues(t, 2, firstRequests.Load())
	require.EqualValues(t, 1, secondRequests.Load())
}

type posterLogoRoundTripFunc func(*http.Request) (*http.Response, error)

func (f posterLogoRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func posterLogoTestImage(width, height int) *image.RGBA {
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			result.SetRGBA(x, y, color.RGBA{R: uint8(50 + x), G: uint8(80 + y), B: 120, A: 255})
		}
	}
	return result
}

func posterLogoPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, png.Encode(&buffer, posterLogoTestImage(width, height)))
	return buffer.Bytes()
}

func posterLogoJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, jpeg.Encode(&buffer, posterLogoTestImage(width, height), nil))
	return buffer.Bytes()
}

func posterLogoGIF(t *testing.T, width, height int) []byte {
	t.Helper()
	var buffer bytes.Buffer
	require.NoError(t, gif.Encode(&buffer, posterLogoTestImage(width, height), nil))
	return buffer.Bytes()
}

func posterLogoWebPFixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("..", "web", "snow", "dist", "assets", "images", "ui", "clove-logo.webp"))
	require.NoError(t, err)
	return body
}

func posterLogoPNGWithDimensions(t *testing.T, width, height uint32) []byte {
	t.Helper()
	body := posterLogoPNG(t, 1, 1)
	require.True(t, bytes.HasPrefix(body, []byte("\x89PNG\r\n\x1a\n")))
	binary.BigEndian.PutUint32(body[16:20], width)
	binary.BigEndian.PutUint32(body[20:24], height)
	binary.BigEndian.PutUint32(body[29:33], crc32.ChecksumIEEE(body[12:29]))
	return body
}

func TestPosterLogoProductionLimits(t *testing.T) {
	require.Equal(t, int64(2*1024*1024), invitationPosterLogoMaxBytes)
	require.Equal(t, int64(16*1024*1024), invitationPosterLogoMaxPixels)
	require.Equal(t, 5*time.Second, invitationPosterLogoTimeout)
	require.Equal(t, 5*time.Minute, invitationPosterLogoCacheTTL)
	require.Equal(t, 15*time.Second, invitationPosterLogoFailureBackoff)
	require.NotNil(t, FetchInvitationPosterLogo)
	require.False(t, strings.Contains(fmt.Sprint(ErrPosterLogoUnavailable), "http"))
}
