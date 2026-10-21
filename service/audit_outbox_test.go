package service

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	quotaLogger "github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupAuditOutboxServiceTest(t *testing.T) (*gorm.DB, *gorm.DB) {
	t.Helper()
	mainDB, err := gorm.Open(sqlite.Open(t.TempDir()+"/main.db"), &gorm.Config{})
	require.NoError(t, err)
	logDB, err := gorm.Open(sqlite.Open(t.TempDir()+"/logs.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, mainDB.AutoMigrate(
		&model.User{}, &model.Checkin{}, &model.WalletLedgerEntry{}, &model.AuditOutboxEvent{},
		&model.IPSecurityEvent{}, &model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
	))
	require.NoError(t, logDB.AutoMigrate(&model.Log{}))

	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	mainSQLDB, err := mainDB.DB()
	require.NoError(t, err)
	logSQLDB, err := logDB.DB()
	require.NoError(t, err)
	model.DB = mainDB
	model.LOG_DB = logDB
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		require.NoError(t, logSQLDB.Close())
		require.NoError(t, mainSQLDB.Close())
	})
	return mainDB, logDB
}

func seedAuditOutboxWalletUser(t *testing.T, db *gorm.DB, id int) {
	t.Helper()
	require.NoError(t, db.Create(&model.User{
		Id:       id,
		Username: "audit-wallet-user",
		Quota:    100,
		Status:   common.UserStatusEnabled,
		AffCode:  "audit-wallet-aff",
	}).Error)
}

func TestAuditOutboxWalletMutationCommitsWithEvent(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 41)

	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		_, err := AdjustWithTx(tx, 41, -25, "order:41", "purchase", WalletReference{Type: "order", ID: "41"})
		return err
	}))

	var event model.AuditOutboxEvent
	require.NoError(t, mainDB.Where("event_id = ?", "wallet:order:41").First(&event).Error)
	require.Equal(t, "wallet.adjust", event.EventType)
	require.Equal(t, model.AuditOutboxStatusPending, event.Status)
}

func TestInvitationRewardLogProjectsOneConciseSystemEntry(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 54)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "invitation:commission:54", Type: "invitation.commission_created",
			LogUserID: 54, Target: "user:54", Params: map[string]any{
				"trigger_type":      model.InviteRewardTriggerCashTopUp,
				"eligible_quota":    "9000",
				"rate_basis_points": "1000",
				"amount":            "900",
				"trigger_id":        "full-provider-order-number-must-stay-private",
			},
		})
	}))

	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	var logs []model.Log
	require.NoError(t, logDB.Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, model.LogTypeSystem, logs[0].Type)
	require.Equal(t, "邀请充值返现 +"+quotaLogger.LogQuota64(900)+"，比例 10%", logs[0].Content)
	require.NotContains(t, logs[0].Content, "full-provider-order-number")
	require.NotContains(t, logs[0].Other, "trigger_id")
}

func TestInvitationCommissionReversalProjectsOneConciseSystemEntry(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 55)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "invitation:commission-reversal:55", Type: "invitation.commission_reversed",
			LogUserID: 55, Target: "user:55", Params: map[string]any{
				"trigger_type":      model.InviteRewardTriggerCashTopUp,
				"rate_basis_points": "1000",
				"amount":            "900",
				"refund_source_id":  "private-refund-number",
			},
		})
	}))

	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	var logs []model.Log
	require.NoError(t, logDB.Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, model.LogTypeSystem, logs[0].Type)
	require.Equal(t, "邀请返现撤销 -"+quotaLogger.LogQuota64(900), logs[0].Content)
	require.NotContains(t, logs[0].Content, "private-refund-number")
	require.NotContains(t, logs[0].Other, "refund_source_id")
}

func TestSubscriptionPurchaseProjectsCanonicalTopUpLog(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 56)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "subscription:purchase:56", Type: "subscription.purchase_completed",
			ActorID: 56, LogUserID: 56, Target: "user:56", Params: map[string]any{
				"amount": "12.50", "method": "stripe", "name": "Pro plan",
				"plan_id": 7, "trade_no": "private-provider-order", "user_subscription_id": 9,
			},
		})
	}))

	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	var logs []model.Log
	require.NoError(t, logDB.Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, model.LogTypeTopup, logs[0].Type)
	require.Equal(t, "订阅购买成功，套餐: Pro plan，支付金额: 12.50，支付方式: stripe", logs[0].Content)
	require.NotContains(t, logs[0].Content, "private-provider-order")
}

func TestBalanceSubscriptionPurchaseProjectsCanonicalTopUpLog(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 57)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "subscription:purchase:57", Type: "subscription.purchase_completed",
			ActorID: 57, LogUserID: 57, Target: "user:57", Params: map[string]any{
				"amount": "2.00", "method": model.PaymentMethodBalance, "name": "Wallet plan",
				"plan_id": 8, "quota": "1000000", "trade_no": "private-balance-order", "user_subscription_id": 10,
			},
		})
	}))

	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	var logs []model.Log
	require.NoError(t, logDB.Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, model.LogTypeTopup, logs[0].Type)
	require.Equal(t, "使用余额购买订阅成功，套餐: Wallet plan，支付金额: 2.00，扣除额度: 1000000", logs[0].Content)
	require.NotContains(t, logs[0].Content, "private-balance-order")
}

