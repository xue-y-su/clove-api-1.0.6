package model

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ipSecurityProjectionLeaseSeconds          = int64(300)
	ipSecurityProjectionCleanupTimeout        = 2 * time.Second
	ipSecurityProjectionTransactionRetries    = 3
	ipSecurityProjectionTransactionRetryDelay = 25 * time.Millisecond
	defaultClickHouseIPSecurityOverlapSeconds = int64(300)
	clickHouseIPSecurityOverlapSecondsEnv     = "IP_SECURITY_CLICKHOUSE_OVERLAP_SECONDS"
	ipSecurityLogSourceGenerationEnv          = "IP_SECURITY_LOG_SOURCE_GENERATION"
)

var errIPSecurityProjectionLeaseLost = errors.New("IP security projection lease lost")

func retryIPSecurityProjectionTransaction(ctx context.Context, operation func() error) error {
	var err error
	for attempt := 0; attempt < ipSecurityProjectionTransactionRetries; attempt++ {
		if err = ctx.Err(); err != nil {
			return err
		}
		err = operation()
		if err == nil || !isRetryableIPSecurityProjectionTransactionError(err) || attempt == ipSecurityProjectionTransactionRetries-1 {
			return err
		}
		timer := time.NewTimer(time.Duration(attempt+1) * ipSecurityProjectionTransactionRetryDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
}

func isRetryableIPSecurityProjectionTransactionError(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "deadlock found") ||
		strings.Contains(message, "deadlock detected") ||
		strings.Contains(message, "error 1213") ||
		strings.Contains(message, "sqlstate 40p01") ||
		strings.Contains(message, "sqlstate[40001]") ||
		(strings.Contains(message, "savepoint") && strings.Contains(message, "does not exist"))
}

func IPSecurityProjectionReady(ctx context.Context) (bool, error) {
	_, ready, err := GetIPSecurityProjectionProgress(ctx)
	return ready, err
}

func GetIPSecurityProjectionProgress(ctx context.Context) (IPSecurityProjectionState, bool, error) {
	if DB == nil {
		return IPSecurityProjectionState{}, false, errors.New("IP security projection database is not initialized")
	}
	var state IPSecurityProjectionState
	result := DB.WithContext(ctx).Where("id = ?", 1).Limit(1).Find(&state)
	if result.Error != nil {
		return IPSecurityProjectionState{}, false, result.Error
	}
	if result.RowsAffected == 0 {
		return IPSecurityProjectionState{ID: 1, Status: IPSecurityProjectionStatusPending}, false, nil
	}
	if state.Status != IPSecurityProjectionStatusReady {
		return state, false, nil
	}
	var pending IPSecurityEvent
	result = DB.WithContext(ctx).Model(&IPSecurityEvent{}).Select("id").
		Where("projection_status = ?", IPSecurityEventProjectionPending).
		Limit(1).Find(&pending)
	if result.Error != nil {
		return IPSecurityProjectionState{}, false, result.Error
	}
	return state, result.RowsAffected == 0, nil
}

type IPSecurityProjectionBatchResult struct {
	Acquired bool
	CaughtUp bool
}

func BackfillIPSecurityProjection(ctx context.Context, batchSize int) (err error) {
	if !IPSecurityCollectionEnabled() {
		return nil
	}
	if DB == nil || LOG_DB == nil {
		return errors.New("IP security projection databases are not initialized")
	}
	if batchSize < 1 {
		batchSize = 500
	}
	if batchSize > 5000 {
		batchSize = 5000
	}
	runID := uuid.NewString()
	state, acquired, err := claimIPSecurityProjectionRun(ctx, runID)
	if err != nil {
		return err
	}
	if !acquired {
		return nil
	}
	defer func() {
		releaseErr := releaseIPSecurityProjectionRun(runID)
		if err == nil {
			err = releaseErr
		}
	}()
	defer func() {
		if err != nil {
			markIPSecurityProjectionFailed(runID, err)
		}
	}()
	if err := ensureIPSecurityProjectionLogSource(ctx, &state, runID); err != nil {
		return err
	}

	initialRun := state.Status != IPSecurityProjectionStatusReady
	if !initialRun {
		if _, err = backfillIPSecurityRegistrations(ctx, batchSize, &state, runID); err != nil {
			return err
		}
		_, err = backfillIPSecurityLogs(ctx, batchSize, &state, runID)
		return err
	}

	now := common.GetTimestamp()
	if err := updateIPSecurityProjectionRun(ctx, runID, map[string]interface{}{
		"status":       IPSecurityProjectionStatusRunning,
		"started_at":   now,
		"completed_at": 0,
		"last_error":   "",
	}); err != nil {
		return err
	}
	state.Status = IPSecurityProjectionStatusRunning
	state.StartedAt = now
	state.CompletedAt = 0
	state.LastError = ""

	processedUsers, err := backfillIPSecurityRegistrations(ctx, batchSize, &state, runID)
	if err != nil {
		return err
	}
	if err := updateIPSecurityProjectionProgress(ctx, runID, 0, processedUsers); err != nil {
		return err
	}
	state.ProcessedUsers = processedUsers
	processedLogs, err := backfillIPSecurityLogs(ctx, batchSize, &state, runID)
	if err != nil {
		return err
	}
	for {
		projected, pendingFailures, err := ProjectPendingIPSecurityEvents(ctx, batchSize)
		if err != nil {
			return err
		}
		if pendingFailures > 0 {
			return fmt.Errorf("%d IP security events remain pending after projection", pendingFailures)
		}
		if projected == 0 {
			break
		}
		if err := renewIPSecurityProjectionRun(ctx, runID); err != nil {
			return err
		}
	}
	var pendingEvents int64
	if err := DB.WithContext(ctx).Model(&IPSecurityEvent{}).
		Where("projection_status = ?", IPSecurityEventProjectionPending).
		Count(&pendingEvents).Error; err != nil {
		return err
	}
	if pendingEvents > 0 {
		return fmt.Errorf("%d IP security events remain pending after projection", pendingEvents)
	}

	completedAt := common.GetTimestamp()
	return updateIPSecurityProjectionRun(ctx, runID, map[string]interface{}{
		"status":          IPSecurityProjectionStatusReady,
		"processed_logs":  processedLogs,
		"processed_users": processedUsers,
		"completed_at":    completedAt,
		"last_error":      "",
	})
}

func BackfillIPSecurityProjectionBatch(ctx context.Context, batchSize int) (result IPSecurityProjectionBatchResult, err error) {
	if !IPSecurityCollectionEnabled() {
		return IPSecurityProjectionBatchResult{CaughtUp: true}, nil
	}
	if DB == nil || LOG_DB == nil {
		return result, errors.New("IP security projection databases are not initialized")
	}
	if batchSize < 1 {
		batchSize = 500
	}
	if batchSize > 5000 {
		batchSize = 5000
	}
	runID := uuid.NewString()
	state, acquired, err := claimIPSecurityProjectionRun(ctx, runID)
	if err != nil {
		return result, err
	}
	result.Acquired = acquired
	if !acquired {
		return result, nil
	}
	defer func() {
		releaseErr := releaseIPSecurityProjectionRun(runID)
		if err == nil {
			err = releaseErr
		}
	}()
	defer func() {
		if err != nil {
			markIPSecurityProjectionFailed(runID, err)
		}
	}()
	if err := ensureIPSecurityProjectionLogSource(ctx, &state, runID); err != nil {
		return result, err
	}

	if state.Status != IPSecurityProjectionStatusRunning && state.Status != IPSecurityProjectionStatusReady {
		startedAt := state.StartedAt
		if startedAt == 0 {
			startedAt = common.GetTimestamp()
		}
		if err := updateIPSecurityProjectionRun(ctx, runID, map[string]interface{}{
			"status": IPSecurityProjectionStatusRunning, "started_at": startedAt,
			"completed_at": 0, "last_error": "",
		}); err != nil {
			return result, err
		}
		state.Status = IPSecurityProjectionStatusRunning
		state.StartedAt = startedAt
	}
	if _, err := backfillIPSecurityRegistrationBatches(ctx, batchSize, &state, runID, 1); err != nil {
		return result, err
	}
	if _, err := backfillIPSecurityLogBatches(ctx, batchSize, &state, runID, 1); err != nil {
		return result, err
	}
	_, pendingFailures, err := ProjectPendingIPSecurityEvents(ctx, batchSize)
	if err != nil {
		return result, err
	}
	if pendingFailures > 0 {
		return result, fmt.Errorf("%d IP security events remain pending after projection", pendingFailures)
	}

	result.CaughtUp, err = ipSecurityProjectionCaughtUp(ctx, state)
	if err != nil {
		return result, err
	}
	if !result.CaughtUp {
		if state.Status == IPSecurityProjectionStatusReady {
			err = updateIPSecurityProjectionRun(ctx, runID, map[string]interface{}{
				"status": IPSecurityProjectionStatusRunning, "completed_at": 0, "last_error": "",
			})
		}
		return result, err
	}
	completedAt := common.GetTimestamp()
	err = updateIPSecurityProjectionRun(ctx, runID, map[string]interface{}{
		"status": IPSecurityProjectionStatusReady, "processed_logs": state.ProcessedLogs,
		"processed_users": state.ProcessedUsers, "completed_at": completedAt, "last_error": "",
	})
	return result, err
}

func ipSecurityProjectionCaughtUp(ctx context.Context, state IPSecurityProjectionState) (bool, error) {
	userHighWater, usersFound, err := ipSecurityRegistrationHighWater(ctx)
	if err != nil {
		return false, err
	}
	if usersFound && state.LastUserID < userHighWater {
		return false, nil
	}
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		logHighWater, logsFound, err := clickHouseIPSecurityBackfillHighWater(ctx)
		if err != nil {
			return false, err
		}
		if logsFound && clickHouseIPSecurityCursorLess(clickHouseIPSecurityCursorFromState(state), logHighWater) {
			return false, nil
		}
	} else {
		logHighWater, logsFound, err := relationalIPSecurityBackfillHighWater(ctx)
		if err != nil {
			return false, err
		}
		if logsFound && state.LastLogID < logHighWater {
			return false, nil
		}
	}
	var pending IPSecurityEvent
	result := DB.WithContext(ctx).Model(&IPSecurityEvent{}).Select("id").
		Where("projection_status = ?", IPSecurityEventProjectionPending).
		Limit(1).Find(&pending)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected == 0, nil
}

