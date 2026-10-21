package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrAuditEventInvalid    = errors.New("audit event is invalid")
	ErrAuditEventConflict   = errors.New("audit event id conflicts with an existing event")
	errAuditOutboxLeaseLost = errors.New("audit outbox delivery lease lost")
)

const (
	auditOutboxMaxBatch        = 1000
	auditOutboxLease           = 30 * time.Second
	auditOutboxExternalTimeout = 10 * time.Second
	auditLogSnapshotVersion    = 1
	maxAuditLogSnapshotLength  = 60 * 1024
)

var (
	auditOutboxNow        = common.GetTimestamp
	auditLogFindCanonical = model.FindCanonicalLogByEventID
	auditLogCreate        = createAuditLog
)

var auditAllowedParamKeys = map[string]struct{}{
	"action": {}, "amount": {}, "aff_skip_at": {}, "aff_skip_reason": {}, "application_id": {},
	"attempt_identifier": {}, "auth_method": {}, "balance_after": {},
	"ban_reason_internal": {}, "ban_reason_public": {}, "category": {}, "channel_id": {}, "claimant_user_id": {}, "code_type": {},
	"count": {}, "delta": {}, "end_timestamp": {}, "expires_at": {}, "failure_category": {},
	"eligible_quota": {}, "from": {}, "full_ip_access": {}, "group": {}, "id": {}, "invite_code_id": {}, "invite_use_id": {}, "inviter_id": {}, "issuer_id": {}, "item_count": {}, "kind": {}, "limit": {}, "login_method": {},
	"masked_key": {}, "max_uses": {}, "method": {}, "model": {}, "name": {}, "operation_id": {},
	"option_id": {}, "option_name": {}, "page": {}, "page_size": {}, "payout_id": {}, "plan_id": {}, "quota": {},
	"range": {}, "rate_basis_points": {}, "reason": {}, "reference_id": {}, "reference_type": {}, "registered_user_id": {}, "request_id": {}, "requested_by_user_id": {}, "result": {}, "revision": {}, "review_reason": {}, "reviewer_id": {},
	"role": {}, "role_from": {}, "role_to": {}, "route": {}, "source": {},
	"settlement": {}, "start_timestamp": {}, "status": {}, "status_from": {}, "status_to": {}, "target_user_id": {},
	"timeline_count": {}, "token_id": {}, "token_user_id": {}, "to": {}, "total": {}, "trade_no": {}, "trigger_type": {}, "type": {}, "use_count": {},
	"user_count": {}, "user_subscription_id": {}, "username": {}, "version": {},
}

type AuditEvent struct {
	EventID   string
	Type      string
	Result    string
	ActorID   int
	LogUserID int
	Target    string
	IP        string
	RequestID string
	Params    map[string]any
}

func EnqueueWithTx(tx *gorm.DB, event AuditEvent) error {
	if tx == nil {
		return ErrAuditEventInvalid
	}
	event.EventID = strings.TrimSpace(event.EventID)
	event.Type = strings.TrimSpace(event.Type)
	event.Result = strings.TrimSpace(event.Result)
	event.Target = strings.TrimSpace(event.Target)
	event.RequestID = strings.TrimSpace(event.RequestID)
	if event.EventID == "" {
		event.EventID = uuid.NewString()
	}
	if event.Result == "" {
		event.Result = "success"
	}
	if event.LogUserID <= 0 {
		event.LogUserID = event.ActorID
	}
	if len(event.EventID) > 191 || !validAuditEventType(event.Type) || len(event.Result) > 32 || len(event.Target) > 255 || len(event.RequestID) > 64 {
		return ErrAuditEventInvalid
	}

	payload, err := common.Marshal(sanitizeAuditParams(event.Params))
	if err != nil {
		return fmt.Errorf("serialize audit event payload: %w", err)
	}
	record := model.AuditOutboxEvent{
		EventID:            event.EventID,
		EventType:          event.Type,
		Result:             event.Result,
		ActorID:            event.ActorID,
		LogUserID:          event.LogUserID,
		Target:             event.Target,
		IP:                 model.NormalizeAuditIP(event.IP),
		RequestID:          event.RequestID,
		Payload:            string(payload),
		Status:             model.AuditOutboxStatusPending,
		LogSnapshotVersion: auditLogSnapshotVersion,
	}

	result := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoNothing: true,
	}).Create(&record)
	if result.Error != nil {
		return result.Error
	}

	var existing model.AuditOutboxEvent
	if err := tx.Where("event_id = ?", event.EventID).First(&existing).Error; err != nil {
		return err
	}
	if existing.EventType != record.EventType || existing.Result != record.Result || existing.ActorID != record.ActorID || existing.LogUserID != record.LogUserID || existing.Target != record.Target || existing.IP != record.IP || existing.RequestID != record.RequestID || existing.Payload != record.Payload {
		return ErrAuditEventConflict
	}
	return nil
}