func TestAuditOutboxWalletMutationRollsBackWithOuterTransaction(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 42)
	sentinel := errors.New("rollback business mutation")

	err := mainDB.Transaction(func(tx *gorm.DB) error {
		_, adjustErr := AdjustWithTx(tx, 42, -25, "order:42", "purchase", WalletReference{Type: "order", ID: "42"})
		require.NoError(t, adjustErr)
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)

	var count int64
	require.NoError(t, mainDB.Model(&model.AuditOutboxEvent{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestRelayWalletMutationKeepsLedgerWithoutRedundantAuditEvent(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 44)

	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		_, err := AdjustWithTx(tx, 44, -25, "relay:request-44:settle:1", "relay", WalletReference{
			Type: "relay_request", ID: "request-44", Context: "settle_debit",
		})
		return err
	}))

	var ledgerCount int64
	require.NoError(t, mainDB.Model(&model.WalletLedgerEntry{}).
		Where("operation_id = ?", "relay:request-44:settle:1").Count(&ledgerCount).Error)
	require.Equal(t, int64(1), ledgerCount)

	var auditCount int64
	require.NoError(t, mainDB.Model(&model.AuditOutboxEvent{}).
		Where("event_type = ?", "wallet.adjust").Count(&auditCount).Error)
	require.Zero(t, auditCount)
}

func TestUserRedemptionWalletEntriesKeepOneUserVisibleBusinessLog(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 43)

	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		if _, err := AdjustWithTx(tx, 43, -25, "user-redemption:7:reserve", userRedemptionCategoryReserve, WalletReference{
			Type: "user_redemption_code", ID: "7", Context: "reserve",
		}); err != nil {
			return err
		}
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "user-redemption:7:create", Type: "user.redemption.create",
			ActorID: 43, LogUserID: 43, Target: "user_redemption_code:7",
			Params: map[string]any{"id": 7, "target_user_id": 43},
		})
	}))

	var ledgerCount int64
	require.NoError(t, mainDB.Model(&model.WalletLedgerEntry{}).
		Where("operation_id = ?", "user-redemption:7:reserve").Count(&ledgerCount).Error)
	require.Equal(t, int64(1), ledgerCount)

	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)

	var logs []model.Log
	require.NoError(t, logDB.Where("user_id = ?", 43).Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Equal(t, "user.redemption.create", logs[0].Content)
}

func TestAuditOutboxPayloadDropsUnknownAndSensitiveParams(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)

	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID:   "redaction-1",
			Type:      "user.role_change",
			Result:    "success",
			ActorID:   1,
			Target:    "user:9",
			IP:        "127.0.0.1",
			RequestID: "request-1",
			Params: map[string]any{
				"target_user_id": 9,
				"role_from":      1,
				"role_to":        10,
				"password":       "do-not-store",
				"authorization":  "Bearer secret",
				"mystery":        "unknown-value",
			},
		})
	}))

	var event model.AuditOutboxEvent
	require.NoError(t, mainDB.Where("event_id = ?", "redaction-1").First(&event).Error)
	require.Contains(t, event.Payload, `"target_user_id":9`)
	require.Contains(t, event.Payload, `"role_from":1`)
	require.NotContains(t, event.Payload, "do-not-store")
	require.NotContains(t, strings.ToLower(event.Payload), "authorization")
	require.NotContains(t, event.Payload, "unknown-value")
}

func TestAuditOutboxPreservesActorWhileAssigningLogOwner(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.Create(&model.User{
		Id: 120, Username: "audit-actor", AffCode: "audit-actor",
		Status: common.UserStatusEnabled, Role: common.RoleAdminUser,
	}).Error)
	require.NoError(t, mainDB.Create(&model.User{
		Id: 121, Username: "audit-owner", AffCode: "audit-owner",
		Status: common.UserStatusEnabled, Role: common.RoleCommonUser,
	}).Error)

	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID:   "redemption-disable-owner-log",
			Type:      "user.redemption.disable",
			ActorID:   120,
			LogUserID: 121,
			Target:    "user_redemption_code:1",
			Params: map[string]any{
				"id":             1,
				"target_user_id": 121,
			},
		})
	}))

	delivered, failed, err := DeliverPending(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)

	var log model.Log
	require.NoError(t, logDB.Where("event_id = ?", "redemption-disable-owner-log").First(&log).Error)
	require.Equal(t, 121, log.UserId)
	require.Equal(t, "audit-owner", log.Username)
	other, err := common.StrToMap(log.Other)
	require.NoError(t, err)
	adminInfo, ok := other["admin_info"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, float64(120), adminInfo["actor_id"])
}

func TestDeliverPendingUsesFreshClaimTimestampForEachCandidate(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	for _, eventID := range []string{"fresh-claim-clock-1", "fresh-claim-clock-2"} {
		require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
			return EnqueueWithTx(tx, AuditEvent{
				EventID: eventID, Type: "user.role_change", ActorID: 1, Target: "user:9",
			})
		}))
	}

	var mu sync.Mutex
	claimDeadlines := make([]int64, 0, 2)
	claimCallback := "audit_outbox_test:capture_claim_deadline"
	require.NoError(t, mainDB.Callback().Update().After("gorm:update").Register(claimCallback, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Schema.Table != "audit_outbox_events" {
			return
		}
		updates, ok := tx.Statement.Dest.(map[string]any)
		if !ok {
			return
		}
		if _, isClaim := updates["attempt_count"]; !isClaim {
			return
		}
		deadline, ok := updates["locked_until"].(int64)
		if !ok {
			return
		}
		mu.Lock()
		claimDeadlines = append(claimDeadlines, deadline)
		mu.Unlock()
	}))
	t.Cleanup(func() { _ = mainDB.Callback().Update().Remove(claimCallback) })

	createCount := 0
	slowFirstCreate := "audit_outbox_test:slow_first_log_create"
	require.NoError(t, logDB.Callback().Create().Before("gorm:create").Register(slowFirstCreate, func(_ *gorm.DB) {
		createCount++
		if createCount != 1 {
			return
		}
		startedAt := time.Now().Unix()
		for time.Now().Unix() == startedAt {
			time.Sleep(10 * time.Millisecond)
		}
	}))
	t.Cleanup(func() { _ = logDB.Callback().Create().Remove(slowFirstCreate) })

	delivered, failed, err := DeliverPending(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, 2, delivered)
	require.Zero(t, failed)
	require.Len(t, claimDeadlines, 2)
	require.Greater(t, claimDeadlines[1], claimDeadlines[0], "each candidate must use the clock at claim time")
}