func ensureIPSecurityProjectionLogSource(ctx context.Context, state *IPSecurityProjectionState, runID string) error {
	sourceType := string(common.LogDatabaseType())
	generation := strings.TrimSpace(common.GetEnvOrDefaultString(ipSecurityLogSourceGenerationEnv, ""))
	if len(generation) > 128 {
		generation = hashIPSecurityEventIdentity("log_source_generation", generation)
	}
	if state.LogSourceType == sourceType && state.LogSourceGeneration == generation {
		return nil
	}
	if err := updateIPSecurityProjectionRun(ctx, runID, map[string]interface{}{
		"processed_logs":          0,
		"last_log_id":             0,
		"last_log_created_at":     0,
		"last_log_request_id":     "",
		"last_log_event_identity": "",
		"log_source_type":         sourceType,
		"log_source_generation":   generation,
	}); err != nil {
		return err
	}
	state.ProcessedLogs = 0
	state.LastLogID = 0
	state.LastLogCreatedAt = 0
	state.LastLogRequestID = ""
	state.LastLogEventIdentity = ""
	state.LogSourceType = sourceType
	state.LogSourceGeneration = generation
	return nil
}

func backfillIPSecurityRegistrations(ctx context.Context, batchSize int, state *IPSecurityProjectionState, runID string) (int64, error) {
	return backfillIPSecurityRegistrationBatches(ctx, batchSize, state, runID, 0)
}