func DeliverPending(ctx context.Context, limit int) (delivered, failed int, err error) {
	if model.DB == nil || model.LOG_DB == nil {
		return 0, 0, errors.New("audit outbox databases are not initialized")
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > auditOutboxMaxBatch {
		limit = auditOutboxMaxBatch
	}

	now := auditOutboxNow()
	var candidates []model.AuditOutboxEvent
	if err := model.DB.WithContext(ctx).
		Where("status = ? AND next_attempt_at <= ? AND (locked_until = 0 OR locked_until < ?)", model.AuditOutboxStatusPending, now, now).
		Order("id asc").
		Limit(limit).
		Find(&candidates).Error; err != nil {
		return 0, 0, err
	}

	workerID := uuid.NewString()
	for i := range candidates {
		if err := ctx.Err(); err != nil {
			return delivered, failed, err
		}
		claimNow := auditOutboxNow()
		claimed, claimErr := claimAuditOutboxEvent(ctx, candidates[i].ID, workerID, claimNow)
		if claimErr != nil {
			return delivered, failed, claimErr
		}
		if !claimed {
			continue
		}
		var event model.AuditOutboxEvent
		if err := model.DB.WithContext(ctx).Where("id = ? AND locked_by = ?", candidates[i].ID, workerID).First(&event).Error; err != nil {
			return delivered, failed, err
		}
		if deliveryErr := deliverClaimedAuditOutboxEvent(ctx, event.ID, workerID); deliveryErr != nil {
			if errors.Is(deliveryErr, errAuditOutboxLeaseLost) {
				continue
			}
			failed++
			if err := markAuditOutboxFailure(ctx, &event, workerID, deliveryErr); err != nil {
				return delivered, failed, err
			}
			continue
		}
		delivered++
	}
	return delivered, failed, nil
}

func validAuditEventType(eventType string) bool {
	if eventType == "" || len(eventType) > 128 {
		return false
	}
	for _, r := range eventType {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func sanitizeAuditParams(params map[string]any) map[string]any {
	clean := make(map[string]any)
	for key, value := range params {
		key = strings.TrimSpace(strings.ToLower(key))
		if _, ok := auditAllowedParamKeys[key]; !ok {
			continue
		}
		if sanitized, ok := sanitizeAuditValue(value); ok {
			clean[key] = sanitized
		}
	}
	return clean
}

func sanitizeAuditValue(value any) (any, bool) {
	switch typed := value.(type) {
	case nil:
		return nil, true
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return typed, true
	case string:
		if len(typed) > 512 {
			typed = typed[:512]
		}
		return typed, true
	case []string:
		if len(typed) > 20 {
			typed = typed[:20]
		}
		clean := make([]string, 0, len(typed))
		for _, item := range typed {
			if len(item) > 128 {
				item = item[:128]
			}
			clean = append(clean, item)
		}
		return clean, true
	default:
		return nil, false
	}
}

func claimAuditOutboxEvent(ctx context.Context, id int64, workerID string, now int64) (bool, error) {
	result := model.DB.WithContext(ctx).Model(&model.AuditOutboxEvent{}).
		Where("id = ? AND status = ? AND next_attempt_at <= ? AND (locked_until = 0 OR locked_until < ?)", id, model.AuditOutboxStatusPending, now, now).
		Updates(map[string]any{
			"locked_by":     workerID,
			"locked_until":  now + int64(auditOutboxLease/time.Second),
			"attempt_count": gorm.Expr("attempt_count + 1"),
		})
	return result.RowsAffected == 1, result.Error
}

func deliverClaimedAuditOutboxEvent(ctx context.Context, id int64, workerID string) error {
	newlyPrepared, err := prepareClaimedAuditOutboxLog(ctx, id, workerID)
	if err != nil {
		return err
	}
	if err := persistClaimedAuditOutboxLog(ctx, id, workerID, newlyPrepared); err != nil {
		return err
	}
	return finalizeClaimedAuditOutboxEvent(ctx, id, workerID)
}

func prepareClaimedAuditOutboxLog(ctx context.Context, id int64, workerID string) (bool, error) {
	event, err := renewClaimedAuditOutboxEvent(ctx, id, workerID)
	if err != nil {
		return false, err
	}
	if event.LogSnapshot != "" {
		_, err := decodeAuditLogSnapshot(event.LogSnapshot, event.LogSnapshotVersion)
		return false, err
	}
	if event.LogSnapshotVersion != 0 && event.LogSnapshotVersion != auditLogSnapshotVersion {
		return false, fmt.Errorf("unsupported audit log snapshot version %d", event.LogSnapshotVersion)
	}

	legacy := event.LogSnapshotVersion == 0
	fresh := !legacy && event.LogPreparedAt == 0
	var log *model.Log
	canonicalFound := false
	if legacy {
		var persisted model.Log
		var found bool
		if model.LOG_DB == model.DB {
			persisted, found, err = findCanonicalAuditLogWithDB(model.DB.WithContext(ctx), event.EventID)
		} else {
			externalCtx, cancel := context.WithTimeout(ctx, auditOutboxExternalTimeout)
			persisted, found, err = auditLogFindCanonical(externalCtx, event.EventID)
			if err == nil {
				err = externalCtx.Err()
			}
			cancel()
		}
		if err != nil {
			return false, err
		}
		if found {
			expected, buildErr := buildAuditLogForOutboxEvent(model.DB.WithContext(ctx), &event, &persisted.Username)
			if buildErr != nil {
				return false, buildErr
			}
			if !sameAuditLog(expected, &persisted) {
				return false, ErrAuditEventConflict
			}
			log = &persisted
			canonicalFound = true
		}
	}
	if log == nil {
		var usernameOverride *string
		if event.LogPreparedAt > 0 {
			username := event.LogUsername
			usernameOverride = &username
		}
		log, err = buildAuditLogForOutboxEvent(model.DB.WithContext(ctx), &event, usernameOverride)
		if err != nil {
			return false, err
		}
	}

	snapshot, err := encodeAuditLogSnapshot(log)
	if err != nil {
		return false, err
	}
	now := auditOutboxNow()
	preparedAt := event.LogPreparedAt
	if preparedAt == 0 {
		preparedAt = now
	}
	updates := map[string]any{
		"log_prepared_at":      preparedAt,
		"log_username":         log.Username,
		"log_snapshot_version": auditLogSnapshotVersion,
		"log_snapshot":         snapshot,
	}
	if canonicalFound {
		updates["log_persisted_at"] = now
	}
	result := model.DB.WithContext(ctx).Model(&model.AuditOutboxEvent{}).
		Where("id = ? AND status = ? AND locked_by = ? AND locked_until >= ? AND (log_snapshot = ? OR log_snapshot IS NULL)", id, model.AuditOutboxStatusPending, workerID, now, "").
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected != 1 {
		return false, errAuditOutboxLeaseLost
	}
	return fresh && !canonicalFound, nil
}

func persistClaimedAuditOutboxLog(ctx context.Context, id int64, workerID string, newlyPrepared bool) error {
	if model.LOG_DB != model.DB {
		return persistClaimedAuditOutboxLogExternal(ctx, id, workerID, newlyPrepared)
	}
	return persistClaimedAuditOutboxLogShared(ctx, id, workerID, newlyPrepared)
}

func persistClaimedAuditOutboxLogShared(ctx context.Context, id int64, workerID string, newlyPrepared bool) error {
	return model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		event, err := lockClaimedAuditOutboxEvent(tx, id, workerID)
		if err != nil {
			return err
		}
		log, err := decodeAuditLogSnapshot(event.LogSnapshot, event.LogSnapshotVersion)
		if err != nil {
			return err
		}
		if event.LogPersistedAt > 0 {
			return nil
		}

		persisted := model.Log{}
		found := false
		if !newlyPrepared {
			persisted, found, err = findCanonicalAuditLogWithDB(tx, event.EventID)
			if err != nil {
				return err
			}
			if found && !sameAuditLog(log, &persisted) {
				return ErrAuditEventConflict
			}
		}
		if !found {
			if err := createAuditLogWithDB(tx, log); err != nil {
				return err
			}
			persisted, found, err = findCanonicalAuditLogWithDB(tx, event.EventID)
			if err != nil {
				return err
			}
			if !found {
				return errors.New("persisted audit log was not observable after delivery")
			}
			if !sameAuditLog(log, &persisted) {
				return ErrAuditEventConflict
			}
		}

		result := tx.Model(&model.AuditOutboxEvent{}).
			Where("id = ? AND status = ? AND locked_by = ? AND log_persisted_at = 0", id, model.AuditOutboxStatusPending, workerID).
			Update("log_persisted_at", auditOutboxNow())
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errAuditOutboxLeaseLost
		}
		return nil
	})
}