func TestAuditOutboxStaleOwnerCannotPrepareOrAppendLog(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.Create(&model.AuditOutboxEvent{
		EventID: "stale-owner-snapshot", EventType: "user.role_change", Result: "success",
		ActorID: 1, Target: "user:9", Payload: "{}", Status: model.AuditOutboxStatusPending,
		LockedBy: "worker-a", LockedUntil: common.GetTimestamp() + 30,
	}).Error)
	var stale model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&stale, "event_id = ?", "stale-owner-snapshot").Error)
	require.NoError(t, mainDB.Model(&model.AuditOutboxEvent{}).
		Where("id = ?", stale.ID).
		Updates(map[string]any{"locked_by": "worker-b", "locked_until": common.GetTimestamp() + 60}).Error)

	err := deliverClaimedAuditOutboxEvent(context.Background(), stale.ID, stale.LockedBy)
	require.Error(t, err)
	var current model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&current, stale.ID).Error)
	require.Zero(t, current.LogPreparedAt)
	var logCount int64
	require.NoError(t, logDB.Model(&model.Log{}).Where("event_id = ?", stale.EventID).Count(&logCount).Error)
	require.Zero(t, logCount)
}

func TestAuditOutboxLegacyPendingAdoptsCanonicalLogAfterUsernameChange(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 60)
	payload, err := common.Marshal(map[string]any{"target_user_id": 9})
	require.NoError(t, err)
	legacy := model.AuditOutboxEvent{
		EventID: "legacy-canonical-rename", EventType: "user.role_change", Result: "success",
		ActorID: 60, Target: "user:9", RequestID: "legacy-canonical-request",
		Payload: string(payload), Status: model.AuditOutboxStatusPending, CreatedAt: common.GetTimestamp(),
	}
	require.NoError(t, mainDB.Create(&legacy).Error)
	eventID := legacy.EventID
	require.NoError(t, logDB.Create(&model.Log{
		EventId: &eventID, UserId: 60, Username: "audit-wallet-user", CreatedAt: legacy.CreatedAt,
		Type: model.LogTypeManage, Content: legacy.EventType, RequestId: legacy.RequestID,
		Other: common.MapToJsonStr(map[string]any{
			"op":         map[string]any{"action": legacy.EventType, "params": map[string]any{"target_user_id": float64(9)}},
			"audit_info": map[string]any{"event_type": legacy.EventType, "result": legacy.Result, "target": legacy.Target},
		}),
	}).Error)
	require.NoError(t, mainDB.Model(&model.User{}).Where("id = ?", 60).Update("username", "renamed-user").Error)

	delivered, failed, err := DeliverPending(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	var persisted model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&persisted, legacy.ID).Error)
	require.Equal(t, model.AuditOutboxStatusDelivered, persisted.Status)
}

func TestAuditOutboxLegacyNullSnapshotIsPreparedAndDelivered(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	legacy := model.AuditOutboxEvent{
		EventID: "legacy-null-snapshot", EventType: "user.role_change", Result: "success",
		Target: "user:9", Payload: "{}", Status: model.AuditOutboxStatusPending,
	}
	require.NoError(t, mainDB.Create(&legacy).Error)
	require.NoError(t, mainDB.Exec(
		"UPDATE audit_outbox_events SET log_snapshot = NULL WHERE id = ?", legacy.ID,
	).Error)

	delivered, failed, err := DeliverPending(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)

	var persisted model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&persisted, legacy.ID).Error)
	require.Equal(t, model.AuditOutboxStatusDelivered, persisted.Status)
	require.NotEmpty(t, persisted.LogSnapshot)
	var logCount int64
	require.NoError(t, logDB.Model(&model.Log{}).Where("event_id = ?", legacy.EventID).Count(&logCount).Error)
	require.EqualValues(t, 1, logCount)
}

func TestAuditOutboxPreparedSnapshotIsImmutableAcrossUsernameChanges(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 61)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "immutable-prepared-snapshot", Type: "user.role_change", Result: "success",
			ActorID: 61, Target: "user:9", RequestID: "immutable-request",
		})
	}))

	var event model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&event, "event_id = ?", "immutable-prepared-snapshot").Error)
	const workerID = "snapshot-worker"
	claimed, err := claimAuditOutboxEvent(context.Background(), event.ID, workerID, common.GetTimestamp())
	require.NoError(t, err)
	require.True(t, claimed)

	newlyPrepared, err := prepareClaimedAuditOutboxLog(context.Background(), event.ID, workerID)
	require.NoError(t, err)
	require.True(t, newlyPrepared)
	require.NoError(t, mainDB.First(&event, event.ID).Error)
	firstSnapshot := event.LogSnapshot
	firstLog, err := decodeAuditLogSnapshot(firstSnapshot, event.LogSnapshotVersion)
	require.NoError(t, err)
	require.Equal(t, "audit-wallet-user", firstLog.Username)

	require.NoError(t, mainDB.Model(&model.User{}).Where("id = ?", 61).Update("username", "renamed-after-prepare").Error)
	newlyPrepared, err = prepareClaimedAuditOutboxLog(context.Background(), event.ID, workerID)
	require.NoError(t, err)
	require.False(t, newlyPrepared)
	require.NoError(t, mainDB.First(&event, event.ID).Error)
	require.Equal(t, firstSnapshot, event.LogSnapshot)
	secondLog, err := decodeAuditLogSnapshot(event.LogSnapshot, event.LogSnapshotVersion)
	require.NoError(t, err)
	require.Equal(t, "audit-wallet-user", secondLog.Username)
}

