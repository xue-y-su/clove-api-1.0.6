# 用户工作流 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 完成邀请码、奖励提现审批、公告和个人资料的端到端用户工作流，使每个状态、弹窗、上传和会话行为都可解释且可回归验证。

**Architecture:** 邀请使用记录继续作为奖励事实源，新增聚合提现申请锁定多笔可用奖励，管理员只审批提现申请并恰好一次入账。公告继续存储在现有设置 JSON 中，但弹窗属性属于单条公告，正文统一走 Markdown + HTML 白名单净化。个人资料沿用现有自助接口和图床直传设计，只修复展示、超时、字段冷却和解绑会话契约。

**Tech Stack:** Go 1.22、Gin、GORM v2、SQLite/MySQL/PostgreSQL、React 19/TypeScript/Bun、React 18/JavaScript/Bun、TanStack Query、DOMPurify、Base UI、Semi UI。

---

## 文件职责

- `model/invitation.go`：邀请码、奖励项和聚合提现申请实体及行锁。
- `service/invitation.go`：10 位邀请码生成、申请/冷却/终态判定。
- `service/invitation_registration.go`：注册消费邀请码并创建 `available` 奖励事实。
- `service/invitation_reward_payout.go`：用户提交提现、管理员审批和钱包入账。
- `controller/invitation.go`、`router/api-router.go`：用户/管理员提现 API 和稳定错误码。
- `service/announcement.go`、`service/announcement_media.go`：单条 popup、签名、统一内容与媒体目录。
- `controller/announcement.go`、`controller/announcement_setting.go`：公告确认、上传和设置写入。
- `controller/user.go`、`model/own_profile.go`：资料字段、冷却、头像 URL 与 Session 安全版本。
- `web/default/src/features/invitations`、`web/classic/src/components/invitations`：邀请码与审批页。
- `web/default/src/features/announcements`、`web/classic/src/components/layout`：登录后公告弹窗和图片灯箱。
- `web/default/src/features/profile`、`web/classic/src/components/settings/personal`：资料头编辑和第三方解绑。

### Task 1: 收口邀请码生成与再次申请

**Files:**
- Modify: `service/invitation.go`
- Create: `service/invitation_test.go`
- Modify: `service/invitation_registration.go`
- Modify: `service/invitation_registration_test.go`
- Modify: `controller/invitation.go`
- Modify: `controller/invitation_test.go`

- [x] **Step 1: Write the failing tests**

断言新码固定 10 位、排除易混淆字符、唯一冲突自动重试；已用尽/过期/撤销且无冷却时可再次申请，管理员关闭申请时返回明确状态：

```go
for index := 0; index < 100; index++ {
	code, err := generateInvitationCode()
	require.NoError(t, err)
	require.Len(t, code, 10)
	require.NotRegexp(t, `[01OIL]`, code)
}
require.True(t, state.CanApply)
require.Equal(t, "applications_disabled", state.ApplyUnavailableReason)
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./service ./controller -run 'Test.*Invitation.*(Generate|Reapply|Exhausted|Disabled)' -count=1`

Expected: FAIL because当前生成长度/字符表和终态展示契约不完整。

- [x] **Step 3: Write the minimal implementation**

Use `crypto/rand` with the exact alphabet and keep the database unique-index retry:

```go
const invitationCodeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"
const invitationCodeLength = 10

func generateInvitationCode() (string, error) {
	var code strings.Builder
	code.Grow(invitationCodeLength)
	limit := big.NewInt(int64(len(invitationCodeAlphabet)))
	for index := 0; index < invitationCodeLength; index++ {
		position, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		code.WriteByte(invitationCodeAlphabet[position.Int64()])
	}
	return code.String(), nil
}
```

