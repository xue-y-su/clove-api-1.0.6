package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupUserRedemptionModelTestDB(t *testing.T) {
	t.Helper()
	prevDB := DB
	prevLogDB := LOG_DB
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:user_redemption_model_%d?mode=memory&cache=shared", time.Now().UnixNano())), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	DB = db
	LOG_DB = db
	t.Cleanup(func() {
		DB = prevDB
		LOG_DB = prevLogDB
		_ = sqlDB.Close()
	})
	require.NoError(t, db.AutoMigrate(&UserRedemptionCode{}, &UserRedemptionClaim{}, &UserRedemptionSubscriptionEntitlement{}))
}

func TestSubscriptionEntitlementUsesOneRowPerCode(t *testing.T) {
	setupUserRedemptionModelTestDB(t)
	code := UserRedemptionCode{
		Code:          "SUB-ONE",
		CreatorUserID: 7,
		RewardType:    UserRedemptionRewardSubscription,
		Kind:          UserRedemptionKindFixed,
		MaxClaims:     2,
		Status:        UserRedemptionStatusActive,
		ExpiresAt:     time.Now().Add(time.Hour).Unix(),
		ReserveOpID:   "user-redemption:sub-one:reserve",
	}
	require.NoError(t, DB.Create(&code).Error)

	first := UserRedemptionSubscriptionEntitlement{
		CodeID:        code.ID,
		PlanID:        9,
		PlanTitle:     "Starter",
		Currency:      "USD",
		UnitPaidQuota: 500,
	}
	require.NoError(t, DB.Create(&first).Error)
	duplicate := first
	duplicate.ID = 0
	require.Error(t, DB.Create(&duplicate).Error)
}

func TestUserRedemptionRewardTypeMigrationDefaultsLegacyRows(t *testing.T) {
	setupUserRedemptionModelTestDB(t)
	code := UserRedemptionCode{
		Code:          "LEGACY-REWARD-TYPE",
		CreatorUserID: 8,
		Kind:          UserRedemptionKindSingle,
		TotalQuota:    100,
		MaxClaims:     1,
		Status:        UserRedemptionStatusActive,
		ExpiresAt:     time.Now().Add(time.Hour).Unix(),
		ReserveOpID:   "user-redemption:legacy-reward-type:reserve",
	}
	require.NoError(t, DB.Create(&code).Error)
	claim := UserRedemptionClaim{
		CodeID:        code.ID,
		ClaimerUserID: 9,
		Quota:         100,
		CreditOpID:    "user-redemption:legacy-reward-type:claim",
		ClaimedAt:     time.Now().Unix(),
	}
	require.NoError(t, DB.Create(&claim).Error)
	require.NoError(t, DB.Model(&UserRedemptionCode{}).Where("id = ?", code.ID).Update("reward_type", "").Error)
	require.NoError(t, DB.Model(&UserRedemptionClaim{}).Where("id = ?", claim.ID).Update("reward_type", "").Error)

	require.NoError(t, MigrateUserRedemptionStatuses(DB))
	require.NoError(t, DB.First(&code, code.ID).Error)
	require.NoError(t, DB.First(&claim, claim.ID).Error)
	require.Equal(t, UserRedemptionRewardBalance, code.RewardType)
	require.Equal(t, UserRedemptionRewardBalance, claim.RewardType)
}

func TestUserRedemptionCodeCRUD(t *testing.T) {
	setupUserRedemptionModelTestDB(t)
	row := UserRedemptionCode{
		Code:          "ABC123XYZ",
		CreatorUserID: 7,
		Kind:          UserRedemptionKindSingle,
		TotalQuota:    10000,
		MaxClaims:     1,
		Status:        UserRedemptionStatusActive,
		ExpiresAt:     time.Now().Add(2 * time.Hour).Unix(),
		ReserveOpID:   "user-redemption:1:reserve",
	}
	require.NoError(t, DB.Create(&row).Error)
	got, err := GetUserRedemptionCodeByCode("ABC123XYZ")
	require.NoError(t, err)
	require.Equal(t, row.CreatorUserID, got.CreatorUserID)

	byID, err := GetUserRedemptionCodeByID(row.ID)
	require.NoError(t, err)
	require.Equal(t, row.Code, byID.Code)

	claim := UserRedemptionClaim{
		CodeID:        row.ID,
		ClaimerUserID: 8,
		Quota:         10000,
		CreditOpID:    "user-redemption:1:claim:8",
		ClaimedAt:     time.Now().Unix(),
	}
	require.NoError(t, DB.Create(&claim).Error)
	claims, err := ListUserRedemptionClaims(row.ID)
	require.NoError(t, err)
	require.Len(t, claims, 1)
	require.Equal(t, 8, claims[0].ClaimerUserID)
}