func TestAuditLogSnapshotRejectsValuesBeyondPortableTextLimit(t *testing.T) {
	eventID := "oversized-audit-log-snapshot"
	_, err := encodeAuditLogSnapshot(&model.Log{
		EventId: &eventID,
		Other:   strings.Repeat("x", 64*1024),
	})
	require.Error(t, err)
}

func TestAuditOutboxClickHouseAppendDoesNotBlockPrimaryWrites(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 63)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "nonblocking-clickhouse-append", Type: "user.role_change", Result: "success",
			ActorID: 63, Target: "user:9", RequestID: "nonblocking-request",
		})
	}))
	var event model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&event, "event_id = ?", "nonblocking-clickhouse-append").Error)

	previousLogType := common.LogDatabaseType()
	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)
	previousCreate := auditLogCreate
	previousFind := auditLogFindCanonical
	t.Cleanup(func() {
		auditLogCreate = previousCreate
		auditLogFindCanonical = previousFind
		common.SetLogDatabaseType(previousLogType)
	})

	createStarted := make(chan struct{})
	releaseCreate := make(chan struct{})
	auditLogCreate = func(_ context.Context, log *model.Log) error {
		close(createStarted)
		<-releaseCreate
		return nil
	}

	const workerA = "worker-a"
	claimed, err := claimAuditOutboxEvent(context.Background(), event.ID, workerA, common.GetTimestamp())
	require.NoError(t, err)
	require.True(t, claimed)
	deliveryDone := make(chan error, 1)
	go func() {
		deliveryDone <- deliverClaimedAuditOutboxEvent(context.Background(), event.ID, workerA)
	}()
	<-createStarted

	primaryWriteDone := make(chan error, 1)
	go func() {
		primaryWriteDone <- mainDB.Model(&model.User{}).
			Where("id = ?", 63).
			Update("display_name", "write-during-clickhouse-append").Error
	}()

	var primaryWriteErr error
	select {
	case primaryWriteErr = <-primaryWriteDone:
	case <-time.After(time.Second):
		primaryWriteErr = errors.New("primary write blocked behind ClickHouse append")
	}
	close(releaseCreate)
	deliveryErr := <-deliveryDone
	require.NoError(t, primaryWriteErr)
	require.NoError(t, deliveryErr)
	require.NoError(t, mainDB.First(&event, event.ID).Error)
	require.Equal(t, model.AuditOutboxStatusDelivered, event.Status)
}

func TestAuditOutboxExpiredLeaseCanBeTakenOverDuringClickHouseAppend(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "clickhouse-lease-takeover", Type: "user.role_change", Result: "success",
			Target: "user:9", RequestID: "takeover-request",
		})
	}))
	var event model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&event, "event_id = ?", "clickhouse-lease-takeover").Error)

	previousLogType := common.LogDatabaseType()
	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)
	previousCreate := auditLogCreate
	previousFind := auditLogFindCanonical
	t.Cleanup(func() {
		auditLogCreate = previousCreate
		auditLogFindCanonical = previousFind
		common.SetLogDatabaseType(previousLogType)
	})

	var storeMu sync.Mutex
	var stored *model.Log
	createStarted := make(chan struct{})
	releaseCreate := make(chan struct{})
	auditLogCreate = func(_ context.Context, log *model.Log) error {
		close(createStarted)
		<-releaseCreate
		clone := *log
		if log.EventId != nil {
			eventID := *log.EventId
			clone.EventId = &eventID
		}
		storeMu.Lock()
		stored = &clone
		storeMu.Unlock()
		return nil
	}
	auditLogFindCanonical = func(_ context.Context, _ string) (model.Log, bool, error) {
		storeMu.Lock()
		defer storeMu.Unlock()
		if stored == nil {
			return model.Log{}, false, nil
		}
		return *stored, true, nil
	}

	const workerA = "worker-a"
	claimed, err := claimAuditOutboxEvent(context.Background(), event.ID, workerA, common.GetTimestamp())
	require.NoError(t, err)
	require.True(t, claimed)
	deliveryDone := make(chan error, 1)
	go func() {
		deliveryDone <- deliverClaimedAuditOutboxEvent(context.Background(), event.ID, workerA)
	}()
	<-createStarted

	type claimResult struct {
		claimed bool
		err     error
	}
	claimDone := make(chan claimResult, 1)
	go func() {
		claimed, claimErr := claimAuditOutboxEvent(
			context.Background(), event.ID, "worker-b",
			common.GetTimestamp()+int64(auditOutboxLease/time.Second)+1,
		)
		claimDone <- claimResult{claimed: claimed, err: claimErr}
	}()
	var takeover claimResult
	claimReturnedBeforeRelease := false
	select {
	case takeover = <-claimDone:
		claimReturnedBeforeRelease = true
	case <-time.After(time.Second):
	}
	close(releaseCreate)
	deliveryErr := <-deliveryDone
	if !claimReturnedBeforeRelease {
		takeover = <-claimDone
	}
	require.True(t, claimReturnedBeforeRelease, "takeover was blocked behind the external append")
	require.NoError(t, takeover.err)
	require.True(t, takeover.claimed)
	require.ErrorIs(t, deliveryErr, errAuditOutboxLeaseLost)

	storeMu.Lock()
	require.NotNil(t, stored)
	storeMu.Unlock()
	require.NoError(t, deliverClaimedAuditOutboxEvent(context.Background(), event.ID, "worker-b"))
	require.NoError(t, mainDB.First(&event, event.ID).Error)
	require.Equal(t, model.AuditOutboxStatusDelivered, event.Status)
}

