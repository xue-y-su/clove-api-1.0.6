package ip_security_setting

import (
	"errors"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

const (
	EnabledKey                         = "ip_security.enabled"
	RetentionDaysKey                   = "ip_security.retention_days"
	ProjectionIntervalSecondsKey       = "ip_security.projection_interval_seconds"
	ProjectionBatchSizeKey             = "ip_security.projection_batch_size"
	SharedRegistrationUserThresholdKey = "ip_security.shared_registration_user_threshold"
	SharedAPIUserThresholdKey          = "ip_security.shared_api_user_threshold"
	UnauthorizedThresholdKey           = "ip_security.unauthorized_threshold"
)

type Settings struct {
	Enabled                         bool `json:"enabled"`
	RetentionDays                   int  `json:"retention_days"`
	ProjectionIntervalSeconds       int  `json:"projection_interval_seconds"`
	ProjectionBatchSize             int  `json:"projection_batch_size"`
	SharedRegistrationUserThreshold int  `json:"shared_registration_user_threshold"`
	SharedAPIUserThreshold          int  `json:"shared_api_user_threshold"`
	UnauthorizedThreshold           int  `json:"unauthorized_threshold"`
}

func DefaultOptions() map[string]string {
	projectionIntervalSeconds := common.GetEnvOrDefault("IP_SECURITY_PROJECTION_RETRY_SECONDS", 60)
	if projectionIntervalSeconds < 5 {
		projectionIntervalSeconds = 5
	}
	if projectionIntervalSeconds > 3600 {
		projectionIntervalSeconds = 3600
	}
	projectionBatchSize := common.GetEnvOrDefault("IP_SECURITY_PROJECTION_BATCH_SIZE", 500)
	if projectionBatchSize < 10 {
		projectionBatchSize = 10
	}
	if projectionBatchSize > 5000 {
		projectionBatchSize = 5000
	}
	return map[string]string{
		EnabledKey:                         "true",
		RetentionDaysKey:                   "90",
		ProjectionIntervalSecondsKey:       strconv.Itoa(projectionIntervalSeconds),
		ProjectionBatchSizeKey:             strconv.Itoa(projectionBatchSize),
		SharedRegistrationUserThresholdKey: "2",
		SharedAPIUserThresholdKey:          "2",
		UnauthorizedThresholdKey:           "10",
	}
}

func GetSettings() Settings {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return Settings{
		Enabled:                         parseBoolWithDefault(common.OptionMap[EnabledKey], true),
		RetentionDays:                   parseIntWithDefault(common.OptionMap[RetentionDaysKey], 90, 0, 3650),
		ProjectionIntervalSeconds:       parseIntWithDefault(common.OptionMap[ProjectionIntervalSecondsKey], 60, 5, 3600),
		ProjectionBatchSize:             parseIntWithDefault(common.OptionMap[ProjectionBatchSizeKey], 500, 10, 5000),
		SharedRegistrationUserThreshold: parseIntWithDefault(common.OptionMap[SharedRegistrationUserThresholdKey], 2, 1, 100000),
		SharedAPIUserThreshold:          parseIntWithDefault(common.OptionMap[SharedAPIUserThresholdKey], 2, 1, 100000),
		UnauthorizedThreshold:           parseIntWithDefault(common.OptionMap[UnauthorizedThresholdKey], 10, 1, 100000),
	}
}

func ValidateSettings(settings Settings) error {
	if settings.RetentionDays < 0 || settings.RetentionDays > 3650 {
		return errors.New("IP security retention must be between 0 and 3650 days")
	}
	if settings.ProjectionIntervalSeconds < 5 || settings.ProjectionIntervalSeconds > 3600 {
		return errors.New("IP security projection interval must be between 5 and 3600 seconds")
	}
	if settings.ProjectionBatchSize < 10 || settings.ProjectionBatchSize > 5000 {
		return errors.New("IP security projection batch size must be between 10 and 5000")
	}
	if settings.SharedRegistrationUserThreshold < 1 || settings.SharedRegistrationUserThreshold > 100000 {
		return errors.New("IP security shared registration threshold must be between 1 and 100000")
	}
	if settings.SharedAPIUserThreshold < 1 || settings.SharedAPIUserThreshold > 100000 {
		return errors.New("IP security shared API threshold must be between 1 and 100000")
	}
	if settings.UnauthorizedThreshold < 1 || settings.UnauthorizedThreshold > 100000 {
		return errors.New("IP security unauthorized threshold must be between 1 and 100000")
	}
	return nil
}

func SettingsToOptions(settings Settings) map[string]string {
	return map[string]string{
		EnabledKey:                         strconv.FormatBool(settings.Enabled),
		RetentionDaysKey:                   strconv.Itoa(settings.RetentionDays),
		ProjectionIntervalSecondsKey:       strconv.Itoa(settings.ProjectionIntervalSeconds),
		ProjectionBatchSizeKey:             strconv.Itoa(settings.ProjectionBatchSize),
		SharedRegistrationUserThresholdKey: strconv.Itoa(settings.SharedRegistrationUserThreshold),
		SharedAPIUserThresholdKey:          strconv.Itoa(settings.SharedAPIUserThreshold),
		UnauthorizedThresholdKey:           strconv.Itoa(settings.UnauthorizedThreshold),
	}
}

func parseBoolWithDefault(value string, fallback bool) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseIntWithDefault(value string, fallback, minimum, maximum int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < minimum || parsed > maximum {
		return fallback
	}
	return parsed
}