Return `can_apply`, `reapply_at`, and `apply_unavailable_reason`. Cooldown expiry must hide the stale rejection banner while retaining the historical application record. Return both `code` and an invitation link built from the configured public site URL plus `?aff=<code>`; never expose owner data from the public validation endpoint.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./service ./controller -run 'Test.*Invitation.*(Generate|Collision|Reapply|Cooldown|Terminal|Disabled)' -count=1`

Expected: PASS with deterministic state semantics and cryptographic code generation.

- [x] **Step 5: Commit**

```bash
git add service/invitation.go service/invitation_test.go service/invitation_registration.go service/invitation_registration_test.go controller/invitation.go controller/invitation_test.go
git commit -m "fix(invitations): tighten code and reapply lifecycle"
```

### Task 2: 建立聚合邀请奖励提现申请

**Files:**
- Modify: `model/invitation.go`
- Modify: `model/main.go`
- Modify: `service/invitation_registration.go`
- Rewrite: `service/invitation_reward_payout.go`
- Modify: `service/invitation_registration_test.go`
- Modify: `service/invitation_reward_worker_test.go`

- [x] **Step 1: Write the failing service tests**

覆盖 `available -> pending_payout -> paid`、拒绝回 `available`、并发提交只生成一笔申请、重复审批不二次入账和历史状态迁移：

```go
request, err := SubmitInvitationRewardPayout(context.Background(), userID)
require.NoError(t, err)
require.EqualValues(t, 30, request.Amount)
require.Equal(t, model.InviteRewardStatusPendingPayout, loadReward(t, rewardID).Status)

require.NoError(t, ReviewInvitationRewardPayoutRequest(ctx, ReviewInvitationRewardPayoutRequestInput{
	RequestID: request.ID, ReviewerID: adminID, Approve: true,
}))
require.EqualValues(t, quotaBefore+30, loadUserQuota(t, userID))
require.ErrorIs(t, ReviewInvitationRewardPayoutRequest(ctx, duplicate), ErrInviteRewardReviewInvalid)
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./model ./service -run 'Test.*InvitationReward.*(Submit|Review|Concurrent|Migration|ExactlyOnce)' -count=1`

Expected: FAIL because current flow先逐笔审批再由用户提现，没有聚合申请。

- [x] **Step 3: Write the minimal implementation**

Add a request entity and relation rows without storing an ID list in JSON:

```go
type InviteRewardPayoutRequest struct {
	ID          int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	OperationID string `json:"operation_id" gorm:"type:varchar(191);not null;uniqueIndex"`
	UserID      int    `json:"user_id" gorm:"not null;index"`
	Amount      int64  `json:"amount" gorm:"type:bigint;not null"`
	Status      string `json:"status" gorm:"type:varchar(32);not null;index"`
	Reason      string `json:"reason" gorm:"type:varchar(500);not null"`
	ReviewerID  int    `json:"reviewer_id" gorm:"not null;default:0"`
	CreatedAt   int64  `json:"created_at" gorm:"bigint;not null"`
	ReviewedAt  int64  `json:"reviewed_at" gorm:"bigint;not null;default:0"`
}

type InviteRewardPayoutRequestItem struct {
	RequestID int64 `gorm:"primaryKey;autoIncrement:false"`
	RewardID  int64 `gorm:"primaryKey;autoIncrement:false;uniqueIndex"`
}
```

Inside one transaction lock the user and all `available` rewards, create one request and relation rows, then mark those rewards `pending_payout`. Approval locks request and relations, calls `AdjustWithTx` with the stable `OperationID`, marks rewards `paid` and request `paid`; rejection marks rewards `available` and request `rejected`. Migrate `pending_review/approved` to `available`, `withdrawn` to `paid`, and keep historical rejected records terminal.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./model ./service -run 'Test.*InvitationReward.*(Submit|Review|Concurrent|Migration|ExactlyOnce|Wallet)' -count=1`

Expected: PASS on SQLite and no second wallet ledger entry after duplicate approval.

- [x] **Step 5: Commit**

```bash
git add model/invitation.go model/main.go service/invitation_registration.go service/invitation_reward_payout.go service/invitation_registration_test.go service/invitation_reward_worker_test.go
git commit -m "feat(invitations): add aggregate reward payout requests"
```

### Task 3: 对齐邀请奖励 API 与双前端

**Files:**
- Modify: `controller/invitation.go`
- Modify: `controller/invitation_test.go`
- Modify: `router/api-router.go`
- Modify: `router/invitation_routes_test.go`
- Modify: `web/default/src/features/invitations/types.ts`
- Modify: `web/default/src/features/invitations/api.ts`
- Modify: `web/default/src/features/invitations/components/user-invitation-card.tsx`
- Modify: `web/default/src/features/invitations/components/invitation-reward-approvals-page.tsx`
- Modify: `web/default/src/features/invitations/invitation-rewards-ui.test.ts`
- Modify: `web/classic/src/components/invitations/api.js`
- Modify: `web/classic/src/components/invitations/AdminInvitationRewards.jsx`
- Modify: `web/classic/src/components/invitations/UserInvitationCard.jsx`