func TestAuditOutboxClickHouseLookupCountsByDeliveryStage(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	previousLogType := common.LogDatabaseType()
	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)
	previousCreate := auditLogCreate
	previousFind := auditLogFindCanonical
	t.Cleanup(func() {
		auditLogCreate = previousCreate
		auditLogFindCanonical = previousFind
		common.SetLogDatabaseType(previousLogType)
	})

	var stored model.Log
	storedPresent := false
	createCount := 0
	findCount := 0
	auditLogCreate = func(_ context.Context, log *model.Log) error {
		createCount++
		stored = *log
		if log.EventId != nil {
			eventID := *log.EventId
			stored.EventId = &eventID
		}
		storedPresent = true
		return nil
	}
	auditLogFindCanonical = func(_ context.Context, _ string) (model.Log, bool, error) {
		findCount++
		return stored, storedPresent, nil
	}

	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "clickhouse-stage-lookups", Type: "user.role_change", Result: "success",
			ActorID: 1, Target: "user:9", RequestID: "lookup-request",
		})
	}))
	delivered, failed, err := DeliverPending(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	require.Equal(t, 1, createCount)
	require.Zero(t, findCount, "a fresh ClickHouse delivery must not scan event_id")

	var event model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&event, "event_id = ?", "clickhouse-stage-lookups").Error)
	require.NoError(t, mainDB.Model(&model.AuditOutboxEvent{}).Where("id = ?", event.ID).Updates(map[string]any{
		"status": model.AuditOutboxStatusPending, "delivered_at": 0, "next_attempt_at": 0,
		"log_persisted_at": 0,
	}).Error)
	delivered, failed, err = DeliverPending(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	require.Equal(t, 1, createCount)
	require.Equal(t, 1, findCount, "an ambiguous prepared retry must read canonical once")

	require.NoError(t, mainDB.Model(&model.AuditOutboxEvent{}).Where("id = ?", event.ID).Updates(map[string]any{
		"status": model.AuditOutboxStatusPending, "delivered_at": 0, "next_attempt_at": 0,
	}).Error)
	delivered, failed, err = DeliverPending(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	require.Equal(t, 1, createCount)
	require.Equal(t, 1, findCount, "a persisted retry must not read ClickHouse")
}

func TestAuditOutboxClickHouseCreateSurvivesPersistedStageFailure(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 62)
	previousLogType := common.LogDatabaseType()
	common.SetLogDatabaseType(common.DatabaseTypeClickHouse)
	previousCreate := auditLogCreate
	previousFind := auditLogFindCanonical
	t.Cleanup(func() {
		auditLogCreate = previousCreate
		auditLogFindCanonical = previousFind
		common.SetLogDatabaseType(previousLogType)
	})

	var stored model.Log
	storedPresent := false
	createCount := 0
	findCount := 0
	auditLogCreate = func(_ context.Context, log *model.Log) error {
		createCount++
		stored = *log
		if log.EventId != nil {
			eventID := *log.EventId
			stored.EventId = &eventID
		}
		storedPresent = true
		return nil
	}
	auditLogFindCanonical = func(_ context.Context, _ string) (model.Log, bool, error) {
		findCount++
		return stored, storedPresent, nil
	}

	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "clickhouse-stage-failure", Type: "user.role_change", Result: "success",
			ActorID: 62, Target: "user:9", RequestID: "stage-failure-request",
		})
	}))
	failure := errors.New("persisted stage unavailable")
	callbackName := "audit_outbox_test:fail_persisted_stage"
	require.NoError(t, mainDB.Callback().Update().Before("gorm:update").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement == nil || tx.Statement.Schema == nil || tx.Statement.Schema.Table != "audit_outbox_events" {
			return
		}
		if updates, ok := tx.Statement.Dest.(map[string]any); ok {
			if _, persists := updates["log_persisted_at"]; persists {
				tx.AddError(failure)
			}
		}
	}))
	callbackRegistered := true
	t.Cleanup(func() {
		if callbackRegistered {
			_ = mainDB.Callback().Update().Remove(callbackName)
		}
	})

	delivered, failed, err := DeliverPending(context.Background(), 1)
	require.NoError(t, err)
	require.Zero(t, delivered)
	require.Equal(t, 1, failed)
	require.Equal(t, 1, createCount)
	require.Zero(t, findCount)

	var event model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&event, "event_id = ?", "clickhouse-stage-failure").Error)
	require.NotEmpty(t, event.LogSnapshot)
	require.Zero(t, event.LogPersistedAt)
	require.NoError(t, mainDB.Model(&model.User{}).Where("id = ?", 62).Update("username", "renamed-after-create").Error)
	require.NoError(t, mainDB.Callback().Update().Remove(callbackName))
	callbackRegistered = false
	require.NoError(t, mainDB.Model(&model.AuditOutboxEvent{}).Where("id = ?", event.ID).Update("next_attempt_at", 0).Error)

	delivered, failed, err = DeliverPending(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	require.Equal(t, 1, createCount)
	require.Equal(t, 1, findCount)
	require.Equal(t, "audit-wallet-user", stored.Username)
}

func TestEnqueueWithTxValidatesCanonicalDuplicateWhenRowsAffectedLooksInserted(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	first := AuditEvent{
		EventID: "mysql-found-rows-enqueue", Type: "user.role_change", Result: "success",
		ActorID: 1, Target: "user:9", Params: map[string]any{"target_user_id": 9},
	}
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, first)
	}))

	callbackName := "audit_outbox_test:simulate_client_found_rows_enqueue"
	require.NoError(t, mainDB.Callback().Create().After("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "audit_outbox_events" {
			tx.RowsAffected = 1
		}
	}))
	t.Cleanup(func() { _ = mainDB.Callback().Create().Remove(callbackName) })

	conflict := first
	conflict.Target = "user:10"
	require.ErrorIs(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, conflict)
	}), ErrAuditEventConflict)
}

