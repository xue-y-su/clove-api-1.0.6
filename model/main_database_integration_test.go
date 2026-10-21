package model

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestInitDBFullMigrationSQLite(t *testing.T) {
	previousSQLitePath := common.SQLitePath
	common.SQLitePath = filepath.Join(t.TempDir(), "cloveapi.db")
	t.Cleanup(func() {
		common.SQLitePath = previousSQLitePath
	})

	testInitDBFullMigration(t, "local", common.DatabaseTypeSQLite)
}

func TestInitDBFullMigrationMySQL(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run the full MySQL migration test")
	}

	testInitDBFullMigration(t, dsn, common.DatabaseTypeMySQL)
}

func TestInitDBFullMigrationPostgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("set TEST_POSTGRES_DSN to run the full PostgreSQL migration test")
	}

	testInitDBFullMigration(t, dsn, common.DatabaseTypePostgreSQL)
}

func testInitDBFullMigration(t *testing.T, dsn string, databaseType common.DatabaseType) {
	t.Helper()

	previousDB := DB
	previousLogDB := LOG_DB
	previousMaster := common.IsMasterNode
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	t.Setenv("SQL_DSN", dsn)
	t.Setenv("LOG_SQL_DSN", "")
	common.IsMasterNode = true
	t.Cleanup(func() {
		closeTestDatabase(DB)
		if LOG_DB != DB {
			closeTestDatabase(LOG_DB)
		}
		DB = previousDB
		LOG_DB = previousLogDB
		common.IsMasterNode = previousMaster
		common.SetDatabaseTypes(previousMainType, previousLogType)
	})

	require.NoError(t, InitDB())
	require.NoError(t, InitLogDB())
	assert.Equal(t, databaseType, common.MainDatabaseType())
	assert.Equal(t, databaseType, common.LogDatabaseType())
	assert.Same(t, DB, LOG_DB)
	require.True(t, DB.Migrator().HasColumn(&TopUp{}, "provider_store_id"))
	require.True(t, DB.Migrator().HasColumn(&SubscriptionOrder{}, "provider_store_id"))

	for name, model := range map[string]any{
		"users":                            &User{},
		"tokens":                           &Token{},
		"channels":                         &Channel{},
		"channel model publications":       &ChannelModelPublication{},
		"tasks":                            &Task{},
		"user redemption codes":            &UserRedemptionCode{},
		"wallet ledger entries":            &WalletLedgerEntry{},
		"invite reward payouts":            &InviteRewardPayout{},
		"invite reward reversals":          &InviteRewardReversal{},
		"invite reward payout actions":     &InviteRewardPayoutAction{},
		"payment provider references":      &PaymentProviderReference{},
		"audit outbox events":              &AuditOutboxEvent{},
		"announcement acknowledgements":    &AnnouncementAcknowledgement{},
		"compliance confirmations":         &ComplianceConfirmation{},
		"source attribution confirmations": &SourceAttributionConfirmation{},
		"authorization roles":              &AuthzRole{},
		"subscription plans":               &SubscriptionPlan{},
	} {
		assert.Truef(t, DB.Migrator().HasTable(model), "missing migrated table for %s", name)
	}
	assertUserWorkflowMigrationContracts(t, databaseType)

	require.NoError(t, migrateDB(), fmt.Sprintf("%s migration must be idempotent", databaseType))
	require.NoError(t, migrateLOGDB(), fmt.Sprintf("%s log migration must be idempotent", databaseType))
}