func persistClaimedAuditOutboxLogExternal(ctx context.Context, id int64, workerID string, newlyPrepared bool) error {
	event, err := renewClaimedAuditOutboxEvent(ctx, id, workerID)
	if err != nil {
		return err
	}
	log, err := decodeAuditLogSnapshot(event.LogSnapshot, event.LogSnapshotVersion)
	if err != nil {
		return err
	}
	if event.LogPersistedAt > 0 {
		return nil
	}

	externalCtx, cancel := context.WithTimeout(ctx, auditOutboxExternalTimeout)
	defer cancel()
	persisted := model.Log{}
	found := false
	if !newlyPrepared {
		persisted, found, err = auditLogFindCanonical(externalCtx, event.EventID)
		if err != nil {
			return err
		}
		if err := externalCtx.Err(); err != nil {
			return err
		}
		if found && !sameAuditLog(log, &persisted) {
			return ErrAuditEventConflict
		}
	}
	if !found {
		if err := auditLogCreate(externalCtx, log); err != nil {
			return err
		}
		if err := externalCtx.Err(); err != nil {
			return err
		}
		if !common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
			persisted, found, err = auditLogFindCanonical(externalCtx, event.EventID)
			if err != nil {
				return err
			}
			if err := externalCtx.Err(); err != nil {
				return err
			}
			if !found {
				return errors.New("persisted audit log was not observable after delivery")
			}
			if !sameAuditLog(log, &persisted) {
				return ErrAuditEventConflict
			}
		}
	}

	now := auditOutboxNow()
	result := model.DB.WithContext(ctx).Model(&model.AuditOutboxEvent{}).
		Where("id = ? AND status = ? AND locked_by = ? AND locked_until >= ? AND log_persisted_at = 0", id, model.AuditOutboxStatusPending, workerID, now).
		Update("log_persisted_at", now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errAuditOutboxLeaseLost
	}
	return nil
}