func backfillIPSecurityRegistrationBatches(ctx context.Context, batchSize int, state *IPSecurityProjectionState, runID string, maxBatches int) (int64, error) {
	processed := state.ProcessedUsers
	lastID := state.LastUserID
	processedBatches := 0
	if lastID == 0 {
		processed = 0
	}
	highWater, found, err := ipSecurityRegistrationHighWater(ctx)
	if err != nil {
		return processed, err
	}
	if !found {
		return processed, nil
	}
	if lastID > highWater {
		processed = 0
		lastID = 0
	}
	for {
		if err := ctx.Err(); err != nil {
			return processed, err
		}
		var users []User
		if err := DB.WithContext(ctx).
			Select("id", "username", "created_at", "registration_ip").
			Where("id > ? AND id <= ?", lastID, highWater).
			Order("id asc").
			Limit(batchSize).
			Find(&users).Error; err != nil {
			return processed, err
		}
		if len(users) == 0 {
			return processed, nil
		}
		nextProcessed := processed
		nextLastID := users[len(users)-1].Id
		if err := retryIPSecurityProjectionTransaction(ctx, func() error {
			attemptProcessed := processed
			err := DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				for i := range users {
					if users[i].RegistrationIP == "" {
						continue
					}
					event := IPSecurityEvent{
						EventID: fmt.Sprintf("user:register:%d", users[i].Id), UserID: users[i].Id,
						Username: users[i].Username, CreatedAt: users[i].CreatedAt,
						IP: users[i].RegistrationIP, Kind: IPSecurityKindRegistration,
					}
					if err := RecordIPSecurityEventWithTx(tx, &event); err != nil {
						if !isQuarantinableIPSecurityProjectionError(err) {
							return err
						}
						identity := strconv.Itoa(users[i].Id)
						if quarantineErr := recordIPSecurityProjectionQuarantineWithTx(
							tx, "user:"+identity, "user", identity, err,
						); quarantineErr != nil {
							return quarantineErr
						}
					}
					attemptProcessed++
				}
				now := common.GetTimestamp()
				result := tx.Model(&IPSecurityProjectionState{}).
					Where("id = ? AND locked_by = ? AND locked_until >= ?", 1, runID, now).
					Updates(map[string]interface{}{
						"processed_users": attemptProcessed,
						"last_user_id":    nextLastID,
						"last_error":      "",
						"locked_until":    now + ipSecurityProjectionLeaseSeconds,
						"lease_version":   gorm.Expr("lease_version + ?", 1),
					})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return errIPSecurityProjectionLeaseLost
				}
				return nil
			})
			if err == nil {
				nextProcessed = attemptProcessed
			}
			return err
		}); err != nil {
			return processed, err
		}
		processed = nextProcessed
		lastID = nextLastID
		state.ProcessedUsers = processed
		state.LastUserID = lastID
		processedBatches++
		if maxBatches > 0 && processedBatches >= maxBatches {
			return processed, nil
		}
	}
}

