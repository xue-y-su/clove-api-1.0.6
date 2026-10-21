# 安全数据完整性 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 修正安全与 IP 审计、API Key 和用户兑换码的事实模型、状态机与管理界面，保证三种数据库上的权限、并发和资金行为一致。

**Architecture:** 保持 `Router -> Controller -> Service -> Model` 分层。IP 列表只读 `IPAuditRollup`，原始事件仅用于展开详情；Key 删除与泄露使用不同的不可逆指纹策略；用户兑换码以显式资金终态替代到期自动退款。Default 与 Classic 使用同一 API 契约，不复制业务判断。

**Tech Stack:** Go 1.22、Gin、GORM v2、SQLite/MySQL/PostgreSQL、React 19 + TypeScript + Bun、React 18 + JavaScript + Bun、TanStack Query、Base UI、Semi UI。

---

## 文件职责

- `setting/ip_security_setting/config.go`：三类风险阈值的配置、默认值与校验。
- `model/ip_security_event.go`：原始安全事实、HTTP 状态、日/全量 rollup 计数。
- `model/ip_security_query.go`：rollup 列表、用户摘要、详情分页和风险排序。
- `service/ip_audit.go`、`service/ip_security_user_projection.go`：权限、视图契约、风险原因与页面 DTO。
- `controller/log.go`、`router/api-router.go`：审计查询参数、详情路由和稳定错误响应。
- `model/token.go`、`model/token_revocation.go`、`service/token_secret.go`：Key 墓碑、泄露撤销指纹和认证拒绝。
- `controller/token.go`：`group_only` 窄更新、自定义 Key 错误映射和审计。
- `model/user_redemption.go`、`service/user_redemption.go`、`service/user_redemption_governance.go`：兑换码状态机、资金处置和批量治理。
- `controller/user_redemption.go`、`router/api-router.go`：兑换码领取、管理和批量处置 API。
- `web/default/src/features/usage-logs`、`web/classic/src/components/table/usage-logs`：IP 审计菜单、风险详情、日志跳转和百度外链。
- `web/default/src/features/keys`、`web/classic/src/components/table/tokens`：Key 状态与分组快捷修改。
- `web/default/src/features/user-redemptions`、`web/classic/src/components/user-redemptions`：状态、复制、详情和管理员批量处置。

### Task 1: 记录 401 安全事实与阈值

**Files:**
- Modify: `setting/ip_security_setting/config.go`
- Modify: `setting/ip_security_setting/config_test.go`
- Modify: `model/ip_security_event.go`
- Modify: `model/ip_security_event_test.go`
- Modify: `model/log.go`
- Modify: `model/main.go`

- [x] **Step 1: Write the failing tests**

在设置测试中断言默认阈值为 `3/5/10`，在事件测试中分别投影 `401`、`429` 和 `503`，只允许 `401` 增加 `UnauthorizedCount`：

```go
func TestIPSecurityProjectionCountsOnlyUnauthorizedFailures(t *testing.T) {
	setupIPSecurityEventTestDB(t)
	for index, status := range []int{401, 429, 503} {
		event := IPSecurityEvent{
			EventID: fmt.Sprintf("http-%d", status), UserID: 7,
			CreatedAt: 1_780_000_000 + int64(index), IP: "203.0.113.7",
			Kind: IPSecurityKindAPIError, HTTPStatus: status,
		}
		require.NoError(t, RecordIPSecurityEventWithTx(DB, &event))
	}
	var rollup IPAuditRollup
	require.NoError(t, DB.Where("bucket = 0 AND user_id = ?", 7).First(&rollup).Error)
	require.Equal(t, 1, rollup.UnauthorizedCount)
	require.Equal(t, 3, rollup.ErrorCount)
}
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./setting/ip_security_setting ./model -run 'Test(IPSecuritySettings|IPSecurityProjectionCountsOnlyUnauthorizedFailures)' -count=1`

Expected: FAIL because `HTTPStatus`, `UnauthorizedCount` and the threshold settings do not exist.

- [x] **Step 3: Write the minimal implementation**

Add portable integer columns and settings; derive the final status from the persisted log metadata rather than from an upstream message string:

