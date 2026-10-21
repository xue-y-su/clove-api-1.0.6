package controller

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/announcement_setting"
	"github.com/gin-gonic/gin"
)

var (
	errAnnouncementAcknowledgementInvalid = errors.New("announcement acknowledgement is invalid")
	errAnnouncementMediaRequired          = errors.New("announcement media is required")
)

func announcementAPIError(c *gin.Context, err error) {
	code := "announcement_operation_failed"
	key := i18n.MsgOperationFailed
	switch {
	case errors.Is(err, errAnnouncementAcknowledgementInvalid):
		code = "announcement_acknowledgement_invalid"
		key = i18n.MsgAnnouncementAcknowledgementInvalid
	case errors.Is(err, service.ErrAnnouncementSignatureUnknown):
		code = "announcement_signature_unknown"
		key = i18n.MsgAnnouncementSignatureUnknown
	case errors.Is(err, errAnnouncementMediaRequired):
		code = "announcement_media_required"
		key = i18n.MsgAnnouncementMediaRequired
	case errors.Is(err, service.ErrAnnouncementMediaTooLarge):
		code = "announcement_media_too_large"
		key = i18n.MsgAnnouncementMediaTooLarge
	case errors.Is(err, service.ErrAnnouncementMediaUnsupported):
		code = "announcement_media_unsupported"
		key = i18n.MsgAnnouncementMediaUnsupported
	case errors.Is(err, service.ErrAnnouncementMediaDimensions):
		code = "announcement_media_dimensions"
		key = i18n.MsgAnnouncementMediaDimensions
	case errors.Is(err, service.ErrAnnouncementMediaInvalidLimit), errors.Is(err, service.ErrAnnouncementMediaStorage):
		code = "announcement_media_storage"
		key = i18n.MsgAnnouncementMediaStorage
	default:
		common.SysError("announcement workflow failed: " + err.Error())
	}
	common.ApiErrorCodeI18n(c, code, key)
}

func GetAnnouncementPopupState(c *gin.Context) {
	userID := c.GetInt("id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":    false,
			"code":       "announcement_auth_required",
			"message":    common.TranslateMessage(c, i18n.MsgAuthNotLoggedIn),
			"request_id": c.GetString(common.RequestIdKey),
		})
		c.Abort()
		return
	}
	state, err := service.GetAnnouncementPopupState(userID, time.Now().UTC())
	if err != nil {
		announcementAPIError(c, err)
		return
	}
	common.ApiSuccess(c, state)
}

type acknowledgeAnnouncementRequest struct {
	Signature string `json:"signature"`
}

func AcknowledgeAnnouncement(c *gin.Context) {
	userID := c.GetInt("id")
	if userID <= 0 {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success":    false,
			"code":       "announcement_auth_required",
			"message":    common.TranslateMessage(c, i18n.MsgAuthNotLoggedIn),
			"request_id": c.GetString(common.RequestIdKey),
		})
		return
	}
	var request acknowledgeAnnouncementRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || request.Signature == "" {
		announcementAPIError(c, errAnnouncementAcknowledgementInvalid)
		return
	}
	created, err := service.AcknowledgeAnnouncement(userID, request.Signature, common.GetTimestamp(), time.Now().UTC())
	if err != nil {
		announcementAPIError(c, err)
		return
	}
	if created {
		recordUserSecurityAudit(c, userID, "announcement.acknowledge", map[string]interface{}{
			"signature": request.Signature,
		})
	}
	common.ApiSuccess(c, gin.H{"acknowledged": true, "created": created})
}

func UploadAnnouncementMedia(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		announcementAPIError(c, errAnnouncementMediaRequired)
		return
	}
	defer file.Close()

	media, err := service.StoreAnnouncementMedia(file, announcement_setting.GetSettings().ImageMaxBytes)
	if err != nil {
		announcementAPIError(c, err)
		return
	}
	recordManageAudit(c, "announcement.media_upload", map[string]interface{}{
		"filename":     media.Filename,
		"content_type": media.ContentType,
		"size":         media.Size,
	})
	common.ApiSuccess(c, media)
}

func GetAnnouncementMedia(c *gin.Context) {
	file, contentType, size, err := service.OpenAnnouncementMedia(c.Param("filename"))
	if err != nil {
		if errors.Is(err, service.ErrAnnouncementMediaInvalidFilename) || errors.Is(err, service.ErrAnnouncementMediaNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		c.Status(http.StatusInternalServerError)
		return
	}
	defer file.Close()
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Length", strconv.FormatInt(size, 10))
	c.DataFromReader(http.StatusOK, size, contentType, file, nil)
}