- [x] **Step 1: Write the failing API and UI tests**

```ts
expect(summary).toEqual({
  invited_count: 3,
  total_earned: 120,
  available: 50,
  pending_payout: 30,
  paid: 40,
})
expect(post).toHaveBeenCalledWith('/api/invitation/rewards/payout-requests')
```

Admin tests must list requests rather than individual reward rows and require a reason on rejection.

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./controller ./router -run 'Test.*Invitation.*(Reward|Payout)' -count=1`

Run from both frontends: `bun test src/features/invitations` and `bun test src/components/invitations`.

Expected: FAIL because API and UI still expose `pending_review/approved/withdrawn` item states.

- [x] **Step 3: Write the minimal implementation**

Use these contracts:

```text
GET  /api/invitation/rewards
POST /api/invitation/rewards/payout-requests
GET  /api/invitation/admin/reward-payout-requests
POST /api/invitation/admin/reward-payout-requests/:id/review
```

The wallet card shows invited count, total earned, available, pending, and paid. “提交提现审核” is enabled only when `available > 0`; submission changes the amount to pending without crediting the wallet. Keep the administrator route/page name “邀请码审批” directly below user redemption codes, but label rows as提现申请 and show included reward count, amount, user, operation ID, status, timestamps and reason.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./controller ./router -run 'Test.*Invitation.*(Reward|Payout)' -count=1`

Run from `web/default`: `bun test src/features/invitations src/features/wallet/wallet-composition.test.ts && bun run typecheck`

Run from `web/classic`: `bun test src/components/invitations src/components/topup`

Expected: PASS; an approval credits exactly once and a rejection restores the available amount.

- [x] **Step 5: Commit**

```bash
git add controller/invitation.go controller/invitation_test.go router/api-router.go router/invitation_routes_test.go web/default/src/features/invitations web/default/src/features/wallet web/classic/src/components/invitations web/classic/src/components/topup
git commit -m "feat(invitations): expose payout request approval flow"
```

### Task 4: 统一公告内容与单条弹窗

**Files:**
- Modify: `setting/announcement_setting/config.go`
- Modify: `setting/announcement_setting/config_test.go`
- Modify: `service/announcement.go`
- Modify: `service/announcement_test.go`
- Modify: `controller/announcement_setting.go`
- Modify: `controller/announcement_test.go`
- Modify: `web/default/src/features/system-settings/content/announcements-section.tsx`
- Modify: `web/default/src/features/system-settings/content/announcements-section.test.ts`
- Modify: `web/default/src/features/announcements/global-announcement-dialog.tsx`
- Modify: `web/classic/src/pages/Setting/Dashboard/SettingsAnnouncements.jsx`
- Modify: `web/classic/src/components/layout/GlobalAnnouncementModal.jsx`

- [x] **Step 1: Write the failing tests**

Test per-item `popup`, 64 KiB UTF-8 limit, no `content_format` selector, HTML/Markdown mixed input, login-only popup and versioned acknowledgement:

```go
require.NoError(t, validateAnnouncementContent(strings.Repeat("你", 20_000)))
require.ErrorIs(t, validateAnnouncementContent(strings.Repeat("a", 65_537)), ErrAnnouncementContentTooLarge)
require.False(t, stateForAnonymous.Popup)
require.NotEqual(t, AnnouncementSignature(before), AnnouncementSignature(after))
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./setting/announcement_setting ./service ./controller -run 'TestAnnouncement.*(Popup|Content|Signature|Anonymous)' -count=1`

Run from both frontends: target the announcement tests.

Expected: FAIL because global `PopupEnabled` and content format branches still participate.

- [x] **Step 3: Write the minimal implementation**

Remove `PopupEnabled` from active settings DTO and audit payload; keep the old option key readable but ignored for backward-compatible startup. Normalize every announcement to one content path:

```go
const AnnouncementContentMaxBytes = 64 * 1024

type Announcement struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	ImageURL    string `json:"image_url,omitempty"`
	PublishDate string `json:"publish_date"`
	Popup       bool   `json:"popup"`
}
```

Render with the existing Markdown parser configured for native HTML, then sanitize final HTML with the existing DOMPurify allowlist. Delete the format dropdown and `content_format` rendering branches from both themes. Query popup state only after authenticated gates resolve; acknowledgement remains keyed by user plus content signature.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./setting/announcement_setting ./model ./service ./controller ./router -run 'TestAnnouncement' -count=1`

Run from `web/default`: `bun test src/features/announcements src/features/system-settings/content/announcements-section.test.ts`

Run from `web/classic`: `bun test src/components/layout/global-announcement.test.js src/pages/Setting/Dashboard`

Expected: PASS; anonymous pages never query/confirm popup announcements and script/event payloads are removed.

- [x] **Step 5: Commit**

```bash
git add setting/announcement_setting service/announcement.go service/announcement_test.go controller/announcement_setting.go controller/announcement_test.go web/default/src/features/announcements web/default/src/features/system-settings/content/announcements-section.tsx web/classic/src/pages/Setting/Dashboard/SettingsAnnouncements.jsx web/classic/src/components/layout/GlobalAnnouncementModal.jsx
git commit -m "fix(announcements): use per-item popup and unified content"
```

### Task 5: 固化公告媒体目录与图片体验

**Files:**
- Modify: `service/announcement_media.go`
- Modify: `service/announcement_media_test.go`
- Modify: `controller/announcement.go`
- Modify: `docker-compose.yml`
- Modify: `docker-compose.dev.yml`
- Modify: `web/default/src/features/announcements/announcement-image-preview.tsx`
- Modify: `web/default/src/features/system-settings/content/announcements-section.tsx`
- Modify: `web/classic/src/pages/Setting/Dashboard/SettingsAnnouncements.jsx`
- Modify: `web/classic/src/components/layout/GlobalAnnouncementModal.jsx`

- [x] **Step 1: Write the failing media tests**

Assert startup detects unwritable media directory, upload accepts only PNG/JPEG/WebP/GIF within 5 MiB and 4096×4096, and same-content uploads retain hash naming and atomic replacement. Frontend tests assert the square add tile and keyboard-operable lightbox.

```go
require.ErrorIs(t, EnsureAnnouncementMediaDirectory(), ErrAnnouncementMediaStorage)
require.NoFileExists(t, filepath.Join(directory, ".announcement-media-leftover"))
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./service ./controller -run 'TestAnnouncementMedia.*(Directory|Permission|Format|Dimensions|Atomic)' -count=1`

Expected: FAIL because no explicit startup writability check repairs/reports legacy volume ownership.

- [x] **Step 3: Write the minimal implementation**

Add `EnsureAnnouncementMediaDirectory()` that creates the directory with `0750`, writes and removes a zero-byte probe, and returns a wrapped stable storage error without exposing the absolute path to users. Call it during startup before serving requests. Keep temp files inside the same directory so `os.Rename` stays atomic. In Docker, run an init command as root only to `mkdir -p /data/announcement-media && chown 10001:10001` before dropping to UID 10001; do not chmod all `/data` world-writable.

Use the existing square upload control. Preview opens a dialog with focus trap, close button, backdrop click and Escape support; image uses `max-h-[85vh] max-w-[95vw] object-contain`.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./service ./controller -run 'TestAnnouncementMedia' -count=1`

Run from both frontends: announcement component tests and build/typecheck.

Run: `docker compose -f docker-compose.dev.yml up -d --build new-api; docker compose -f docker-compose.dev.yml exec -T new-api sh -lc 'test -w /data/announcement-media'`

Expected: tests PASS and the container exits `0` for the writability probe.

- [x] **Step 5: Commit**

```bash
git add service/announcement_media.go service/announcement_media_test.go controller/announcement.go docker-compose.yml docker-compose.dev.yml web/default/src/features/announcements web/default/src/features/system-settings/content/announcements-section.tsx web/classic/src/pages/Setting/Dashboard/SettingsAnnouncements.jsx web/classic/src/components/layout/GlobalAnnouncementModal.jsx
git commit -m "fix(announcements): harden media storage and preview"
```

