package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func openSourceAttributionModelTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	gin.SetMode(gin.TestMode)
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false

	previousDB := DB
	previousLogDB := LOG_DB

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	DB = db
	LOG_DB = db
	require.NoError(t, db.AutoMigrate(&User{}, &SourceAttributionConfirmation{}, &Log{}))

	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

func TestConfirmSourceAttributionSetsInviterWithoutInviteRewards(t *testing.T) {
	openSourceAttributionModelTestDB(t)
	inviter := User{Id: 1, Username: "inviter", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "abcd"}
	user := User{Id: 2, Username: "alice", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "efgh"}
	require.NoError(t, DB.Create(&inviter).Error)
	require.NoError(t, DB.Create(&user).Error)

	result, err := ConfirmSourceAttribution(2, "alice", SourceAttributionSelection{
		OptionId:          "xiaoc",
		OptionName:        "小c",
		TargetAffCode:     "abcd",
		SourceDescription: "线下邀请",
		Ip:                "203.0.113.10",
		UserAgent:         "ua",
		ConfirmedAt:       100,
	})
	require.NoError(t, err)
	require.False(t, result.AlreadyConfirmed)

	var updated User
	require.NoError(t, DB.First(&updated, 2).Error)
	require.Equal(t, inviter.Id, updated.InviterId)
	require.Equal(t, InviterSourceSourceAttribution, updated.InviterSource)

	require.NoError(t, DB.First(&inviter, 1).Error)
	require.Equal(t, 0, inviter.AffCount)
	require.Equal(t, 0, inviter.AffQuota)
	used, err := CountSuccessfulInvites(inviter.Id)
	require.NoError(t, err)
	require.Equal(t, 0, used)

	var confirmation SourceAttributionConfirmation
	require.NoError(t, DB.First(&confirmation, "user_id = ?", 2).Error)
	require.Equal(t, "xiaoc", confirmation.OptionId)
	require.Equal(t, "abcd", confirmation.TargetAffCode)
	require.Equal(t, "线下邀请", confirmation.SourceDescription)
}

func TestConfirmSourceAttributionIgnoresInviteLimitOverride(t *testing.T) {
	openSourceAttributionModelTestDB(t)
	inviter := User{
		Id:               1,
		Username:         "capped-inviter",
		Password:         "password",
		Role:             common.RoleCommonUser,
		Status:           common.UserStatusEnabled,
		AffCode:          "cap0",
		InviteLimitMode:  InviteLimitModeLimited,
		InviteLimitCount: 0,
	}
	user := User{Id: 2, Username: "alice", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "efgh"}
	require.NoError(t, DB.Create(&inviter).Error)
	require.NoError(t, DB.Create(&user).Error)

	result, err := ConfirmSourceAttribution(2, "alice", SourceAttributionSelection{
		OptionId:          "manual-source",
		OptionName:        "Manual Source",
		TargetAffCode:     "cap0",
		SourceDescription: "offline source attribution",
	})
	require.NoError(t, err)
	require.Equal(t, inviter.Id, result.InviterId)

	var updated User
	require.NoError(t, DB.First(&updated, 2).Error)
	require.Equal(t, inviter.Id, updated.InviterId)
	require.Equal(t, InviterSourceSourceAttribution, updated.InviterSource)
}

func TestConfirmSourceAttributionBindsTargetByUserID(t *testing.T) {
	openSourceAttributionModelTestDB(t)
	inviter := User{Id: 11, Username: "source-owner", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "legacy-only"}
	user := User{Id: 12, Username: "source-user", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "source-user"}
	require.NoError(t, DB.Create(&inviter).Error)
	require.NoError(t, DB.Create(&user).Error)

	result, err := ConfirmSourceAttribution(user.Id, user.Username, SourceAttributionSelection{
		OptionId: "partner", OptionName: "Partner", TargetUserID: inviter.Id,
	})
	require.NoError(t, err)
	require.Equal(t, inviter.Id, result.InviterId)
	require.NotNil(t, result.Confirmation)
	require.Equal(t, inviter.Id, result.Confirmation.TargetUserID)

	var updated User
	require.NoError(t, DB.First(&updated, user.Id).Error)
	require.Equal(t, inviter.Id, updated.InviterId)
	require.Equal(t, InviterSourceSourceAttribution, updated.InviterSource)
}

