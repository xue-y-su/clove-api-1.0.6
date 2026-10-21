package gemini

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const maxVeoImageSize = 20 * 1024 * 1024 // 20 MB

// ExtractMultipartImage reads the first `input_reference` file from a multipart
// form upload and returns a VeoImageInput. Returns nil if no file is present.
func ExtractMultipartImage(c *gin.Context, info *relaycommon.RelayInfo) (*VeoImageInput, error) {
	mf, err := c.MultipartForm()
	if err != nil {
		if errors.Is(err, http.ErrNotMultipart) {
			return nil, nil
		}
		return nil, fmt.Errorf("parse multipart form: %w", err)
	}
	files, exists := mf.File["input_reference"]
	if !exists || len(files) == 0 {
		return nil, nil
	}
	fh := files[0]
	if fh.Size > maxVeoImageSize {
		return nil, fmt.Errorf("input_reference exceeds %d bytes", maxVeoImageSize)
	}
	file, err := fh.Open()
	if err != nil {
		return nil, fmt.Errorf("open input_reference: %w", err)
	}
	defer file.Close()

	fileBytes, err := io.ReadAll(io.LimitReader(file, maxVeoImageSize+1))
	if err != nil {
		return nil, fmt.Errorf("read input_reference: %w", err)
	}
	image, err := imageInputFromBytes(fileBytes, fh.Header.Get("Content-Type"))
	if err != nil {
		return nil, fmt.Errorf("invalid input_reference: %w", err)
	}

	setVeoImageAction(info)
	return image, nil
}

// ParseImageInput parses an HTTP URL, data URI, or raw base64 image into a
// bounded Veo image payload.
func ParseImageInput(ctx context.Context, imageStr string, client *http.Client) (*VeoImageInput, error) {
	imageStr = strings.TrimSpace(imageStr)
	if imageStr == "" {
		return nil, nil
	}

	parsedURL, err := url.Parse(imageStr)
	if err == nil && (strings.EqualFold(parsedURL.Scheme, "http") || strings.EqualFold(parsedURL.Scheme, "https")) {
		if client == nil {
			return nil, errors.New("image HTTP client is not initialized")
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, imageStr, nil)
		if err != nil {
			return nil, fmt.Errorf("create image request: %w", err)
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, fmt.Errorf("download image: %w", err)
		}
		defer response.Body.Close()
		if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
			return nil, fmt.Errorf("download image returned HTTP %d", response.StatusCode)
		}
		if response.ContentLength > maxVeoImageSize {
			return nil, fmt.Errorf("remote image exceeds %d bytes", maxVeoImageSize)
		}
		imageBytes, err := io.ReadAll(io.LimitReader(response.Body, maxVeoImageSize+1))
		if err != nil {
			return nil, fmt.Errorf("read remote image: %w", err)
		}
		image, err := imageInputFromBytes(imageBytes, response.Header.Get("Content-Type"))
		if err != nil {
			return nil, fmt.Errorf("invalid remote image: %w", err)
		}
		return image, nil
	}

	if strings.HasPrefix(imageStr, "data:") {
		return parseDataURI(imageStr)
	}

	if base64.StdEncoding.DecodedLen(len(imageStr)) > maxVeoImageSize {
		return nil, fmt.Errorf("image exceeds %d bytes", maxVeoImageSize)
	}
	raw, err := base64.StdEncoding.DecodeString(imageStr)
	if err != nil {
		return nil, fmt.Errorf("decode image base64: %w", err)
	}
	return imageInputFromBytes(raw, "")
}

func parseDataURI(uri string) (*VeoImageInput, error) {
	rest := uri[len("data:"):]
	idx := strings.Index(rest, ",")
	if idx < 0 {
		return nil, errors.New("data URI is missing a comma")
	}
	meta := rest[:idx]
	encoded := rest[idx+1:]
	if encoded == "" {
		return nil, errors.New("data URI image is empty")
	}

	parts := strings.Split(meta, ";")
	mimeType := parts[0]
	base64Encoded := false
	for _, part := range parts[1:] {
		if strings.EqualFold(strings.TrimSpace(part), "base64") {
			base64Encoded = true
			break
		}
	}
	if !base64Encoded {
		return nil, errors.New("data URI image must use base64 encoding")
	}
	if base64.StdEncoding.DecodedLen(len(encoded)) > maxVeoImageSize {
		return nil, fmt.Errorf("image exceeds %d bytes", maxVeoImageSize)
	}
	imageBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode data URI image: %w", err)
	}
	return imageInputFromBytes(imageBytes, mimeType)
}

