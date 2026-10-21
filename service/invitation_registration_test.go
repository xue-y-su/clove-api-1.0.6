package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/registration_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupInvitationServiceTest(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s/invitations.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", t.TempDir())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Checkin{}, &model.InviteCodeApplication{}, &model.InviteCode{}, &model.InviteCodeUse{}, &model.InviteRewardPayout{},
		&model.InviteRewardReversal{},
		&model.InviteRewardPayoutRequest{}, &model.InviteRewardPayoutRequestItem{},
		&model.InviteRewardPayoutAction{},
		&model.WalletLedgerEntry{}, &model.AuditOutboxEvent{}, &model.Log{},
		&model.IPSecurityEvent{}, &model.IPAuditRollup{}, &model.IPSecurityProjectionState{},
	))
	previousDB := model.DB
	previousLogDB := model.LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	previousQuotaForNewUser := common.QuotaForNewUser
	previousQuotaForInviter := common.QuotaForInviter
	previousQuotaForInvitee := common.QuotaForInvitee
	previousInvitationIncentiveSetting := *operation_setting.GetInvitationIncentiveSetting()
	common.OptionMapRWMutex.Lock()
	previousOptionMap := common.OptionMap
	common.OptionMap = registration_setting.DefaultOptions()
	common.OptionMapRWMutex.Unlock()
	model.DB = db
	model.LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.QuotaForNewUser = 0
	common.QuotaForInviter = 0
	common.QuotaForInvitee = 0
	*operation_setting.GetInvitationIncentiveSetting() = operation_setting.DefaultInvitationIncentiveSetting()
	t.Cleanup(func() {
		model.DB = previousDB
		model.LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled = previousRedisEnabled
		common.QuotaForNewUser = previousQuotaForNewUser
		common.QuotaForInviter = previousQuotaForInviter
		common.QuotaForInvitee = previousQuotaForInvitee
		*operation_setting.GetInvitationIncentiveSetting() = previousInvitationIncentiveSetting
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
		require.NoError(t, sqlDB.Close())
	})
	return db
}

func TestInvitationRegistrationSwitchesRewardsIndependently(t *testing.T) {
	tests := []struct {
		name                 string
		masterEnabled        bool
		inviterEnabled       bool
		inviteeEnabled       bool
		complianceEnabled    bool
		wantInviterReward    int64
		wantInviteeGift      int64
		wantInviterUseStatus string
		wantInviteeUseStatus string
	}{
		{
			name: "both enabled", masterEnabled: true, inviterEnabled: true, inviteeEnabled: true,
			complianceEnabled: true, wantInviterReward: 10, wantInviteeGift: 5,
			wantInviterUseStatus: model.InviteRewardStatusAvailable, wantInviteeUseStatus: model.InviteRewardStatusPaid,
		},
		{
			name: "inviter only", masterEnabled: true, inviterEnabled: true, inviteeEnabled: false,
			complianceEnabled: true, wantInviterReward: 10,
			wantInviterUseStatus: model.InviteRewardStatusAvailable, wantInviteeUseStatus: model.InviteRewardStatusSkipped,
		},
		{
			name: "invitee only", masterEnabled: true, inviterEnabled: false, inviteeEnabled: true,
			complianceEnabled: true, wantInviteeGift: 5,
			wantInviterUseStatus: model.InviteRewardStatusSkipped, wantInviteeUseStatus: model.InviteRewardStatusPaid,
		},
		{
			name: "master disabled", masterEnabled: false, inviterEnabled: true, inviteeEnabled: true,
			complianceEnabled:    true,
			wantInviterUseStatus: model.InviteRewardStatusSkipped, wantInviteeUseStatus: model.InviteRewardStatusSkipped,
		},
		{
			name: "payment compliance disabled", masterEnabled: true, inviterEnabled: true, inviteeEnabled: true,
			complianceEnabled:    false,
			wantInviterUseStatus: model.InviteRewardStatusSkipped, wantInviteeUseStatus: model.InviteRewardStatusSkipped,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupInvitationServiceTest(t)
			seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
			seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
			invite := seedInviteCode(t, db, 10, "switch-code", 3)
			common.QuotaForInviter = 10
			common.QuotaForInvitee = 5
			setting := operation_setting.GetInvitationIncentiveSetting()
			setting.Enabled = test.masterEnabled
			setting.InviterRegistrationRewardEnabled = test.inviterEnabled
			setting.InviteeRegistrationGiftEnabled = test.inviteeEnabled
			payment := operation_setting.GetPaymentSetting()
			previousPayment := *payment
			payment.ComplianceConfirmed = test.complianceEnabled
			payment.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
			t.Cleanup(func() { *payment = previousPayment })

			result, err := RegisterWithInvitation(context.Background(), RegistrationRequest{
				User:           &model.User{Username: "switch-user", Password: "password123"},
				InvitationCode: invite.Code,
				Source:         "password",
			})
			require.NoError(t, err)
			require.NotZero(t, result.InviteUseID)
			require.Equal(t, 10, result.InviterID)

			var storedInvite model.InviteCode
			require.NoError(t, db.First(&storedInvite, invite.ID).Error)
			require.Equal(t, 1, storedInvite.UsedCount)
			var use model.InviteCodeUse
			require.NoError(t, db.First(&use, result.InviteUseID).Error)
			require.Equal(t, test.wantInviterUseStatus, use.InviterRewardStatus)
			require.Equal(t, test.wantInviteeUseStatus, use.InviteeRewardStatus)
			require.NoError(t, ProcessInvitationUseRewards(context.Background(), result.InviteUseID))
			require.NoError(t, ProcessInvitationUseRewards(context.Background(), result.InviteUseID))

			var inviterReward int64
			require.NoError(t, db.Model(&model.InviteRewardPayout{}).
				Where("recipient_user_id = ? AND status = ?", 10, model.InviteRewardStatusAvailable).
				Select("COALESCE(SUM(amount), 0)").Scan(&inviterReward).Error)
			require.Equal(t, test.wantInviterReward, inviterReward)
			var invitee model.User
			require.NoError(t, db.First(&invitee, result.UserID).Error)
			require.Equal(t, test.wantInviteeGift, invitee.Quota)
			var giftEntries int64
			require.NoError(t, db.Model(&model.WalletLedgerEntry{}).
				Where("category = ?", "invitation_gift").Count(&giftEntries).Error)
			if test.wantInviteeGift > 0 {
				require.Equal(t, int64(1), giftEntries)
			} else {
				require.Zero(t, giftEntries)
			}
		})
	}
}

