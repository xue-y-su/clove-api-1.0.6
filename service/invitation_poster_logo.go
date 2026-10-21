package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/image/webp"
	"golang.org/x/sync/singleflight"
)

const (
	invitationPosterLogoMaxBytes       = int64(2 * 1024 * 1024)
	invitationPosterLogoMaxPixels      = int64(16 * 1024 * 1024)
	invitationPosterLogoTimeout        = 5 * time.Second
	invitationPosterLogoCacheTTL       = 5 * time.Minute
	invitationPosterLogoFailureBackoff = 15 * time.Second
)

var ErrPosterLogoUnavailable = errors.New("poster logo unavailable")

type PosterLogo struct {
	Body        []byte
	ContentType string
	ETag        string
	Width       int
	Height      int
}

type posterLogoLoader struct {
	client    *http.Client
	timeout   time.Duration
	maxBytes  int64
	maxPixels int64
	now       func() time.Time
	validate  func(context.Context, string) error

	cacheMutex  sync.Mutex
	cachedURL   string
	cachedLogo  PosterLogo
	cachedUntil time.Time
	failedURL   string
	failedUntil time.Time
	requests    singleflight.Group
}

var invitationPosterLogoLoader = posterLogoLoader{
	timeout:   invitationPosterLogoTimeout,
	maxBytes:  invitationPosterLogoMaxBytes,
	maxPixels: invitationPosterLogoMaxPixels,
	validate:  ValidateSSRFProtectedFetchURLContext,
}

func FetchInvitationPosterLogo(ctx context.Context, logoURL string) (PosterLogo, error) {
	return invitationPosterLogoLoader.load(ctx, logoURL)
}

func (loader *posterLogoLoader) load(ctx context.Context, logoURL string) (PosterLogo, error) {
	logoURL = strings.TrimSpace(logoURL)
	if err := validatePosterLogoURL(logoURL); err != nil {
		return PosterLogo{}, err
	}
	if loader.timeout <= 0 || loader.maxBytes <= 0 || loader.maxPixels <= 0 {
		return PosterLogo{}, fmt.Errorf("%w: invalid loader limits", ErrPosterLogoUnavailable)
	}

	resultChannel := loader.requests.DoChan(logoURL, func() (any, error) {
		operationContext, cancel := context.WithTimeout(context.Background(), loader.timeout)
		defer cancel()
		now := loader.currentTime()
		if loader.failureBackoffActive(logoURL, now) {
			return nil, fmt.Errorf("%w: upstream retry deferred", ErrPosterLogoUnavailable)
		}
		if loader.validate != nil {
			if err := loader.validate(operationContext, logoURL); err != nil {
				loader.invalidate(logoURL)
				return nil, fmt.Errorf("%w: URL blocked by fetch policy: %w", ErrPosterLogoUnavailable, err)
			}
		}
		if logo, ok := loader.readCached(logoURL, now); ok {
			return logo, nil
		}
		logo, err := loader.fetch(operationContext, logoURL)
		if err != nil {
			loader.rememberFailure(logoURL, loader.currentTime())
			return nil, err
		}
		loader.rememberSuccess(logoURL, logo, loader.currentTime())
		return logo, nil
	})
	select {
	case <-ctx.Done():
		return PosterLogo{}, fmt.Errorf("%w: request canceled: %w", ErrPosterLogoUnavailable, ctx.Err())
	case result := <-resultChannel:
		if result.Err != nil {
			return PosterLogo{}, result.Err
		}
		logo, ok := result.Val.(PosterLogo)
		if !ok {
			return PosterLogo{}, fmt.Errorf("%w: invalid shared result", ErrPosterLogoUnavailable)
		}
		return clonePosterLogo(logo), nil
	}
}

func (loader *posterLogoLoader) currentTime() time.Time {
	if loader.now != nil {
		return loader.now()
	}
	return time.Now()
}

func (loader *posterLogoLoader) readCached(logoURL string, now time.Time) (PosterLogo, bool) {
	loader.cacheMutex.Lock()
	defer loader.cacheMutex.Unlock()
	if loader.cachedURL != logoURL || len(loader.cachedLogo.Body) == 0 || !now.Before(loader.cachedUntil) {
		if loader.cachedURL == logoURL {
			loader.cachedURL = ""
			loader.cachedLogo = PosterLogo{}
			loader.cachedUntil = time.Time{}
		}
		return PosterLogo{}, false
	}
	return clonePosterLogo(loader.cachedLogo), true
}

func (loader *posterLogoLoader) failureBackoffActive(logoURL string, now time.Time) bool {
	loader.cacheMutex.Lock()
	defer loader.cacheMutex.Unlock()
	if loader.failedURL != logoURL || !now.Before(loader.failedUntil) {
		if loader.failedURL == logoURL {
			loader.failedURL = ""
			loader.failedUntil = time.Time{}
		}
		return false
	}
	return true
}

func (loader *posterLogoLoader) rememberFailure(logoURL string, now time.Time) {
	loader.cacheMutex.Lock()
	defer loader.cacheMutex.Unlock()
	loader.failedURL = logoURL
	loader.failedUntil = now.Add(invitationPosterLogoFailureBackoff)
}

