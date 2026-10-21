package model

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type legacyIPSecurityProjectionState struct {
	ID             int    `gorm:"primaryKey;autoIncrement:false"`
	Status         string `gorm:"type:varchar(16);not null;default:'pending'"`
	ProcessedLogs  int64  `gorm:"bigint;not null;default:0"`
	ProcessedUsers int64  `gorm:"bigint;not null;default:0"`
	StartedAt      int64  `gorm:"bigint;not null;default:0"`
	CompletedAt    int64  `gorm:"bigint;not null;default:0"`
	UpdatedAt      int64  `gorm:"bigint;not null;default:0"`
	LastError      string `gorm:"type:text;not null"`
}

func (legacyIPSecurityProjectionState) TableName() string {
	return "ip_security_projection_states"
}

func openIPSecurityEventTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&IPSecurityEvent{}, &IPAuditRollup{}, &IPSecurityProjectionState{}))
	return db
}

func recordIPSecurityEventForTest(t *testing.T, db *gorm.DB, event IPSecurityEvent) {
	t.Helper()
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return RecordIPSecurityEventWithTx(tx, &event)
	}))
}

func TestRecordIPSecurityEventWithTxOnlyCountsEventIDOnce(t *testing.T) {
	db := openIPSecurityEventTestDB(t)
	createdAt := time.Date(2026, 7, 14, 12, 30, 0, 0, time.UTC).Unix()
	event := IPSecurityEvent{
		EventID: "request-1", UserID: 7, Username: "alice", CreatedAt: createdAt,
		IP: "203.0.113.17", Kind: IPSecurityKindAPIRequest, Quota: 19, RequestID: "req-1",
	}

	recordIPSecurityEventForTest(t, db, event)
	recordIPSecurityEventForTest(t, db, event)

	var eventCount int64
	require.NoError(t, db.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)

	var rollups []IPAuditRollup
	require.NoError(t, db.Order("bucket asc").Find(&rollups).Error)
	require.Len(t, rollups, 2)
	for _, rollup := range rollups {
		require.Equal(t, 1, rollup.RequestCount)
		require.Equal(t, 1, rollup.APIRequestCount)
		require.Equal(t, 19, rollup.Quota)
		require.Equal(t, createdAt, rollup.FirstSeenAt)
		require.Equal(t, createdAt, rollup.LastSeenAt)
	}
}

func TestRecordIPSecurityEventWithTxLoadsCanonicalRowWhenUpsertReportsMatchedRow(t *testing.T) {
	db := openIPSecurityEventTestDB(t)
	original := IPSecurityEvent{
		EventID: "matched-row-replay", UserID: 22, Username: "matched-row-user",
		CreatedAt: 1_752_500_050, IP: "203.0.113.22",
		Kind: IPSecurityKindAPIRequest, RequestID: "matched-row-request",
	}
	recordIPSecurityEventForTest(t, db, original)

	callbackName := "ip_security_event_test:simulate_client_found_rows"
	require.NoError(t, db.Callback().Create().After("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Schema != nil && tx.Statement.Schema.Table == "ip_security_events" {
			tx.RowsAffected = 1
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Create().Remove(callbackName) })

	replay := original
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return RecordIPSecurityEventWithTx(tx, &replay)
	}))
	require.NotZero(t, replay.ID)
	require.Equal(t, IPSecurityEventProjectionProjected, replay.ProjectionStatus)
	var eventCount int64
	require.NoError(t, db.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
	var rollupCount int64
	require.NoError(t, db.Model(&IPAuditRollup{}).Count(&rollupCount).Error)
	require.EqualValues(t, 2, rollupCount)
}