func finalizeClaimedAuditOutboxEvent(ctx context.Context, id int64, workerID string) error {
	return model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		event, err := lockClaimedAuditOutboxEvent(tx, id, workerID)
		if err != nil {
			return err
		}
		if event.LogPersistedAt == 0 {
			return errors.New("audit outbox log is not persisted")
		}
		log, err := decodeAuditLogSnapshot(event.LogSnapshot, event.LogSnapshotVersion)
		if err != nil {
			return err
		}
		if err := projectDeliveredAuditLogWithTx(tx, &event, log); err != nil {
			return err
		}
		return markAuditOutboxDeliveredWithTx(tx, event.ID, workerID)
	})
}

func lockClaimedAuditOutboxEvent(tx *gorm.DB, id int64, workerID string) (model.AuditOutboxEvent, error) {
	if tx == nil || id <= 0 || workerID == "" {
		return model.AuditOutboxEvent{}, errAuditOutboxLeaseLost
	}
	now := auditOutboxNow()
	result := tx.Model(&model.AuditOutboxEvent{}).
		Where("id = ? AND status = ? AND locked_by = ? AND locked_until >= ?", id, model.AuditOutboxStatusPending, workerID, now).
		Update("locked_until", now+int64(auditOutboxLease/time.Second))
	if result.Error != nil {
		return model.AuditOutboxEvent{}, result.Error
	}
	var event model.AuditOutboxEvent
	if err := tx.Where("id = ? AND status = ? AND locked_by = ? AND locked_until >= ?", id, model.AuditOutboxStatusPending, workerID, now).
		First(&event).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.AuditOutboxEvent{}, errAuditOutboxLeaseLost
		}
		return model.AuditOutboxEvent{}, err
	}
	return event, nil
}

