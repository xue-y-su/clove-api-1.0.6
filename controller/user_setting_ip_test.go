package controller

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestUpdateUserSettingForcesRecordIPLogOn(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}))

	user := &model.User{
		Id:       1001,
		Username: "ip-setting-user",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, db.Create(user).Error)

	body := map[string]any{
		"notify_type":                          "email",
		"quota_warning_threshold":              1,
		"accept_unset_model_ratio_model":       false,
		"record_ip_log":                        false,
		"notification_email":                   "",
		"upstream_model_update_notify_enabled": false,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPut, "/api/user/setting", body, user.Id)

	UpdateUserSetting(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)

	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.True(t, stored.GetSetting().RecordIpLog)
}

func TestManageUserRejectsNegativeQuotaOverride(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Log{}))

	user := &model.User{
		Id:       1002,
		Username: "quota-override-user",
		Password: "password",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Quota:    100,
	}
	require.NoError(t, db.Create(user).Error)

	body := map[string]any{
		"id":     user.Id,
		"action": "add_quota",
		"mode":   "override",
		"value":  -1,
	}
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/user/manage", body, 1)
	ctx.Set("role", common.RoleRootUser)
	ctx.Set("username", "root")

	ManageUser(ctx)

	response := decodeAPIResponse(t, recorder)
	require.False(t, response.Success)

	var stored model.User
	require.NoError(t, db.First(&stored, user.Id).Error)
	require.Equal(t, int64(100), stored.Quota)
}