func TestUserRedemptionGovernanceFieldsMigratePortably(t *testing.T) {
	setupUserRedemptionModelTestDB(t)

	require.True(t, DB.Migrator().HasColumn(&UserRedemptionCode{}, "ForfeitedQuota"))
	require.True(t, DB.Migrator().HasColumn(&UserRedemptionCode{}, "DisabledAt"))
	require.True(t, DB.Migrator().HasColumn(&UserRedemptionCode{}, "TerminalActorID"))
	require.True(t, DB.Migrator().HasColumn(&UserRedemptionCode{}, "DeletedAt"))
}

type legacyUserRedemptionCode struct {
	ID            int64  `gorm:"primaryKey;autoIncrement"`
	Code          string `gorm:"type:varchar(64);not null;uniqueIndex"`
	CreatorUserID int    `gorm:"not null;index"`
	Kind          string `gorm:"type:varchar(16);not null;index"`
	TotalQuota    int    `gorm:"not null"`
	ClaimedQuota  int    `gorm:"not null;default:0"`
	RefundedQuota int    `gorm:"not null;default:0"`
	MaxClaims     int    `gorm:"not null"`
	ClaimCount    int    `gorm:"not null;default:0"`
	Status        string `gorm:"type:varchar(32);not null;index"`
	ExpiresAt     int64  `gorm:"bigint;not null;index"`
	ReserveOpID   string `gorm:"column:reserve_operation_id;type:varchar(191);not null;uniqueIndex"`
	RefundOpID    string `gorm:"column:refund_operation_id;type:varchar(191);not null;default:'';index"`
	CreatedAt     int64  `gorm:"bigint;autoCreateTime;index"`
	UpdatedAt     int64  `gorm:"bigint;autoUpdateTime"`
}

func (legacyUserRedemptionCode) TableName() string { return "user_redemption_codes" }

func TestUserRedemptionGovernanceMigrationPreservesLegacyRows(t *testing.T) {
	setupUserRedemptionModelTestDB(t)
	require.NoError(t, DB.Migrator().DropTable(&UserRedemptionCode{}))
	require.NoError(t, DB.AutoMigrate(&legacyUserRedemptionCode{}))

	legacy := legacyUserRedemptionCode{
		Code:          "LEGACY-CODE",
		CreatorUserID: 7,
		Kind:          UserRedemptionKindSingle,
		TotalQuota:    10000,
		MaxClaims:     1,
		Status:        UserRedemptionStatusActive,
		ExpiresAt:     time.Now().Add(time.Hour).Unix(),
		ReserveOpID:   "user-redemption:legacy:reserve",
	}
	require.NoError(t, DB.Create(&legacy).Error)

	require.NoError(t, DB.AutoMigrate(&UserRedemptionCode{}))

	var migrated UserRedemptionCode
	require.NoError(t, DB.First(&migrated, legacy.ID).Error)
	require.Equal(t, legacy.Code, migrated.Code)
	require.Empty(t, migrated.DisabledReason)
	require.Empty(t, migrated.TerminalReason)
	require.Empty(t, migrated.DeleteReason)
}

func TestUserRedemptionStatusMigrationPreservesExplicitSettlement(t *testing.T) {
	setupUserRedemptionModelTestDB(t)
	rows := []UserRedemptionCode{
		{Code: "LEGACY-COMPLETED", CreatorUserID: 1, Kind: UserRedemptionKindSingle, TotalQuota: 100, ClaimedQuota: 100, MaxClaims: 1, ClaimCount: 1, Status: UserRedemptionStatusCompleted, ReserveOpID: "legacy:completed"},
		{Code: "LEGACY-EXPIRED", CreatorUserID: 2, Kind: UserRedemptionKindSingle, TotalQuota: 100, MaxClaims: 1, Status: UserRedemptionStatusExpired, ReserveOpID: "legacy:expired"},
		{Code: "LEGACY-CANCELLED", CreatorUserID: 3, Kind: UserRedemptionKindSingle, TotalQuota: 100, MaxClaims: 1, Status: UserRedemptionStatusCancelled, ReserveOpID: "legacy:cancelled"},
		{Code: "LEGACY-REFUNDED", CreatorUserID: 4, Kind: UserRedemptionKindSingle, TotalQuota: 100, RefundedQuota: 100, MaxClaims: 1, Status: UserRedemptionStatusCancelled, ReserveOpID: "legacy:refunded"},
		{Code: "LEGACY-FORFEITED", CreatorUserID: 5, Kind: UserRedemptionKindSingle, TotalQuota: 100, ForfeitedQuota: 100, MaxClaims: 1, Status: UserRedemptionStatusCancelled, ReserveOpID: "legacy:forfeited"},
	}
	require.NoError(t, DB.Create(&rows).Error)

	require.NoError(t, MigrateUserRedemptionStatuses(DB))

	got := make([]UserRedemptionCode, 0, len(rows))
	require.NoError(t, DB.Order("id asc").Find(&got).Error)
	require.Equal(t, []string{
		UserRedemptionStatusExhausted,
		UserRedemptionStatusExpiredPending,
		UserRedemptionStatusExpiredPending,
		UserRedemptionStatusRefunded,
		UserRedemptionStatusForfeited,
	}, []string{got[0].Status, got[1].Status, got[2].Status, got[3].Status, got[4].Status})
}