func TestAuditOutboxDeliveryValidatesCanonicalLogWhenRowsAffectedLooksInserted(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "mysql-found-rows-log", Type: "user.role_change", Result: "success",
			ActorID: 1, Target: "user:9", RequestID: "canonical-request",
		})
	}))
	conflictingEventID := "mysql-found-rows-log"
	require.NoError(t, logDB.Create(&model.Log{
		EventId: &conflictingEventID, UserId: 99, CreatedAt: 1, Type: model.LogTypeManage,
		Content: "conflicting.event", RequestId: "conflicting-request",
	}).Error)
	callbackName := "audit_outbox_test:simulate_client_found_rows_log"
	require.NoError(t, logDB.Callback().Create().After("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "logs" {
			tx.RowsAffected = 1
		}
	}))
	t.Cleanup(func() { _ = logDB.Callback().Create().Remove(callbackName) })

	var event model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&event, "event_id = ?", conflictingEventID).Error)
	const workerID = "canonical-conflict-worker"
	claimed, err := claimAuditOutboxEvent(context.Background(), event.ID, workerID, common.GetTimestamp())
	require.NoError(t, err)
	require.True(t, claimed)
	require.ErrorIs(t, deliverClaimedAuditOutboxEvent(context.Background(), event.ID, workerID), ErrAuditEventConflict)
}

func TestAuditOutboxRelationalDeliveryIsIdempotentByEventID(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "delivery-1", Type: "user.role_change", Result: "success", ActorID: 1,
			Target: "user:9", RequestID: "request-delivery-1", Params: map[string]any{"target_user_id": 9},
		})
	}))

	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)

	require.NoError(t, mainDB.Model(&model.AuditOutboxEvent{}).
		Where("event_id = ?", "delivery-1").
		Updates(map[string]any{"status": model.AuditOutboxStatusPending, "delivered_at": 0, "next_attempt_at": 0}).Error)
	delivered, failed, err = DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)

	var count int64
	require.NoError(t, logDB.Model(&model.Log{}).Where("event_id = ?", "delivery-1").Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestAuditOutboxLoginFailureDeliveryAssociatesKnownTargetUser(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 51)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "login-failure-known", Type: "user.login_failed", Result: "failure", ActorID: 0,
			Target: "user:51", RequestID: "request-login-known", Params: map[string]any{
				"attempt_identifier": "redacted-attempt", "login_method": "password",
				"failure_category": "invalid_credentials", "target_user_id": 51,
			},
		})
	}))

	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)

	var log model.Log
	require.NoError(t, logDB.First(&log, "event_id = ?", "login-failure-known").Error)
	require.Equal(t, 51, log.UserId)
	require.Empty(t, log.Username)
}

func TestAuditOutboxLoginFailureDeliveryKeepsUnknownTargetAtZero(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "login-failure-unknown", Type: "user.login_failed", Result: "failure", ActorID: 0,
			Target: "login-attempt:redacted", RequestID: "request-login-unknown", Params: map[string]any{
				"attempt_identifier": "redacted-attempt", "login_method": "oauth:audit",
				"failure_category": "token_exchange_failed",
			},
		})
	}))

	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)

	var log model.Log
	require.NoError(t, logDB.First(&log, "event_id = ?", "login-failure-unknown").Error)
	require.Zero(t, log.UserId)
}

func TestAuditOutboxRegistrationAndLogoutDeliveryUseSecurityEventProjection(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 52)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		if err := EnqueueWithTx(tx, AuditEvent{
			EventID: "registration-security-event", Type: "user.register", Result: "success", ActorID: 42,
			Target: "user:52", IP: "203.0.113.52", RequestID: "request-register-52", Params: map[string]any{
				"target_user_id": 52, "source": "admin",
			},
		}); err != nil {
			return err
		}
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "logout-security-event", Type: "user.logout", Result: "success", ActorID: 52,
			Target: "user:52", IP: "203.0.113.52", RequestID: "request-logout-52",
		})
	}))

	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 2, delivered)
	require.Zero(t, failed)
	var logs []model.Log
	require.NoError(t, logDB.Order("event_id asc").Find(&logs).Error)
	require.Len(t, logs, 2)
	for _, log := range logs {
		require.Equal(t, 52, log.UserId)
		require.Equal(t, model.LogTypeLogin, log.Type)
	}
	require.Equal(t, "user.logout", logs[0].Content)
	require.Equal(t, "user.register", logs[1].Content)
	var securityEvents []model.IPSecurityEvent
	require.NoError(t, mainDB.Order("event_id asc").Find(&securityEvents).Error)
	require.Len(t, securityEvents, 2)
	require.Equal(t, model.IPSecurityKindLogout, securityEvents[0].Kind)
	require.Equal(t, model.IPSecurityKindRegistration, securityEvents[1].Kind)
	for _, event := range securityEvents {
		require.Equal(t, 52, event.UserID)
		require.Equal(t, "203.0.113.52", event.IP)
	}
}

func TestAuditOutboxMissingRegistrationUserDoesNotEmitRecordNotFoundWarning(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	setIPSecurityEnabledForServiceTest(t, true)
	const userID = 2880
	require.NoError(t, mainDB.Create(&model.User{
		Id: userID, Username: "deleted-registration-user", AffCode: "deleted-registration-aff",
		Status: common.UserStatusEnabled,
	}).Error)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "missing-registration-user", Type: "user.register", Result: "success",
			ActorID: userID, Target: "user:2880", IP: "203.0.113.80", RequestID: "missing-registration-request",
			Params: map[string]any{"target_user_id": userID},
		})
	}))
	require.NoError(t, mainDB.Delete(&model.User{}, userID).Error)

	var output strings.Builder
	model.DB = mainDB.Session(&gorm.Session{Logger: logger.New(log.New(&output, "", 0), logger.Config{
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: false,
		Colorful:                  false,
	})})
	delivered, failed, err := DeliverPending(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	require.NotContains(t, output.String(), "record not found")
}