```go
type IPSecurityEvent struct {
	// existing fields...
	HTTPStatus int `json:"http_status" gorm:"not null;default:0;index"`
}

type IPAuditRollup struct {
	// existing fields...
	UnauthorizedCount int `json:"unauthorized_count" gorm:"not null;default:0"`
}

type Settings struct {
	Enabled                           bool `json:"enabled"`
	RetentionDays                     int  `json:"retention_days"`
	ProjectionIntervalSeconds         int  `json:"projection_interval_seconds"`
	ProjectionBatchSize               int  `json:"projection_batch_size"`
	SharedRegistrationUserThreshold   int  `json:"shared_registration_user_threshold"`
	SharedAPIUserThreshold            int  `json:"shared_api_user_threshold"`
	UnauthorizedThreshold             int  `json:"unauthorized_threshold"`
}
```

In `incrementIPAuditRollup`, set `UnauthorizedCount = 1` only when `event.Kind == IPSecurityKindAPIError && event.HTTPStatus == http.StatusUnauthorized`; retain `ErrorCount` for operational history but never use it as a security signal. Include `HTTPStatus` in immutable event conflict checks and both columns in `AutoMigrate` through their existing models.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./setting/ip_security_setting ./model -run 'Test(IPSecuritySettings|IPSecurityProjectionCountsOnlyUnauthorizedFailures|IPSecurityEvent)' -count=1`

Expected: PASS; `429` and `503` remain ordinary errors and contribute zero unauthorized risk.

- [x] **Step 5: Commit**

```bash
git add setting/ip_security_setting model/ip_security_event.go model/ip_security_event_test.go model/log.go model/main.go
git commit -m "fix(ip-audit): project unauthorized status separately"
```

### Task 2: 用 rollup 生成可解释风险

**Files:**
- Modify: `model/ip_security_query.go`
- Modify: `model/ip_security_query_test.go`
- Modify: `service/ip_audit.go`
- Modify: `service/ip_audit_test.go`
- Modify: `service/ip_security_user_projection.go`
- Modify: `service/ip_security_user_projection_test.go`

- [x] **Step 1: Write the failing tests**

构造四个 IP：共享注册达到 3 人、共享调用达到 5 人、10 分钟内 10 次 `401`、只有 `5xx`。断言前三者风险置顶且原因完整，最后一个不进入风险视图；用户摘要查询不得读取原始事件。

```go
require.Equal(t, []string{
	"shared_registration_ip",
	"shared_api_ip",
	"unauthorized_burst",
}, page.Items[0].RiskReasons)
require.Equal(t, 0, fiveXXOnly.RiskLevel)
require.Equal(t, 10, page.Items[0].UnauthorizedCount)
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./model ./service -run 'Test.*IPAudit.*(Risk|Rollup|Unauthorized|Summary)' -count=1`

Expected: FAIL because current queries use `ErrorCount/AnomalyCount` and do not expose threshold-specific reasons.

- [x] **Step 3: Write the minimal implementation**

Extend `ipAuditProjectionRow`, `IPAuditItem`, user summary DTOs and aggregate selects with `unauthorized_count`. Compute risk in `ipAuditProjectionItem` from the configured thresholds and fixed windows, and sort with a stable order:

```go
item.RiskReasons = make([]string, 0, 3)
if row.RegistrationUserCount >= int64(settings.SharedRegistrationUserThreshold) {
	item.RiskReasons = append(item.RiskReasons, "shared_registration_ip")
}
if row.APIUserCount >= int64(settings.SharedAPIUserThreshold) {
	item.RiskReasons = append(item.RiskReasons, "shared_api_ip")
}
if row.UnauthorizedCount >= int64(settings.UnauthorizedThreshold) {
	item.RiskReasons = append(item.RiskReasons, "unauthorized_burst")
}
item.RiskLevel = len(item.RiskReasons)
```

Keep list and user-summary SQL rooted at `ip_audit_rollups`; only `QueryIPAuditProjectionDetailPage` may read `ip_security_events`. Return `total`, `page`, `page_size`, `has_more`, separate account/request counters, and preserve Root versus administrator IP projection.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./model ./service -run 'Test.*IP(Audit|Security).*(Risk|Rollup|Unauthorized|Projection|Pagination)' -count=1`

Expected: PASS with deterministic risk-first ordering and no risk from `429` or `5xx`.

- [x] **Step 5: Commit**

```bash
git add model/ip_security_query.go model/ip_security_query_test.go service/ip_audit.go service/ip_audit_test.go service/ip_security_user_projection.go service/ip_security_user_projection_test.go
git commit -m "fix(ip-audit): rank explainable rollup risks"
```

### Task 3: 完成 IP 审计交互