func seedInvitationUser(t *testing.T, db *gorm.DB, id int, username string, role int) model.User {
	t.Helper()
	user := model.User{
		Id: id, Username: username, Password: "hashed", DisplayName: username, Role: role,
		Status: common.UserStatusEnabled, Group: "default", AffCode: fmt.Sprintf("legacy-%d", id),
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func seedInviteCode(t *testing.T, db *gorm.DB, ownerID int, code string, maxUses int) model.InviteCode {
	t.Helper()
	kind := model.InviteCodeKindMulti
	if maxUses == 1 {
		kind = model.InviteCodeKindSingle
	}
	invite := model.InviteCode{
		OwnerUserID: ownerID, Code: code, Kind: kind, MaxUses: maxUses,
		Status: model.InviteCodeStatusActive, ActiveOwnerSlot: &ownerID,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(), IssuedBy: 1,
	}
	require.NoError(t, db.Create(&invite).Error)
	return invite
}

func TestInvitationCodeGenerationUsesTenUnambiguousCharacters(t *testing.T) {
	previousReader := invitationCodeReader
	invitationCodeReader = bytes.NewReader(bytes.Repeat([]byte{25}, invitationCodeLength))
	t.Cleanup(func() { invitationCodeReader = previousReader })

	code, err := generateInvitationCode()

	require.NoError(t, err)
	require.Equal(t, "VVVVVVVVVV", code)
	require.Len(t, code, 10)
	require.NotRegexp(t, `[01OILU]`, code)
}

func TestDirectIssueInviteCodeRetriesGeneratedCodeCollision(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "root", common.RoleRootUser)
	seedInvitationUser(t, db, 9, "target", common.RoleCommonUser)
	seedInvitationUser(t, db, 10, "existing-owner", common.RoleCommonUser)
	require.NoError(t, db.Create(&model.InviteCode{
		OwnerUserID: 10, Code: "2222222222", Kind: model.InviteCodeKindSingle,
		MaxUses: 1, Status: model.InviteCodeStatusRevoked,
		ExpiresAt: time.Now().Add(24 * time.Hour).Unix(), IssuedBy: 1,
	}).Error)

	previousReader := invitationCodeReader
	invitationCodeReader = bytes.NewReader(append(make([]byte, 10), bytes.Repeat([]byte{1}, 10)...))
	t.Cleanup(func() { invitationCodeReader = previousReader })

	issued, err := DirectIssueInviteCode(context.Background(), InviteCodeDirectIssueRequest{
		IssuerID: 1, TargetUserID: 9, Reason: "approved for project invitations",
		Issue: InviteCodeIssueSpec{
			Kind: model.InviteCodeKindSingle, MaxUses: 1,
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		},
	})

	require.NoError(t, err)
	require.Equal(t, "3333333333", issued.Code)
}

func TestInvitationValidationAcceptsLowercaseShortCodeAndLegacyLongCode(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "short-owner", common.RoleCommonUser)
	seedInvitationUser(t, db, 2, "legacy-owner", common.RoleCommonUser)
	seedInviteCode(t, db, 1, "ABCD2345", 3)
	seedInviteCode(t, db, 2, "inv_aBcDefGhijkLmNoPqRsTuvWx", 3)

	require.NoError(t, ValidateInvitationCode(context.Background(), "abcd2345", false))
	require.NoError(t, ValidateInvitationCode(context.Background(), "inv_aBcDefGhijkLmNoPqRsTuvWx", false))
}

func TestRegisterWithInvitationPersistsNormalizedRegistrationIPWithoutInvite(t *testing.T) {
	db := setupInvitationServiceTest(t)
	user := model.User{
		Username: "registration-ip-user", Password: "password", DisplayName: "Registration IP User",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "registration-ip-aff",
	}

	_, err := RegisterWithInvitation(context.Background(), RegistrationRequest{
		User: &user, Source: "password", RegistrationIP: "[2001:db8::7]:443",
	})
	require.NoError(t, err)

	var registrationIP string
	err = db.Raw("SELECT registration_ip FROM users WHERE id = ?", user.Id).Scan(&registrationIP).Error
	require.NoError(t, err)
	require.Equal(t, "2001:db8::7", registrationIP)
}