func renewClaimedAuditOutboxEvent(ctx context.Context, id int64, workerID string) (model.AuditOutboxEvent, error) {
	var event model.AuditOutboxEvent
	err := model.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		event, err = lockClaimedAuditOutboxEvent(tx, id, workerID)
		return err
	})
	return event, err
}

func buildAuditLogForOutboxEvent(tx *gorm.DB, event *model.AuditOutboxEvent, usernameOverride *string) (*model.Log, error) {
	if tx == nil || event == nil {
		return nil, ErrAuditEventInvalid
	}
	params := map[string]any{}
	if event.Payload != "" {
		if err := common.UnmarshalJsonStr(event.Payload, &params); err != nil {
			return nil, fmt.Errorf("decode audit event payload: %w", err)
		}
	}
	other := map[string]any{
		"op": map[string]any{
			"action": event.EventType,
			"params": params,
		},
		"audit_info": map[string]any{
			"event_type": event.EventType,
			"result":     event.Result,
			"target":     event.Target,
		},
	}
	eventID := event.EventID
	logUserID := event.LogUserID
	if logUserID <= 0 {
		logUserID = event.ActorID
	}
	if event.EventType == "user.login_failed" || event.EventType == "user.register" {
		if targetUserID, ok := params["target_user_id"].(float64); ok {
			candidate := int(targetUserID)
			if candidate > 0 && float64(candidate) == targetUserID {
				logUserID = candidate
			}
		}
	}
	username := ""
	if usernameOverride != nil {
		username = *usernameOverride
	} else if event.EventType != "user.login_failed" && logUserID > 0 {
		var user model.User
		result := tx.Select("username").Where("id = ?", logUserID).Limit(1).Find(&user)
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 1 {
			username = user.Username
		}
	}
	logType := model.LogTypeManage
	content := event.EventType
	if strings.HasPrefix(event.EventType, "user.login") || event.EventType == "user.register" || event.EventType == "user.logout" {
		logType = model.LogTypeLogin
	}
	if event.EventType == "invitation.commission_created" {
		triggerType, triggerOK := params["trigger_type"].(string)
		amountText, amountOK := params["amount"].(string)
		rateText, rateOK := params["rate_basis_points"].(string)
		amount, amountErr := strconv.ParseInt(amountText, 10, 64)
		rateBasisPoints, rateErr := strconv.ParseInt(rateText, 10, 64)
		if !triggerOK || !amountOK || !rateOK || amountErr != nil || rateErr != nil || amount <= 0 || rateBasisPoints < 0 {
			return nil, ErrAuditEventInvalid
		}
		logType = model.LogTypeSystem
		content = model.FormatInvitationCommissionLogContent(triggerType, amount, rateBasisPoints)
	}
	if event.EventType == "invitation.commission_reversed" {
		amountText, ok := params["amount"].(string)
		amount, amountErr := strconv.ParseInt(amountText, 10, 64)
		if !ok || amountErr != nil || amount <= 0 {
			return nil, ErrAuditEventInvalid
		}
		logType = model.LogTypeSystem
		content = model.FormatInvitationCommissionReversalLogContent(amount)
	}
	if event.EventType == "subscription.purchase_completed" {
		planTitle, titleOK := params["name"].(string)
		amountText, amountOK := params["amount"].(string)
		paymentMethod, methodOK := params["method"].(string)
		amount, amountErr := strconv.ParseFloat(amountText, 64)
		if !titleOK || !amountOK || !methodOK || amountErr != nil || amount < 0 {
			return nil, ErrAuditEventInvalid
		}
		logType = model.LogTypeTopup
		if paymentMethod == model.PaymentMethodBalance {
			quotaText, quotaOK := params["quota"].(string)
			chargedQuota, quotaErr := strconv.ParseInt(quotaText, 10, 64)
			if !quotaOK || quotaErr != nil || chargedQuota < 0 {
				return nil, ErrAuditEventInvalid
			}
			content = fmt.Sprintf("使用余额购买订阅成功，套餐: %s，支付金额: %.2f，扣除额度: %d", planTitle, amount, chargedQuota)
		} else {
			content = fmt.Sprintf("订阅购买成功，套餐: %s，支付金额: %.2f，支付方式: %s", planTitle, amount, paymentMethod)
		}
	}
	if event.ActorID > 0 && event.ActorID != logUserID {
		other["admin_info"] = map[string]any{"actor_id": event.ActorID}
	}
	return &model.Log{
		EventId:   &eventID,
		UserId:    logUserID,
		Username:  username,
		CreatedAt: event.CreatedAt,
		Type:      logType,
		Content:   content,
		Ip:        event.IP,
		RequestId: event.RequestID,
		Other:     common.MapToJsonStr(other),
	}, nil
}

