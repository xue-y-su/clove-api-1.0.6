package model

import (
	"os"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestSourceAttributionConfirmationAutoMigrateMySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run mysql migration compatibility test")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	defer sqlDB.Close()

	require.NoError(t, db.Migrator().DropTable(&SourceAttributionConfirmation{}))
	require.NoError(t, db.AutoMigrate(&SourceAttributionConfirmation{}))

	type indexColumn struct {
		ColumnName string `gorm:"column:Column_name"`
		SeqInIndex int    `gorm:"column:Seq_in_index"`
	}
	var columns []indexColumn
	require.NoError(t, db.Raw("SHOW INDEX FROM source_attribution_confirmations WHERE Key_name = ?", "idx_source_attribution_confirmations_user_id").Scan(&columns).Error)
	require.Len(t, columns, 1)
	require.Equal(t, "user_id", columns[0].ColumnName)
	require.Equal(t, 1, columns[0].SeqInIndex)
}

func TestConfirmSourceAttributionBindsNullInviterMySQL(t *testing.T) {
	dsn := os.Getenv("TEST_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run mysql source attribution compatibility test")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&User{}, &SourceAttributionConfirmation{}))

	const inviterID = 9_100_001
	const userID = 9_100_002
	require.NoError(t, db.Unscoped().Where("user_id = ?", userID).Delete(&SourceAttributionConfirmation{}).Error)
	require.NoError(t, db.Unscoped().Where("id IN ?", []int{inviterID, userID}).Delete(&User{}).Error)

	previousDB := DB
	previousRedisEnabled := common.RedisEnabled
	DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		DB = previousDB
		common.RedisEnabled = previousRedisEnabled
		_ = db.Unscoped().Where("user_id = ?", userID).Delete(&SourceAttributionConfirmation{}).Error
		_ = db.Unscoped().Where("id IN ?", []int{inviterID, userID}).Delete(&User{}).Error
		_ = sqlDB.Close()
	})

	require.NoError(t, db.Create(&User{
		Id: inviterID, Username: "mysql-source-target", Password: "password",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "mysql-source-target",
	}).Error)
	require.NoError(t, db.Create(&User{
		Id: userID, Username: "mysql-imported-user", Password: "password",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "mysql-imported-user",
	}).Error)
	require.NoError(t, db.Model(&User{}).Where("id = ?", userID).UpdateColumn("inviter_id", nil).Error)

	result, err := ConfirmSourceAttribution(userID, "mysql-imported-user", SourceAttributionSelection{
		OptionId: "partner", OptionName: "Partner", TargetUserID: inviterID,
	})
	require.NoError(t, err)
	require.False(t, result.AlreadyConfirmed)
	require.NotNil(t, result.Confirmation)

	var updated User
	require.NoError(t, db.First(&updated, userID).Error)
	require.Equal(t, inviterID, updated.InviterId)
}