func TestRecordIPSecurityEventWithTxRejectsConflictingEventID(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*IPSecurityEvent)
	}{
		{name: "user", mutate: func(event *IPSecurityEvent) { event.UserID++ }},
		{name: "created at", mutate: func(event *IPSecurityEvent) { event.CreatedAt++ }},
		{name: "ip", mutate: func(event *IPSecurityEvent) { event.IP = "203.0.113.99" }},
		{name: "kind", mutate: func(event *IPSecurityEvent) {
			event.Kind = IPSecurityKindAPIError
			event.HTTPStatus = http.StatusUnauthorized
		}},
		{name: "HTTP status", mutate: func(event *IPSecurityEvent) { event.HTTPStatus = 401 }},
		{name: "quota", mutate: func(event *IPSecurityEvent) { event.Quota++ }},
		{name: "request id", mutate: func(event *IPSecurityEvent) { event.RequestID = "other-request" }},
	}

	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			db := openIPSecurityEventTestDB(t)
			original := IPSecurityEvent{
				EventID: "shared-event", UserID: 17, Username: " alice ", CreatedAt: 1_752_500_300,
				IP: "203.0.113.17", Kind: IPSecurityKindAPIRequest, Quota: 23, RequestID: "request-17",
			}
			recordIPSecurityEventForTest(t, db, original)
			conflicting := original
			mutation.mutate(&conflicting)

			err := db.Transaction(func(tx *gorm.DB) error {
				return RecordIPSecurityEventWithTx(tx, &conflicting)
			})
			require.ErrorIs(t, err, ErrIPSecurityEventConflict)

			var eventCount int64
			require.NoError(t, db.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
			require.EqualValues(t, 1, eventCount)
			var rollups []IPAuditRollup
			require.NoError(t, db.Find(&rollups).Error)
			require.Len(t, rollups, 2)
			for _, rollup := range rollups {
				require.Equal(t, 1, rollup.RequestCount)
			}
		})
	}
}

func TestRecordIPSecurityEventWithTxAcceptsMutableReplayMetadata(t *testing.T) {
	tests := []struct {
		name   string
		kind   string
		replay func(*IPSecurityEvent)
	}{
		{
			name: "username snapshot", kind: IPSecurityKindAPIRequest,
			replay: func(event *IPSecurityEvent) { event.Username = "renamed-user" },
		},
		{
			name: "registration request id", kind: IPSecurityKindRegistration,
			replay: func(event *IPSecurityEvent) {
				event.Username = "renamed-user"
				event.RequestID = ""
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := openIPSecurityEventTestDB(t)
			original := IPSecurityEvent{
				EventID: "replayed-event", UserID: 21, Username: "original-user", CreatedAt: 1_752_500_700,
				IP: "203.0.113.21", Kind: test.kind, RequestID: "original-request",
			}
			recordIPSecurityEventForTest(t, db, original)
			replayed := original
			test.replay(&replayed)

			require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
				return RecordIPSecurityEventWithTx(tx, &replayed)
			}))
			var stored IPSecurityEvent
			require.NoError(t, db.First(&stored, "event_id = ?", original.EventID).Error)
			require.Equal(t, "original-user", stored.Username)
			require.Equal(t, "original-request", stored.RequestID)
			var rollupCount int64
			require.NoError(t, db.Model(&IPAuditRollup{}).Count(&rollupCount).Error)
			require.EqualValues(t, 2, rollupCount)
		})
	}
}

func TestRecordIPSecurityEventWithTxDetectsStoredMaskedIPConflict(t *testing.T) {
	db := openIPSecurityEventTestDB(t)
	event := IPSecurityEvent{
		EventID: "masked-conflict", UserID: 18, Username: "bob", CreatedAt: 1_752_500_400,
		IP: "198.51.100.18", Kind: IPSecurityKindLoginSuccess,
	}
	recordIPSecurityEventForTest(t, db, event)
	require.NoError(t, db.Model(&IPSecurityEvent{}).
		Where("event_id = ?", event.EventID).
		UpdateColumn("masked_ip", "corrupt-mask").Error)

	err := db.Transaction(func(tx *gorm.DB) error {
		return RecordIPSecurityEventWithTx(tx, &event)
	})
	require.ErrorIs(t, err, ErrIPSecurityEventConflict)
}

func TestRecordIPSecurityEventWithTxRejectsOversizedUsername(t *testing.T) {
	db := openIPSecurityEventTestDB(t)
	event := IPSecurityEvent{
		EventID: "oversized-username", UserID: 19, Username: strings.Repeat("a", 256),
		CreatedAt: 1_752_500_500, IP: "192.0.2.19", Kind: IPSecurityKindAPIRequest,
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		return RecordIPSecurityEventWithTx(tx, &event)
	})
	require.ErrorIs(t, err, ErrInvalidIPSecurityEvent)
	var eventCount int64
	require.NoError(t, db.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.Zero(t, eventCount)
}