func TestRegisterWithInvitationPreservesIPv4MappedIPv6RegistrationIP(t *testing.T) {
	db := setupInvitationServiceTest(t)
	user := model.User{
		Username: "mapped-ip-user", Password: "password", DisplayName: "Mapped Registration IP",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "mapped-registration-aff",
	}

	_, err := RegisterWithInvitation(context.Background(), RegistrationRequest{
		User: &user, Source: "password", RegistrationIP: "::ffff:203.0.113.57",
	})
	require.NoError(t, err)

	var registrationIP string
	require.NoError(t, db.Raw("SELECT registration_ip FROM users WHERE id = ?", user.Id).Scan(&registrationIP).Error)
	require.Equal(t, "::ffff:203.0.113.57", registrationIP)
}

func TestRegisterWithInvitationEnqueuesRegistrationAuditInSameTransaction(t *testing.T) {
	db := setupInvitationServiceTest(t)
	user := model.User{
		Username: "reg-audit", Password: "password", DisplayName: "Registration Audit",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "registration-audit-aff",
	}

	result, err := RegisterWithInvitation(context.Background(), RegistrationRequest{
		User: &user, Source: "admin", RegistrationIP: "::ffff:203.0.113.58",
		ActorID: 42, RequestID: "registration-request-42",
	})
	require.NoError(t, err)
	var event model.AuditOutboxEvent
	require.NoError(t, db.First(&event, "event_type = ?", "user.register").Error)
	var registrationAuditCount int64
	require.NoError(t, db.Model(&model.AuditOutboxEvent{}).
		Where("event_type = ?", "user.register").Count(&registrationAuditCount).Error)
	require.EqualValues(t, 1, registrationAuditCount)
	require.Equal(t, 42, event.ActorID)
	require.Equal(t, fmt.Sprintf("user:%d", result.UserID), event.Target)
	require.Equal(t, "::ffff:203.0.113.58", event.IP)
	require.Equal(t, "registration-request-42", event.RequestID)
	require.Contains(t, event.Payload, fmt.Sprintf(`"target_user_id":%d`, result.UserID))
	require.Contains(t, event.Payload, `"source":"admin"`)
	var securityEvent model.IPSecurityEvent
	require.NoError(t, db.First(&securityEvent, "event_id = ?", fmt.Sprintf("user:register:%d", result.UserID)).Error)
	require.Equal(t, model.IPSecurityKindRegistration, securityEvent.Kind)
	require.Equal(t, result.UserID, securityEvent.UserID)
	require.Equal(t, "::ffff:203.0.113.58", securityEvent.IP)
	require.Equal(t, "registration-request-42", securityEvent.RequestID)
	var lifetime model.IPAuditRollup
	require.NoError(t, db.First(&lifetime, "bucket = ? AND user_id = ?", 0, result.UserID).Error)
	require.Equal(t, 1, lifetime.RegistrationCount)
	require.Zero(t, lifetime.RequestCount)

	delivered, failed, err := DeliverPending(context.Background(), 10)
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Zero(t, failed)
	var projectionCount int64
	require.NoError(t, db.Model(&model.IPSecurityEvent{}).
		Where("event_id = ?", fmt.Sprintf("user:register:%d", result.UserID)).Count(&projectionCount).Error)
	require.EqualValues(t, 1, projectionCount)
	require.NoError(t, db.First(&lifetime, "bucket = ? AND user_id = ?", 0, result.UserID).Error)
	require.Equal(t, 1, lifetime.RegistrationCount)
	require.Zero(t, lifetime.RequestCount)
	require.NoError(t, model.BackfillIPSecurityProjection(context.Background(), 1))
	ready, err := model.IPSecurityProjectionReady(context.Background())
	require.NoError(t, err)
	require.True(t, ready)
	require.NoError(t, db.Model(&model.IPSecurityEvent{}).
		Where("event_id = ?", fmt.Sprintf("user:register:%d", result.UserID)).Count(&projectionCount).Error)
	require.EqualValues(t, 1, projectionCount)
	require.NoError(t, db.First(&lifetime, "bucket = ? AND user_id = ?", 0, result.UserID).Error)
	require.Equal(t, 1, lifetime.RegistrationCount)
	require.Zero(t, lifetime.RequestCount)
}

func TestRegisterWithInvitationRollsBackWhenRegistrationAuditCannotBeEnqueued(t *testing.T) {
	db := setupInvitationServiceTest(t)
	failure := errors.New("registration audit unavailable")
	require.NoError(t, db.Callback().Create().Before("gorm:create").Register("registration_audit_test:fail_outbox", func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "audit_outbox_events" {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove("registration_audit_test:fail_outbox") })
	user := model.User{
		Username: "reg-rollback", Password: "password", DisplayName: "Registration Rollback",
		Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AffCode: "registration-rollback-aff",
	}

	_, err := RegisterWithInvitation(context.Background(), RegistrationRequest{
		User: &user, Source: "password", RegistrationIP: "203.0.113.59",
	})
	require.ErrorIs(t, err, failure)
	var count int64
	require.NoError(t, db.Model(&model.User{}).Where("username = ?", user.Username).Count(&count).Error)
	require.Zero(t, count)
}

func TestLegacyAffCodeCannotRegisterAfterInvitationMigration(t *testing.T) {
	db := setupInvitationServiceTest(t)
	legacy := seedInvitationUser(t, db, 7, "legacy-inviter", common.RoleCommonUser)

	_, err := RegisterWithInvitation(context.Background(), RegistrationRequest{
		User:           &model.User{Username: "new-user", Password: "password123"},
		InvitationCode: legacy.AffCode,
		Source:         "password",
	})
	require.ErrorIs(t, err, ErrInvitationInvalid)
	var count int64
	require.NoError(t, db.Model(&model.User{}).Where("username = ?", "new-user").Count(&count).Error)
	require.Zero(t, count)
}

func TestFailedUserInsertDoesNotConsumeInvitation(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "inviter", common.RoleCommonUser)
	invite := seedInviteCode(t, db, 1, "valid-code", 1)
	seedInvitationUser(t, db, 2, "duplicate", common.RoleCommonUser)

	_, err := RegisterWithInvitation(context.Background(), RegistrationRequest{
		User:           &model.User{Username: "duplicate", Password: "password123"},
		InvitationCode: invite.Code,
		Source:         "password",
	})
	require.Error(t, err)

	var stored model.InviteCode
	require.NoError(t, db.First(&stored, invite.ID).Error)
	require.Zero(t, stored.UsedCount)
	var uses int64
	require.NoError(t, db.Model(&model.InviteCodeUse{}).Count(&uses).Error)
	require.Zero(t, uses)
}

