package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/setting/ip_security_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	IPSecurityKindRegistration   = "registration"
	IPSecurityKindAPIRequest     = "api_request"
	IPSecurityKindAPIError       = "api_error"
	IPSecurityKindLoginSuccess   = "login_success"
	IPSecurityKindLoginFailure   = "login_failure"
	IPSecurityKindLogout         = "logout"
	IPSecurityKindActivityPrefix = "activity_"

	IPSecurityProjectionStatusPending = "pending"
	IPSecurityProjectionStatusRunning = "running"
	IPSecurityProjectionStatusReady   = "ready"
	IPSecurityProjectionStatusFailed  = "failed"

	IPSecurityEventProjectionPending   = "pending"
	IPSecurityEventProjectionProjected = "projected"
)

var (
	ErrInvalidIPSecurityEvent  = errors.New("invalid IP security event")
	ErrIPSecurityEventConflict = errors.New("IP security event ID conflicts with an existing event")
)

func IPSecurityCollectionEnabled() bool {
	return ip_security_setting.GetSettings().Enabled
}

type IPSecurityEvent struct {
	ID               int64  `json:"id" gorm:"primaryKey;autoIncrement;index:idx_ip_security_events_projection_pending,priority:3;index:idx_ip_security_events_ip_timeline,priority:4;index:idx_ip_security_events_user_timeline,priority:4"`
	EventID          string `json:"event_id" gorm:"type:varchar(191);not null;uniqueIndex"`
	UserID           int    `json:"user_id" gorm:"not null;default:0;index:idx_ip_security_events_user_created,priority:1;index:idx_ip_security_events_user_timeline,priority:1"`
	Username         string `json:"username" gorm:"type:varchar(255);not null;default:''"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;not null;index:idx_ip_security_events_created;index:idx_ip_security_events_user_created,priority:2;index:idx_ip_security_events_ip_created,priority:2;index:idx_ip_security_events_masked_created,priority:2;index:idx_ip_security_events_ip_timeline,priority:2;index:idx_ip_security_events_user_timeline,priority:2"`
	IP               string `json:"ip" gorm:"type:varchar(64);not null;index:idx_ip_security_events_ip_created,priority:1;index:idx_ip_security_events_ip_timeline,priority:1"`
	MaskedIP         string `json:"masked_ip" gorm:"type:varchar(64);not null;index:idx_ip_security_events_masked_created,priority:1"`
	Kind             string `json:"kind" gorm:"type:varchar(64);not null;index"`
	Quota            int    `json:"quota" gorm:"not null;default:0"`
	HTTPStatus       int    `json:"http_status" gorm:"not null;default:0;index"`
	RequestID        string `json:"request_id" gorm:"type:varchar(64);not null;default:'';index:idx_ip_security_events_ip_timeline,priority:3;index:idx_ip_security_events_user_timeline,priority:3"`
	ProjectionStatus string `json:"-" gorm:"type:varchar(16);not null;default:'projected';index:idx_ip_security_events_projection_pending,priority:1"`
	ProjectionError  string `json:"-" gorm:"type:varchar(2000);not null;default:''"`
	AttemptCount     int    `json:"-" gorm:"not null;default:0"`
	NextAttemptAt    int64  `json:"-" gorm:"bigint;not null;default:0;index:idx_ip_security_events_projection_pending,priority:2"`
	ProjectedAt      int64  `json:"-" gorm:"bigint;not null;default:0"`
}