func TestRecordIPSecurityEventWithTxMaintainsDailyAndLifetimeRollups(t *testing.T) {
	db := openIPSecurityEventTestDB(t)
	firstAt := time.Date(2026, 7, 14, 23, 59, 0, 0, time.UTC).Unix()
	secondAt := time.Date(2026, 7, 15, 0, 1, 0, 0, time.UTC).Unix()
	recordIPSecurityEventForTest(t, db, IPSecurityEvent{
		EventID: "error-1", UserID: 8, Username: "bob", CreatedAt: firstAt,
		IP: "198.51.100.8", Kind: IPSecurityKindAPIError, HTTPStatus: http.StatusUnauthorized, Quota: 3,
	})
	recordIPSecurityEventForTest(t, db, IPSecurityEvent{
		EventID: "login-failure-1", UserID: 8, Username: "bob", CreatedAt: secondAt,
		IP: "198.51.100.8", Kind: IPSecurityKindLoginFailure, Quota: 5,
	})

	var lifetime IPAuditRollup
	require.NoError(t, db.Where("bucket = ? AND user_id = ? AND ip = ?", 0, 8, "198.51.100.8").First(&lifetime).Error)
	require.Equal(t, 2, lifetime.RequestCount)
	require.Equal(t, 1, lifetime.ErrorCount)
	require.Equal(t, 1, lifetime.LoginFailureCount)
	require.Equal(t, 2, lifetime.AnomalyCount)
	require.Equal(t, 8, lifetime.Quota)
	require.Equal(t, firstAt, lifetime.FirstSeenAt)
	require.Equal(t, secondAt, lifetime.LastSeenAt)

	var daily []IPAuditRollup
	require.NoError(t, db.Where("bucket <> ?", 0).Order("bucket asc").Find(&daily).Error)
	require.Len(t, daily, 2)
	require.Equal(t, utcDayBucket(firstAt), daily[0].Bucket)
	require.Equal(t, 1, daily[0].ErrorCount)
	require.Equal(t, 0, daily[0].LoginFailureCount)
	require.Equal(t, utcDayBucket(secondAt), daily[1].Bucket)
	require.Equal(t, 0, daily[1].ErrorCount)
	require.Equal(t, 1, daily[1].LoginFailureCount)
}

