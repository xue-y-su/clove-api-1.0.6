package controller

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/model_catalog_setting"
	"github.com/gin-gonic/gin"
)

type modelCatalogVisibilityUpdateRequest struct {
	Config       model_catalog_setting.Visibility `json:"config"`
	ExpectedETag string                           `json:"expected_etag"`
}

func GetModelCatalogVisibility(c *gin.Context) {
	state, err := model.GetCatalogAdminState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "failed to load model catalog visibility",
		})
		return
	}
	common.ApiSuccess(c, state)
}

func UpdateModelCatalogVisibility(c *gin.Context) {
	var request modelCatalogVisibilityUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid model catalog visibility request",
		})
		return
	}
	state, change, err := model.UpdateCatalogVisibility(request.ExpectedETag, request.Config)
	if errors.Is(err, model.ErrCatalogVisibilityConflict) {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "model catalog visibility changed; reload and try again",
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	recordManageAudit(c, "model_catalog.visibility_update", map[string]interface{}{
		"old_mode":      change.OldMode,
		"new_mode":      change.NewMode,
		"old_enabled":   change.OldEnabled,
		"new_enabled":   change.NewEnabled,
		"added_count":   change.AddedCount,
		"removed_count": change.RemovedCount,
		"config_digest": change.ConfigDigest,
	})
	common.ApiSuccess(c, state)
}