func assertUserWorkflowMigrationContracts(t *testing.T, databaseType common.DatabaseType) {
	t.Helper()

	suffix := fmt.Sprintf("%s-%x", databaseType[:1], time.Now().UnixNano())
	user := User{
		Username: suffix,
		Password: "password-hash",
		Role:     common.RoleCommonUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
		AffCode:  suffix,
	}
	require.NoError(t, DB.Create(&user).Error)

	pendingUserSlot := user.Id
	request := InviteRewardPayoutRequest{
		OperationID:     suffix,
		UserID:          user.Id,
		PendingUserSlot: &pendingUserSlot,
		Amount:          1,
		Status:          InviteRewardPayoutRequestStatusPending,
	}
	require.NoError(t, DB.Create(&request).Error)

	duplicateRequest := InviteRewardPayoutRequest{
		OperationID:     suffix + "-duplicate",
		UserID:          user.Id,
		PendingUserSlot: &pendingUserSlot,
		Amount:          1,
		Status:          InviteRewardPayoutRequestStatusPending,
	}
	require.Error(t, DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)}).Create(&duplicateRequest).Error, "pending payout requests must remain unique per user")

	use := InviteCodeUse{
		InviteCodeID:        request.ID,
		InviterUserID:       user.Id,
		RegisteredUserID:    user.Id + 1000000,
		RegisteredAt:        time.Now().Unix(),
		InviterRewardStatus: InviteRewardStatusPendingReview,
		InviteeRewardStatus: InviteRewardStatusCompleted,
		AuditEventID:        suffix + "-use",
	}
	require.NoError(t, DB.Create(&use).Error)
	legacyPayout := InviteRewardPayout{
		InviteCodeUseID: use.ID,
		RecipientKind:   InviteRewardRecipientInviter,
		RecipientUserID: user.Id,
		Amount:          1,
		Status:          InviteRewardStatusWithdrawn,
	}
	require.NoError(t, DB.Create(&legacyPayout).Error)
	require.NoError(t, MigrateInvitationRewardStatuses(DB))
	require.NoError(t, MigrateInvitationRewardFacts(DB))

	var migratedUse InviteCodeUse
	require.NoError(t, DB.First(&migratedUse, use.ID).Error)
	assert.Equal(t, InviteRewardStatusAvailable, migratedUse.InviterRewardStatus)
	assert.Equal(t, InviteRewardStatusPaid, migratedUse.InviteeRewardStatus)
	var migratedPayout InviteRewardPayout
	require.NoError(t, DB.First(&migratedPayout, legacyPayout.ID).Error)
	assert.Equal(t, InviteRewardStatusPaid, migratedPayout.Status)
	assert.Equal(t, InviteRewardTriggerRegistrationInviter, migratedPayout.TriggerType)
	assert.Equal(t, fmt.Sprintf("%d", use.ID), migratedPayout.TriggerID)

	transactionReward := InviteRewardPayout{
		InviteCodeUseID: use.ID,
		RecipientKind:   InviteRewardRecipientInviter,
		RecipientUserID: user.Id,
		TriggerType:     InviteRewardTriggerCashTopUp,
		TriggerID:       suffix + "-payment",
		EligibleQuota:   10,
		RateBasisPoints: 1_000,
		Amount:          1,
		Status:          InviteRewardStatusAvailable,
		ReversalState:   InviteRewardReversalNone,
	}
	require.NoError(t, DB.Create(&transactionReward).Error)
	duplicateReward := transactionReward
	duplicateReward.ID = 0
	require.Error(t, DB.Create(&duplicateReward).Error)

	reversal := InviteRewardReversal{
		RewardID:                 transactionReward.ID,
		RefundSourceType:         "cash_refund",
		RefundSourceID:           suffix + "-refund",
		RefundedBase:             5,
		Amount:                   1,
		CumulativeReversedAmount: 1,
		RewardStatusBefore:       InviteRewardStatusAvailable,
	}
	require.NoError(t, DB.Create(&reversal).Error)
	duplicateReversal := reversal
	duplicateReversal.ID = 0
	require.Error(t, DB.Create(&duplicateReversal).Error)

	signature := strings.Repeat("a", 64)
	created, err := RecordAnnouncementAcknowledgement(user.Id, signature, time.Now().Unix())
	require.NoError(t, err)
	assert.True(t, created)
	created, err = RecordAnnouncementAcknowledgement(user.Id, signature, time.Now().Unix())
	require.NoError(t, err)
	assert.False(t, created, "announcement acknowledgement must be idempotent per user")

	var storedUser User
	require.NoError(t, DB.First(&storedUser, user.Id).Error)
	beforeVersion := storedUser.SecurityVersion
	var updatedUser *User
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		updatedUser, err = UpdateOwnProfileWithTx(tx, user.Id, OwnProfilePatch{PasswordHash: "updated-password-hash"}, time.Now())
		return err
	}))
	require.NotNil(t, updatedUser)
	assert.Equal(t, beforeVersion+1, updatedUser.SecurityVersion)
	assert.Equal(t, "updated-password-hash", updatedUser.Password)
}

func closeTestDatabase(db *gorm.DB) {
	if db == nil {
		return
	}
	sqlDB, err := db.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}