type IPAuditRollup struct {
	ID                int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	Bucket            int64  `json:"bucket" gorm:"bigint;not null;uniqueIndex:idx_ip_audit_rollup_identity,priority:1;index:idx_ip_audit_rollup_bucket_ip,priority:1;index:idx_ip_audit_rollup_bucket_masked,priority:1;index:idx_ip_audit_rollup_ip_bucket_user,priority:2"`
	UserID            int    `json:"user_id" gorm:"not null;default:0;uniqueIndex:idx_ip_audit_rollup_identity,priority:2;index;index:idx_ip_audit_rollup_ip_bucket_user,priority:3"`
	Username          string `json:"username" gorm:"type:varchar(255);not null;default:''"`
	IP                string `json:"ip" gorm:"type:varchar(64);not null;uniqueIndex:idx_ip_audit_rollup_identity,priority:3;index:idx_ip_audit_rollup_bucket_ip,priority:2;index:idx_ip_audit_rollup_ip_bucket_user,priority:1"`
	MaskedIP          string `json:"masked_ip" gorm:"type:varchar(64);not null;index:idx_ip_audit_rollup_bucket_masked,priority:2"`
	RequestCount      int    `json:"request_count" gorm:"not null;default:0"`
	RegistrationCount int    `json:"registration_count" gorm:"not null;default:0"`
	APIRequestCount   int    `json:"api_request_count" gorm:"not null;default:0"`
	ErrorCount        int    `json:"error_count" gorm:"not null;default:0"`
	UnauthorizedCount int    `json:"unauthorized_count" gorm:"not null;default:0"`
	LoginSuccessCount int    `json:"login_success_count" gorm:"not null;default:0"`
	LoginFailureCount int    `json:"login_failure_count" gorm:"not null;default:0"`
	LogoutCount       int    `json:"logout_count" gorm:"not null;default:0"`
	AnomalyCount      int    `json:"anomaly_count" gorm:"not null;default:0"`
	Quota             int    `json:"quota" gorm:"not null;default:0"`
	FirstSeenAt       int64  `json:"first_seen_at" gorm:"bigint;not null;default:0"`
	LastSeenAt        int64  `json:"last_seen_at" gorm:"bigint;not null;default:0"`
}

type IPSecurityProjectionState struct {
	ID                   int    `json:"id" gorm:"primaryKey;autoIncrement:false"`
	Status               string `json:"status" gorm:"type:varchar(16);not null;default:'pending'"`
	ProcessedLogs        int64  `json:"processed_logs" gorm:"bigint;not null;default:0"`
	ProcessedUsers       int64  `json:"processed_users" gorm:"bigint;not null;default:0"`
	LastUserID           int    `json:"-" gorm:"not null;default:0"`
	LastLogID            int    `json:"-" gorm:"not null;default:0"`
	LastLogCreatedAt     int64  `json:"-" gorm:"bigint;not null;default:0"`
	LastLogRequestID     string `json:"-" gorm:"type:text"`
	LastLogEventIdentity string `json:"-" gorm:"type:text"`
	LogSourceType        string `json:"-" gorm:"type:varchar(16);not null;default:''"`
	LogSourceGeneration  string `json:"-" gorm:"type:varchar(128);not null;default:''"`
	LockedBy             string `json:"-" gorm:"type:varchar(128);not null;default:''"`
	LockedUntil          int64  `json:"-" gorm:"bigint;not null;default:0;index"`
	LeaseVersion         int64  `json:"-" gorm:"bigint;not null;default:0"`
	StartedAt            int64  `json:"started_at" gorm:"bigint;not null;default:0"`
	CompletedAt          int64  `json:"completed_at" gorm:"bigint;not null;default:0"`
	UpdatedAt            int64  `json:"updated_at" gorm:"bigint;not null;autoUpdateTime"`
	LastError            string `json:"last_error" gorm:"type:text;not null"`
}

func IPSecurityKindFromLog(log *Log) (string, bool) {
	if log == nil {
		return "", false
	}
	switch log.Type {
	case LogTypeConsume:
		return IPSecurityKindAPIRequest, true
	case LogTypeError:
		return IPSecurityKindAPIError, true
	case LogTypeLogin:
		switch log.Content {
		case "user.login_failed":
			return IPSecurityKindLoginFailure, true
		case "user.logout":
			return IPSecurityKindLogout, true
		case "user.register":
			return "", false
		default:
			return IPSecurityKindLoginSuccess, true
		}
	default:
		return "", false
	}
}

