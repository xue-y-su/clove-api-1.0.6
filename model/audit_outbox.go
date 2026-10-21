package model

const (
	AuditOutboxStatusPending   = "pending"
	AuditOutboxStatusDelivered = "delivered"
)

type AuditOutboxEvent struct {
	ID                 int64  `json:"id" gorm:"primaryKey;autoIncrement;index:idx_audit_outbox_delivered_retention,priority:3"`
	EventID            string `json:"event_id" gorm:"type:varchar(191);not null;uniqueIndex"`
	EventType          string `json:"event_type" gorm:"type:varchar(128);not null;index"`
	Result             string `json:"result" gorm:"type:varchar(32);not null;default:''"`
	ActorID            int    `json:"actor_id" gorm:"not null;default:0;index"`
	LogUserID          int    `json:"log_user_id" gorm:"not null;default:0;index"`
	Target             string `json:"target" gorm:"type:varchar(255);not null;default:'';index"`
	IP                 string `json:"ip" gorm:"type:varchar(64);not null;default:''"`
	RequestID          string `json:"request_id" gorm:"type:varchar(64);not null;default:'';index"`
	Payload            string `json:"payload" gorm:"type:text;not null"`
	Status             string `json:"status" gorm:"type:varchar(32);not null;index:idx_audit_outbox_ready,priority:1;index:idx_audit_outbox_delivered_retention,priority:1"`
	AttemptCount       int    `json:"attempt_count" gorm:"not null;default:0"`
	NextAttemptAt      int64  `json:"next_attempt_at" gorm:"bigint;not null;default:0;index:idx_audit_outbox_ready,priority:2"`
	LastError          string `json:"last_error" gorm:"type:text;not null"`
	LockedBy           string `json:"locked_by" gorm:"type:varchar(128);not null;default:''"`
	LockedUntil        int64  `json:"locked_until" gorm:"bigint;not null;default:0;index"`
	LogPreparedAt      int64  `json:"log_prepared_at" gorm:"bigint;not null;default:0"`
	LogUsername        string `json:"log_username" gorm:"type:varchar(191);not null;default:''"`
	LogSnapshotVersion int    `json:"log_snapshot_version" gorm:"not null;default:0"`
	LogSnapshot        string `json:"log_snapshot" gorm:"type:text"`
	LogPersistedAt     int64  `json:"log_persisted_at" gorm:"bigint;not null;default:0"`
	CreatedAt          int64  `json:"created_at" gorm:"bigint;autoCreateTime;index"`
	UpdatedAt          int64  `json:"updated_at" gorm:"bigint;autoUpdateTime"`
	DeliveredAt        int64  `json:"delivered_at" gorm:"bigint;not null;default:0;index:idx_audit_outbox_delivered_retention,priority:2"`
}