**Files:**
- Modify: `controller/log.go`
- Modify: `router/api-router.go`
- Modify: `web/default/src/features/usage-logs/api.ts`
- Modify: `web/default/src/features/usage-logs/components/ip-audit-controls.tsx`
- Modify: `web/default/src/features/usage-logs/components/ip-audit-table.tsx`
- Create: `web/default/src/features/usage-logs/lib/ip-audit-links.ts`
- Create: `web/default/src/features/usage-logs/lib/ip-audit-links.test.ts`
- Modify: `web/classic/src/components/table/usage-logs/IPAuditPanel.jsx`
- Modify: `web/classic/src/i18n/ip-audit-locales.test.js`

- [x] **Step 1: Write the failing frontend tests**

Test five “查看”视图、完整分页、IPv6 wrapping、详情展开、`request_id` 日志跳转以及仅公网地址显示百度外链：

```ts
expect(buildIPThreatURL('8.8.8.8', true)).toBe(
  'https://cloud.baidu.com/product-s/afd_s/ip-threat.html?s=8.8.8.8'
)
expect(buildIPThreatURL('2001:4860:4860::8888', true)).toContain(
  encodeURIComponent('2001:4860:4860::8888')
)
expect(buildIPThreatURL('127.0.0.1', false)).toBeNull()
expect(buildUsageLogSearch('req-401')).toEqual({ request_id: 'req-401' })
```

- [x] **Step 2: Run tests to verify they fail**

Run from `web/default`: `bun test src/features/usage-logs/lib/ip-audit-links.test.ts src/i18n/ip-audit-locales.test.ts`

Run from `web/classic`: `bun test src/i18n/ip-audit-locales.test.js`

Expected: FAIL because the URL helper and complete interaction contract are absent.

- [x] **Step 3: Write the minimal implementation**

Use the existing project `Select` for `risk/shared_api/shared_registration/authentication/all`. Render the reason with counters, keep action columns fixed, and use a small `ExternalLink` icon:

```ts
export function buildIPThreatURL(ip: string, allowed: boolean): string | null {
  if (!allowed || !ip.trim()) return null
  return `https://cloud.baidu.com/product-s/afd_s/ip-threat.html?s=${encodeURIComponent(ip.trim())}`
}
```

The backend must return `can_query_external_profile` only when the actor can see the complete address and Go `net/netip` classifies it as valid, global-unicast, non-private, non-loopback and non-link-local. The link must use `target='_blank' rel='noreferrer noopener'`, must not appear for masked/private/reserved addresses, and must not widen administrator data access. Use `font-mono break-anywhere min-w-0` on IP cells. Timeline request IDs navigate to the existing usage-log page search state instead of opening a new error-detail surface.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./controller ./router -run 'Test.*IPAudit' -count=1`

Run from `web/default`: `bun test src/features/usage-logs src/i18n/ip-audit-locales.test.ts && bun run typecheck`

Run from `web/classic`: `bun test src/components/table/usage-logs src/i18n/ip-audit-locales.test.js`

Expected: PASS; IPv6 never overlaps badges/actions and all result pages remain reachable.

- [x] **Step 5: Commit**

```bash
git add controller/log.go router/api-router.go web/default/src/features/usage-logs web/classic/src/components/table/usage-logs web/classic/src/i18n/ip-audit-locales.test.js
git commit -m "feat(ip-audit): expose actionable risk details"
```

### Task 4: 分离 Key 普通删除与泄露撤销

**Files:**
- Create: `model/token_revocation.go`
- Create: `model/token_revocation_test.go`
- Modify: `model/token.go`
- Modify: `model/token_test.go`
- Modify: `model/main.go`
- Modify: `service/token_secret.go`
- Modify: `service/token_secret_test.go`
- Modify: `controller/token.go`
- Modify: `controller/token_test.go`

- [x] **Step 1: Write the failing tests**

覆盖 MySQL 唯一索引上的连续删除、普通删除后重建、泄露删除后禁止重建和自然生成 Key 保留 `sk-`：

```go
require.NoError(t, first.Delete())
require.NoError(t, second.Delete())
require.NotEqual(t, first.Key, second.Key)

manager, err := service.CurrentTokenSecretManager()
require.NoError(t, err)
fingerprint := manager.Fingerprint("asdfghjkl1234567")
require.NoError(t, model.RecordRevokedTokenFingerprint(tx, fingerprint, "compromised"))
require.True(t, model.IsTokenFingerprintRevoked(tx, fingerprint))
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./model ./service ./controller -run 'Test.*Token.*(Delete|Tombstone|Revoked|Custom|Generated)' -count=1`