### Task 6: 修复公告页签闪烁

**Files:**
- Modify: `web/default/src/features/dashboard/components/overview/announcements-panel.tsx`
- Modify: `web/default/src/features/dashboard/components/overview/announcement-detail-dialog.tsx`
- Create: `web/default/src/features/dashboard/components/overview/announcements-panel.test.ts`
- Modify: `web/classic/src/components/layout/headerbar/NotificationButton.jsx`
- Create: `web/classic/src/components/layout/headerbar/notification-tabs.test.js`

- [x] **Step 1: Write the failing UI tests**

Test that switching between notifications and timeline keeps both cached datasets, preserves the panel's measured minimum height, and does not remount the shared header/loading shell.

- [x] **Step 2: Run tests to verify they fail**

Run from `web/default`: `bun test src/features/dashboard/components/overview/announcements-panel.test.ts`

Run from `web/classic`: `bun test src/components/layout/headerbar/notification-tabs.test.js`

Expected: FAIL because tab content is currently torn down/recreated or collapses while querying.

- [x] **Step 3: Write the minimal implementation**

Give each tab a distinct stable React Query key, keep previous data during refetch, and render both panels under one fixed-height container with `hidden`/`aria-hidden` rather than replacing the complete shell. Do not add arbitrary animation delays.

```tsx
<div className='min-h-72'>
  <section hidden={tab !== 'notifications'}>{notificationsPanel}</section>
  <section hidden={tab !== 'timeline'}>{timelinePanel}</section>
</div>
```

- [x] **Step 4: Run tests to verify they pass**

Run both new tests plus `bun run typecheck` in Default.

Expected: PASS; tab switch causes no loading flash or layout collapse.

- [x] **Step 5: Commit**

```bash
git add web/default/src/features/dashboard/components/overview web/classic/src/components/layout/headerbar
git commit -m "fix(announcements): preserve tab state while switching"
```

### Task 7: 收口个人资料与解绑会话

**Files:**
- Modify: `controller/user.go`
- Modify: `controller/own_profile_test.go`
- Modify: `model/own_profile.go`
- Modify: `web/default/src/features/profile/components/profile-header.tsx`
- Modify: `web/default/src/features/profile/components/profile-identity-card.tsx`
- Modify: `web/default/src/features/profile/components/tabs/account-bindings-tab.tsx`
- Modify: `web/default/src/features/profile/hooks/use-profile.ts`
- Modify: `web/default/src/features/profile/lib/profile-identity.test.ts`
- Modify: `web/default/src/stores/auth-store.ts`
- Modify: `web/default/src/components/profile-dropdown.tsx`
- Modify: `web/default/src/components/layout/components/mobile-drawer.tsx`
- Modify: `web/classic/src/components/settings/personal/components/UserInfoHeader.jsx`
- Modify: `web/classic/src/components/settings/personal/cards/ProfileIdentity.jsx`
- Modify: `web/classic/src/components/settings/personal/cards/AccountManagement.jsx`
- Modify: `web/classic/src/components/layout/headerbar/UserArea.jsx`
- Modify: `web/classic/src/components/settings/personal/profile-identity.test.js`

- [x] **Step 1: Write the failing tests**

Assert subtitle is `@clove · Admin` and only存在邀请人时追加其值；昵称冷却只禁用昵称字段，头像上传有超时并返回具体错误；自助解绑更新当前 Session 安全版本而不登出当前设备。头像保存并重新获取 `/api/user/self` 后，资料页、右上角用户菜单和移动抽屉必须立即使用相同的 `avatar_url`；Classic 顶栏也必须从刷新后的用户状态读取该 URL，加载失败时才退回字母头像。

```ts
expect(buildProfileSubtitle(profile)).toEqual(['@clove', 'Admin'])
expect(buildProfileSubtitle(withInviter)).toEqual(['@clove', 'Admin', '@alice'])
expect(screen.queryByText(/Inviter:|No Inviter/)).toBeNull()
expect(mergeAuthUserProfile(authUser, profile).avatar_url).toBe(
  'https://img.example/new-avatar.png'
)
```

- [x] **Step 2: Run tests to verify they fail**

Run: `go test ./controller ./model -run 'Test.*(UpdateSelf|OwnProfile|Unbind|SecurityVersion)' -count=1`