func TestIPSecurityProjectionCountsOnlyUnauthorizedFailures(t *testing.T) {
	db := openIPSecurityEventTestDB(t)
	createdAt := time.Date(2026, 7, 26, 12, 0, 0, 0, time.UTC).Unix()
	for index, status := range []int{401, 429, 503} {
		recordIPSecurityEventForTest(t, db, IPSecurityEvent{
			EventID: fmt.Sprintf("http-status-%d", status), UserID: 7, Username: "alice",
			CreatedAt: createdAt + int64(index), IP: "203.0.113.7",
			Kind: IPSecurityKindAPIError, HTTPStatus: status,
		})
	}

	var lifetime IPAuditRollup
	require.NoError(t, db.Where("bucket = ? AND user_id = ?", 0, 7).First(&lifetime).Error)
	var eventCount int64
	require.NoError(t, db.Model(&IPSecurityEvent{}).Where("user_id = ?", 7).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
	require.Equal(t, 1, lifetime.RequestCount)
	require.Equal(t, 1, lifetime.ErrorCount)
	require.Equal(t, 1, lifetime.UnauthorizedCount)
	require.Equal(t, 1, lifetime.AnomalyCount)
}

func TestIPSecurityEventFromLogSkipsNonUnauthorizedAPIErrors(t *testing.T) {
	for _, status := range []int{0, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		log := Log{
			UserId: 7, Username: "alice", CreatedAt: 1_752_500_000,
			Type: LogTypeError, Ip: "203.0.113.7", RequestId: fmt.Sprintf("request-%d", status),
		}
		if status != 0 {
			log.Other = fmt.Sprintf(`{"status_code":%d}`, status)
		}

		event, project := IPSecurityEventFromLog(&log)

		require.False(t, project)
		require.Nil(t, event)
	}
}

func TestRecordIPSecurityEventWithTxPreservesMappedIPv6Mask(t *testing.T) {
	db := openIPSecurityEventTestDB(t)
	recordIPSecurityEventForTest(t, db, IPSecurityEvent{
		EventID: "mapped-1", UserID: 9, Username: "carol", CreatedAt: 1_752_499_800,
		IP: "[::ffff:203.0.113.57]:443", Kind: IPSecurityKindLoginSuccess,
	})

	var event IPSecurityEvent
	require.NoError(t, db.First(&event, "event_id = ?", "mapped-1").Error)
	require.Equal(t, "::ffff:203.0.113.57", event.IP)
	require.Equal(t, "::ffff:203.0.113.*", event.MaskedIP)

	var lifetime IPAuditRollup
	require.NoError(t, db.First(&lifetime, "bucket = ? AND user_id = ?", 0, 9).Error)
	require.Equal(t, event.IP, lifetime.IP)
	require.Equal(t, event.MaskedIP, lifetime.MaskedIP)
}

func TestRegistrationSecurityEventDoesNotIncrementRequestCount(t *testing.T) {
	db := openIPSecurityEventTestDB(t)
	recordIPSecurityEventForTest(t, db, IPSecurityEvent{
		EventID: "registration-10", UserID: 10, Username: "dave", CreatedAt: 1_752_499_900,
		IP: "192.0.2.10", Kind: IPSecurityKindRegistration,
	})

	var lifetime IPAuditRollup
	require.NoError(t, db.First(&lifetime, "bucket = ? AND user_id = ?", 0, 10).Error)
	require.Equal(t, 1, lifetime.RegistrationCount)
	require.Equal(t, 0, lifetime.RequestCount)
	require.Equal(t, 0, lifetime.APIRequestCount)
}

func TestIPSecurityKindFromLogMapsStableKinds(t *testing.T) {
	tests := []struct {
		name    string
		log     Log
		kind    string
		project bool
	}{
		{name: "consume", log: Log{Type: LogTypeConsume}, kind: IPSecurityKindAPIRequest, project: true},
		{name: "error", log: Log{Type: LogTypeError}, kind: IPSecurityKindAPIError, project: true},
		{name: "login success", log: Log{Type: LogTypeLogin, Content: "user.login_success"}, kind: IPSecurityKindLoginSuccess, project: true},
		{name: "login failure", log: Log{Type: LogTypeLogin, Content: "user.login_failed"}, kind: IPSecurityKindLoginFailure, project: true},
		{name: "logout", log: Log{Type: LogTypeLogin, Content: "user.logout"}, kind: IPSecurityKindLogout, project: true},
		{name: "registration comes from user fact", log: Log{Type: LogTypeLogin, Content: "user.register"}, project: false},
		{name: "other activity", log: Log{Type: LogTypeManage}, project: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			kind, project := IPSecurityKindFromLog(&test.log)
			require.Equal(t, test.project, project)
			require.Equal(t, test.kind, kind)
		})
	}
}