func ipSecurityRegistrationHighWater(ctx context.Context) (int, bool, error) {
	var user User
	result := DB.WithContext(ctx).Select("id").Order("id desc").Limit(1).Find(&user)
	if result.Error != nil {
		return 0, false, result.Error
	}
	return user.Id, result.RowsAffected == 1, nil
}

func backfillIPSecurityLogs(ctx context.Context, batchSize int, state *IPSecurityProjectionState, runID string) (int64, error) {
	return backfillIPSecurityLogBatches(ctx, batchSize, state, runID, 0)
}

func backfillIPSecurityLogBatches(ctx context.Context, batchSize int, state *IPSecurityProjectionState, runID string, maxBatches int) (int64, error) {
	processed := state.ProcessedLogs
	lastID := state.LastLogID
	processedBatches := 0
	clickHouse := common.UsingLogDatabase(common.DatabaseTypeClickHouse)
	clickHouseCursor := clickHouseIPSecurityCursorFromState(*state)
	clickHouseScanCursor := clickHouseIPSecurityReconciliationStart(clickHouseCursor, clickHouseIPSecurityOverlapSeconds())
	var clickHouseHighWater clickHouseIPSecurityCursor
	if clickHouse {
		if clickHouseCursor.EventIdentity == "" {
			processed = 0
		}
		var found bool
		var err error
		clickHouseHighWater, found, err = clickHouseIPSecurityBackfillHighWater(ctx)
		if err != nil {
			return processed, err
		}
		if !found {
			return processed, nil
		}
	} else {
		if lastID == 0 {
			processed = 0
		}
		var found bool
		var highWater int
		var err error
		highWater, found, err = relationalIPSecurityBackfillHighWater(ctx)
		if err != nil {
			return processed, err
		}
		if !found {
			return processed, nil
		}
		for {
			if err := ctx.Err(); err != nil {
				return processed, err
			}
			var logs []Log
			if err := LogReadDB().WithContext(ctx).
				Where("logs.id > ? AND logs.id <= ?", lastID, highWater).
				Order("logs.id asc").Limit(batchSize).Find(&logs).Error; err != nil {
				return processed, err
			}
			if len(logs) == 0 {
				return processed, nil
			}
			nextProcessed := processed + int64(len(logs))
			nextLastID := logs[len(logs)-1].Id
			if err := projectIPSecurityLogBatchAndUpdate(ctx, logs, runID, map[string]interface{}{
				"processed_logs": nextProcessed,
				"last_log_id":    nextLastID,
				"last_error":     "",
			}); err != nil {
				return processed, err
			}
			processed = nextProcessed
			lastID = nextLastID
			state.ProcessedLogs = processed
			state.LastLogID = lastID
			processedBatches++
			if maxBatches > 0 && processedBatches >= maxBatches {
				return processed, nil
			}
		}
	}
	query := clickHouseIPSecurityBackfillRows(ctx, clickHouseScanCursor, clickHouseHighWater)
	if maxBatches > 0 {
		query = query.Limit(batchSize * maxBatches)
	}
	rows, err := query.Rows()
	if err != nil {
		return processed, err
	}
	defer rows.Close()
	logs := make([]Log, 0, batchSize)
	flush := func() error {
		if len(logs) == 0 {
			return nil
		}
		pageCursor := clickHouseIPSecurityCursorFromLog(logs[len(logs)-1])
		if pageCursor.EventIdentity == "" {
			return errors.New("ClickHouse IP security backfill row is missing its event identity")
		}
		nextCursor := clickHouseCursor
		nextProcessed := processed
		for i := range logs {
			rowCursor := clickHouseIPSecurityCursorFromLog(logs[i])
			if clickHouseIPSecurityCursorLess(nextCursor, rowCursor) {
				nextProcessed++
				nextCursor = rowCursor
			}
		}
		if err := projectIPSecurityLogBatchAndUpdate(ctx, logs, runID, map[string]interface{}{
			"processed_logs":          nextProcessed,
			"last_log_created_at":     nextCursor.CreatedAt,
			"last_log_request_id":     nextCursor.RequestID,
			"last_log_event_identity": nextCursor.EventIdentity,
			"last_error":              "",
		}); err != nil {
			return err
		}
		processed = nextProcessed
		clickHouseCursor = nextCursor
		state.ProcessedLogs = processed
		state.LastLogCreatedAt = clickHouseCursor.CreatedAt
		state.LastLogRequestID = clickHouseCursor.RequestID
		state.LastLogEventIdentity = clickHouseCursor.EventIdentity
		logs = logs[:0]
		return nil
	}
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return processed, err
		}
		var log Log
		if err := LOG_DB.ScanRows(rows, &log); err != nil {
			return processed, err
		}
		logs = append(logs, log)
		if len(logs) == batchSize {
			if err := flush(); err != nil {
				return processed, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return processed, err
	}
	if err := flush(); err != nil {
		return processed, err
	}
	return processed, nil
}