func TestOneUseInvitationAllowsOneConcurrentRegistration(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "inviter", common.RoleCommonUser)
	invite := seedInviteCode(t, db, 1, "one-use-code", 1)

	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range errs {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			_, errs[index] = RegisterWithInvitation(context.Background(), RegistrationRequest{
				User:           &model.User{Username: fmt.Sprintf("concurrent-%d", index), Password: "password123"},
				InvitationCode: invite.Code,
				Source:         "password",
			})
		}(i)
	}
	close(start)
	wg.Wait()

	successes := 0
	exhausted := 0
	for _, err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrInvitationExhausted) {
			exhausted++
		} else {
			t.Fatalf("unexpected registration error: %v", err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, exhausted)
	var uses int64
	require.NoError(t, db.Model(&model.InviteCodeUse{}).Count(&uses).Error)
	require.EqualValues(t, 1, uses)
	var stored model.InviteCode
	require.NoError(t, db.First(&stored, invite.ID).Error)
	require.Equal(t, 1, stored.UsedCount)
	require.Equal(t, model.InviteCodeStatusExhausted, stored.Status)
	require.Nil(t, stored.ActiveOwnerSlot)
}

func TestInvitationApplicationEnforcesPendingAndReapplyRules(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 9, "applicant", common.RoleCommonUser)

	first, err := ApplyForInviteCode(context.Background(), 9, "I want to invite my team")
	require.NoError(t, err)
	_, err = ApplyForInviteCode(context.Background(), 9, "duplicate request")
	require.ErrorIs(t, err, ErrInviteApplicationPending)
	require.NoError(t, WithdrawInviteCodeApplication(context.Background(), 9, first.ID))
	second, err := ApplyForInviteCode(context.Background(), 9, "request after withdrawal")
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.InviteCodeApplication{}).Where("id = ?", second.ID).Updates(map[string]any{
		"status": model.InviteApplicationStatusRejected, "pending_user_slot": nil,
		"reviewed_at": time.Now().Unix(), "review_reason": "not yet",
	}).Error)
	_, err = ApplyForInviteCode(context.Background(), 9, "too soon after rejection")
	require.ErrorIs(t, err, ErrInviteReapplyLater)
}

func TestInvitationUserStateUsesApplicationIDAsCreationOrderWhenTimestampsMoveBackward(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 9, "applicant", common.RoleCommonUser)
	now := common.GetTimestamp()

	older := model.InviteCodeApplication{
		UserID: 9, Reason: "first request", Status: model.InviteApplicationStatusRejected,
		ReviewedAt: now, ReviewReason: "try later", CreatedAt: now,
	}
	require.NoError(t, db.Create(&older).Error)
	newer := model.InviteCodeApplication{
		UserID: 9, Reason: "second request", Status: model.InviteApplicationStatusWithdrawn,
		CreatedAt: now - 1,
	}
	require.NoError(t, db.Create(&newer).Error)
	require.Greater(t, newer.ID, older.ID)

	state, err := GetInvitationUserState(context.Background(), 9)
	require.NoError(t, err)
	require.NotNil(t, state.Application)
	require.Equal(t, newer.ID, state.Application.ID)
	require.Equal(t, model.InviteApplicationStatusWithdrawn, state.Application.Status)
}

func TestInvitationReapplyUsesApplicationIDAsCreationOrderWhenTimestampsMoveBackward(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 9, "applicant", common.RoleCommonUser)
	now := common.GetTimestamp()

	older := model.InviteCodeApplication{
		UserID: 9, Reason: "first request", Status: model.InviteApplicationStatusRejected,
		ReviewedAt: now, ReviewReason: "try later", CreatedAt: now,
	}
	require.NoError(t, db.Create(&older).Error)
	newer := model.InviteCodeApplication{
		UserID: 9, Reason: "second request", Status: model.InviteApplicationStatusWithdrawn,
		CreatedAt: now - 1,
	}
	require.NoError(t, db.Create(&newer).Error)
	require.Greater(t, newer.ID, older.ID)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[registration_setting.ApplicationsEnabledKey] = "true"
	common.OptionMap[registration_setting.ReapplyCooldownSecondsKey] = "3600"
	common.OptionMapRWMutex.Unlock()

	reapplied, err := ApplyForInviteCode(context.Background(), 9, "third request after withdrawal")
	require.NoError(t, err)
	require.Greater(t, reapplied.ID, newer.ID)
}