func TestIPSecurityEventFromLogUsesStableIdentity(t *testing.T) {
	explicitID := "audit-outbox-event-1"
	explicit, project := IPSecurityEventFromLog(&Log{
		EventId: &explicitID, UserId: 12, Username: "eve", CreatedAt: 1_752_500_000,
		Type: LogTypeError, Ip: "[::ffff:198.51.100.12]:443", RequestId: "req-explicit", Quota: 7,
		Other: `{"status_code":401}`,
	})
	require.True(t, project)
	require.Equal(t, explicitID, explicit.EventID)
	require.Equal(t, "::ffff:198.51.100.12", explicit.IP)
	require.Equal(t, "::ffff:198.51.100.*", explicit.MaskedIP)
	require.Equal(t, IPSecurityKindAPIError, explicit.Kind)
	require.Equal(t, 401, explicit.HTTPStatus)

	base := Log{
		UserId: 13, Username: "frank", CreatedAt: 1_752_500_100, Type: LogTypeConsume,
		Ip: "203.0.113.13", RequestId: "req-stable", Quota: 11, Content: "first content",
	}
	first, project := IPSecurityEventFromLog(&base)
	require.True(t, project)
	changedPayload := base
	changedPayload.Content = "different sensitive content"
	changedPayload.Other = "different sensitive metadata"
	changedPayload.Quota = 99
	second, project := IPSecurityEventFromLog(&changedPayload)
	require.True(t, project)
	require.Equal(t, first.EventID, second.EventID)

	differentRequest := base
	differentRequest.RequestId = "req-other"
	third, project := IPSecurityEventFromLog(&differentRequest)
	require.True(t, project)
	require.NotEqual(t, first.EventID, third.EventID)

	fallback := base
	fallback.RequestId = ""
	fallback.UpstreamRequestId = ""
	fallbackFirst, project := IPSecurityEventFromLog(&fallback)
	require.True(t, project)
	fallbackSecond, project := IPSecurityEventFromLog(&fallback)
	require.True(t, project)
	require.Equal(t, fallbackFirst.EventID, fallbackSecond.EventID)
	require.NotEmpty(t, fallbackFirst.EventID)
	fallbackSensitiveChange := fallback
	fallbackSensitiveChange.Username = "different-user-snapshot"
	fallbackSensitiveChange.Content = "different legacy content"
	fallbackSensitiveChange.Other = "different legacy metadata"
	fallbackSensitiveEvent, project := IPSecurityEventFromLog(&fallbackSensitiveChange)
	require.True(t, project)
	require.Equal(t, fallbackFirst.EventID, fallbackSensitiveEvent.EventID)
	fallbackDifferentIP := fallback
	fallbackDifferentIP.Ip = "198.51.100.99"
	differentIPEvent, project := IPSecurityEventFromLog(&fallbackDifferentIP)
	require.True(t, project)
	require.NotEqual(t, fallbackFirst.EventID, differentIPEvent.EventID)
	fallbackDifferentID := fallback
	fallbackDifferentID.Id = 99
	differentIDEvent, project := IPSecurityEventFromLog(&fallbackDifferentID)
	require.True(t, project)
	require.NotEqual(t, fallbackFirst.EventID, differentIDEvent.EventID)
}

func TestIPSecurityEventFromLogSkipsRegistrationAndMissingIP(t *testing.T) {
	event, project := IPSecurityEventFromLog(&Log{
		UserId: 14, CreatedAt: 1_752_500_200, Type: LogTypeLogin,
		Content: "user.register", Ip: "192.0.2.14", RequestId: "req-registration",
	})
	require.False(t, project)
	require.Nil(t, event)

	event, project = IPSecurityEventFromLog(&Log{
		UserId: 14, CreatedAt: 1_752_500_200, Type: LogTypeLogin,
		Content: "user.login_success", RequestId: "req-without-ip",
	})
	require.False(t, project)
	require.Nil(t, event)
}

func TestCreateLogKeepsBusinessLogWithoutIPSecurityProjectionWhenDisabled(t *testing.T) {
	db := openIPSecurityEventTestDB(t)
	require.NoError(t, db.AutoMigrate(&Log{}))
	previousDB := DB
	previousLogDB := LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	previousOptionMap := common.OptionMap
	DB = db
	LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.OptionMapRWMutex.Lock()
	common.OptionMap = map[string]string{"ip_security.enabled": "false"}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		DB = previousDB
		LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.OptionMapRWMutex.Lock()
		common.OptionMap = previousOptionMap
		common.OptionMapRWMutex.Unlock()
	})

	require.NoError(t, createLogWithContext(context.Background(), &Log{
		UserId: 7, Username: "disabled-audit-user", CreatedAt: 1_752_501_000,
		Type: LogTypeConsume, Ip: "203.0.113.7", RequestId: "disabled-audit-request",
	}))

	var logCount int64
	require.NoError(t, db.Model(&Log{}).Count(&logCount).Error)
	require.EqualValues(t, 1, logCount)
	var eventCount int64
	require.NoError(t, db.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.Zero(t, eventCount)
}