func TestConfirmSourceAttributionBindsImportedNullInviter(t *testing.T) {
	openSourceAttributionModelTestDB(t)
	inviter := User{Id: 21, Username: "source-target", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "source-target"}
	user := User{Id: 22, Username: "imported-user", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "imported-user"}
	require.NoError(t, DB.Create(&inviter).Error)
	require.NoError(t, DB.Create(&user).Error)
	require.NoError(t, DB.Exec("UPDATE users SET inviter_id = NULL WHERE id = ?", user.Id).Error)

	result, err := ConfirmSourceAttribution(user.Id, user.Username, SourceAttributionSelection{
		OptionId: "partner", OptionName: "Partner", TargetUserID: inviter.Id,
	})
	require.NoError(t, err)
	require.False(t, result.AlreadyConfirmed)
	require.NotNil(t, result.Confirmation)

	var updated User
	require.NoError(t, DB.First(&updated, user.Id).Error)
	require.Equal(t, inviter.Id, updated.InviterId)
	require.Equal(t, InviterSourceSourceAttribution, updated.InviterSource)
}

func TestConfirmSourceAttributionCannotOverwriteOrSelfInvite(t *testing.T) {
	openSourceAttributionModelTestDB(t)
	inviter := User{Id: 1, Username: "inviter", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "abcd"}
	user := User{Id: 2, Username: "alice", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "self", InviterId: 1}
	require.NoError(t, DB.Create(&inviter).Error)
	require.NoError(t, DB.Create(&user).Error)

	result, err := ConfirmSourceAttribution(2, "alice", SourceAttributionSelection{
		OptionId:      "again",
		OptionName:    "Again",
		TargetAffCode: "abcd",
	})
	require.NoError(t, err)
	require.True(t, result.AlreadyConfirmed)

	require.NoError(t, DB.Model(&User{}).Where("id = ?", 2).Update("inviter_id", 0).Error)
	_, err = ConfirmSourceAttribution(2, "alice", SourceAttributionSelection{
		OptionId:      "self",
		OptionName:    "Self",
		TargetAffCode: "self",
	})
	require.Error(t, err)
}

func TestConfirmSourceAttributionValidationMessagesAreChinese(t *testing.T) {
	openSourceAttributionModelTestDB(t)

	_, err := ConfirmSourceAttribution(0, "alice", SourceAttributionSelection{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "用户")

	_, err = ConfirmSourceAttribution(2, "alice", SourceAttributionSelection{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "来源归属")

	_, err = ConfirmSourceAttribution(2, "alice", SourceAttributionSelection{
		OptionId: "root",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "用户 ID")
}

func TestSourceAttributionConfirmationPreservesIPv6LoopbackIP(t *testing.T) {
	openSourceAttributionModelTestDB(t)
	inviter := User{Id: 1, Username: "inviter", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "abcd"}
	user := User{Id: 2, Username: "alice", Password: "password", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "efgh"}
	require.NoError(t, DB.Create(&inviter).Error)
	require.NoError(t, DB.Create(&user).Error)

	_, err := ConfirmSourceAttribution(2, "alice", SourceAttributionSelection{
		OptionId:      "local",
		OptionName:    "Local",
		TargetAffCode: "abcd",
		Ip:            "::1",
	})
	require.NoError(t, err)

	var confirmation SourceAttributionConfirmation
	require.NoError(t, DB.First(&confirmation, "user_id = ?", 2).Error)
	require.Equal(t, "::1", confirmation.Ip)
}

func TestSourceAttributionConfirmationListNormalizesAndFiltersLegacyLoopbackIP(t *testing.T) {
	openSourceAttributionModelTestDB(t)
	require.NoError(t, DB.Create(&SourceAttributionConfirmation{
		UserId:      2,
		Username:    "alice",
		OptionId:    "legacy",
		OptionName:  "Legacy",
		Ip:          "::1",
		ConfirmedAt: 100,
	}).Error)

	items, total, err := GetSourceAttributionConfirmations(SourceAttributionConfirmationFilter{
		Ip: "127.0.0.1",
	}, 0, 10)

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, "::1", items[0].Ip)
}

func TestSourceAttributionConfirmationListFiltersTargetUserID(t *testing.T) {
	openSourceAttributionModelTestDB(t)
	require.NoError(t, DB.Create(&[]SourceAttributionConfirmation{
		{UserId: 2, Username: "alice", OptionId: "a", TargetUserID: 11, ConfirmedAt: 100},
		{UserId: 3, Username: "bob", OptionId: "b", TargetUserID: 12, ConfirmedAt: 101},
	}).Error)

	items, total, err := GetSourceAttributionConfirmations(SourceAttributionConfirmationFilter{
		TargetUserID: 12,
	}, 0, 10)

	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, items, 1)
	require.Equal(t, 12, items[0].TargetUserID)
}