func TestInvitationUserStateUsesConfiguredCooldownAndPolicy(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 9, "applicant", common.RoleCommonUser)

	setPolicy := func(enabled bool, cooldown int64) {
		common.OptionMapRWMutex.Lock()
		common.OptionMap[registration_setting.ApplicationsEnabledKey] = fmt.Sprintf("%t", enabled)
		common.OptionMap[registration_setting.ReapplyCooldownSecondsKey] = fmt.Sprintf("%d", cooldown)
		common.OptionMapRWMutex.Unlock()
	}

	t.Run("zero cooldown clears expired rejection", func(t *testing.T) {
		now := common.GetTimestamp()
		rejected := model.InviteCodeApplication{
			UserID: 9, Reason: "old request", Status: model.InviteApplicationStatusRejected,
			ReviewedAt: now - 3600, ReviewReason: "try later",
		}
		require.NoError(t, db.Create(&rejected).Error)
		setPolicy(true, 0)

		state, err := GetInvitationUserState(context.Background(), 9)
		require.NoError(t, err)
		require.True(t, state.CanApply)
		require.Zero(t, state.ReapplyAt)
		require.Empty(t, state.ApplyUnavailableReason)
	})

	t.Run("one second before configured expiry remains blocked", func(t *testing.T) {
		setPolicy(true, 3600)
		now := common.GetTimestamp()
		rejected := model.InviteCodeApplication{
			UserID: 9, Reason: "recent request", Status: model.InviteApplicationStatusRejected,
			ReviewedAt: now - 3599, ReviewReason: "try later",
		}
		require.NoError(t, db.Create(&rejected).Error)

		state, err := GetInvitationUserState(context.Background(), 9)
		require.NoError(t, err)
		require.False(t, state.CanApply)
		require.Equal(t, rejected.ReviewedAt+3600, state.ReapplyAt)
		require.Equal(t, "reapply_cooldown", state.ApplyUnavailableReason)
	})

	t.Run("exact configured expiry becomes available", func(t *testing.T) {
		setPolicy(true, 3600)
		now := common.GetTimestamp()
		rejected := model.InviteCodeApplication{
			UserID: 9, Reason: "expired request", Status: model.InviteApplicationStatusRejected,
			ReviewedAt: now - 3600, ReviewReason: "try later",
		}
		require.NoError(t, db.Create(&rejected).Error)

		state, err := GetInvitationUserState(context.Background(), 9)
		require.NoError(t, err)
		require.True(t, state.CanApply)
		require.Zero(t, state.ReapplyAt)
		require.Empty(t, state.ApplyUnavailableReason)
	})

	t.Run("disabled applications block state and creation", func(t *testing.T) {
		setPolicy(false, 0)
		state, err := GetInvitationUserState(context.Background(), 9)
		require.NoError(t, err)
		require.False(t, state.CanApply)
		require.Equal(t, "applications_disabled", state.ApplyUnavailableReason)

		_, err = ApplyForInviteCode(context.Background(), 9, "application while disabled")
		require.EqualError(t, err, "invite_applications_disabled")
	})

	t.Run("active code remains authoritative", func(t *testing.T) {
		setPolicy(true, 0)
		seedInviteCode(t, db, 9, "active-policy-code", 1)

		state, err := GetInvitationUserState(context.Background(), 9)
		require.NoError(t, err)
		require.False(t, state.CanApply)
		require.Equal(t, "active_code", state.ApplyUnavailableReason)
	})
}

func TestInvitationTerminalCodesAllowReapplication(t *testing.T) {
	for _, status := range []string{
		model.InviteCodeStatusExhausted,
		model.InviteCodeStatusExpired,
		model.InviteCodeStatusRevoked,
	} {
		t.Run(status, func(t *testing.T) {
			db := setupInvitationServiceTest(t)
			seedInvitationUser(t, db, 9, "applicant", common.RoleCommonUser)
			invite := model.InviteCode{
				OwnerUserID: 9, Code: "TERMINAL23", Kind: model.InviteCodeKindSingle,
				MaxUses: 1, UsedCount: 1, Status: status,
				ExpiresAt: time.Now().Add(time.Hour).Unix(), IssuedBy: 1,
			}
			require.NoError(t, db.Create(&invite).Error)

			state, err := GetInvitationUserState(context.Background(), 9)
			require.NoError(t, err)
			require.True(t, state.CanApply)
			require.Empty(t, state.ApplyUnavailableReason)
		})
	}
}

func TestInvitationApplicationCooldownErrorCarriesRetryTimestamp(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 9, "applicant", common.RoleCommonUser)
	now := common.GetTimestamp()
	rejected := model.InviteCodeApplication{
		UserID: 9, Reason: "recent request", Status: model.InviteApplicationStatusRejected,
		ReviewedAt: now, ReviewReason: "try later",
	}
	require.NoError(t, db.Create(&rejected).Error)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[registration_setting.ApplicationsEnabledKey] = "true"
	common.OptionMap[registration_setting.ReapplyCooldownSecondsKey] = "3600"
	common.OptionMapRWMutex.Unlock()

	_, err := ApplyForInviteCode(context.Background(), 9, "try again too soon")
	require.ErrorIs(t, err, ErrInviteReapplyLater)
	var retryErr interface {
		error
		RetryTimestamp() int64
	}
	require.ErrorAs(t, err, &retryErr)
	require.Equal(t, rejected.ReviewedAt+3600, retryErr.RetryTimestamp())
}