Expected: FAIL because deleted rows share empty unique values and no independent revocation table exists.

- [x] **Step 3: Write the minimal implementation**

Create an HMAC-fingerprint-only revocation model and replace every deleted row's key material with a per-row random tombstone in the same transaction:

```go
type RevokedTokenFingerprint struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	Fingerprint string `gorm:"type:varchar(64);not null;uniqueIndex"`
	Reason      string `gorm:"type:varchar(64);not null"`
	CreatedAt   int64  `gorm:"bigint;not null"`
}

func tokenKeyMaterialReleaseUpdates(tokenID int) map[string]interface{} {
	tombstone := fmt.Sprintf("deleted:%d:%s", tokenID, uuid.NewString())
	return map[string]interface{}{
		"key": tombstone, "key_fingerprint": "", "key_ciphertext": "",
		"key_prefix": "", "key_suffix": "",
	}
}
```

Use `TokenSecretManager.Fingerprint()` for compromised keys. Collision with an active key disables the original and writes one revoked fingerprint/audit event; collision with a revoked fingerprint returns stable `token_key_revoked`. Ordinary deletion never writes the revocation table. Never persist or log the plaintext custom key.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./model ./service ./controller -run 'Test.*Token.*(Delete|Tombstone|Revoked|Collision|Custom|Generated)' -count=1`

Expected: PASS; duplicate deletion does not produce MySQL 1062 and compromised material cannot be recreated.

- [x] **Step 5: Commit**

```bash
git add model/token_revocation.go model/token_revocation_test.go model/token.go model/token_test.go model/main.go service/token_secret.go service/token_secret_test.go controller/token.go controller/token_test.go
git commit -m "fix(tokens): separate deletion tombstones from revocation"
```

### Task 5: 增加 Key 分组窄更新

**Files:**
- Modify: `controller/token.go`
- Modify: `controller/token_test.go`
- Modify: `router/api-router.go`
- Modify: `web/default/src/features/keys/api.ts`
- Modify: `web/default/src/features/keys/components/api-keys-columns.tsx`
- Modify: `web/default/src/features/keys/components/api-key-group-combobox.tsx`
- Create: `web/default/src/features/keys/components/api-key-group-editor.test.ts`
- Modify: `web/classic/src/components/table/tokens/TokensColumnDefs.jsx`
- Create: `web/classic/src/components/table/tokens/token-group-update.test.js`

- [x] **Step 1: Write the failing tests**

Assert `group_only=true` changes only `group`, rejects unavailable groups, writes `token.group_update`, and the UI displays the complete “疑似泄露” badge without ellipsis.

```go
request := httptest.NewRequest(http.MethodPut, "/api/token/9?group_only=true", strings.NewReader(`{"group":"vip"}`))
require.Equal(t, "old-name", stored.Name)
require.Equal(t, "vip", stored.Group)
require.Equal(t, "token.group_update", audit.EventType)
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./controller ./router -run 'Test.*Token.*GroupOnly' -count=1`

Run from `web/default`: `bun test src/features/keys/components/api-key-group-editor.test.ts`

Expected: FAIL because only status-only/full updates exist and the group badge is not editable.

- [x] **Step 3: Write the minimal implementation**

In `UpdateToken`, branch before binding the full edit DTO:

```go
if c.Query("group_only") == "true" {
	var request struct { Group string `json:"group" binding:"required"` }
	if err := c.ShouldBindJSON(&request); err != nil { common.ApiError(c, err); return }
	if err := service.ValidateUserTokenGroup(c.Request.Context(), userID, request.Group); err != nil {
		common.ApiError(c, err); return
	}
	if err := model.UpdateTokenGroup(tokenID, userID, request.Group); err != nil {
		common.ApiError(c, err); return
	}
	recordUserSecurityAudit(c, userID, "token.group_update", map[string]interface{}{"token_id": tokenID, "group": request.Group})
	common.ApiSuccess(c, nil)
	return
}
```

Wrap the Default `GroupBadge` with a real button and existing `ApiKeyGroupCombobox`; use Semi `Select` in Classic. Only disable the active row while saving, invalidate the token list on success, and prevent stale edit forms from overwriting unrelated fields.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./controller ./router -run 'Test.*Token.*(GroupOnly|GroupUpdate)' -count=1`

Run from `web/default`: `bun test src/features/keys && bun run typecheck`