func TestIPSecurityProjectionMigrationsCreateAllTables(t *testing.T) {
	for _, test := range []struct {
		name    string
		migrate func() error
	}{
		{name: "full", migrate: migrateDB},
		{name: "fast", migrate: migrateDBFast},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(t.TempDir()+"/projection.db"), &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

			previousDB := DB
			previousMainType := common.MainDatabaseType()
			DB = db
			common.SetMainDatabaseType(common.DatabaseTypeSQLite)
			t.Cleanup(func() {
				DB = previousDB
				common.SetMainDatabaseType(previousMainType)
			})

			require.NoError(t, test.migrate())
			require.True(t, db.Migrator().HasTable(&IPSecurityEvent{}))
			require.True(t, db.Migrator().HasIndex(&IPSecurityEvent{}, "idx_ip_security_events_created"))
			require.True(t, db.Migrator().HasIndex(&IPSecurityEvent{}, "idx_ip_security_events_user_timeline"))
			var timelineIndexColumns []struct {
				Name string `gorm:"column:name"`
			}
			require.NoError(t, db.Raw("PRAGMA index_info('idx_ip_security_events_user_timeline')").Scan(&timelineIndexColumns).Error)
			timelineColumnNames := make([]string, len(timelineIndexColumns))
			for index, column := range timelineIndexColumns {
				timelineColumnNames[index] = column.Name
			}
			require.Equal(t, []string{"user_id", "created_at", "request_id", "id"}, timelineColumnNames)
			require.True(t, db.Migrator().HasColumn(&IPSecurityEvent{}, "HTTPStatus"))
			require.True(t, db.Migrator().HasColumn(&IPSecurityEvent{}, "AttemptCount"))
			require.True(t, db.Migrator().HasColumn(&IPSecurityEvent{}, "NextAttemptAt"))
			require.True(t, db.Migrator().HasTable(&IPAuditRollup{}))
			require.True(t, db.Migrator().HasColumn(&IPAuditRollup{}, "UnauthorizedCount"))
			require.True(t, db.Migrator().HasTable(&IPSecurityProjectionState{}))
			require.True(t, db.Migrator().HasTable(&IPSecurityProjectionQuarantine{}))
			for _, field := range []string{
				"LastUserID", "LastLogID", "LastLogCreatedAt", "LastLogRequestID",
				"LastLogEventIdentity", "LogSourceType", "LogSourceGeneration",
				"LockedBy", "LockedUntil", "LeaseVersion",
			} {
				require.Truef(t, db.Migrator().HasColumn(&IPSecurityProjectionState{}, field), "missing projection state column %s", field)
			}
		})
	}
}

func TestIPSecurityProjectionStateMigratesExistingRowWithUnboundedClickHouseCursors(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/projection-upgrade.db"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&legacyIPSecurityProjectionState{}))
	require.NoError(t, db.Create(&legacyIPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusReady, ProcessedLogs: 7, ProcessedUsers: 3, LastError: "",
	}).Error)

	require.NoError(t, db.AutoMigrate(&IPSecurityProjectionState{}))
	var state IPSecurityProjectionState
	require.NoError(t, db.First(&state, "id = ?", 1).Error)
	require.Equal(t, IPSecurityProjectionStatusReady, state.Status)
	require.EqualValues(t, 7, state.ProcessedLogs)
	require.EqualValues(t, 3, state.ProcessedUsers)
	require.Empty(t, state.LastLogRequestID)
	require.Empty(t, state.LastLogEventIdentity)

	columnTypes, err := db.Migrator().ColumnTypes(&IPSecurityProjectionState{})
	require.NoError(t, err)
	columnTypeByName := map[string]string{}
	for _, columnType := range columnTypes {
		columnTypeByName[columnType.Name()] = strings.ToLower(columnType.DatabaseTypeName())
	}
	require.Contains(t, columnTypeByName["last_log_request_id"], "text")
	require.Contains(t, columnTypeByName["last_log_event_identity"], "text")
}