func IPSecurityEventFromLog(log *Log) (*IPSecurityEvent, bool) {
	kind, project := IPSecurityKindFromLog(log)
	if !project {
		return nil, false
	}
	httpStatus := log.IPSecurityHTTPStatus()
	if kind == IPSecurityKindAPIError && httpStatus != http.StatusUnauthorized {
		return nil, false
	}
	ip := NormalizeAuditIP(log.Ip)
	if ip == "" {
		return nil, false
	}
	eventID := ""
	if log.EventId != nil {
		eventID = strings.TrimSpace(*log.EventId)
	}
	if eventID == "" {
		eventID = deterministicIPSecurityLogEventID(log, ip)
	}
	return &IPSecurityEvent{
		EventID:    eventID,
		UserID:     log.UserId,
		Username:   strings.TrimSpace(log.Username),
		CreatedAt:  log.CreatedAt,
		IP:         ip,
		MaskedIP:   MaskAuditIP(ip),
		Kind:       kind,
		Quota:      log.Quota,
		HTTPStatus: httpStatus,
		RequestID:  strings.TrimSpace(log.RequestId),
	}, true
}

func deterministicIPSecurityLogEventID(log *Log, normalizedIP string) string {
	requestID := strings.TrimSpace(log.RequestId)
	if requestID != "" {
		return hashIPSecurityEventIdentity(
			"request",
			strconv.Itoa(log.Type),
			strconv.Itoa(log.UserId),
			requestID,
		)
	}
	return hashIPSecurityEventIdentity(
		"fallback",
		strconv.Itoa(log.Type),
		strconv.Itoa(log.UserId),
		strconv.FormatInt(log.CreatedAt, 10),
		normalizedIP,
		strconv.Itoa(log.Id),
		strconv.Itoa(log.ChannelId),
		strconv.Itoa(log.TokenId),
		strconv.Itoa(log.Quota),
		strconv.Itoa(log.PromptTokens),
		strconv.Itoa(log.CompletionTokens),
		strconv.Itoa(log.UseTime),
		strconv.FormatBool(log.IsStream),
		strings.TrimSpace(log.UpstreamRequestId),
	)
}

func hashIPSecurityEventIdentity(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = hash.Write([]byte(strconv.Itoa(len(part))))
		_, _ = hash.Write([]byte{':'})
		_, _ = hash.Write([]byte(part))
	}
	return "log:" + hex.EncodeToString(hash.Sum(nil))
}

func RecordIPSecurityEventWithTx(tx *gorm.DB, event *IPSecurityEvent) error {
	if tx == nil || event == nil {
		return ErrInvalidIPSecurityEvent
	}
	if !IPSecurityCollectionEnabled() {
		return nil
	}
	if event.Kind == IPSecurityKindAPIError && event.HTTPStatus != http.StatusUnauthorized {
		return nil
	}
	record := *event
	record.EventID = strings.TrimSpace(record.EventID)
	record.Username = strings.TrimSpace(record.Username)
	record.IP = NormalizeAuditIP(record.IP)
	record.MaskedIP = MaskAuditIP(record.IP)
	record.Kind = strings.TrimSpace(record.Kind)
	record.RequestID = strings.TrimSpace(record.RequestID)
	record.ProjectionStatus = IPSecurityEventProjectionPending
	record.ProjectionError = ""
	record.AttemptCount = 0
	record.NextAttemptAt = 0
	record.ProjectedAt = 0
	if record.EventID == "" || len(record.EventID) > 191 || len(record.Username) > 255 || record.CreatedAt <= 0 || record.IP == "" || len(record.IP) > 64 || record.Kind == "" || len(record.Kind) > 64 || len(record.RequestID) > 64 {
		return ErrInvalidIPSecurityEvent
	}

	return tx.Transaction(func(inner *gorm.DB) error {
		result := inner.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "event_id"}},
			DoNothing: true,
		}).Create(&record)
		if result.Error != nil {
			return result.Error
		}
		var canonical IPSecurityEvent
		if err := lockForUpdate(inner).
			Where("event_id = ?", record.EventID).First(&canonical).Error; err != nil {
			return err
		}
		if !matchingIPSecurityEvent(canonical, record) {
			return ErrIPSecurityEventConflict
		}
		record = canonical
		shouldAttemptProjection := record.NextAttemptAt <= time.Now().Unix()
		if record.ProjectionStatus == IPSecurityEventProjectionPending && shouldAttemptProjection {
			if _, err := attemptIPSecurityEventProjectionWithTx(inner, &record); err != nil {
				return err
			}
		}
		*event = record
		return nil
	})
}