func projectIPSecurityLogBatchAndUpdate(ctx context.Context, logs []Log, runID string, updates map[string]interface{}) error {
	return retryIPSecurityProjectionTransaction(ctx, func() error {
		return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			for i := range logs {
				event, project := IPSecurityEventFromLog(&logs[i])
				if !project {
					continue
				}
				if err := RecordIPSecurityEventWithTx(tx, event); err != nil {
					if !isQuarantinableIPSecurityProjectionError(err) {
						return err
					}
					sourceKey, sourceType, sourceIdentity := ipSecurityLogSourceIdentity(logs[i])
					if quarantineErr := recordIPSecurityProjectionQuarantineWithTx(
						tx, sourceKey, sourceType, sourceIdentity, err,
					); quarantineErr != nil {
						return quarantineErr
					}
				}
			}
			now := common.GetTimestamp()
			updates["locked_until"] = now + ipSecurityProjectionLeaseSeconds
			updates["lease_version"] = gorm.Expr("lease_version + ?", 1)
			result := tx.Model(&IPSecurityProjectionState{}).
				Where("id = ? AND locked_by = ? AND locked_until >= ?", 1, runID, now).
				Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return errIPSecurityProjectionLeaseLost
			}
			return nil
		})
	})
}

func ipSecurityLogSourceIdentity(log Log) (sourceKey string, sourceType string, sourceIdentity string) {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		identity := log.AuditEventIdentity
		if len(identity) > 191 {
			identity = hashIPSecurityEventIdentity("projection_source", identity)
		}
		return "log:clickhouse:" + identity, "log_clickhouse", identity
	}
	identity := strconv.Itoa(log.Id)
	return "log:relational:" + identity, "log_relational", identity
}