func TestIPAuditRollupUpsertSQLIsPortable(t *testing.T) {
	tests := []struct {
		name                      string
		dialector                 gorm.Dialector
		conflictSQL               string
		qualifiedRequestCountSQL  string
		qualifiedFirstSeenCaseSQL string
		qualifiedLastSeenCaseSQL  string
	}{
		{
			name: "mysql",
			dialector: mysql.New(mysql.Config{
				DSN:                       "gorm:gorm@tcp(localhost:9910)/gorm?charset=utf8mb4&parseTime=True&loc=Local",
				SkipInitializeWithVersion: true,
			}),
			conflictSQL:               "ON DUPLICATE KEY UPDATE",
			qualifiedRequestCountSQL:  "`ip_audit_rollups`.`request_count` +",
			qualifiedFirstSeenCaseSQL: "CASE WHEN `ip_audit_rollups`.`first_seen_at` = 0 OR `ip_audit_rollups`.`first_seen_at` >",
			qualifiedLastSeenCaseSQL:  "CASE WHEN `ip_audit_rollups`.`last_seen_at` <",
		},
		{
			name: "postgres",
			dialector: postgres.New(postgres.Config{
				DSN:                  "host=localhost user=gorm password=gorm dbname=gorm port=9920 sslmode=disable TimeZone=UTC",
				PreferSimpleProtocol: true,
			}),
			conflictSQL:               "ON CONFLICT (\"bucket\",\"user_id\",\"ip\") DO UPDATE SET",
			qualifiedRequestCountSQL:  "\"ip_audit_rollups\".\"request_count\" +",
			qualifiedFirstSeenCaseSQL: "CASE WHEN \"ip_audit_rollups\".\"first_seen_at\" = 0 OR \"ip_audit_rollups\".\"first_seen_at\" >",
			qualifiedLastSeenCaseSQL:  "CASE WHEN \"ip_audit_rollups\".\"last_seen_at\" <",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db, err := gorm.Open(test.dialector, &gorm.Config{
				DryRun:                 true,
				DisableAutomaticPing:   true,
				SkipDefaultTransaction: true,
			})
			require.NoError(t, err)
			var generatedSQL string
			require.NoError(t, db.Callback().Create().After("gorm:create").Register("capture_ip_rollup_sql", func(tx *gorm.DB) {
				generatedSQL = tx.Statement.SQL.String()
			}))
			event := &IPSecurityEvent{
				EventID: "dry-run", UserID: 20, Username: "dry-run", CreatedAt: 1_752_500_600,
				IP: "203.0.113.20", MaskedIP: "203.0.113.*", Kind: IPSecurityKindAPIRequest, Quota: 3,
			}
			require.NoError(t, incrementIPAuditRollup(db, event, utcDayBucket(event.CreatedAt)))
			require.Contains(t, generatedSQL, test.conflictSQL)
			require.Contains(t, generatedSQL, test.qualifiedRequestCountSQL)
			require.Contains(t, generatedSQL, test.qualifiedFirstSeenCaseSQL)
			require.Contains(t, generatedSQL, test.qualifiedLastSeenCaseSQL)
		})
	}
}

func TestRecordIPSecurityEventWithTxAccumulatesPostgresRollup(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("set TEST_POSTGRES_DSN to run postgres rollup integration test")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })

	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { tx.Rollback() })
	require.NoError(t, tx.AutoMigrate(
		&IPSecurityEvent{}, &IPAuditRollup{}, &IPSecurityProjectionState{}, &IPSecurityProjectionQuarantine{},
	))
	for _, field := range []string{"AttemptCount", "NextAttemptAt"} {
		require.Truef(t, tx.Migrator().HasColumn(&IPSecurityEvent{}, field), "missing postgres event column %s", field)
	}
	for _, field := range []string{
		"LastUserID", "LastLogID", "LastLogCreatedAt", "LastLogRequestID",
		"LastLogEventIdentity", "LogSourceType", "LogSourceGeneration",
		"LockedBy", "LockedUntil", "LeaseVersion",
	} {
		require.Truef(t, tx.Migrator().HasColumn(&IPSecurityProjectionState{}, field), "missing postgres state column %s", field)
	}
	require.True(t, tx.Migrator().HasTable(&IPSecurityProjectionQuarantine{}))

	const (
		userID = 2_000_000_001
		ip     = "2001:db8:ffff::201"
	)
	require.NoError(t, tx.Where("event_id IN ?", []string{"postgres-rollup-1", "postgres-rollup-2"}).Delete(&IPSecurityEvent{}).Error)
	require.NoError(t, tx.Where("user_id = ? AND ip = ?", userID, ip).Delete(&IPAuditRollup{}).Error)

	firstAt := time.Date(2026, 7, 16, 10, 0, 0, 0, time.UTC).Unix()
	for _, event := range []IPSecurityEvent{
		{EventID: "postgres-rollup-1", UserID: userID, Username: "postgres-user", CreatedAt: firstAt, IP: ip, Kind: IPSecurityKindAPIRequest, Quota: 3},
		{EventID: "postgres-rollup-2", UserID: userID, Username: "postgres-user", CreatedAt: firstAt + 1, IP: ip, Kind: IPSecurityKindAPIError, HTTPStatus: http.StatusUnauthorized, Quota: 5},
	} {
		event := event
		require.NoError(t, RecordIPSecurityEventWithTx(tx, &event))
	}

	var lifetime IPAuditRollup
	require.NoError(t, tx.Where("bucket = ? AND user_id = ? AND ip = ?", 0, userID, ip).First(&lifetime).Error)
	require.Equal(t, 2, lifetime.RequestCount)
	require.Equal(t, 1, lifetime.APIRequestCount)
	require.Equal(t, 1, lifetime.ErrorCount)
	require.Equal(t, 1, lifetime.AnomalyCount)
	require.Equal(t, 8, lifetime.Quota)
}

