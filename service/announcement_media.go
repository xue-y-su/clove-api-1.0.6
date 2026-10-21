package service

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/console_setting"
	"golang.org/x/image/webp"
)

const (
	announcementMediaMaxWidth        = 4096
	announcementMediaMaxHeight       = 4096
	announcementMediaCleanupMaxFiles = 1000
)

const announcementMediaCleanupAge = 24 * time.Hour

var announcementMediaChtimes = os.Chtimes

var (
	ErrAnnouncementMediaInvalidLimit    = errors.New("announcement media size limit is invalid")
	ErrAnnouncementMediaTooLarge        = errors.New("announcement media exceeds the configured size limit")
	ErrAnnouncementMediaUnsupported     = errors.New("announcement media format is unsupported")
	ErrAnnouncementMediaDimensions      = errors.New("announcement media dimensions exceed 4096 x 4096")
	ErrAnnouncementMediaStorage         = errors.New("announcement media storage failed")
	ErrAnnouncementMediaInvalidFilename = errors.New("announcement media filename is invalid")
	ErrAnnouncementMediaNotFound        = errors.New("announcement media was not found")
	announcementMediaFilenamePattern    = regexp.MustCompile(`^[a-f0-9]{64}\.(jpg|png|webp|gif)$`)
	announcementMediaMutationMutex      sync.Mutex
)

type AnnouncementMedia struct {
	URL         string `json:"url"`
	Filename    string `json:"-"`
	ContentType string `json:"-"`
	Size        int64  `json:"-"`
}

func EnsureAnnouncementMediaDirectory() error {
	return ensureAnnouncementMediaDirectory(announcementMediaDirectory())
}