func encodeAuditLogSnapshot(log *model.Log) (string, error) {
	if log == nil || log.EventId == nil || *log.EventId == "" {
		return "", ErrAuditEventInvalid
	}
	payload, err := common.Marshal(log)
	if err != nil {
		return "", fmt.Errorf("encode audit log snapshot: %w", err)
	}
	if len(payload) > maxAuditLogSnapshotLength {
		return "", errors.New("audit log snapshot is too large")
	}
	return string(payload), nil
}

func decodeAuditLogSnapshot(snapshot string, version int) (*model.Log, error) {
	if version != auditLogSnapshotVersion || snapshot == "" {
		return nil, errors.New("audit log snapshot is unavailable")
	}
	var log model.Log
	if err := common.UnmarshalJsonStr(snapshot, &log); err != nil {
		return nil, fmt.Errorf("decode audit log snapshot: %w", err)
	}
	if log.EventId == nil || *log.EventId == "" {
		return nil, errors.New("audit log snapshot has no event id")
	}
	return &log, nil
}

func createAuditLog(ctx context.Context, log *model.Log) error {
	if common.UsingLogDatabase(common.DatabaseTypeClickHouse) {
		return model.LOG_DB.WithContext(ctx).Create(log).Error
	}
	return createAuditLogWithDB(model.LOG_DB.WithContext(ctx), log)
}

func createAuditLogWithDB(db *gorm.DB, log *model.Log) error {
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}},
		DoNothing: true,
	}).Create(log).Error
}

func findCanonicalAuditLogWithDB(db *gorm.DB, eventID string) (model.Log, bool, error) {
	var log model.Log
	result := db.Where("event_id = ?", eventID).Limit(1).Find(&log)
	return log, result.RowsAffected == 1, result.Error
}