func TestInvitationApplicationDoesNotReactivateAnOlderRejection(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 9, "applicant", common.RoleCommonUser)
	now := common.GetTimestamp()
	rejected := model.InviteCodeApplication{
		UserID: 9, Reason: "first request", Status: model.InviteApplicationStatusRejected,
		ReviewedAt: now, ReviewReason: "try later",
	}
	require.NoError(t, db.Create(&rejected).Error)
	common.OptionMapRWMutex.Lock()
	common.OptionMap[registration_setting.ApplicationsEnabledKey] = "true"
	common.OptionMap[registration_setting.ReapplyCooldownSecondsKey] = "0"
	common.OptionMapRWMutex.Unlock()

	reapplied, err := ApplyForInviteCode(context.Background(), 9, "second request")
	require.NoError(t, err)
	require.NoError(t, WithdrawInviteCodeApplication(context.Background(), 9, reapplied.ID))
	common.OptionMapRWMutex.Lock()
	common.OptionMap[registration_setting.ReapplyCooldownSecondsKey] = "3600"
	common.OptionMapRWMutex.Unlock()

	state, err := GetInvitationUserState(context.Background(), 9)
	require.NoError(t, err)
	require.True(t, state.CanApply)
	_, err = ApplyForInviteCode(context.Background(), 9, "third request")
	require.NoError(t, err)
}

func TestInvitationReviewRechecksTargetCurrentRole(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 2, "admin-reviewer", common.RoleAdminUser)
	seedInvitationUser(t, db, 3, "target", common.RoleCommonUser)
	application, err := ApplyForInviteCode(context.Background(), 3, "please approve")
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 3).Update("role", common.RoleAdminUser).Error)

	_, err = ReviewInviteCodeApplication(context.Background(), InviteCodeReviewRequest{
		ReviewerID: 2, ApplicationID: application.ID, Approve: true,
		Issue: InviteCodeIssueSpec{Kind: model.InviteCodeKindSingle, MaxUses: 1, ExpiresAt: time.Now().Add(24 * time.Hour).Unix()},
	})
	require.ErrorIs(t, err, ErrInviteTargetForbidden)
	var stored model.InviteCodeApplication
	require.NoError(t, db.First(&stored, application.ID).Error)
	require.Equal(t, model.InviteApplicationStatusPending, stored.Status)
	require.NotNil(t, stored.PendingUserSlot)
}

func TestDirectInvitationIssueEnforcesActorTargetMatrix(t *testing.T) {
	tests := []struct {
		name       string
		actorID    int
		actorRole  int
		targetID   int
		targetRole int
		allowed    bool
	}{
		{name: "admin to user", actorID: 2, actorRole: common.RoleAdminUser, targetID: 3, targetRole: common.RoleCommonUser, allowed: true},
		{name: "admin to self", actorID: 2, actorRole: common.RoleAdminUser, targetID: 2, targetRole: common.RoleAdminUser},
		{name: "admin to admin", actorID: 2, actorRole: common.RoleAdminUser, targetID: 3, targetRole: common.RoleAdminUser},
		{name: "admin to root", actorID: 2, actorRole: common.RoleAdminUser, targetID: 3, targetRole: common.RoleRootUser},
		{name: "root to user", actorID: 2, actorRole: common.RoleRootUser, targetID: 3, targetRole: common.RoleCommonUser, allowed: true},
		{name: "root to admin", actorID: 2, actorRole: common.RoleRootUser, targetID: 3, targetRole: common.RoleAdminUser, allowed: true},
		{name: "root to self", actorID: 2, actorRole: common.RoleRootUser, targetID: 2, targetRole: common.RoleRootUser},
		{name: "root to root", actorID: 2, actorRole: common.RoleRootUser, targetID: 3, targetRole: common.RoleRootUser},
		{name: "primary root to self", actorID: 1, actorRole: common.RoleRootUser, targetID: 1, targetRole: common.RoleRootUser, allowed: true},
		{name: "primary root to root", actorID: 1, actorRole: common.RoleRootUser, targetID: 3, targetRole: common.RoleRootUser, allowed: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupInvitationServiceTest(t)
			seedInvitationUser(t, db, test.actorID, "actor", test.actorRole)
			if test.targetID != test.actorID {
				seedInvitationUser(t, db, test.targetID, "target", test.targetRole)
			}
			invite, err := DirectIssueInviteCode(context.Background(), InviteCodeDirectIssueRequest{
				IssuerID: test.actorID, TargetUserID: test.targetID, Reason: "approved for testing",
				Issue: InviteCodeIssueSpec{Kind: model.InviteCodeKindSingle, MaxUses: 1, ExpiresAt: time.Now().Add(24 * time.Hour).Unix()},
			})
			if test.allowed {
				require.NoError(t, err)
				require.NotNil(t, invite)
				require.Equal(t, test.targetID, invite.OwnerUserID)
			} else {
				require.ErrorIs(t, err, ErrInviteTargetForbidden)
				require.Nil(t, invite)
			}
		})
	}
}