func imageInputFromBytes(imageBytes []byte, declaredMimeType string) (*VeoImageInput, error) {
	if len(imageBytes) == 0 {
		return nil, errors.New("image is empty")
	}
	if len(imageBytes) > maxVeoImageSize {
		return nil, fmt.Errorf("image exceeds %d bytes", maxVeoImageSize)
	}
	mimeType := strings.TrimSpace(declaredMimeType)
	if parsedMimeType, _, err := mime.ParseMediaType(mimeType); err == nil {
		mimeType = parsedMimeType
	}
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = http.DetectContentType(imageBytes)
	}
	if !strings.HasPrefix(strings.ToLower(mimeType), "image/") {
		return nil, fmt.Errorf("unsupported image content type %q", mimeType)
	}
	return &VeoImageInput{
		BytesBase64Encoded: base64.StdEncoding.EncodeToString(imageBytes),
		MimeType:           mimeType,
	}, nil
}

func BuildVeoInstance(c *gin.Context, info *relaycommon.RelayInfo, request relaycommon.TaskSubmitReq) (VeoInstance, error) {
	instance := VeoInstance{Prompt: request.Prompt}
	multipartImage, err := ExtractMultipartImage(c, info)
	if err != nil {
		return instance, err
	}
	if multipartImage != nil {
		instance.Image = multipartImage
	} else if len(request.Images) > 0 {
		instance.Image, err = ParseImageInput(c.Request.Context(), request.Images[0], service.GetSSRFProtectedHTTPClient())
		if err != nil {
			return instance, fmt.Errorf("invalid image: %w", err)
		}
		if instance.Image != nil {
			setVeoImageAction(info)
		}
	}

	if err = applyVeoInstanceMetadata(c.Request.Context(), service.GetSSRFProtectedHTTPClient(), request.Metadata, &instance); err != nil {
		return instance, err
	}
	if instance.LastFrame != nil || len(instance.ReferenceImages) > 0 {
		setVeoImageAction(info)
	}
	return instance, nil
}

func applyVeoInstanceMetadata(ctx context.Context, client *http.Client, metadata map[string]any, instance *VeoInstance) error {
	if metadata == nil {
		return nil
	}
	if rawLastFrame, exists := metadata["lastFrame"]; exists {
		lastFrame, err := parseVeoMetadataImage(ctx, client, rawLastFrame)
		if err != nil {
			return fmt.Errorf("invalid lastFrame: %w", err)
		}
		if lastFrame == nil {
			return errors.New("lastFrame image is empty")
		}
		instance.LastFrame = lastFrame
	}

	rawReferences, exists := metadata["referenceImages"]
	if !exists {
		return nil
	}
	encodedReferences, err := common.Marshal(rawReferences)
	if err != nil {
		return fmt.Errorf("marshal referenceImages: %w", err)
	}
	var references []any
	if err = common.Unmarshal(encodedReferences, &references); err != nil {
		return fmt.Errorf("referenceImages must be an array: %w", err)
	}
	if len(references) > 3 {
		return errors.New("referenceImages supports at most 3 images")
	}
	for index, rawReference := range references {
		referenceType := "asset"
		imageValue := rawReference
		if reference, ok := rawReference.(map[string]any); ok {
			var exists bool
			imageValue, exists = reference["image"]
			if !exists {
				return fmt.Errorf("referenceImages[%d].image is required", index)
			}
			if value, ok := reference["referenceType"].(string); ok && strings.TrimSpace(value) != "" {
				referenceType = strings.ToLower(strings.TrimSpace(value))
			}
		}
		if referenceType != "asset" && referenceType != "style" {
			return fmt.Errorf("referenceImages[%d].referenceType must be asset or style", index)
		}
		image, err := parseVeoMetadataImage(ctx, client, imageValue)
		if err != nil {
			return fmt.Errorf("invalid referenceImages[%d].image: %w", index, err)
		}
		if image == nil {
			return fmt.Errorf("referenceImages[%d].image is empty", index)
		}
		instance.ReferenceImages = append(instance.ReferenceImages, VeoReferenceImage{
			Image:         *image,
			ReferenceType: referenceType,
		})
	}
	return nil
}

func parseVeoMetadataImage(ctx context.Context, client *http.Client, value any) (*VeoImageInput, error) {
	switch image := value.(type) {
	case string:
		return ParseImageInput(ctx, image, client)
	case map[string]any:
		if imageURL, ok := image["url"].(string); ok {
			return ParseImageInput(ctx, imageURL, client)
		}
		encoded, ok := image["bytesBase64Encoded"].(string)
		if !ok || strings.TrimSpace(encoded) == "" {
			return nil, errors.New("image must contain a URL or bytesBase64Encoded")
		}
		mimeType, _ := image["mimeType"].(string)
		if base64.StdEncoding.DecodedLen(len(encoded)) > maxVeoImageSize {
			return nil, fmt.Errorf("image exceeds %d bytes", maxVeoImageSize)
		}
		imageBytes, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode image base64: %w", err)
		}
		return imageInputFromBytes(imageBytes, mimeType)
	default:
		return nil, fmt.Errorf("unsupported image value type %T", value)
	}
}

func setVeoImageAction(info *relaycommon.RelayInfo) {
	if info != nil && info.TaskRelayInfo != nil {
		info.Action = constant.TaskActionGenerate
	}
}