Run profile tests in both frontends.

Expected: FAIL on the remaining presentation, timeout or session-refresh regression.

- [x] **Step 3: Write the minimal implementation**

Keep one edit button in the top profile card and one modal. Return field metadata separately:

```json
{
  "display_name_next_change_at": 1785000000,
  "avatar_upload": {
    "enabled": true,
    "endpoint": "https://trusted.example/upload",
    "allowed_domains": ["trusted.example"]
  }
}
```

The browser uploads directly to the configured image host with a 20-second `AbortController` timeout; the server stores only an allowlisted HTTPS URL. During nickname cooldown leave the modal and avatar controls enabled, disable only the nickname input and show the exact next timestamp. After built-in or custom OAuth unbind, update the current Gin session's `security_version` to the committed value before responding.

Add `avatar_url?: string` to `AuthUser`. When `useProfile` successfully reloads the current user's profile, merge `display_name`, `avatar_url`, `group` and other shared display fields into the existing auth user and persist through `auth.setUser`; do not replace permissions or unrelated session fields. Render `AvatarImage` before `AvatarFallback` in Default `ProfileDropdown` and `MobileDrawer`, removing the hard-coded `/avatars/01.png`. Pass `userState.user.avatar_url` to the Classic header `Avatar`. All entry points retain their existing fixed dimensions so image loading cannot shift header layout.

- [x] **Step 4: Run tests to verify they pass**

Run: `go test ./controller ./model -run 'Test.*(UpdateSelf|OwnProfile|Avatar|Unbind|SecurityVersion)' -count=1`

Run from `web/default`: `bun test src/features/profile src/components/profile-dropdown.test.tsx src/components/layout/components/mobile-drawer.test.tsx && bun run typecheck`

Run from `web/classic`: `bun test src/components/settings/personal src/components/layout/headerbar/user-area-avatar.test.js`

Expected: PASS; profile page remains usable after nickname change, upload failure and OAuth unbind, and the saved avatar appears immediately in desktop/mobile account entry points without re-login.

- [x] **Step 5: Commit**

```bash
git add controller/user.go controller/own_profile_test.go model/own_profile.go web/default/src/features/profile web/default/src/stores/auth-store.ts web/default/src/components/profile-dropdown.tsx web/default/src/components/layout/components/mobile-drawer.tsx web/classic/src/components/settings/personal web/classic/src/components/layout/headerbar/UserArea.jsx
git commit -m "fix(profile): preserve editing and current session"
```

### Task 8: 验证工作流与数据库迁移

**Files:**
- Modify: `model/main_database_integration_test.go`
- Create: `docs/superpowers/specs/2026-07-26-user-workflows-evidence.md`

- [x] **Step 1: Add integration assertions**

Extend the existing database test to verify payout request tables/indexes, legacy reward migration, announcement acknowledgement uniqueness and profile security-version transaction behavior on every configured dialect.

- [x] **Step 2: Run backend suites**

Run: `go test ./model ./service ./controller ./router -run 'Test.*(Invitation|Announcement|Profile|Binding)' -count=1`

Expected: PASS on SQLite.

- [x] **Step 3: Run MySQL/PostgreSQL suites**

```powershell
if (-not $env:TEST_MYSQL_DSN) { throw 'TEST_MYSQL_DSN must point to an isolated test database' }
if (-not $env:TEST_POSTGRES_DSN) { throw 'TEST_POSTGRES_DSN must point to an isolated test database' }
go test ./model ./service ./controller -run 'Test.*(MainDatabase|Invitation|Announcement|Profile)' -count=1
```

Expected: PASS with identical reward and acknowledgement outcomes.

- [x] **Step 4: Record UI acceptance evidence**

Using Root, administrator and normal-user accounts, record exact pass/fail outcomes for code reapply, registration validation, payout submission/review, popup acknowledgement, media upload/lightbox, profile edit and OAuth unbind. Record request IDs only; omit passwords, tokens, invitation codes and image-host credentials. Run `git diff --check`.

- [x] **Step 5: Commit**

```bash
git add model/main_database_integration_test.go docs/superpowers/specs/2026-07-26-user-workflows-evidence.md
git commit -m "test(workflows): verify invitation announcement profile flows"
```