func TestDisabledPrimaryRootCannotIssueInvitationCode(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 3, "target", common.RoleCommonUser)
	require.NoError(t, db.Model(&model.User{}).Where("id = ?", 1).Update("status", common.UserStatusDisabled).Error)

	invite, err := DirectIssueInviteCode(context.Background(), InviteCodeDirectIssueRequest{
		IssuerID:     1,
		TargetUserID: 3,
		Reason:       "must not bypass disabled status",
		Issue: InviteCodeIssueSpec{
			Kind:      model.InviteCodeKindSingle,
			MaxUses:   1,
			ExpiresAt: time.Now().Add(24 * time.Hour).Unix(),
		},
	})

	require.ErrorIs(t, err, ErrInviteTargetForbidden)
	require.Nil(t, invite)
}

func TestRevokedInvitationClearsActiveSlotAndAllowsNewIssue(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 9, "target", common.RoleCommonUser)
	spec := InviteCodeIssueSpec{Kind: model.InviteCodeKindMulti, MaxUses: 5, ExpiresAt: time.Now().Add(24 * time.Hour).Unix()}
	first, err := DirectIssueInviteCode(context.Background(), InviteCodeDirectIssueRequest{
		IssuerID: 1, TargetUserID: 9, Reason: "first issue", Issue: spec,
	})
	require.NoError(t, err)
	require.NoError(t, RevokeInviteCode(context.Background(), 1, first.ID, "security rotation"))
	var revoked model.InviteCode
	require.NoError(t, db.First(&revoked, first.ID).Error)
	require.Equal(t, model.InviteCodeStatusRevoked, revoked.Status)
	require.Nil(t, revoked.ActiveOwnerSlot)
	second, err := DirectIssueInviteCode(context.Background(), InviteCodeDirectIssueRequest{
		IssuerID: 1, TargetUserID: 9, Reason: "replacement issue", Issue: spec,
	})
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
}

func TestInvitationRewardsAutoCreditInviteeAndRequireInviterApproval(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	invite := seedInviteCode(t, db, 10, "reward-code", 3)
	common.QuotaForInviter = 10
	common.QuotaForInvitee = 5
	payment := operation_setting.GetPaymentSetting()
	previousPayment := *payment
	payment.ComplianceConfirmed = true
	payment.ComplianceTermsVersion = operation_setting.CurrentComplianceTermsVersion
	t.Cleanup(func() { *payment = previousPayment })
	newUser := &model.User{Username: "rewarded-user", Password: "password123"}

	result, err := RegisterWithInvitation(context.Background(), RegistrationRequest{
		User: newUser, InvitationCode: invite.Code, Source: "password", RegistrationIP: "203.0.113.45",
	})
	require.NoError(t, err)

	var inviter model.User
	require.NoError(t, db.First(&inviter, 10).Error)
	require.Zero(t, inviter.Quota)
	require.Zero(t, inviter.AffCount)
	require.Zero(t, inviter.AffHistoryQuota)
	var invitee model.User
	require.NoError(t, db.First(&invitee, result.UserID).Error)
	require.EqualValues(t, 5, invitee.Quota)
	var entries []model.WalletLedgerEntry
	require.NoError(t, db.Order("operation_id asc").Find(&entries).Error)
	require.Len(t, entries, 1)
	require.Equal(t, fmt.Sprintf("invitee_gift:%d", invitee.Id), entries[0].OperationID)
	require.Equal(t, "invitation_gift", entries[0].Category)
	var use model.InviteCodeUse
	require.NoError(t, db.First(&use, result.InviteUseID).Error)
	require.Equal(t, model.InviteRewardStatusAvailable, use.InviterRewardStatus)
	require.Equal(t, model.InviteRewardStatusPaid, use.InviteeRewardStatus)
	require.Zero(t, use.RewardsCompletedAt)

	var payouts []model.InviteRewardPayout
	require.NoError(t, db.Order("recipient_kind asc").Find(&payouts).Error)
	require.Len(t, payouts, 1)
	require.Equal(t, model.InviteRewardRecipientInviter, payouts[0].RecipientKind)
	require.Equal(t, model.InviteRewardStatusAvailable, payouts[0].Status)

	inviteeSummary, err := GetInvitationRewardSummary(context.Background(), invitee.Id)
	require.NoError(t, err)
	require.Equal(t, InvitationRewardSummary{CashCommissionRate: "0.00"}, inviteeSummary)
	_, err = SubmitInvitationRewardPayout(context.Background(), invitee.Id)
	require.ErrorIs(t, err, ErrInviteRewardUnavailable)

	var inviterPayout model.InviteRewardPayout
	require.NoError(t, db.Where("recipient_user_id = ?", inviter.Id).First(&inviterPayout).Error)
	request, err := SubmitInvitationRewardPayout(context.Background(), inviter.Id)
	require.NoError(t, err)
	require.EqualValues(t, 10, request.Amount)
	require.NoError(t, ReviewInvitationRewardPayoutRequest(context.Background(), ReviewInvitationRewardPayoutRequestInput{
		ReviewerID: 1, RequestID: request.ID, Approve: true,
	}))
	require.ErrorIs(t, ReviewInvitationRewardPayoutRequest(context.Background(), ReviewInvitationRewardPayoutRequestInput{
		ReviewerID: 1, RequestID: request.ID, Approve: true,
	}), ErrInviteRewardReviewInvalid)

	require.NoError(t, db.First(&inviter, inviter.Id).Error)
	require.EqualValues(t, 10, inviter.Quota)
	require.Equal(t, 1, inviter.AffCount)
	require.Equal(t, 10, inviter.AffHistoryQuota)
	require.NoError(t, db.First(&invitee, invitee.Id).Error)
	require.EqualValues(t, 5, invitee.Quota)
	require.NoError(t, db.First(&inviterPayout, inviterPayout.ID).Error)
	require.Equal(t, model.InviteRewardStatusPaid, inviterPayout.Status)
	require.NoError(t, db.Find(&entries).Error)
	require.Len(t, entries, 2)
	require.ElementsMatch(t, []string{"invitation_gift", "invitation_reward_payout"}, []string{entries[0].Category, entries[1].Category})

	var auditEvents []model.AuditOutboxEvent
	require.NoError(t, db.Where("event_type IN ?", []string{"invitation.reward_payout_submitted", "invitation.reward_payout_reviewed"}).Find(&auditEvents).Error)
	require.Len(t, auditEvents, 2)
}

