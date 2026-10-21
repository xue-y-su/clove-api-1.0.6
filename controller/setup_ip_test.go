package controller

import (
	"errors"
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestPostSetupPersistsNormalizedRootRegistrationIP(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Option{}, &model.Setup{}, &model.AuditOutboxEvent{},
		&model.IPSecurityEvent{}, &model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
	))
	common.OptionMapRWMutex.RLock()
	previousOptionMap := common.OptionMap
	common.OptionMapRWMutex.RUnlock()
	model.InitOptionMap()
	previousSetup := constant.Setup
	previousSelfUse := operation_setting.SelfUseModeEnabled
	previousDemo := operation_setting.DemoSiteEnabled
	constant.Setup = false
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
		constant.Setup = previousSetup
		operation_setting.SelfUseModeEnabled = previousSelfUse
		operation_setting.DemoSiteEnabled = previousDemo
	})
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/setup", SetupRequest{
		Username: "root", Password: "StrongPassword123", ConfirmPassword: "StrongPassword123",
	}, 0)
	ctx.Request.RemoteAddr = "[2001:db8::7]:4050"

	PostSetup(ctx)

	response := decodeAPIResponse(t, recorder)
	require.True(t, response.Success, response.Message)
	var root model.User
	require.NoError(t, model.DB.First(&root, "role = ?", common.RoleRootUser).Error)
	require.Equal(t, "2001:db8::7", root.RegistrationIP)
	var event model.AuditOutboxEvent
	require.NoError(t, model.DB.First(&event, "event_type = ?", "user.register").Error)
	require.Equal(t, "user:"+strconv.Itoa(root.Id), event.Target)
	require.Equal(t, "2001:db8::7", event.IP)
	require.Contains(t, event.Payload, `"source":"setup"`)
	require.Contains(t, event.Payload, `"target_user_id":`+strconv.Itoa(root.Id))
	var securityEvent model.IPSecurityEvent
	require.NoError(t, model.DB.First(&securityEvent, "event_id = ?", "user:register:"+strconv.Itoa(root.Id)).Error)
	require.Equal(t, model.IPSecurityKindRegistration, securityEvent.Kind)
	require.Equal(t, "2001:db8::7", securityEvent.IP)
}

func TestPostSetupRollsBackRootWhenRegistrationAuditCannotBeEnqueued(t *testing.T) {
	db := openTokenControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Option{}, &model.Setup{}, &model.AuditOutboxEvent{},
		&model.IPSecurityEvent{}, &model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
	))
	common.OptionMapRWMutex.RLock()
	previousOptionMap := common.OptionMap
	common.OptionMapRWMutex.RUnlock()
	model.InitOptionMap()
	previousSetup := constant.Setup
	previousSelfUse := operation_setting.SelfUseModeEnabled
	previousDemo := operation_setting.DemoSiteEnabled
	constant.Setup = false
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("setup_ip_test:fail_outbox", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "audit_outbox_events" {
			tx.AddError(errors.New("outbox unavailable"))
		}
	}))
	t.Cleanup(func() {
		_ = db.Callback().Create().Remove("setup_ip_test:fail_outbox")
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
		constant.Setup = previousSetup
		operation_setting.SelfUseModeEnabled = previousSelfUse
		operation_setting.DemoSiteEnabled = previousDemo
	})
	ctx, recorder := newAuthenticatedContext(t, http.MethodPost, "/api/setup", SetupRequest{
		Username: "root", Password: "StrongPassword123", ConfirmPassword: "StrongPassword123",
	}, 0)

	PostSetup(ctx)

	require.False(t, decodeAPIResponse(t, recorder).Success)
	var rootCount int64
	require.NoError(t, model.DB.Model(&model.User{}).Where("role = ?", common.RoleRootUser).Count(&rootCount).Error)
	require.Zero(t, rootCount)
	require.False(t, constant.Setup)
}