func StoreAnnouncementMedia(reader io.Reader, maxBytes int64) (AnnouncementMedia, error) {
	if reader == nil || maxBytes <= 0 {
		return AnnouncementMedia{}, ErrAnnouncementMediaInvalidLimit
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return AnnouncementMedia{}, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	if int64(len(data)) > maxBytes {
		return AnnouncementMedia{}, ErrAnnouncementMediaTooLarge
	}

	contentType, extension, err := validateAnnouncementMedia(data)
	if err != nil {
		return AnnouncementMedia{}, err
	}
	digest := sha256.Sum256(data)
	filename := hex.EncodeToString(digest[:]) + "." + extension
	media := AnnouncementMedia{
		URL:         "/api/announcement-media/" + filename,
		Filename:    filename,
		ContentType: contentType,
		Size:        int64(len(data)),
	}

	announcementMediaMutationMutex.Lock()
	defer announcementMediaMutationMutex.Unlock()
	directory := announcementMediaDirectory()
	if err := ensureAnnouncementMediaDirectory(directory); err != nil {
		return AnnouncementMedia{}, err
	}
	target := filepath.Join(directory, filename)
	if matches, err := announcementMediaFileMatches(target, digest, media.Size); err != nil {
		return AnnouncementMedia{}, err
	} else if matches {
		now := time.Now()
		if err := announcementMediaChtimes(target, now, now); err == nil {
			return media, nil
		}
	}

	temporary, err := os.CreateTemp(directory, ".announcement-media-*")
	if err != nil {
		return AnnouncementMedia{}, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return AnnouncementMedia{}, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return AnnouncementMedia{}, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	if err := temporary.Close(); err != nil {
		return AnnouncementMedia{}, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		if matches, matchErr := announcementMediaFileMatches(target, digest, media.Size); matchErr == nil && matches {
			return media, nil
		}
		return AnnouncementMedia{}, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	removeTemporary = false
	return media, nil
}

func OpenAnnouncementMedia(filename string) (io.ReadCloser, string, int64, error) {
	if !announcementMediaFilenamePattern.MatchString(filename) {
		return nil, "", 0, ErrAnnouncementMediaInvalidFilename
	}
	root, err := os.OpenRoot(announcementMediaDirectory())
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", 0, ErrAnnouncementMediaNotFound
	}
	if err != nil {
		return nil, "", 0, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	defer root.Close()
	linkInfo, err := root.Lstat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", 0, ErrAnnouncementMediaNotFound
	}
	if err != nil {
		return nil, "", 0, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 || !linkInfo.Mode().IsRegular() {
		return nil, "", 0, ErrAnnouncementMediaInvalidFilename
	}
	file, err := root.Open(filename)
	if errors.Is(err, os.ErrNotExist) {
		return nil, "", 0, ErrAnnouncementMediaNotFound
	}
	if err != nil {
		return nil, "", 0, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, "", 0, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, "", 0, ErrAnnouncementMediaInvalidFilename
	}
	return file, announcementMediaContentType(filepath.Ext(filename)), info.Size(), nil
}

func CleanupAnnouncementMedia(announcementsJSON string, now time.Time) (int, error) {
	announcementMediaMutationMutex.Lock()
	defer announcementMediaMutationMutex.Unlock()
	return cleanupAnnouncementMedia(announcementsJSON, now)
}

func CleanupCurrentAnnouncementMedia(now time.Time) (int, error) {
	announcementMediaMutationMutex.Lock()
	defer announcementMediaMutationMutex.Unlock()
	return cleanupAnnouncementMedia(console_setting.GetConsoleSetting().Announcements, now)
}

func SaveAnnouncements(announcementsJSON string, now time.Time) error {
	announcementMediaMutationMutex.Lock()
	defer announcementMediaMutationMutex.Unlock()
	if err := console_setting.ValidateConsoleSettings(announcementsJSON, "Announcements"); err != nil {
		return err
	}
	normalized, err := console_setting.NormalizeAnnouncementsJSON(announcementsJSON)
	if err != nil {
		return err
	}
	if err := console_setting.ValidateConsoleSettings(normalized, "Announcements"); err != nil {
		return err
	}
	if err := validateAnnouncementMediaReferences(normalized); err != nil {
		return err
	}
	if err := model.UpdateOption("console_setting.announcements", normalized); err != nil {
		return err
	}
	if _, err := cleanupAnnouncementMedia(normalized, now); err != nil {
		common.SysError("failed to clean announcement media: " + err.Error())
	}
	return nil
}

func cleanupAnnouncementMedia(announcementsJSON string, now time.Time) (int, error) {
	referenced, err := announcementMediaReferences(announcementsJSON)
	if err != nil {
		return 0, err
	}

	entries, err := os.ReadDir(announcementMediaDirectory())
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	if now.IsZero() {
		now = time.Now()
	}
	removed := 0
	for _, entry := range entries {
		if removed >= announcementMediaCleanupMaxFiles {
			break
		}
		filename := entry.Name()
		if entry.Type()&os.ModeSymlink != 0 || !announcementMediaFilenamePattern.MatchString(filename) {
			continue
		}
		if _, keep := referenced[filename]; keep {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return removed, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
		}
		if !info.Mode().IsRegular() || now.Sub(info.ModTime()) <= announcementMediaCleanupAge {
			continue
		}
		if err := os.Remove(filepath.Join(announcementMediaDirectory(), filename)); err != nil {
			return removed, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
		}
		removed++
	}
	return removed, nil
}

func validateAnnouncementMediaReferences(announcementsJSON string) error {
	referenced, err := announcementMediaReferences(announcementsJSON)
	if err != nil {
		return err
	}
	for filename := range referenced {
		info, err := os.Lstat(filepath.Join(announcementMediaDirectory(), filename))
		if errors.Is(err, os.ErrNotExist) {
			return ErrAnnouncementMediaNotFound
		}
		if err != nil {
			return fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
		}
		if !info.Mode().IsRegular() {
			return ErrAnnouncementMediaNotFound
		}
	}
	return nil
}

func announcementMediaReferences(announcementsJSON string) (map[string]struct{}, error) {
	var announcements []Announcement
	if strings.TrimSpace(announcementsJSON) != "" {
		if err := common.UnmarshalJsonStr(announcementsJSON, &announcements); err != nil {
			return nil, err
		}
	}
	referenced := make(map[string]struct{})
	for _, announcement := range announcements {
		imageURL := strings.TrimSpace(announcement.ImageURL)
		const prefix = "/api/announcement-media/"
		if !strings.HasPrefix(imageURL, prefix) {
			continue
		}
		filename := strings.TrimPrefix(imageURL, prefix)
		if announcementMediaFilenamePattern.MatchString(filename) {
			referenced[filename] = struct{}{}
		}
	}
	return referenced, nil
}

func validateAnnouncementMedia(data []byte) (string, string, error) {
	if len(data) == 0 {
		return "", "", ErrAnnouncementMediaUnsupported
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		config, err = webp.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return "", "", ErrAnnouncementMediaUnsupported
		}
		format = "webp"
	}
	if config.Width <= 0 || config.Height <= 0 ||
		config.Width > announcementMediaMaxWidth || config.Height > announcementMediaMaxHeight {
		return "", "", ErrAnnouncementMediaDimensions
	}

	contentType := ""
	extension := ""
	switch strings.ToLower(format) {
	case "jpeg", "jpg":
		contentType, extension = "image/jpeg", "jpg"
	case "png":
		contentType, extension = "image/png", "png"
	case "gif":
		contentType, extension = "image/gif", "gif"
	case "webp":
		contentType, extension = "image/webp", "webp"
	default:
		return "", "", ErrAnnouncementMediaUnsupported
	}

	if extension == "webp" {
		_, err = webp.Decode(bytes.NewReader(data))
	} else {
		_, _, err = image.Decode(bytes.NewReader(data))
	}
	if err != nil {
		return "", "", ErrAnnouncementMediaUnsupported
	}
	return contentType, extension, nil
}

func announcementMediaDirectory() string {
	directory := strings.TrimSpace(common.GetEnvOrDefaultString("ANNOUNCEMENT_MEDIA_DIR", ""))
	if directory != "" {
		return filepath.Clean(directory)
	}
	return filepath.Join(".", "announcement-media")
}

func ensureAnnouncementMediaDirectory(directory string) error {
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}

	probe, err := os.CreateTemp(directory, ".announcement-media-probe-*")
	if err != nil {
		return fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	probePath := probe.Name()
	if err := probe.Close(); err != nil {
		_ = os.Remove(probePath)
		return fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	if err := os.Remove(probePath); err != nil {
		return fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	return nil
}

func announcementMediaFileMatches(path string, digest [sha256.Size]byte, expectedSize int64) (bool, error) {
	directory := filepath.Clean(announcementMediaDirectory())
	path = filepath.Clean(path)
	filename := filepath.Base(path)
	if filepath.Dir(path) != directory || !announcementMediaFilenamePattern.MatchString(filename) {
		return false, ErrAnnouncementMediaInvalidFilename
	}
	root, err := os.OpenRoot(directory)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	defer root.Close()
	info, err := root.Lstat(filename)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() != expectedSize {
		return false, fmt.Errorf("%w: existing media path is not the expected file", ErrAnnouncementMediaStorage)
	}
	existing, err := root.ReadFile(filename)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrAnnouncementMediaStorage, err)
	}
	return sha256.Sum256(existing) == digest, nil
}

func announcementMediaContentType(extension string) string {
	switch strings.ToLower(extension) {
	case ".jpg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	default:
		return "application/octet-stream"
	}
}