func TestInvitationRewardReviewRejectsMismatchedRequestItems(t *testing.T) {
	db := setupInvitationServiceTest(t)
	enableInvitationPayoutCompliance(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	inviter := seedInvitationUser(t, db, 10, "inviter", common.RoleCommonUser)
	invite := seedInviteCode(t, db, inviter.Id, "review-race-code", 1)
	registered := seedInvitationUser(t, db, 11, "registered", common.RoleCommonUser)
	use := model.InviteCodeUse{
		InviteCodeID: invite.ID, InviterUserID: inviter.Id, RegisteredUserID: registered.Id,
		RegisteredAt: time.Now().Unix(), Source: "password", InviterReward: 10,
		InviterRewardStatus: model.InviteRewardStatusAvailable,
		InviteeRewardStatus: model.InviteRewardStatusSkipped, AuditEventID: "review-race-use",
	}
	require.NoError(t, db.Create(&use).Error)
	payout := model.InviteRewardPayout{
		InviteCodeUseID: use.ID, TriggerType: model.InviteRewardTriggerRegistrationInviter,
		TriggerID: fmt.Sprint(use.ID), RecipientKind: model.InviteRewardRecipientInviter,
		RecipientUserID: inviter.Id, EligibleQuota: 10, Amount: 10,
		ReversalState: model.InviteRewardReversalNone, Status: model.InviteRewardStatusAvailable,
	}
	require.NoError(t, db.Create(&payout).Error)
	request, err := SubmitInvitationRewardPayout(context.Background(), inviter.Id)
	require.NoError(t, err)
	require.NoError(t, db.Model(&model.InviteRewardPayout{}).Where("id = ?", payout.ID).
		Update("status", model.InviteRewardStatusAvailable).Error)

	err = ReviewInvitationRewardPayoutRequest(context.Background(), ReviewInvitationRewardPayoutRequestInput{
		ReviewerID: 1, RequestID: request.ID, Approve: true,
	})

	require.ErrorIs(t, err, ErrInviteRewardReviewInvalid)
	var updatedUse model.InviteCodeUse
	require.NoError(t, db.First(&updatedUse, use.ID).Error)
	require.Equal(t, model.InviteRewardStatusPendingPayout, updatedUse.InviterRewardStatus)
	var ledgerCount int64
	require.NoError(t, db.Model(&model.WalletLedgerEntry{}).Count(&ledgerCount).Error)
	require.Zero(t, ledgerCount)
	var auditEvents []model.AuditOutboxEvent
	require.NoError(t, db.Where("event_type = ?", "invitation.reward_payout_reviewed").Find(&auditEvents).Error)
	require.Empty(t, auditEvents)
}

func TestInvitationUserStateIncludesApplicationCodeAndUses(t *testing.T) {
	db := setupInvitationServiceTest(t)
	seedInvitationUser(t, db, 1, "primary-root", common.RoleRootUser)
	seedInvitationUser(t, db, 9, "owner", common.RoleCommonUser)
	application, err := ApplyForInviteCode(context.Background(), 9, "invite my workspace")
	require.NoError(t, err)
	state, err := GetInvitationUserState(context.Background(), 9)
	require.NoError(t, err)
	require.NotNil(t, state.Application)
	require.Equal(t, application.ID, state.Application.ID)
	require.False(t, state.CanApply)
	require.Nil(t, state.Code)

	issued, err := ReviewInviteCodeApplication(context.Background(), InviteCodeReviewRequest{
		ReviewerID: 1, ApplicationID: application.ID, Approve: true,
		Issue: InviteCodeIssueSpec{Kind: model.InviteCodeKindMulti, MaxUses: 3, ExpiresAt: time.Now().Add(24 * time.Hour).Unix()},
	})
	require.NoError(t, err)
	registered := &model.User{Username: "state-invitee", Password: "password123"}
	_, err = RegisterWithInvitation(context.Background(), RegistrationRequest{
		User: registered, InvitationCode: issued.Code, Source: "password", RegistrationIP: "203.0.113.45",
	})
	require.NoError(t, err)

	state, err = GetInvitationUserState(context.Background(), 9)
	require.NoError(t, err)
	require.NotNil(t, state.Code)
	require.Equal(t, issued.Code, state.Code.Code)
	require.Equal(t, 1, state.Code.UsedCount)
	require.Len(t, state.Uses, 1)
	require.Equal(t, registered.Id, state.Uses[0].RegisteredUserID)
	payload, err := common.Marshal(state)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "registration_ip")
	require.NotContains(t, string(payload), "reward_last_error")
	require.NotContains(t, string(payload), "audit_event_id")
	require.NotContains(t, string(payload), "reviewer_id")
	require.NotContains(t, string(payload), "issued_by")
	require.NotContains(t, string(payload), "owner_user_id")
	require.False(t, state.CanApply)
}