func relationalIPSecurityBackfillHighWater(ctx context.Context) (int, bool, error) {
	var log Log
	result := LogReadDB().WithContext(ctx).Select("logs.id").Order("logs.id desc").Limit(1).Find(&log)
	if result.Error != nil {
		return 0, false, result.Error
	}
	return log.Id, result.RowsAffected == 1, nil
}

type clickHouseIPSecurityCursor struct {
	CreatedAt     int64
	RequestID     string
	EventIdentity string
}

func clickHouseIPSecurityCursorFromLog(log Log) clickHouseIPSecurityCursor {
	return clickHouseIPSecurityCursor{
		CreatedAt: log.CreatedAt, RequestID: log.RequestId, EventIdentity: log.AuditEventIdentity,
	}
}

func clickHouseIPSecurityCursorFromState(state IPSecurityProjectionState) clickHouseIPSecurityCursor {
	return clickHouseIPSecurityCursor{
		CreatedAt: state.LastLogCreatedAt, RequestID: state.LastLogRequestID,
		EventIdentity: state.LastLogEventIdentity,
	}
}

func clickHouseIPSecurityReconciliationStart(cursor clickHouseIPSecurityCursor, overlapSeconds int64) clickHouseIPSecurityCursor {
	if cursor.EventIdentity == "" || overlapSeconds >= cursor.CreatedAt {
		return clickHouseIPSecurityCursor{}
	}
	return clickHouseIPSecurityCursor{CreatedAt: cursor.CreatedAt - overlapSeconds}
}

func clickHouseIPSecurityOverlapSeconds() int64 {
	overlap := int64(common.GetEnvOrDefault(clickHouseIPSecurityOverlapSecondsEnv, int(defaultClickHouseIPSecurityOverlapSeconds)))
	if overlap < 1 {
		return 1
	}
	return overlap
}

func clickHouseIPSecurityCursorLess(left clickHouseIPSecurityCursor, right clickHouseIPSecurityCursor) bool {
	if left.CreatedAt != right.CreatedAt {
		return left.CreatedAt < right.CreatedAt
	}
	if left.RequestID != right.RequestID {
		return left.RequestID < right.RequestID
	}
	return left.EventIdentity < right.EventIdentity
}

func clickHouseIPSecurityBackfillHighWater(ctx context.Context) (clickHouseIPSecurityCursor, bool, error) {
	var log Log
	identity := clickHouseAuditEventIdentitySQL()
	result := LOG_DB.WithContext(ctx).Table("logs").
		Select("created_at, request_id, " + identity + " AS audit_event_identity").
		Order("created_at desc, request_id desc, audit_event_identity desc").
		Limit(1).
		Find(&log)
	if result.Error != nil {
		return clickHouseIPSecurityCursor{}, false, result.Error
	}
	if result.RowsAffected == 0 {
		return clickHouseIPSecurityCursor{}, false, nil
	}
	cursor := clickHouseIPSecurityCursorFromLog(log)
	if cursor.EventIdentity == "" {
		return clickHouseIPSecurityCursor{}, false, errors.New("ClickHouse IP security backfill high-water row is missing its event identity")
	}
	return cursor, true, nil
}