func TestAuditOutboxDeliveryKeepsLogWithoutIPSecurityProjectionWhenDisabled(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 54)
	setIPSecurityEnabledForServiceTest(t, false)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "disabled-ip-projection", Type: "user.logout", Result: "success", ActorID: 54,
			Target: "user:54", IP: "203.0.113.54", RequestID: "disabled-ip-projection-request",
		})
	}))

	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	var logCount int64
	require.NoError(t, logDB.Model(&model.Log{}).Where("event_id = ?", "disabled-ip-projection").Count(&logCount).Error)
	require.EqualValues(t, 1, logCount)
	var eventCount int64
	require.NoError(t, mainDB.Model(&model.IPSecurityEvent{}).Count(&eventCount).Error)
	require.Zero(t, eventCount)
	var event model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&event, "event_id = ?", "disabled-ip-projection").Error)
	require.Equal(t, model.AuditOutboxStatusDelivered, event.Status)
}

func TestAuditOutboxPreservesIPv4MappedIPv6ThroughDelivery(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "mapped-ip-delivery", Type: "user.login_failed", Result: "failure",
			IP: "::ffff:203.0.113.57", Target: "login-attempt:redacted",
		})
	}))

	var event model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&event, "event_id = ?", "mapped-ip-delivery").Error)
	require.Equal(t, "::ffff:203.0.113.57", event.IP)
	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	var log model.Log
	require.NoError(t, logDB.First(&log, "event_id = ?", "mapped-ip-delivery").Error)
	require.Equal(t, "::ffff:203.0.113.57", log.Ip)
}

func TestAuditOutboxFailedDeliveryRemainsRetryable(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{EventID: "retry-1", Type: "wallet.adjust", Result: "success", ActorID: 1, Target: "user:1"})
	}))

	failure := errors.New("log database unavailable")
	require.NoError(t, logDB.Callback().Create().Before("gorm:create").Register("audit_outbox_test:fail", func(tx *gorm.DB) {
		tx.AddError(failure)
	}))
	t.Cleanup(func() {
		_ = logDB.Callback().Create().Remove("audit_outbox_test:fail")
	})

	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Zero(t, delivered)
	require.Equal(t, 1, failed)

	var event model.AuditOutboxEvent
	require.NoError(t, mainDB.Where("event_id = ?", "retry-1").First(&event).Error)
	require.Equal(t, model.AuditOutboxStatusPending, event.Status)
	require.Equal(t, 1, event.AttemptCount)
	require.Contains(t, event.LastError, failure.Error())
	require.Greater(t, event.NextAttemptAt, event.UpdatedAt)
}

func TestAuditOutboxProjectionFailureRetriesWithoutDuplicateLogOrRollup(t *testing.T) {
	mainDB, logDB := setupAuditOutboxServiceTest(t)
	seedAuditOutboxWalletUser(t, mainDB, 53)
	require.NoError(t, mainDB.Transaction(func(tx *gorm.DB) error {
		return EnqueueWithTx(tx, AuditEvent{
			EventID: "projection-retry-1", Type: "user.logout", Result: "success", ActorID: 53,
			Target: "user:53", IP: "203.0.113.53", RequestID: "projection-retry-request",
		})
	}))

	failure := errors.New("IP security projection unavailable")
	callbackName := "audit_outbox_test:fail_ip_security_projection"
	require.NoError(t, mainDB.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "ip_security_events" {
			tx.AddError(failure)
		}
	}))
	callbackRegistered := true
	t.Cleanup(func() {
		if callbackRegistered {
			_ = mainDB.Callback().Create().Remove(callbackName)
		}
	})

	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Zero(t, delivered)
	require.Equal(t, 1, failed)
	var event model.AuditOutboxEvent
	require.NoError(t, mainDB.First(&event, "event_id = ?", "projection-retry-1").Error)
	require.Equal(t, model.AuditOutboxStatusPending, event.Status)
	require.Equal(t, 1, event.AttemptCount)
	require.Contains(t, event.LastError, failure.Error())
	require.Greater(t, event.LogPersistedAt, int64(0))
	var logCount int64
	require.NoError(t, logDB.Model(&model.Log{}).Where("event_id = ?", "projection-retry-1").Count(&logCount).Error)
	require.EqualValues(t, 1, logCount)
	var securityEventCount int64
	require.NoError(t, mainDB.Model(&model.IPSecurityEvent{}).Where("event_id = ?", "projection-retry-1").Count(&securityEventCount).Error)
	require.Zero(t, securityEventCount)
	var persistedLog model.Log
	require.NoError(t, logDB.First(&persistedLog, "event_id = ?", "projection-retry-1").Error)
	require.NoError(t, mainDB.Model(&model.User{}).Where("id = ?", 53).Update("username", "renamed-after-log").Error)

	require.NoError(t, mainDB.Callback().Create().Remove(callbackName))
	callbackRegistered = false
	require.NoError(t, mainDB.Model(&model.AuditOutboxEvent{}).
		Where("event_id = ?", "projection-retry-1").Update("next_attempt_at", 0).Error)
	delivered, failed, err = DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)

	require.NoError(t, logDB.Model(&model.Log{}).Where("event_id = ?", "projection-retry-1").Count(&logCount).Error)
	require.EqualValues(t, 1, logCount)
	require.NoError(t, mainDB.Model(&model.IPSecurityEvent{}).Where("event_id = ?", "projection-retry-1").Count(&securityEventCount).Error)
	require.EqualValues(t, 1, securityEventCount)
	var securityEvent model.IPSecurityEvent
	require.NoError(t, mainDB.First(&securityEvent, "event_id = ?", "projection-retry-1").Error)
	require.Equal(t, persistedLog.Username, securityEvent.Username)
	var lifetime model.IPAuditRollup
	require.NoError(t, mainDB.First(&lifetime, "bucket = ? AND user_id = ?", 0, 53).Error)
	require.Equal(t, 1, lifetime.RequestCount)
	require.Equal(t, 1, lifetime.LogoutCount)
	require.Zero(t, lifetime.AnomalyCount)
	require.NoError(t, mainDB.First(&event, "event_id = ?", "projection-retry-1").Error)
	require.Equal(t, model.AuditOutboxStatusDelivered, event.Status)
	require.Equal(t, 2, event.AttemptCount)
	require.Empty(t, event.LastError)

	require.NoError(t, mainDB.Model(&model.User{}).Where("id = ?", 53).Update("username", "renamed-after-delivery").Error)
	require.NoError(t, mainDB.Model(&model.AuditOutboxEvent{}).
		Where("event_id = ?", "projection-retry-1").
		Updates(map[string]any{"status": model.AuditOutboxStatusPending, "delivered_at": 0, "next_attempt_at": 0}).Error)
	delivered, failed, err = DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	require.NoError(t, logDB.Model(&model.Log{}).Where("event_id = ?", "projection-retry-1").Count(&logCount).Error)
	require.EqualValues(t, 1, logCount)
	require.NoError(t, mainDB.Model(&model.IPSecurityEvent{}).Where("event_id = ?", "projection-retry-1").Count(&securityEventCount).Error)
	require.EqualValues(t, 1, securityEventCount)
	require.NoError(t, mainDB.First(&lifetime, "bucket = ? AND user_id = ?", 0, 53).Error)
	require.Equal(t, 1, lifetime.RequestCount)
	require.Equal(t, 1, lifetime.LogoutCount)
	require.NoError(t, mainDB.First(&event, "event_id = ?", "projection-retry-1").Error)
	require.Equal(t, model.AuditOutboxStatusDelivered, event.Status)
	require.Equal(t, 3, event.AttemptCount)
}