Run from `web/classic`: `bun test src/components/table/tokens`

Expected: PASS; generated keys retain `sk-`, custom keys remain exactly as entered, and status text is fully visible.

- [x] **Step 5: Commit**

```bash
git add controller/token.go controller/token_test.go router/api-router.go web/default/src/features/keys web/classic/src/components/table/tokens
git commit -m "feat(tokens): add focused group updates"
```

### Task 6: 迁移用户兑换码资金状态机

**Files:**
- Modify: `model/user_redemption.go`
- Modify: `model/user_redemption_test.go`
- Modify: `model/main.go`
- Modify: `service/user_redemption.go`
- Modify: `service/user_redemption_governance.go`
- Modify: `service/user_redemption_test.go`

- [x] **Step 1: Write the failing tests**

将旧状态迁移和所有允许/禁止转换写成表驱动测试；到期任务只能进入 `expired_pending`，不得改钱包：

```go
require.NoError(t, FinalizeExpiredUserRedemptionCodes(context.Background(), now))
require.NoError(t, model.DB.First(&code, code.ID).Error)
require.Equal(t, model.UserRedemptionStatusExpiredPending, code.Status)
require.Equal(t, creatorQuotaBefore, loadUserQuota(t, code.CreatorUserID))
```

Expected states: `active`, `disabled`, `exhausted`, `expired_pending`, `refunded`, `forfeited`, `deleted`.

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./model ./service -run 'Test.*UserRedemption.*(Migration|Expired|State|Settlement)' -count=1`

Expected: FAIL because expiration currently refunds automatically and legacy `completed/cancelled/expired` semantics remain.

- [x] **Step 3: Write the minimal implementation**

Replace constants and centralize transitions:

```go
const (
	UserRedemptionStatusActive         = "active"
	UserRedemptionStatusDisabled       = "disabled"
	UserRedemptionStatusExhausted      = "exhausted"
	UserRedemptionStatusExpiredPending = "expired_pending"
	UserRedemptionStatusRefunded       = "refunded"
	UserRedemptionStatusForfeited      = "forfeited"
	UserRedemptionStatusDeleted        = "deleted"
)
```

Migration rules: fully consumed `completed -> exhausted`; `expired` and unsettled `cancelled -> expired_pending`; rows with positive `RefundedQuota -> refunded`; rows with positive `ForfeitedQuota -> forfeited`. Do not infer a refund from timestamps alone. `FinalizeExpiredUserRedemptionCodes` updates only eligible `active/disabled` rows to `expired_pending` and emits one audit event without wallet adjustment.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./model ./service -run 'Test.*UserRedemption.*(Migration|Expired|State|Settlement|Idempotent)' -count=1`

Expected: PASS; refunds and forfeitures are explicit one-time administrator decisions.

- [x] **Step 5: Commit**

```bash
git add model/user_redemption.go model/user_redemption_test.go model/main.go service/user_redemption.go service/user_redemption_governance.go service/user_redemption_test.go
git commit -m "fix(redemptions): make expiry settlement explicit"
```

### Task 7: 允许自领并补齐管理员批量处置

**Files:**
- Modify: `service/user_redemption.go`
- Modify: `service/user_redemption_governance.go`
- Modify: `service/user_redemption_test.go`
- Modify: `service/user_redemption_concurrency_test.go`
- Modify: `controller/user_redemption.go`
- Modify: `controller/user_redemption_test.go`
- Modify: `router/api-router.go`
- Modify: `web/default/src/features/user-redemptions/api.ts`
- Modify: `web/default/src/features/user-redemptions/types.ts`
- Modify: `web/default/src/features/user-redemptions/components/user-redemptions-table.tsx`
- Modify: `web/default/src/features/user-redemptions/components/admin-governance-dialog.tsx`
- Create: `web/default/src/features/user-redemptions/components/admin-bulk-settlement.test.ts`
- Modify: `web/classic/src/components/user-redemptions/UserRedemptionsTable.jsx`
- Modify: `web/classic/src/components/user-redemptions/AdminUserRedemptionGovernanceModal.jsx`

- [x] **Step 1: Write the failing tests**

Replace the self-claim rejection test with quota conservation assertions for fixed and red-packet modes; add concurrent claim, duplicate claim, delete-with-disposition and mixed-result bulk settlement cases:

```go
systemTotalBefore := sumWalletQuota(t) + code.RemainingQuota()
claim, err := ClaimUserRedemptionCode(ctx, code.CreatorUserID, code.Code)
require.NoError(t, err)
require.Positive(t, claim.Quota)
require.Equal(t, systemTotalBefore, sumWalletQuota(t)+loadRemainingQuota(t, code.ID))
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./service ./controller -run 'Test.*UserRedemption.*(SelfClaim|Concurrent|Bulk|Delete|Error)' -count=1`

Expected: FAIL at the explicit creator prohibition, old deletion behavior and missing batch contract.

- [x] **Step 3: Write the minimal implementation**

Remove only the creator prohibition; retain transaction locks, unique `(code_id, claimant_user_id)`, remaining quota checks and idempotent wallet operation IDs. Add:

```go
type UserRedemptionBatchDispositionRequest struct {
	IDs         []int64 `json:"ids" binding:"required,min=1,max=100"`
	Disposition string  `json:"disposition" binding:"oneof=refund forfeit"`
	Reason      string  `json:"reason" binding:"required,max=500"`
}

type UserRedemptionBatchDispositionResult struct {
	ID      int64  `json:"id"`
	Success bool   `json:"success"`
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}
```

Process each ID in its own idempotent transaction and return every result. Active deletion requires a disposition and settles before setting `deleted`; terminal rows only become `deleted` and never move funds again. Map field errors to `amount`, `total_amount`, `max_claims`, `minimum_share`, `balance`, or `expires_in` so the UI can show the actual invalid field.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./service ./controller ./router -run 'Test.*UserRedemption' -count=1`

Run from `web/default`: `bun test src/features/user-redemptions && bun run typecheck`

Run from `web/classic`: `bun test src/components/user-redemptions`

Expected: PASS; self-claim conserves quota, copy uses the returned code, `expired_pending` is risk-prioritized, and bulk results expose partial failure.

- [x] **Step 5: Commit**

```bash
git add service/user_redemption.go service/user_redemption_governance.go service/user_redemption_test.go service/user_redemption_concurrency_test.go controller/user_redemption.go controller/user_redemption_test.go router/api-router.go web/default/src/features/user-redemptions web/classic/src/components/user-redemptions
git commit -m "feat(redemptions): allow self claim and explicit disposition"
```

### Task 8: 验证三数据库数据完整性

**Files:**
- Modify: `model/main_database_integration_test.go`
- Modify: `model/ip_security_event_test.go`
- Modify: `service/user_redemption_concurrency_test.go`
- Modify: `controller/token_test.go`
- Create: `docs/superpowers/specs/2026-07-26-security-data-integrity-evidence.md`

- [x] **Step 1: Write the failing integration assertions**

让现有 `TEST_MYSQL_DSN`/`TEST_POSTGRES_DSN` 测试覆盖新列、撤销唯一索引、兑换码状态迁移和并发资金守恒；证据文档预先列出精确命令、数据库版本、提交 SHA 和结果表。

- [x] **Step 2: Run SQLite first**

Run: `go test ./setting/ip_security_setting ./model ./service ./controller ./router -run 'Test.*(IPAudit|IPSecurity|Token|UserRedemption)' -count=1`

Expected: PASS on SQLite without skipped local behavior tests.

- [x] **Step 3: Run MySQL and PostgreSQL**

Require the operator-provided environment variables to point to disposable databases, then run:

```powershell
if (-not $env:TEST_MYSQL_DSN) { throw 'TEST_MYSQL_DSN must point to an isolated test database' }
if (-not $env:TEST_POSTGRES_DSN) { throw 'TEST_POSTGRES_DSN must point to an isolated test database' }
go test ./model ./service ./controller -run 'Test.*(MainDatabase|IPSecurity|Token|UserRedemption)' -count=1
```

Expected: PASS on MySQL 5.7.8+ and PostgreSQL 9.6+; no duplicate key, unsupported lock or dialect SQL errors.

- [x] **Step 4: Record evidence and run diff checks**

Write only non-secret DSN labels, versions, durations and PASS/FAIL to the evidence file. Run: `gofmt -w` on changed Go files, then `git diff --check` and `go test ./model ./service ./controller ./router -count=1`.

Expected: all checks PASS and no credential appears in Git diff.

- [x] **Step 5: Commit**

```bash
git add model/main_database_integration_test.go model/ip_security_event_test.go service/user_redemption_concurrency_test.go controller/token_test.go docs/superpowers/specs/2026-07-26-security-data-integrity-evidence.md
git commit -m "test(data): verify security and wallet invariants"
```