func attemptIPSecurityEventProjectionWithTx(tx *gorm.DB, event *IPSecurityEvent) (bool, error) {
	if event.ProjectionStatus != IPSecurityEventProjectionPending {
		return false, nil
	}
	projectionErr := tx.Transaction(func(projectionTx *gorm.DB) error {
		for _, bucket := range []int64{0, utcDayBucket(event.CreatedAt)} {
			if err := incrementIPAuditRollup(projectionTx, event, bucket); err != nil {
				return err
			}
		}
		projectedAt := time.Now().Unix()
		result := projectionTx.Model(&IPSecurityEvent{}).
			Where("id = ? AND projection_status = ?", event.ID, IPSecurityEventProjectionPending).
			Updates(map[string]interface{}{
				"projection_status": IPSecurityEventProjectionProjected,
				"projection_error":  "",
				"attempt_count":     0,
				"next_attempt_at":   0,
				"projected_at":      projectedAt,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("IP security event projection state was not updated")
		}
		event.ProjectionStatus = IPSecurityEventProjectionProjected
		event.ProjectionError = ""
		event.AttemptCount = 0
		event.NextAttemptAt = 0
		event.ProjectedAt = projectedAt
		return nil
	})
	if projectionErr == nil {
		return true, nil
	}

	message := strings.TrimSpace(projectionErr.Error())
	if len(message) > 2000 {
		message = message[:2000]
	}
	attemptCount := event.AttemptCount + 1
	nextAttemptAt := time.Now().Unix() + ipSecurityProjectionBackoffSeconds(attemptCount)
	result := tx.Session(&gorm.Session{NewDB: true}).Model(&IPSecurityEvent{}).
		Where("id = ? AND projection_status = ?", event.ID, IPSecurityEventProjectionPending).
		Updates(map[string]interface{}{
			"projection_error": message,
			"attempt_count":    attemptCount,
			"next_attempt_at":  nextAttemptAt,
		})
	if result.Error != nil {
		return false, errors.Join(projectionErr, result.Error)
	}
	if result.RowsAffected != 1 {
		return false, errors.Join(projectionErr, errors.New("IP security event projection failure state was not updated"))
	}
	event.ProjectionError = message
	event.AttemptCount = attemptCount
	event.NextAttemptAt = nextAttemptAt
	return false, nil
}

func ipSecurityProjectionBackoffSeconds(attemptCount int) int64 {
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

func ProjectPendingIPSecurityEvents(ctx context.Context, limit int) (projected, failed int, err error) {
	if !IPSecurityCollectionEnabled() {
		return 0, 0, nil
	}
	if DB == nil {
		return 0, 0, errors.New("IP security projection database is not initialized")
	}
	if limit < 1 {
		limit = 100
	}
	if limit > 5000 {
		limit = 5000
	}
	var ids []int64
	now := time.Now().Unix()
	if err := DB.WithContext(ctx).Model(&IPSecurityEvent{}).
		Where("projection_status = ? AND next_attempt_at <= ?", IPSecurityEventProjectionPending, now).
		Order("id asc").Limit(limit).Pluck("id", &ids).Error; err != nil {
		return 0, 0, err
	}
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return projected, failed, err
		}
		attempted := false
		attemptFailed := false
		err := retryIPSecurityProjectionTransaction(ctx, func() error {
			return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				var event IPSecurityEvent
				if err := lockForUpdate(tx).First(&event, id).Error; err != nil {
					return err
				}
				if event.ProjectionStatus != IPSecurityEventProjectionPending || event.NextAttemptAt > time.Now().Unix() {
					return nil
				}
				attempted = true
				wasProjected, err := attemptIPSecurityEventProjectionWithTx(tx, &event)
				if err != nil {
					return err
				}
				attemptFailed = !wasProjected
				return nil
			})
		})
		if err != nil {
			return projected, failed, err
		}
		if !attempted {
			continue
		}
		if attemptFailed {
			failed++
		} else {
			projected++
		}
	}
	return projected, failed, nil
}