func TestAuditOutboxMetricsExposeBacklogAgeFailuresAndLastError(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	now := common.GetTimestamp()
	require.NoError(t, mainDB.Create(&[]model.AuditOutboxEvent{
		{EventID: "metrics-old", EventType: "wallet.adjust", Status: model.AuditOutboxStatusPending, CreatedAt: now - 120},
		{EventID: "metrics-failed", EventType: "user.role_change", Status: model.AuditOutboxStatusPending, AttemptCount: 2, LastError: "latest delivery failure", CreatedAt: now - 30, UpdatedAt: now - 5},
		{EventID: "metrics-delivered", EventType: "wallet.adjust", Status: model.AuditOutboxStatusDelivered, AttemptCount: 1, CreatedAt: now - 300, DeliveredAt: now - 250},
	}).Error)

	metrics, err := GetAuditOutboxMetrics(context.Background(), now)
	require.NoError(t, err)
	require.EqualValues(t, 2, metrics.PendingCount)
	require.EqualValues(t, 120, metrics.OldestPendingAgeSeconds)
	require.EqualValues(t, 1, metrics.FailureCount)
	require.Equal(t, "latest delivery failure", metrics.LastError)
}

func TestAuditOutboxWorkerReturnsWhenContextIsAlreadyCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.NoError(t, runAuditOutboxWorker(ctx, time.Hour, 10))
}

func TestAuditOutboxWorkerStopIsIdempotent(t *testing.T) {
	setupAuditOutboxServiceTest(t)
	worker := startAuditOutboxWorker(context.Background(), time.Hour, 10)
	stopContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, worker.Stop(stopContext))
	require.NoError(t, worker.Stop(stopContext))
}

func TestSystemInstanceReportIncludesAuditAndIPSecurityProjectionMetrics(t *testing.T) {
	mainDB, _ := setupAuditOutboxServiceTest(t)
	require.NoError(t, mainDB.AutoMigrate(&model.SystemInstance{}, &model.IPSecurityProjectionQuarantine{}))
	require.NoError(t, mainDB.Create(&model.AuditOutboxEvent{
		EventID: "instance-metrics", EventType: "wallet.adjust", Status: model.AuditOutboxStatusPending,
	}).Error)
	require.NoError(t, mainDB.Create(&model.IPSecurityProjectionState{
		ID: 1, Status: model.IPSecurityProjectionStatusReady,
	}).Error)
	require.NoError(t, mainDB.Create(&model.IPSecurityEvent{
		EventID: "instance-pending-security", UserID: 25, CreatedAt: 1_752_501_600,
		IP: "203.0.113.25", MaskedIP: "203.0.113.*", Kind: model.IPSecurityKindAPIRequest,
		ProjectionStatus: model.IPSecurityEventProjectionPending,
	}).Error)
	require.NoError(t, mainDB.Create(&model.IPSecurityProjectionQuarantine{
		SourceKey: "log:relational:25", SourceType: "log_relational", SourceIdentity: "25",
		Error: model.ErrInvalidIPSecurityEvent.Error(), AttemptCount: 1,
		FirstSeenAt: 1_752_501_601, LastSeenAt: 1_752_501_602,
	}).Error)

	require.NoError(t, ReportCurrentSystemInstance())
	var instance model.SystemInstance
	require.NoError(t, mainDB.Order("last_seen_at desc").First(&instance).Error)
	var info map[string]any
	require.NoError(t, common.UnmarshalJsonStr(instance.Info, &info))
	extra, ok := info["extra"].(map[string]any)
	require.True(t, ok)
	outbox, ok := extra["audit_outbox"].(map[string]any)
	require.True(t, ok)
	require.EqualValues(t, 1, outbox["pending_count"])
	projection, ok := extra["ip_security_projection"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, model.IPSecurityProjectionStatusReady, projection["status"])
	require.EqualValues(t, 1, projection["pending_count"])
	require.EqualValues(t, 1, projection["quarantine_count"])
	require.EqualValues(t, 1_752_501_602, projection["last_quarantine_at"])
}