func sameAuditLog(expected *model.Log, actual *model.Log) bool {
	if expected == nil || actual == nil || expected.EventId == nil || actual.EventId == nil {
		return false
	}
	return *expected.EventId == *actual.EventId &&
		expected.UserId == actual.UserId &&
		expected.CreatedAt == actual.CreatedAt &&
		expected.Type == actual.Type &&
		expected.Content == actual.Content &&
		expected.Username == actual.Username &&
		expected.TokenName == actual.TokenName &&
		expected.ModelName == actual.ModelName &&
		expected.Quota == actual.Quota &&
		expected.PromptTokens == actual.PromptTokens &&
		expected.CompletionTokens == actual.CompletionTokens &&
		expected.UseTime == actual.UseTime &&
		expected.IsStream == actual.IsStream &&
		expected.ChannelId == actual.ChannelId &&
		expected.TokenId == actual.TokenId &&
		expected.Group == actual.Group &&
		expected.Ip == actual.Ip &&
		expected.RequestId == actual.RequestId &&
		expected.UpstreamRequestId == actual.UpstreamRequestId &&
		expected.Other == actual.Other
}

func projectDeliveredAuditLogWithTx(tx *gorm.DB, outbox *model.AuditOutboxEvent, log *model.Log) error {
	if tx == nil {
		return errors.New("audit outbox projection transaction is not initialized")
	}
	if !model.IPSecurityCollectionEnabled() {
		return nil
	}
	securityEvent, project := model.IPSecurityEventFromLog(log)
	if outbox.EventType == "user.register" && outbox.IP != "" {
		securityEvent = &model.IPSecurityEvent{
			EventID: outbox.EventID, UserID: log.UserId, Username: log.Username,
			CreatedAt: outbox.CreatedAt, IP: outbox.IP, Kind: model.IPSecurityKindRegistration,
			RequestID: outbox.RequestID,
		}
		var user model.User
		result := tx.Select("id", "username", "created_at", "registration_ip").Where("id = ?", log.UserId).Limit(1).Find(&user)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 1 {
			securityEvent.Username = user.Username
			if user.CreatedAt > 0 {
				securityEvent.CreatedAt = user.CreatedAt
			}
			if user.RegistrationIP != "" {
				securityEvent.IP = user.RegistrationIP
			}
		}
		project = true
	}
	if !project {
		return nil
	}
	return model.RecordIPSecurityEventWithTx(tx, securityEvent)
}

func markAuditOutboxFailure(ctx context.Context, event *model.AuditOutboxEvent, workerID string, deliveryErr error) error {
	now := auditOutboxNow()
	lastError := deliveryErr.Error()
	if len(lastError) > 2000 {
		lastError = lastError[:2000]
	}
	result := model.DB.WithContext(ctx).Model(&model.AuditOutboxEvent{}).
		Where("id = ? AND status = ? AND locked_by = ?", event.ID, model.AuditOutboxStatusPending, workerID).
		Updates(map[string]any{
			"last_error":      lastError,
			"next_attempt_at": now + auditOutboxBackoffSeconds(event.AttemptCount),
			"locked_by":       "",
			"locked_until":    0,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("audit outbox failure state was not updated")
	}
	return nil
}

func auditOutboxBackoffSeconds(attemptCount int) int64 {
	if attemptCount < 1 {
		attemptCount = 1
	}
	if attemptCount > 10 {
		attemptCount = 10
	}
	seconds := int64(5) << (attemptCount - 1)
	if seconds > 3600 {
		return 3600
	}
	return seconds
}

func markAuditOutboxDeliveredWithTx(tx *gorm.DB, id int64, workerID string) error {
	now := auditOutboxNow()
	result := tx.Model(&model.AuditOutboxEvent{}).
		Where("id = ? AND status = ? AND locked_by = ?", id, model.AuditOutboxStatusPending, workerID).
		Updates(map[string]any{
			"status":          model.AuditOutboxStatusDelivered,
			"delivered_at":    now,
			"last_error":      "",
			"next_attempt_at": 0,
			"locked_by":       "",
			"locked_until":    0,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("audit outbox delivery state was not updated")
	}
	return nil
}
