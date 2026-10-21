package controller

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupEmailTemplateControllerTest(t *testing.T) {
	t.Helper()
	setupUserRootBindingControllerTest(t)
	require.NoError(t, model.DB.AutoMigrate(&model.EmailTemplate{}))
	common.SystemName = "CloveAPI"
	common.VerificationValidMinutes = 10
}

func TestEmailTemplateCRUDAndDefaultAPI(t *testing.T) {
	setupEmailTemplateControllerTest(t)

	listCtx, listRec := newAuthenticatedContext(t, http.MethodGet, "/api/email-template/", nil, 1)
	listCtx.Set("role", common.RoleRootUser)
	ListEmailTemplates(listCtx)
	listResp := decodeAPIResponse(t, listRec)
	require.True(t, listResp.Success, listResp.Message)

	createCtx, createRec := newAuthenticatedContext(t, http.MethodPost, "/api/email-template/", map[string]any{
		"name":      "Custom",
		"subject":   "Hi {{site_name}}",
		"html_body": "<p>code {{code}}</p>",
	}, 1)
	createCtx.Set("role", common.RoleRootUser)
	CreateEmailTemplate(createCtx)
	createResp := decodeAPIResponse(t, createRec)
	require.True(t, createResp.Success, createResp.Message)

	var custom model.EmailTemplate
	require.NoError(t, model.DB.Where("name = ?", "Custom").First(&custom).Error)

	setCtx, setRec := newAuthenticatedContext(t, http.MethodPost, fmt.Sprintf("/api/email-template/%d/default", custom.ID), nil, 1)
	setCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(custom.ID)}}
	setCtx.Set("role", common.RoleRootUser)
	SetDefaultEmailTemplate(setCtx)
	require.True(t, decodeAPIResponse(t, setRec).Success)

	previewCtx, previewRec := newAuthenticatedContext(t, http.MethodGet, fmt.Sprintf("/api/email-template/%d/preview", custom.ID), nil, 1)
	previewCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(custom.ID)}}
	previewCtx.Set("role", common.RoleRootUser)
	PreviewEmailTemplate(previewCtx)
	require.True(t, decodeAPIResponse(t, previewRec).Success)

	// reject script body
	badCtx, badRec := newAuthenticatedContext(t, http.MethodPost, "/api/email-template/", map[string]any{
		"name":      "bad",
		"subject":   "s",
		"html_body": "<script>alert(1)</script>",
	}, 1)
	badCtx.Set("role", common.RoleRootUser)
	CreateEmailTemplate(badCtx)
	require.False(t, decodeAPIResponse(t, badRec).Success)
}

func TestEmailTemplateDeleteBuiltinRejected(t *testing.T) {
	setupEmailTemplateControllerTest(t)
	// ensure builtin exists via list
	listCtx, _ := newAuthenticatedContext(t, http.MethodGet, "/api/email-template/", nil, 1)
	listCtx.Set("role", common.RoleRootUser)
	ListEmailTemplates(listCtx)

	var builtin model.EmailTemplate
	require.NoError(t, model.DB.Where("kind = ?", model.EmailTemplateKindBuiltin).First(&builtin).Error)

	delCtx, delRec := newAuthenticatedContext(t, http.MethodDelete, fmt.Sprintf("/api/email-template/%d", builtin.ID), nil, 1)
	delCtx.Params = gin.Params{{Key: "id", Value: fmt.Sprint(builtin.ID)}}
	delCtx.Set("role", common.RoleRootUser)
	DeleteEmailTemplate(delCtx)
	require.False(t, decodeAPIResponse(t, delRec).Success)
}