func matchingIPSecurityEvent(existing IPSecurityEvent, candidate IPSecurityEvent) bool {
	immutableFieldsMatch := existing.EventID == candidate.EventID &&
		existing.UserID == candidate.UserID &&
		existing.CreatedAt == candidate.CreatedAt &&
		existing.IP == candidate.IP &&
		existing.MaskedIP == candidate.MaskedIP &&
		existing.Kind == candidate.Kind &&
		existing.Quota == candidate.Quota &&
		existing.HTTPStatus == candidate.HTTPStatus
	if !immutableFieldsMatch {
		return false
	}
	return existing.Kind == IPSecurityKindRegistration || existing.RequestID == candidate.RequestID
}

func utcDayBucket(timestamp int64) int64 {
	return timestamp - timestamp%86400
}

func incrementIPAuditRollup(tx *gorm.DB, event *IPSecurityEvent, bucket int64) error {
	rollup := IPAuditRollup{
		Bucket:      bucket,
		UserID:      event.UserID,
		Username:    event.Username,
		IP:          event.IP,
		MaskedIP:    event.MaskedIP,
		Quota:       event.Quota,
		FirstSeenAt: event.CreatedAt,
		LastSeenAt:  event.CreatedAt,
	}
	if event.Kind != IPSecurityKindRegistration {
		rollup.RequestCount = 1
	}
	switch event.Kind {
	case IPSecurityKindRegistration:
		rollup.RegistrationCount = 1
	case IPSecurityKindAPIRequest:
		rollup.APIRequestCount = 1
	case IPSecurityKindAPIError:
		rollup.ErrorCount = 1
		rollup.UnauthorizedCount = 1
		rollup.AnomalyCount = 1
	case IPSecurityKindLoginSuccess:
		rollup.LoginSuccessCount = 1
	case IPSecurityKindLoginFailure:
		rollup.LoginFailureCount = 1
		rollup.AnomalyCount = 1
	case IPSecurityKindLogout:
		rollup.LogoutCount = 1
	}

	currentColumn := func(name string) clause.Column {
		return clause.Column{Table: clause.CurrentTable, Name: name}
	}
	updates := map[string]interface{}{
		"username":            gorm.Expr("CASE WHEN ? <= ? THEN ? ELSE ? END", currentColumn("last_seen_at"), event.CreatedAt, event.Username, currentColumn("username")),
		"masked_ip":           event.MaskedIP,
		"request_count":       gorm.Expr("? + ?", currentColumn("request_count"), rollup.RequestCount),
		"registration_count":  gorm.Expr("? + ?", currentColumn("registration_count"), rollup.RegistrationCount),
		"api_request_count":   gorm.Expr("? + ?", currentColumn("api_request_count"), rollup.APIRequestCount),
		"error_count":         gorm.Expr("? + ?", currentColumn("error_count"), rollup.ErrorCount),
		"unauthorized_count":  gorm.Expr("? + ?", currentColumn("unauthorized_count"), rollup.UnauthorizedCount),
		"login_success_count": gorm.Expr("? + ?", currentColumn("login_success_count"), rollup.LoginSuccessCount),
		"login_failure_count": gorm.Expr("? + ?", currentColumn("login_failure_count"), rollup.LoginFailureCount),
		"logout_count":        gorm.Expr("? + ?", currentColumn("logout_count"), rollup.LogoutCount),
		"anomaly_count":       gorm.Expr("? + ?", currentColumn("anomaly_count"), rollup.AnomalyCount),
		"quota":               gorm.Expr("? + ?", currentColumn("quota"), rollup.Quota),
		"first_seen_at":       gorm.Expr("CASE WHEN ? = 0 OR ? > ? THEN ? ELSE ? END", currentColumn("first_seen_at"), currentColumn("first_seen_at"), event.CreatedAt, event.CreatedAt, currentColumn("first_seen_at")),
		"last_seen_at":        gorm.Expr("CASE WHEN ? < ? THEN ? ELSE ? END", currentColumn("last_seen_at"), event.CreatedAt, event.CreatedAt, currentColumn("last_seen_at")),
	}
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "bucket"}, {Name: "user_id"}, {Name: "ip"}},
		DoUpdates: clause.Assignments(updates),
	}).Create(&rollup).Error
}
