package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStoreAnnouncementMediaUsesContentAddressedPathAndDeduplicates(t *testing.T) {
	mediaDir := t.TempDir()
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", mediaDir)
	data := announcementTestPNG(2, 3)

	first, err := StoreAnnouncementMedia(bytes.NewReader(data), int64(len(data)+1))
	require.NoError(t, err)
	digest := sha256.Sum256(data)
	filename := hex.EncodeToString(digest[:]) + ".png"
	assert.Equal(t, filename, first.Filename)
	assert.Equal(t, "/api/announcement-media/"+filename, first.URL)
	assert.Equal(t, "image/png", first.ContentType)
	assert.Equal(t, int64(len(data)), first.Size)

	second, err := StoreAnnouncementMedia(bytes.NewReader(data), int64(len(data)+1))
	require.NoError(t, err)
	assert.Equal(t, first, second)

	entries, err := os.ReadDir(mediaDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, filename, entries[0].Name())
}

func TestEnsureAnnouncementMediaDirectoryCreatesWritableDirectory(t *testing.T) {
	mediaDir := filepath.Join(t.TempDir(), "announcement-media")
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", mediaDir)

	require.NoError(t, EnsureAnnouncementMediaDirectory())
	info, err := os.Stat(mediaDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
	assert.NoFileExists(t, filepath.Join(mediaDir, ".announcement-media-probe"))
}

func TestEnsureAnnouncementMediaDirectoryReportsStorageFailure(t *testing.T) {
	mediaPath := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(mediaPath, []byte("file"), 0o600))
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", mediaPath)

	require.ErrorIs(t, EnsureAnnouncementMediaDirectory(), ErrAnnouncementMediaStorage)
}

func TestStoreAnnouncementMediaRefreshesDeduplicatedFileAge(t *testing.T) {
	mediaDir := t.TempDir()
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", mediaDir)
	data := announcementTestPNG(2, 3)
	stored, err := StoreAnnouncementMedia(bytes.NewReader(data), int64(len(data)+1))
	require.NoError(t, err)
	pathName := filepath.Join(mediaDir, stored.Filename)
	oldTime := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(pathName, oldTime, oldTime))

	_, err = StoreAnnouncementMedia(bytes.NewReader(data), int64(len(data)+1))
	require.NoError(t, err)
	info, err := os.Stat(pathName)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), info.ModTime(), 5*time.Second)
}

func TestStoreAnnouncementMediaReplacesDeduplicatedFileWhenTimestampRefreshFails(t *testing.T) {
	mediaDir := t.TempDir()
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", mediaDir)
	data := announcementTestPNG(2, 3)
	stored, err := StoreAnnouncementMedia(bytes.NewReader(data), int64(len(data)+1))
	require.NoError(t, err)
	pathName := filepath.Join(mediaDir, stored.Filename)
	oldTime := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(pathName, oldTime, oldTime))

	originalChtimes := announcementMediaChtimes
	announcementMediaChtimes = func(string, time.Time, time.Time) error {
		return os.ErrPermission
	}
	t.Cleanup(func() { announcementMediaChtimes = originalChtimes })

	restored, err := StoreAnnouncementMedia(bytes.NewReader(data), int64(len(data)+1))
	require.NoError(t, err)
	assert.Equal(t, stored, restored)
	assert.FileExists(t, pathName)
	contents, err := os.ReadFile(pathName)
	require.NoError(t, err)
	assert.Equal(t, data, contents)
	info, err := os.Stat(pathName)
	require.NoError(t, err)
	assert.WithinDuration(t, time.Now(), info.ModTime(), 5*time.Second)
}

func TestStoreAnnouncementMediaRejectsUnsupportedAndOversizedFiles(t *testing.T) {
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", t.TempDir())

	_, err := StoreAnnouncementMedia(strings.NewReader("<svg></svg>"), 1024)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAnnouncementMediaUnsupported)

	data := announcementTestPNG(1, 1)
	_, err = StoreAnnouncementMedia(bytes.NewReader(data), int64(len(data)-1))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAnnouncementMediaTooLarge)
}

func TestStoreAnnouncementMediaRejectsImagesOverPixelLimit(t *testing.T) {
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", t.TempDir())
	data := announcementTestPNG(4097, 1)

	_, err := StoreAnnouncementMedia(bytes.NewReader(data), int64(len(data)+1))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAnnouncementMediaDimensions)
}