func (loader *posterLogoLoader) rememberSuccess(logoURL string, logo PosterLogo, now time.Time) {
	loader.cacheMutex.Lock()
	defer loader.cacheMutex.Unlock()
	loader.cachedURL = logoURL
	loader.cachedLogo = clonePosterLogo(logo)
	loader.cachedUntil = now.Add(invitationPosterLogoCacheTTL)
	loader.failedURL = ""
	loader.failedUntil = time.Time{}
}

func (loader *posterLogoLoader) invalidate(logoURL string) {
	loader.cacheMutex.Lock()
	defer loader.cacheMutex.Unlock()
	if loader.cachedURL == logoURL {
		loader.cachedURL = ""
		loader.cachedLogo = PosterLogo{}
		loader.cachedUntil = time.Time{}
	}
	if loader.failedURL == logoURL {
		loader.failedURL = ""
		loader.failedUntil = time.Time{}
	}
}

func (loader *posterLogoLoader) fetch(ctx context.Context, logoURL string) (PosterLogo, error) {
	client := loader.client
	if client == nil {
		client = GetSSRFProtectedHTTPClient()
	}
	if client == nil {
		return PosterLogo{}, fmt.Errorf("%w: HTTP client is not initialized", ErrPosterLogoUnavailable)
	}

	requestContext, cancel := context.WithTimeout(ctx, loader.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, logoURL, nil)
	if err != nil {
		return PosterLogo{}, fmt.Errorf("%w: invalid request: %w", ErrPosterLogoUnavailable, err)
	}
	request.Header.Set("Accept", "image/png, image/jpeg, image/gif, image/webp")

	response, err := client.Do(request)
	if err != nil {
		return PosterLogo{}, fmt.Errorf("%w: upstream request failed: %w", ErrPosterLogoUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return PosterLogo{}, fmt.Errorf("%w: upstream returned HTTP %d", ErrPosterLogoUnavailable, response.StatusCode)
	}
	if response.ContentLength > loader.maxBytes {
		return PosterLogo{}, fmt.Errorf("%w: image exceeds byte limit", ErrPosterLogoUnavailable)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, loader.maxBytes+1))
	if err != nil {
		return PosterLogo{}, fmt.Errorf("%w: reading image failed: %w", ErrPosterLogoUnavailable, err)
	}
	if int64(len(body)) > loader.maxBytes {
		return PosterLogo{}, fmt.Errorf("%w: image exceeds byte limit", ErrPosterLogoUnavailable)
	}

	config, format, contentType, err := detectPosterLogo(body)
	if err != nil {
		return PosterLogo{}, fmt.Errorf("%w: invalid image: %w", ErrPosterLogoUnavailable, err)
	}
	if config.Width <= 0 || config.Height <= 0 || int64(config.Width) > loader.maxPixels/int64(config.Height) {
		return PosterLogo{}, fmt.Errorf("%w: image exceeds pixel limit", ErrPosterLogoUnavailable)
	}
	if err := decodePosterLogo(body, format); err != nil {
		return PosterLogo{}, fmt.Errorf("%w: invalid image: %w", ErrPosterLogoUnavailable, err)
	}

	digest := sha256.New()
	_, _ = io.WriteString(digest, logoURL)
	_, _ = digest.Write([]byte{0})
	_, _ = digest.Write(body)
	logo := PosterLogo{
		Body:        body,
		ContentType: contentType,
		ETag:        fmt.Sprintf("\"%x\"", digest.Sum(nil)),
		Width:       config.Width,
		Height:      config.Height,
	}
	return logo, nil
}

func validatePosterLogoURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("%w: logo URL is empty", ErrPosterLogoUnavailable)
	}
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" || !parsed.IsAbs() {
		return fmt.Errorf("%w: logo URL must be absolute", ErrPosterLogoUnavailable)
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		return nil
	default:
		return fmt.Errorf("%w: logo URL protocol is unsupported", ErrPosterLogoUnavailable)
	}
}

func detectPosterLogo(body []byte) (image.Config, string, string, error) {
	config, format, err := image.DecodeConfig(bytes.NewReader(body))
	if err != nil {
		config, err = webp.DecodeConfig(bytes.NewReader(body))
		if err != nil {
			return image.Config{}, "", "", err
		}
		format = "webp"
	}

	contentType := ""
	switch strings.ToLower(format) {
	case "png":
		contentType = "image/png"
	case "jpeg", "jpg":
		contentType = "image/jpeg"
	case "gif":
		contentType = "image/gif"
	case "webp":
		contentType = "image/webp"
	default:
		return image.Config{}, "", "", fmt.Errorf("unsupported image format %q", format)
	}
	return config, format, contentType, nil
}

func decodePosterLogo(body []byte, format string) error {
	var err error
	if format == "webp" {
		_, err = webp.Decode(bytes.NewReader(body))
	} else {
		_, _, err = image.Decode(bytes.NewReader(body))
	}
	return err
}

func clonePosterLogo(logo PosterLogo) PosterLogo {
	logo.Body = bytes.Clone(logo.Body)
	return logo
}