func TestIPSecurityProjectionMySQL57Compatibility(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("TEST_MYSQL_DSN"))
	if dsn == "" {
		t.Skip("set TEST_MYSQL_DSN to run MySQL IP security integration test")
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	for _, table := range []interface{}{
		&IPSecurityProjectionQuarantine{}, &IPAuditRollup{}, &IPSecurityEvent{}, &IPSecurityProjectionState{},
	} {
		require.NoError(t, db.Migrator().DropTable(table))
	}
	t.Cleanup(func() {
		for _, table := range []interface{}{
			&IPSecurityProjectionQuarantine{}, &IPAuditRollup{}, &IPSecurityEvent{}, &IPSecurityProjectionState{},
		} {
			_ = db.Migrator().DropTable(table)
		}
	})

	require.NoError(t, db.AutoMigrate(&legacyIPSecurityProjectionState{}))
	require.NoError(t, db.Create(&legacyIPSecurityProjectionState{
		ID: 1, Status: IPSecurityProjectionStatusReady, ProcessedLogs: 7, ProcessedUsers: 3, LastError: "",
	}).Error)
	require.NoError(t, db.AutoMigrate(
		&IPSecurityEvent{}, &IPAuditRollup{}, &IPSecurityProjectionState{}, &IPSecurityProjectionQuarantine{},
	))
	var state IPSecurityProjectionState
	require.NoError(t, db.First(&state, "id = ?", 1).Error)
	require.EqualValues(t, 7, state.ProcessedLogs)
	require.Empty(t, state.LastLogRequestID)
	require.Empty(t, state.LastLogEventIdentity)
	columnTypes, err := db.Migrator().ColumnTypes(&IPSecurityProjectionState{})
	require.NoError(t, err)
	columnTypeByName := map[string]string{}
	for _, columnType := range columnTypes {
		columnTypeByName[columnType.Name()] = strings.ToLower(columnType.DatabaseTypeName())
	}
	require.Contains(t, columnTypeByName["last_log_request_id"], "text")
	require.Contains(t, columnTypeByName["last_log_event_identity"], "text")

	event := IPSecurityEvent{
		EventID: "mysql-found-rows-event", UserID: 24, Username: "mysql-user",
		CreatedAt: 1_752_501_500, IP: "203.0.113.24",
		Kind: IPSecurityKindAPIRequest, RequestID: "mysql-found-rows",
	}
	for range 2 {
		replay := event
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			return RecordIPSecurityEventWithTx(tx, &replay)
		}))
	}
	var eventCount int64
	require.NoError(t, db.Model(&IPSecurityEvent{}).Count(&eventCount).Error)
	require.EqualValues(t, 1, eventCount)
	var rollupCount int64
	require.NoError(t, db.Model(&IPAuditRollup{}).Count(&rollupCount).Error)
	require.EqualValues(t, 2, rollupCount)

	for range 2 {
		require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
			return recordIPSecurityProjectionQuarantineWithTx(
				tx, "log:relational:24", "log_relational", "24", ErrInvalidIPSecurityEvent,
			)
		}))
	}
	var quarantine IPSecurityProjectionQuarantine
	require.NoError(t, db.First(&quarantine, "source_key = ?", "log:relational:24").Error)
	require.Equal(t, 2, quarantine.AttemptCount)
}
