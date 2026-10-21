package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type emailTemplateRequest struct {
	Name     string `json:"name"`
	Subject  string `json:"subject"`
	HTMLBody string `json:"html_body"`
	Enabled  *bool  `json:"enabled"`
}

type emailTemplateTestSendRequest struct {
	To string `json:"to"`
}

func ListEmailTemplates(c *gin.Context) {
	items, err := service.ListEmailTemplates(c.Request.Context())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, items)
}

func CreateEmailTemplate(c *gin.Context) {
	var req emailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	tpl, err := service.CreateEmailTemplate(c.Request.Context(), service.EmailTemplateInput{
		Name:     req.Name,
		Subject:  req.Subject,
		HTMLBody: req.HTMLBody,
		Enabled:  req.Enabled,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, tpl)
}

func UpdateEmailTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiError(c, service.ErrEmailTemplateInvalid)
		return
	}
	var req emailTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	tpl, err := service.UpdateEmailTemplate(c.Request.Context(), id, service.EmailTemplateInput{
		Name:     req.Name,
		Subject:  req.Subject,
		HTMLBody: req.HTMLBody,
		Enabled:  req.Enabled,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, tpl)
}

func DeleteEmailTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiError(c, service.ErrEmailTemplateInvalid)
		return
	}
	if err := service.DeleteEmailTemplate(c.Request.Context(), id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func SetDefaultEmailTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiError(c, service.ErrEmailTemplateInvalid)
		return
	}
	if err := service.SetDefaultEmailTemplate(c.Request.Context(), id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func PreviewEmailTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiError(c, service.ErrEmailTemplateInvalid)
		return
	}
	preview, err := service.PreviewEmailTemplate(c.Request.Context(), id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, preview)
}

func TestSendEmailTemplate(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		common.ApiError(c, service.ErrEmailTemplateInvalid)
		return
	}
	var req emailTemplateTestSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := service.TestSendEmailTemplate(c.Request.Context(), service.EmailTemplateTestSendInput{
		TemplateID: id,
		To:         req.To,
		ActorID:    c.GetInt("id"),
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
