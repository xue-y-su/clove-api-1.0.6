package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/compliance_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

const complianceCacheTTL = 5 * time.Minute

type complianceCacheEntry struct {
	confirmed bool
	expiresAt time.Time
}

var complianceConfirmationCache sync.Map

func init() {
	go complianceCacheSweeper()
}

// complianceCacheSweeper periodically removes expired entries from
// complianceConfirmationCache so that a burst of unique users does not
// leave stale entries in memory indefinitely.
func complianceCacheSweeper() {
	ticker := time.NewTicker(complianceCacheTTL)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		complianceConfirmationCache.Range(func(key, value any) bool {
			if entry, ok := value.(complianceCacheEntry); ok {
				if now.After(entry.expiresAt) {
					complianceConfirmationCache.Delete(key)
				}
			}
			return true
		})
	}
}

func ClearComplianceConfirmationCache() {
	complianceConfirmationCache = sync.Map{}
}

func ClearComplianceConfirmationCacheFor(userId int, version string, contentHash string) {
	complianceConfirmationCache.Delete(complianceCacheKey(userId, version, contentHash))
}

func EnsureDashboardComplianceConfirmed(c *gin.Context) bool {
	if shouldSkipDashboardCompliance(c) {
		return true
	}
	return EnsureAuthenticatedUserComplianceConfirmed(c)
}

func EnsureAuthenticatedUserComplianceConfirmed(c *gin.Context) bool {
	if c.GetInt("role") == common.RoleRootUser {
		return true
	}
	settings := compliance_setting.GetSettings()
	if !settings.StatementEnabled {
		return true
	}
	if compliance_setting.ValidateSettings(settings) != nil {
		return true
	}
	if isUserComplianceConfirmed(c.GetInt("id"), settings) {
		return true
	}
	abortWithComplianceMessage(c)
	return false
}

func EnsureTokenComplianceConfirmed(c *gin.Context) bool {
	settings := compliance_setting.GetSettings()
	if !settings.StatementEnabled || !settings.TokenAPIEnforcementEnabled {
		return true
	}
	if compliance_setting.ValidateSettings(settings) != nil {
		return true
	}
	if isUserComplianceConfirmed(c.GetInt("id"), settings) {
		return true
	}
	abortWithOpenAiMessage(c, http.StatusForbidden, "请先确认合规声明", types.ErrorCodeComplianceConfirmationRequired)
	return false
}

func shouldSkipDashboardCompliance(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return true
	}
	path := c.Request.URL.Path
	if !strings.HasPrefix(path, "/api/") {
		return true
	}
	if strings.HasPrefix(path, "/api/compliance/") {
		return true
	}
	if strings.HasPrefix(path, "/api/source-attribution/") {
		return true
	}
	if strings.HasPrefix(path, "/api/registration/") {
		return true
	}
	switch {
	case path == "/api/user/self" && c.Request.Method == http.MethodGet:
		return true
	case path == "/api/user/logout":
		return true
	case strings.HasPrefix(path, "/api/user/2fa/"):
		return true
	case strings.HasPrefix(path, "/api/oauth/"):
		return true
	default:
		return false
	}
}

func isUserComplianceConfirmed(userId int, settings compliance_setting.Settings) bool {
	if userId <= 0 {
		return false
	}
	snapshots, err := model.EnsureComplianceStatementSnapshots(settings)
	if err != nil {
		common.SysLog(fmt.Sprintf("生成合规声明快照失败：%v", err))
		return false
	}
	key := complianceSnapshotsCacheKey(userId, snapshots)
	if cached, ok := complianceConfirmationCache.Load(key); ok {
		entry, ok := cached.(complianceCacheEntry)
		if ok && time.Now().Before(entry.expiresAt) {
			return entry.confirmed
		}
		complianceConfirmationCache.Delete(key)
	}
	confirmed, err := model.HasAllComplianceSnapshotConfirmations(userId, snapshots)
	if err != nil {
		common.SysLog(fmt.Sprintf("检查合规声明确认状态失败：%v", err))
		return false
	}
	if !confirmed {
		// Fall back to legacy confirmations (snapshot_id=0) for users who
		// confirmed before multi-snapshot support was introduced.
		allLegacyConfirmed := true
		for _, snapshot := range snapshots {
			legacyConfirmed, err := model.HasComplianceConfirmation(userId, snapshot.Version, snapshot.ContentHash)
			if err != nil {
				common.SysLog(fmt.Sprintf("检查旧版合规声明确认状态失败：%v", err))
				return false
			}
			if !legacyConfirmed {
				allLegacyConfirmed = false
				break
			}
		}
		if allLegacyConfirmed {
			confirmed = true
		}
	}
	complianceConfirmationCache.Store(key, complianceCacheEntry{
		confirmed: confirmed,
		expiresAt: time.Now().Add(complianceCacheTTL),
	})
	return confirmed
}

func complianceCacheKey(userId int, version string, contentHash string) string {
	return fmt.Sprintf("%d:%s:%s", userId, version, contentHash)
}

func complianceSnapshotsCacheKey(userId int, snapshots []model.ComplianceStatementSnapshot) string {
	parts := make([]string, 0, len(snapshots))
	for _, snapshot := range snapshots {
		parts = append(parts, fmt.Sprintf("%d:%s", snapshot.Id, snapshot.ContentHash))
	}
	return fmt.Sprintf("%d:%s", userId, strings.Join(parts, ","))
}

func abortWithComplianceMessage(c *gin.Context) {
	c.JSON(http.StatusForbidden, gin.H{
		"success": false,
		"message": "请先确认合规声明",
		"code":    string(types.ErrorCodeComplianceConfirmationRequired),
	})
	c.Abort()
}