func clickHouseIPSecurityBackfillRows(
	ctx context.Context,
	cursor clickHouseIPSecurityCursor,
	highWater clickHouseIPSecurityCursor,
) *gorm.DB {
	const tuple = "tuple(logs.created_at, logs.request_id, logs.audit_event_identity)"
	query := LogReadDB()
	if cursor.CreatedAt != 0 || cursor.RequestID != "" || cursor.EventIdentity != "" {
		identity := clickHouseAuditEventIdentitySQL()
		sourceTuple := "tuple(created_at, request_id, " + identity + ")"
		query = clickHouseScopedLogReadDB(
			sourceTuple+" > tuple(?, ?, ?)", cursor.CreatedAt, cursor.RequestID, cursor.EventIdentity,
		)
	}
	query = query.WithContext(ctx).
		Where(tuple+" <= tuple(?, ?, ?)", highWater.CreatedAt, highWater.RequestID, highWater.EventIdentity)
	if cursor.CreatedAt != 0 || cursor.RequestID != "" || cursor.EventIdentity != "" {
		query = query.Where(tuple+" > tuple(?, ?, ?)", cursor.CreatedAt, cursor.RequestID, cursor.EventIdentity)
	}
	return query.Order("logs.created_at asc, logs.request_id asc, logs.audit_event_identity asc")
}

func updateIPSecurityProjectionProgress(ctx context.Context, runID string, processedLogs int64, processedUsers int64) error {
	updates := map[string]interface{}{"processed_users": processedUsers}
	if processedLogs > 0 {
		updates["processed_logs"] = processedLogs
	}
	return updateIPSecurityProjectionRun(ctx, runID, updates)
}

func claimIPSecurityProjectionRun(ctx context.Context, runID string) (IPSecurityProjectionState, bool, error) {
	seed := IPSecurityProjectionState{ID: 1, Status: IPSecurityProjectionStatusPending, LastError: ""}
	if err := DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}}, DoNothing: true,
	}).Create(&seed).Error; err != nil {
		return IPSecurityProjectionState{}, false, err
	}

	now := common.GetTimestamp()
	result := DB.WithContext(ctx).Model(&IPSecurityProjectionState{}).
		Where("id = ? AND (locked_by = ? OR locked_until <= ?)", 1, "", now).
		Updates(map[string]interface{}{
			"locked_by": runID, "locked_until": now + ipSecurityProjectionLeaseSeconds,
			"lease_version": gorm.Expr("lease_version + ?", 1),
		})
	if result.Error != nil {
		return IPSecurityProjectionState{}, false, result.Error
	}
	if result.RowsAffected != 1 {
		return IPSecurityProjectionState{}, false, nil
	}
	var state IPSecurityProjectionState
	if err := DB.WithContext(ctx).Where("id = ? AND locked_by = ?", 1, runID).First(&state).Error; err != nil {
		return IPSecurityProjectionState{}, false, err
	}
	return state, true, nil
}

func renewIPSecurityProjectionRun(ctx context.Context, runID string) error {
	return updateIPSecurityProjectionRun(ctx, runID, nil)
}

func updateIPSecurityProjectionRun(ctx context.Context, runID string, updates map[string]interface{}) error {
	if updates == nil {
		updates = map[string]interface{}{}
	}
	now := common.GetTimestamp()
	updates["locked_until"] = now + ipSecurityProjectionLeaseSeconds
	updates["lease_version"] = gorm.Expr("lease_version + ?", 1)
	result := DB.WithContext(ctx).Model(&IPSecurityProjectionState{}).
		Where("id = ? AND locked_by = ? AND locked_until >= ?", 1, runID, now).
		Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errIPSecurityProjectionLeaseLost
	}
	return nil
}

func releaseIPSecurityProjectionRun(runID string) error {
	ctx, cancel := ipSecurityProjectionCleanupContext()
	defer cancel()
	return DB.WithContext(ctx).Model(&IPSecurityProjectionState{}).
		Where("id = ? AND locked_by = ?", 1, runID).
		Updates(map[string]interface{}{"locked_by": "", "locked_until": 0}).Error
}

func markIPSecurityProjectionFailed(runID string, runErr error) {
	message := strings.TrimSpace(runErr.Error())
	if len(message) > 2000 {
		message = message[:2000]
	}
	ctx, cancel := ipSecurityProjectionCleanupContext()
	defer cancel()
	_ = DB.WithContext(ctx).Model(&IPSecurityProjectionState{}).
		Where("id = ? AND locked_by = ?", 1, runID).
		Updates(map[string]interface{}{
			"status": IPSecurityProjectionStatusFailed, "last_error": message,
		}).Error
}

func ipSecurityProjectionCleanupContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), ipSecurityProjectionCleanupTimeout)
}