func TestOpenAnnouncementMediaRequiresGeneratedFilenameAndReturnsMetadata(t *testing.T) {
	mediaDir := t.TempDir()
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", mediaDir)
	data := announcementTestPNG(1, 1)
	stored, err := StoreAnnouncementMedia(bytes.NewReader(data), int64(len(data)+1))
	require.NoError(t, err)

	reader, contentType, size, err := OpenAnnouncementMedia(stored.Filename)
	require.NoError(t, err)
	defer reader.Close()
	assert.Equal(t, "image/png", contentType)
	assert.Equal(t, int64(len(data)), size)
	readBack, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, data, readBack)

	_, _, _, err = OpenAnnouncementMedia(".." + string(filepath.Separator) + stored.Filename)
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrAnnouncementMediaInvalidFilename)
}

func TestOpenAnnouncementMediaRejectsSymlinkEscapingMediaDirectory(t *testing.T) {
	mediaDir := t.TempDir()
	outsideDir := t.TempDir()
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", mediaDir)
	filename := strings.Repeat("a", 64) + ".png"
	outsidePath := filepath.Join(outsideDir, "outside.png")
	require.NoError(t, os.WriteFile(outsidePath, announcementTestPNG(1, 1), 0o600))
	if err := os.Symlink(outsidePath, filepath.Join(mediaDir, filename)); err != nil {
		t.Skipf("symbolic links are not available in this environment: %v", err)
	}

	reader, _, _, err := OpenAnnouncementMedia(filename)
	if reader != nil {
		_ = reader.Close()
	}
	require.Error(t, err)
}

func TestAnnouncementMediaFileMatchesRejectsOutsideConfiguredDirectory(t *testing.T) {
	mediaDir := t.TempDir()
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", mediaDir)
	data := announcementTestPNG(1, 1)
	digest := sha256.Sum256(data)
	outsidePath := filepath.Join(t.TempDir(), hex.EncodeToString(digest[:])+".png")
	require.NoError(t, os.WriteFile(outsidePath, data, 0o600))

	matches, err := announcementMediaFileMatches(outsidePath, digest, int64(len(data)))
	require.Error(t, err)
	assert.False(t, matches)
}

func TestCleanupAnnouncementMediaKeepsReferencedRecentAndExternalFiles(t *testing.T) {
	mediaDir := t.TempDir()
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", mediaDir)
	data := announcementTestPNG(1, 1)
	stored, err := StoreAnnouncementMedia(bytes.NewReader(data), int64(len(data)+1))
	require.NoError(t, err)

	digest := sha256.Sum256([]byte("orphan"))
	orphanName := hex.EncodeToString(digest[:]) + ".png"
	orphanPath := filepath.Join(mediaDir, orphanName)
	require.NoError(t, os.WriteFile(orphanPath, []byte("orphan"), 0o600))
	oldTime := time.Now().Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(orphanPath, oldTime, oldTime))

	recentDigest := sha256.Sum256([]byte("recent"))
	recentName := hex.EncodeToString(recentDigest[:]) + ".png"
	recentPath := filepath.Join(mediaDir, recentName)
	require.NoError(t, os.WriteFile(recentPath, []byte("recent"), 0o600))

	now := time.Now()
	announcements := `[{"imageUrl":"/api/announcement-media/` + stored.Filename + `"},{"imageUrl":"https://cdn.example.test/banner.png"}]`
	removed, err := CleanupAnnouncementMedia(announcements, now)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.FileExists(t, filepath.Join(mediaDir, stored.Filename))
	assert.NoFileExists(t, orphanPath)
	assert.FileExists(t, recentPath)
}

func TestCleanupAnnouncementMediaScansPastDeletionLimit(t *testing.T) {
	mediaDir := t.TempDir()
	t.Setenv("ANNOUNCEMENT_MEDIA_DIR", mediaDir)

	for index := 0; index < announcementMediaCleanupMaxFiles; index++ {
		filename := fmt.Sprintf("%064x.png", index)
		require.NoError(t, os.WriteFile(filepath.Join(mediaDir, filename), []byte("recent"), 0o600))
	}
	orphanPath := filepath.Join(mediaDir, strings.Repeat("f", 64)+".png")
	require.NoError(t, os.WriteFile(orphanPath, []byte("orphan"), 0o600))
	now := time.Now()
	oldTime := now.Add(-48 * time.Hour)
	require.NoError(t, os.Chtimes(orphanPath, oldTime, oldTime))

	removed, err := CleanupAnnouncementMedia("", now)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)
	assert.NoFileExists(t, orphanPath)
}

func announcementTestPNG(width int, height int) []byte {
	imageData := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			imageData.Set(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 128, A: 255})
		}
	}
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, imageData); err != nil {
		panic(err)
	}
	return buffer.Bytes()
}
