# Full Feature Workspace Completion Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish the approved governance, invitation, wallet, Token, audit, Playground, email-template, user-redemption, and Snow-home work without breaking the existing New API application shell or the two frontend themes.

**Architecture:** Complete the work as thirteen independently testable vertical commits. New money, invitation, sensitive-read, and registration paths use narrow model/service modules with portable GORM transactions and stable idempotency keys. Controllers expose only authenticated contracts, while Default and Classic implement the same state machine and rely on the server for authorization. Existing partial changes are treated as input: preserve them, test them, and only modify overlapping code after reading the current diff.

**Tech Stack:** Go 1.25, Gin, GORM v2, SQLite/MySQL/PostgreSQL, optional ClickHouse log database, Redis/in-memory cache, React 19/TanStack Router/Tailwind/Base UI, React 18/React Router/Semi UI, Bun, Playwright.

---

## Working Rules

- Work from D:\cloveapi\extracted-source on branch feature/user-governance-wallet-audit-playground.
- Never use reset, checkout-overwrite, clean, or a repository-wide git add.
- Before each commit run git diff --check, inspect git diff --cached --name-only, and scan staged files for credentials and complete Token values.
- Only stage the paths listed in the task being committed. Existing unrelated modifications remain in the working tree.
- Keep New API, QuantumNous, module paths, and protected copyright headers intact. Historical default display-name migration is the only permitted user-facing title migration.
- New schema and service code must work with SQLite, MySQL 5.7.8+, and PostgreSQL 9.6+. Use the existing lockForUpdate(tx) helper for databases that support row locks and condition/unique-key updates for SQLite.
- Complete Keys, passwords, cookies, authorization headers, Tavily/SMTP credentials, prompts, and image bytes never enter logs, audit payloads, URLs, browser storage, or error responses.
- Every behavior change starts with a focused failing test, followed by the smallest implementation, then the package/frontend/build checks named below.

## File Map

### Existing foundation to preserve or extend

- model/user.go, model/user_governance.go, model/user_cache.go, model/main.go, model/option.go: current user fields, role guards, migration registration, cache invalidation, and option loading.
- controller/user.go, controller/oauth.go, controller/wechat.go, controller/registration.go, controller/token.go, controller/playground.go, controller/log.go, controller/audit.go: current route behavior and partial changes already in the worktree.
- middleware/auth.go, middleware/audit.go, middleware/secure_verification.go, model/token.go, model/log.go: authentication, session freshness, Token lookup, and log query primitives.
- router/api-router.go, router/web-router.go, main.go: route groups and Default/Classic/Snow embedding.
- web/default/src/features/profile/api.ts, web/default/src/features/profile/types.ts, web/default/src/features/profile/index.tsx, web/default/src/features/keys/api.ts, web/default/src/features/keys/types.ts, web/default/src/features/wallet/api.ts, web/default/src/features/wallet/types.ts, web/default/src/features/playground/api.ts, web/default/src/features/playground/types.ts, web/default/src/features/usage-logs/api.ts, web/default/src/features/usage-logs/types.ts, web/default/src/features/users/api.ts, web/default/src/features/users/types.ts: existing feature contracts and navigation consumers.

### New domain modules

- model/own_profile.go, controller/own_profile.go: self-profile contract separate from broad admin updates.
- model/wallet_ledger.go, service/wallet_ledger.go: integer wallet ledger, idempotent adjustments, and post-commit cache synchronization.
- model/audit_outbox.go, service/audit_outbox.go, service/audit_outbox_worker.go: transactional audit events, delivery receipts, retry, and metrics.
- model/invitation.go, service/invitation_registration.go, controller/invitation.go: application, issued-code, use, approval, and registration state machines.
- model/token_secret.go, service/token_secret.go, middleware/token_reveal.go: phased Key migration, password verification window, and sensitive-read audit.
- model/user_redemption.go, service/user_redemption.go, controller/user_redemption.go: wallet-funded user codes and claims, separate from administrator model/redemption.go.
- model/email_template.go, service/email_template.go, controller/email_template.go: built-in/custom verification templates and safe rendering.
- service/ip_audit.go, controller/ip_audit.go: object-authorized user security/IP details.

### Frontend and delivery files

- Default locale set: web/default/src/i18n/locales/en.json, web/default/src/i18n/locales/fr.json, web/default/src/i18n/locales/ja.json, web/default/src/i18n/locales/ru.json, web/default/src/i18n/locales/vi.json, web/default/src/i18n/locales/zh-TW.json, web/default/src/i18n/locales/zh.json.
- Classic locale set: web/classic/src/i18n/locales/en.json, web/classic/src/i18n/locales/fr.json, web/classic/src/i18n/locales/ja.json, web/classic/src/i18n/locales/ru.json, web/classic/src/i18n/locales/vi.json, web/classic/src/i18n/locales/zh-CN.json, web/classic/src/i18n/locales/zh-TW.json, web/classic/src/i18n/locales/zh.json.
- Snow: web/snow/dist/index.html, web/snow/dist/assets/index-2eb69c09.js, web/snow/dist/assets/App3D-f554a111.js, router/web-router.go, router/web_router_test.go, docs/superpowers/specs/2026-07-14-snow-home-delivery.md.
- Verification: docs/superpowers/verification/2026-07-14-workspace-baseline.md, docs/superpowers/verification/2026-07-14-final-delivery.md.

## Task 1: Classify Workspace And Record Baseline

**Files:**
- Modify: .gitignore
- Create: docs/superpowers/verification/2026-07-14-workspace-baseline.md
- Test: existing Go and frontend test suites, recorded rather than changed

- [ ] **Step 1: Capture the current file classes and branch state**

Run:

~~~powershell
git status --short --branch
git diff --stat
rg --files web/snow/dist | Measure-Object
Get-ChildItem web/snow/dist -Recurse -File | Measure-Object -Property Length -Sum
~~~

Record the 111 Snow files and approximately 18.99 MiB vendored size, the 148 current status entries, and the existing branch in docs/superpowers/verification/2026-07-14-workspace-baseline.md.

- [ ] **Step 2: Verify ignored local-only paths without hiding business files**

Run:

~~~powershell
git check-ignore -v .build-cache/restore-web-format-churn.txt .superpowers/brainstorm/304-1784035950
git ls-files web/snow/dist | Measure-Object
git ls-files --others --exclude-standard
~~~

Add only generated caches, local databases, logs, screenshots, traces, and temporary verification output to .gitignore. Do not ignore web/snow/dist/**, source, tests, or design/plan documents.

- [ ] **Step 3: Record the executable baseline**

Run:

~~~powershell
go test ./...
Push-Location web/default; bun test; bun run typecheck; bun run build; Pop-Location
Push-Location web/classic; bun test; bun run build; Pop-Location
~~~

Write command results, package versions, and any pre-existing failure classification to the baseline file. A baseline failure is recorded and fixed in its owning task; it is never suppressed by changing the test command.

- [ ] **Step 4: Validate the documentation-only staged set and commit**

~~~powershell
git add -- .gitignore docs/superpowers/verification/2026-07-14-workspace-baseline.md
git diff --cached --check
git diff --cached --name-only
git commit -m "chore(workspace): classify files and record test baseline"
~~~

Expected staged paths are exactly the two listed files.

## Task 2: Finish OwnProfile And Governed User Mutations

**Files:**
- Create: model/own_profile.go, controller/own_profile.go
- Modify: model/user.go, model/user_governance.go, model/user_cache.go, model/errors.go, model/main.go, router/api-router.go, controller/user.go, middleware/auth.go
- Modify Default: web/default/src/features/profile/api.ts, web/default/src/features/profile/types.ts, web/default/src/features/profile/index.tsx, web/default/src/features/profile/components/profile-header.tsx, web/default/src/features/users/api.ts, web/default/src/features/users/types.ts, web/default/src/features/users/components/users-mutate-drawer.tsx, and the seven Default locale files listed in the File Map
- Modify Classic: web/classic/src/components/settings/personal/components/UserInfoHeader.jsx, web/classic/src/components/table/users/modals/EditUserModal.jsx, web/classic/src/components/table/users/UsersTable.jsx, and the eight Classic locale files listed in the File Map
- Test: model/user_governance_test.go, model/user_update_test.go, model/user_root_binding_test.go, controller/user_root_binding_test.go, middleware/token_auth_user_status_test.go, new model/own_profile_test.go, new controller/own_profile_test.go, frontend permission/form tests

- [ ] **Step 1: Add failing model tests for self-profile and role matrix**

Add the role matrix as a table test and pin visible-name validation with concrete values:

~~~go
func TestValidateDisplayNameAllowsUnicodeAndRejectsUnsafeText(t *testing.T) {
    setupGovernanceOptions(t)
    require.NoError(t, ValidateDisplayName("雪屋 API 👋"))
    for _, value := range []string{"Alice\x00", "Alice\u202e", "<b>Alice</b>", "Alice  Bob"} {
        require.Error(t, ValidateDisplayName(value), value)
    }
}
~~~

The same test file must use actor/target rows for Admin to User allowed; Admin to Admin/Root denied; Root to User/Admin allowed; Root to Root denied; ID=1 to another Root allowed; and ID=1 self-demotion denied. A separate transaction test must prove the last enabled Root cannot be disabled.

Run go test ./model ./controller ./middleware -run 'OwnProfile|Governance|Root' -count=1 and confirm the new tests fail for the missing service boundary or missing route.

- [ ] **Step 2: Implement the narrow OwnProfile service boundary**

Implement this contract in model/own_profile.go and keep all permission decisions server-side:

~~~go
type OwnProfilePatch struct {
    DisplayName *string
    AvatarURL   *string
}

func UpdateOwnProfileWithTx(tx *gorm.DB, userID int, patch OwnProfilePatch, now time.Time) (*User, error) {
    var current User
    if err := lockForUpdate(tx).First(&current, userID).Error; err != nil {
        return nil, err
    }
    updates := map[string]any{}
    if patch.DisplayName != nil {
        name := strings.TrimSpace(*patch.DisplayName)
        if err := ValidateDisplayName(name); err != nil { return nil, err }
        _, cooldown := displayNamePolicy()
        if name != current.DisplayName && current.DisplayNameUpdatedAt > 0 && now.Unix() < current.DisplayNameUpdatedAt+cooldown {
            return nil, ErrNicknameCooldown
        }
        updates["display_name"] = name
        updates["display_name_updated_at"] = now.Unix()
    }
    if patch.AvatarURL != nil {
        if err := ValidateAvatarURL(*patch.AvatarURL); err != nil { return nil, err }
        updates["avatar_url"] = *patch.AvatarURL
    }
    if len(updates) > 0 {
        if err := tx.Model(&User{}).Where("id = ?", userID).Updates(updates).Error; err != nil { return nil, err }
    }
    if err := tx.First(&current, userID).Error; err != nil { return nil, err }
    return &current, nil
}
~~~

GET /api/user/self returns avatar_url, display_name, username, role, bound email state, inviter display data, and display_name_next_change_at; PUT /api/user/self accepts only the patch fields and rejects role, quota, status, bindings, or sidebar privilege changes. Keep admin edits on the governed UpdateUser path and reuse validators without sharing its writable field map.

- [ ] **Step 3: Wire session/token invalidation into every governed transition**

For role, status, password, 2FA, Passkey, email, binding, and username changes, call the existing invalidation helpers after a successful transaction. Add a test that logs in, changes status or role, then proves the old session and access Token are rejected while an unrelated user's Token remains valid.

- [ ] **Step 4: Add both theme profile and user-management states**

Default uses the existing profile drawer/dialog patterns and Classic uses Semi UI modals. Both themes must show the same fields and server error codes, hide internal ban notes, and expose the same role matrix. Add tests for loading, success, cooldown error, disabled-account error, and mobile single-column rendering.

- [ ] **Step 5: Run checks and commit**

~~~powershell
go test ./model ./controller ./middleware ./router -count=1
Push-Location web/default; bun test; bun run typecheck; Pop-Location
Push-Location web/classic; bun test; bun run eslint; Pop-Location
git diff --check
git add -- model/own_profile.go controller/own_profile.go model/user.go model/user_governance.go model/user_cache.go model/errors.go model/main.go router/api-router.go controller/user.go middleware/auth.go model/own_profile_test.go controller/own_profile_test.go model/user_governance_test.go model/user_update_test.go model/user_root_binding_test.go controller/user_root_binding_test.go middleware/token_auth_user_status_test.go web/default/src/features/profile/api.ts web/default/src/features/profile/types.ts web/default/src/features/profile/index.tsx web/default/src/features/profile/components/profile-header.tsx web/default/src/features/users/api.ts web/default/src/features/users/types.ts web/default/src/features/users/components/users-mutate-drawer.tsx web/classic/src/components/settings/personal/components/UserInfoHeader.jsx web/classic/src/components/table/users/modals/EditUserModal.jsx web/classic/src/components/table/users/UsersTable.jsx
git commit -m "feat(governance): enforce own profile and role boundaries"
~~~

## Task 3: Add WalletLedger As The Shared Money Primitive

**Files:**
- Create: model/wallet_ledger.go, service/wallet_ledger.go, model/wallet_ledger_test.go, service/wallet_ledger_test.go
- Modify: model/main.go, model/user.go, service/token_balance.go, service/quota.go, controller/topup.go, controller/token_balance.go
- Test: controller/token_balance_test.go, service/token_balance_test.go, migration tests for all supported database types

- [ ] **Step 1: Add failing idempotency and non-negative balance tests**

Use an isolated SQLite database and assert:

~~~go
func TestAdjustWithTxIsIdempotentByOperationID(t *testing.T) {
    db := setupWalletLedgerTestDB(t)
    require.NoError(t, db.Create(&model.User{Id: 7, Quota: 100}).Error)
    require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
        first, err := AdjustWithTx(tx, 7, -25, "case:7", "test", Reference{})
        require.NoError(t, err)
        second, err := AdjustWithTx(tx, 7, -25, "case:7", "test", Reference{})
        require.NoError(t, err)
        require.Equal(t, first.ID, second.ID)
        return nil
    }))
    var user model.User
    require.NoError(t, db.First(&user, 7).Error)
    require.Equal(t, 75, user.Quota)
    var count int64
    require.NoError(t, db.Model(&model.WalletLedgerEntry{}).Count(&count).Error)
    require.EqualValues(t, 1, count)
}
~~~

The insufficient-balance case starts at quota 10 and adjusts -11, then asserts the stable insufficient-balance error, unchanged quota, and zero ledger rows. The rollback case returns a sentinel error from the transaction and asserts neither the database nor cache publishes the tentative balance.

Run go test ./model ./service -run WalletLedger -count=1; the tests must fail before the model and service exist.

- [ ] **Step 2: Create the portable ledger model and migration entry**

Use integer quota and a portable string category/reference rather than database enums or JSON:

~~~go
type WalletLedgerEntry struct {
    ID            int64
    UserID        int
    OperationID   string
    Category      string
    Delta         int
    BalanceAfter  int
    ReferenceType string
    ReferenceID   string
    Context       string
    CreatedAt     int64
}
~~~

Register WalletLedgerEntry in both migrateDB and migrateDBFast, add it to the migration fixture, and assert repeated AutoMigrate keeps the unique operation index on SQLite/MySQL/PostgreSQL.

- [ ] **Step 3: Implement the transaction API and cache boundary**

Implement service.AdjustWithTx(tx, userID, delta, operationID, category, reference) with a locked user read, an idempotent existing-entry read, conditional non-negative update, and ledger insert. Return the original entry on a repeated operation. service.Adjust owns the transaction and calls model.InvalidateUserCache only after commit; composite callers collect changed user IDs and call the same post-commit hook once.

- [ ] **Step 4: Migrate new money paths without rewriting historical data**

Route user redemption funding, invitation rewards, and future wallet additions through this service. Keep legacy consumption/top-up history as historical records and label the new ledger as starting at its activation timestamp; do not fabricate old ledger rows. Update balance snapshots to return wallet_quota, token_callable_quota, and a separate subscription section.

- [ ] **Step 5: Verify and commit**

~~~powershell
go test ./model ./service ./controller -run 'WalletLedger|TokenBalance' -count=1
git diff --check
git add -- model/wallet_ledger.go service/wallet_ledger.go model/main.go model/user.go service/token_balance.go service/quota.go controller/topup.go controller/token_balance.go model/wallet_ledger_test.go service/wallet_ledger_test.go service/token_balance_test.go controller/token_balance_test.go
git commit -m "feat(wallet-ledger): add idempotent wallet accounting"
~~~

## Task 4: Implement Transactional Audit Outbox And Delivery

**Files:**
- Create: model/audit_outbox.go, service/audit_outbox.go, service/audit_outbox_worker.go, model/audit_outbox_test.go, service/audit_outbox_test.go
- Modify: model/main.go, model/log.go, controller/audit.go, controller/log.go, middleware/audit.go, main.go, common/constants.go
- Test: model/clickhouse_log_test.go, model/log_format_test.go, new relational/ClickHouse delivery tests

- [ ] **Step 1: Add failing outbox and duplicate-delivery tests**

Cover a committed business mutation with an outbox row, a rolled-back mutation with no row, retry after a failed log database write, relational duplicate event_id, and ClickHouse duplicate query results. Run go test ./model ./service -run AuditOutbox -count=1 before implementation.

- [ ] **Step 2: Add portable outbox records and migration**

Use a stable event ID, redacted payload string, status, attempt count, retry time, and last error. Add a unique event_id index to relational Log storage; add event_id to the ClickHouse table and keep MergeTree semantics explicit rather than claiming uniqueness.

- [ ] **Step 3: Expose transaction-scoped recording and worker delivery**

Implement:

~~~go
type AuditEvent struct {
    EventID   string
    Type      string
    Result    string
    ActorID   int
    Target    string
    IP        string
    RequestID string
    Params    map[string]any
}

func EnqueueWithTx(tx *gorm.DB, event AuditEvent) error
func DeliverPending(ctx context.Context, limit int) (delivered, failed int, err error)
~~~

Serialize only allow-listed redacted parameters. The worker claims rows, writes to relational logs with event_id idempotency, or writes ClickHouse with the same ID and marks the main outbox row delivered. If the acknowledgment is lost, relational writes are ignored by the unique key and ClickHouse queries deduplicate by event_id.

- [ ] **Step 4: Move security and financial events to the outbox**

Update role/status changes, invitation state changes, WalletLedger actions, Token reveals/collision handling, login success/failure, and sensitive IP reads to enqueue in the same main transaction. Keep low-risk operational logs on the existing path, but make their event types distinct.

- [ ] **Step 5: Add retry metrics and commit**

Expose pending count, oldest age, failure count, and last error through the existing system metrics path. Start the worker from main.go after databases initialize and stop it during shutdown.

~~~powershell
go test ./model ./service ./controller ./middleware -run 'Audit|Login|Wallet|Token' -count=1
git diff --check
git add -- model/audit_outbox.go service/audit_outbox.go service/audit_outbox_worker.go model/main.go model/log.go controller/audit.go controller/log.go middleware/audit.go main.go common/constants.go model/audit_outbox_test.go service/audit_outbox_test.go model/clickhouse_log_test.go model/log_format_test.go
git commit -m "feat(audit-outbox): deliver critical events reliably"
~~~


## Task 5: Replace Legacy Aff Codes With Approved Invitation Codes

**Files:**
- Create: model/invitation.go, service/invitation_registration.go, controller/invitation.go
- Modify: model/main.go, model/user.go, model/invite_limit.go, controller/registration.go, controller/user.go, controller/oauth.go, controller/wechat.go, router/api-router.go, controller/audit.go
- Create Default: web/default/src/features/invitations/api.ts, types.ts, components/user-invitation-card.tsx, components/admin-invitations-page.tsx, components/invitation-qr-dialog.tsx, routes/_authenticated/invitations/index.tsx
- Modify Default: web/default/src/features/wallet/index.tsx, web/default/src/hooks/use-sidebar-data.ts, web/default/src/hooks/use-sidebar-config.ts, and the seven Default locale files listed in the File Map
- Create Classic: web/classic/src/pages/Invitation/index.jsx, components/invitations/UserInvitationCard.jsx, AdminInvitations.jsx, InvitationQRModal.jsx
- Modify Classic: web/classic/src/App.jsx, web/classic/src/components/layout/SiderBar.jsx, web/classic/src/hooks/common/useSidebar.js, web/classic/src/pages/Setting/Personal/SettingsSidebarModulesUser.jsx, web/classic/src/pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx, and the eight Classic locale files listed in the File Map
- Test: model/invitation_test.go, service/invitation_registration_test.go, controller/invitation_test.go, controller/registration_aff_test.go, frontend invitation state tests

- [ ] **Step 1: Add failing schema, concurrency, permission, and registration tests**

Test one pending application per user, one active code per owner, 24-hour reapply delay, one-use and multi-use exhaustion, 365-day limit, Admin/Root/ID=1 object matrix, target-role change during review, and registration rollback. Include two concurrent approval/registration goroutines and assert one durable winner on SQLite.

~~~go
func TestLegacyAffCodeCannotRegisterAfterInvitationMigration(t *testing.T) {
    setupInvitationTestDB(t)
    legacy := seedLegacyInviter(t, "legacy-aff")
    _, err := RegisterWithInvitation(context.Background(), RegistrationRequest{
        User: &model.User{Username: "new-user", Password: "hashed"},
        InvitationCode: legacy.AffCode,
        Source: "password",
    })
    require.ErrorIs(t, err, ErrInvitationInvalid)
}

func TestFailedUserInsertDoesNotConsumeInvitation(t *testing.T) {
    setupInvitationTestDB(t)
    code := seedInvitationCode(t, 1)
    seedUser(t, "duplicate")
    _, err := RegisterWithInvitation(context.Background(), RegistrationRequest{
        User: &model.User{Username: "duplicate", Password: "hashed"},
        InvitationCode: code.Code,
        Source: "password",
    })
    require.Error(t, err)
    require.Equal(t, 0, reloadInvitationCode(t, code.ID).UsedCount)
    require.Zero(t, countInvitationUses(t, code.ID))
}
~~~

The concurrency test releases two goroutines against a one-use code and asserts one success, one exhausted error, one use row, and used_count equal to one. The permission table asserts Admin cannot issue to self/Admin/Root, Root cannot issue to self/Root, and ID=1 can issue to any target including itself.

Run go test ./model ./service ./controller -run 'Invitation|Register.*Aff' -count=1 and confirm the legacy resolver still accepts users.aff_code before implementation.

- [ ] **Step 2: Add the three portable invitation models**

Define InviteCodeApplication, InviteCode, and InviteCodeUse with nullable unique PendingUserSlot and ActiveOwnerSlot pointers. Use string statuses and kinds with service validation, not database enums. Add unique indexes for code and invite_code_id plus registered_user_id, register all three in normal/fast migration, and prove terminal records null their active slot.

- [ ] **Step 3: Implement application, review, direct issue, revoke, and list services**

Use one transaction per state change. Lock the current user/target and application/code where supported, re-check the actor/target role matrix inside the transaction, conditionally update the expected status, enqueue the audit event, and return stable errors such as invite_application_pending, invite_reapply_later, invite_target_forbidden, and invite_code_exhausted.

- [ ] **Step 4: Centralize all registration entry points**

Implement one entry point:

~~~go
type RegistrationRequest struct {
    User           *model.User
    InvitationCode string
    Source         string
    RegistrationIP string
}

type RegistrationResult struct {
    UserID      int
    InviteUseID int64
    InviterID   int
}

func RegisterWithInvitation(ctx context.Context, req RegistrationRequest) (RegistrationResult, error)
~~~

Password registration, OAuth, WeChat, and administrator-created users call it. The service normalizes and persists RegistrationIP from the trusted Gin client-IP boundary. Inside one transaction, lock and validate the new code, insert the user, insert InviteCodeUse, conditionally increment used_count, clear ActiveOwnerSlot on exhaustion, and enqueue reward/notification events keyed by invite_use_id. Existing finishInsert and FinalizeOAuthUserCreation stop applying rewards for new invitation uses. The worker credits both sides through WalletLedger operation IDs invite:{useID}:{side} and can retry without duplicate quota.

- [ ] **Step 5: Add user wallet and administrator views in both themes**

The wallet card supports submit, withdraw, rejection reason, current code, usage, copy code/link, and a QR icon. qrcode.react renders the registration URL in memory; download converts its canvas/SVG to PNG without an API call. Administrator navigation adds Invitation Codes with application, issued-code, and use tabs plus approve/reject/direct-issue/revoke dialogs. Both themes test loading, empty, pending, rejected, active, exhausted, and permission-denied states.

- [ ] **Step 6: Verify and commit**

~~~powershell
go test ./model ./service ./controller ./router -run 'Invitation|Register|OAuth|WeChat' -count=1
Push-Location web/default; bun test; bun run typecheck; Pop-Location
Push-Location web/classic; bun test; bun run eslint; Pop-Location
git diff --check
git add -- model/invitation.go service/invitation_registration.go controller/invitation.go model/main.go model/user.go model/invite_limit.go controller/registration.go controller/user.go controller/oauth.go controller/wechat.go router/api-router.go controller/audit.go model/invitation_test.go service/invitation_registration_test.go controller/invitation_test.go controller/registration_aff_test.go web/default/src/features/invitations web/default/src/features/wallet web/default/src/routes/_authenticated/invitations web/default/src/hooks/use-sidebar-data.ts web/default/src/hooks/use-sidebar-config.ts web/default/src/i18n/locales web/classic/src/pages/Invitation web/classic/src/components/invitations web/classic/src/App.jsx web/classic/src/components/layout/SiderBar.jsx web/classic/src/hooks/common/useSidebar.js web/classic/src/i18n/locales
git commit -m "feat(invites): require approval for registration codes"
~~~

## Task 6: Encrypt Token Secrets And Gate Every Reveal Path

**Files:**
- Create: common/token_secret_config.go, model/token_secret.go, service/token_secret.go, service/token_secret_migration.go, middleware/token_reveal.go
- Modify: model/token.go, model/main.go, model/errors.go, controller/token.go, controller/user.go, middleware/auth.go, router/api-router.go, main.go
- Modify Default: web/default/src/features/keys/api.ts, types.ts, constants.ts, components/api-keys-columns.tsx, components/data-table-row-actions.tsx, components/token-reveal-dialog.tsx, lib/api-key-form.ts, chat preset/Fluent/Dashboard Key consumers
- Modify Classic: web/classic/src/components/table/tokens/index.jsx, web/classic/src/components/table/tokens/TokensColumnDefs.jsx, web/classic/src/components/table/tokens/TokensActions.jsx, web/classic/src/components/table/tokens/modals/CopyTokensModal.jsx, web/classic/src/components/table/tokens/modals/EditTokenModal.jsx, web/classic/src/components/table/tokens/token-custom-key.js, web/classic/src/helpers/token.js, and the existing chat/Fluent/Dashboard Key consumers
- Test: common/token_secret_config_test.go, model/token_secret_test.go, service/token_secret_test.go, service/token_secret_migration_test.go, controller/token_test.go, middleware/token_reveal_test.go, frontend Key form/reveal tests

- [ ] **Step 1: Add failing crypto migration and reveal-window tests**

Cover stable key configuration, missing/invalid key rejection, dual-write round trip, fingerprint-first lookup, legacy plaintext fallback, batch backfill, exact/full/prefix/suffix search, one sk- compatibility strip, collision quarantine, password verification expiry at 30 minutes, passwordless-account response, and one outbox event per returned Key.

~~~go
func TestRevealWindowExpiresAfterThirtyMinutes(t *testing.T) {
    verifiedAt := time.Unix(1_000, 0)
    require.True(t, RevealWindowValid(verifiedAt, verifiedAt.Add(30*time.Minute-time.Second)))
    require.False(t, RevealWindowValid(verifiedAt, verifiedAt.Add(30*time.Minute)))
}

func TestTokenLookupNeverSplitsCustomKeyOnHyphen(t *testing.T) {
    require.Equal(t, []string{"part-one-two"}, model.TokenKeyLookupCandidates("part-one-two"))
    require.Equal(t, []string{"sk-part-one-two", "part-one-two"}, model.TokenKeyLookupCandidates("sk-part-one-two"))
}

func TestBackfillLeavesLegacyKeyReadableOnEncryptionFailure(t *testing.T) {
    db := setupTokenSecretTestDB(t)
    token := seedLegacyToken(t, db, "legacy-key-value")
    migrator := NewTokenSecretMigrator(failingCipher{})
    result := migrator.BackfillBatch(context.Background(), db, token.Id, 1)
    require.Equal(t, 1, result.Failed)
    require.Equal(t, "legacy-key-value", reloadToken(t, db, token.Id).Key)
}
~~~

The batch-reveal controller test seeds three owned Tokens, opens a valid password window, calls the batch endpoint, then asserts three token.secret_reveal outbox rows, one token.secret_reveal_batch row, no Key substring in any payload, and Cache-Control: no-store.

Run go test ./common ./model ./service ./controller ./middleware -run 'TokenSecret|Reveal|CustomKey|Collision' -count=1 and confirm the new migration/window tests fail.

- [ ] **Step 2: Add dedicated stable secret configuration**

Read TOKEN_HMAC_KEY and TOKEN_ENCRYPTION_KEYS from environment only. TOKEN_ENCRYPTION_KEYS is an ordered version-to-base64-key list whose first entry writes new ciphertext and remaining entries decrypt old versions. Reject invalid key length, duplicate versions, or a configured cipher-only phase without keys. Never expose these values through OptionMap or settings APIs.

- [ ] **Step 3: Add phased Token columns and crypto helpers**

Add key_fingerprint, key_ciphertext, key_version, key_prefix, key_suffix, compromise_state, compromised_at, and compromise_notice_sent_at. Phase behavior is:

1. schema: add columns and validate configured secrets;
2. dual-write: new/updated records write legacy key plus fingerprint/ciphertext, then a bounded background system task backfills IDs;
3. dual-read: authenticate fingerprint first and plaintext second while reporting mismatches, then stop new plaintext writes after all active records verify;
4. cipher-only: clear legacy plaintext only in a later explicit deployment.

Fingerprint with HMAC-SHA-256 and compare in constant time; encrypt with AES-GCM using a fresh nonce. Migration logs record counts and record IDs only.

- [ ] **Step 4: Implement search, collision, and immutable custom Keys**

Validate 16-128 ASCII letters/digits/underscore/hyphen. Users can set a custom Key only while creating their own Token; it cannot be edited. Name search remains fuzzy; a complete Key is HMAC exact search; exactly four leading or trailing characters use prefix/suffix columns. Reject percent-based arbitrary fragments. A collision returns a generic unavailable error, conditionally quarantines the existing Token once, enqueues one security event, and sends one owner notification.

- [ ] **Step 5: Implement password verification and the 30-minute session window**

POST /api/token/reveal/verify accepts the current account password and writes token_reveal_verified_at to the authenticated server session; it returns verified_until only. OAuth/Passkey accounts without a password receive password_setup_required. POST /api/user/self/password/setup requires an authenticated session, a bound/verified email, and the existing email-code verifier; administrators cannot set it for another user.

POST /api/token/:id/key and POST /api/token/batch/keys require the window, owner match, user/IP/Token rate limits, and DisableCache. They begin a main transaction, enqueue each token.secret_reveal event plus a batch summary, commit, then decrypt and return with Cache-Control: no-store. Audit failure prevents the plaintext response. middleware/token_reveal.go also records rejected, expired-window, wrong-owner, and rate-limited attempts with actor/IP/request ID and masked Token identity; these events never contain the submitted password or a Key.

- [ ] **Step 6: Remove browser-side bypasses**

Create a memory-only reveal provider in each theme. Eye, copy, bulk copy, chat presets, Fluent, and Dashboard examples all call the same reveal API; no full Key is stored in localStorage/sessionStorage, URL state, query cache, or telemetry. Clear memory on expiry, logout, password/security change, and component unmount.

- [ ] **Step 7: Verify and commit**

~~~powershell
go test ./common ./model ./service ./controller ./middleware ./router -run 'Token|Reveal|Secret|Password' -count=1
Push-Location web/default; bun test; bun run typecheck; Pop-Location
Push-Location web/classic; bun test; bun run eslint; Pop-Location
git diff --check
git add -- common/token_secret_config.go model/token_secret.go service/token_secret.go service/token_secret_migration.go middleware/token_reveal.go model/token.go model/main.go model/errors.go controller/token.go controller/user.go middleware/auth.go router/api-router.go main.go common/token_secret_config_test.go model/token_secret_test.go service/token_secret_test.go service/token_secret_migration_test.go controller/token_test.go middleware/token_reveal_test.go web/default/src/features/keys web/classic/src/components/table/tokens web/classic/src/helpers/token.js
git commit -m "feat(tokens): encrypt keys and require password reveal"
~~~

## Task 7: Align Wallet And Token Balance Views

**Files:**
- Modify: service/token_balance.go, controller/token_balance.go, controller/topup.go, model/subscription.go, router/api-router.go
- Modify Default: web/default/src/features/token-balance/api.ts, web/default/src/features/token-balance/types.ts, web/default/src/features/token-balance/index.tsx, web/default/src/features/token-balance/components/balance-metrics.tsx, web/default/src/features/token-balance/components/balance-progress.tsx, web/default/src/features/token-balance/components/balance-restrictions.tsx, web/default/src/features/wallet/api.ts, web/default/src/features/wallet/types.ts, web/default/src/features/wallet/components/wallet-stats-card.tsx, web/default/src/features/wallet/components/wallet-ledger-table.tsx
- Modify Classic: web/classic/src/pages/TokenBalance/index.jsx, web/classic/src/pages/TokenBalance/balance.js, web/classic/src/pages/TokenBalance/components/BalanceDetails.jsx, web/classic/src/pages/TokenBalance/components/BalanceMetrics.jsx, web/classic/src/pages/TokenBalance/components/BalanceProgress.jsx, web/classic/src/components/topup/RechargeCard.jsx, web/classic/src/components/topup/wallet-summary.js, web/classic/src/components/topup/WalletLedgerTable.jsx
- Test: service/token_balance_test.go, controller/token_balance_test.go, Default and Classic balance/ledger tests

- [ ] **Step 1: Add failing cross-theme contract tests**

Pin the server response to wallet_quota, token_callable_quota, token_limit, token_unlimited, and a separate subscription object. Add boundary cases for disabled user/Token, unlimited Token, zero wallet, smaller Token limit, and active subscription with positive remaining quota that does not inflate wallet_quota.

- [ ] **Step 2: Return one server-computed snapshot**

Compute token_callable_quota as zero for disabled/expired states and otherwise the minimum of wallet quota and finite Token limit. Subscription remains informational. Add paginated GET /api/wallet/ledger/self with a true total and activation-date note; never label the new ledger as all historical transactions.

- [ ] **Step 3: Render identical semantics in both themes**

Both themes display wallet balance as the primary number, Token callable quota separately, subscription status/remaining in its own block, and ledger rows with category, signed integer quota, resulting balance, reference, and timestamp. Keep each theme's existing components and mobile layout.

- [ ] **Step 4: Verify and commit**

~~~powershell
go test ./service ./controller -run TokenBalance -count=1
Push-Location web/default; bun test src/features/token-balance src/features/wallet; bun run typecheck; Pop-Location
Push-Location web/classic; bun test src/pages/TokenBalance src/components/topup; Pop-Location
git diff --check
git add -- service/token_balance.go controller/token_balance.go controller/topup.go model/subscription.go router/api-router.go service/token_balance_test.go controller/token_balance_test.go web/default/src/features/token-balance web/default/src/features/wallet web/classic/src/pages/TokenBalance web/classic/src/components/topup
git commit -m "feat(wallet-view): separate wallet token and subscription quota"
~~~

## Task 8: Complete Playground Search, Reasoning, And Image Parity

**Files:**
- Create: controller/playground_metadata_writer.go, controller/playground_metadata_writer_test.go
- Modify: service/playground_search.go, service/playground_search_test.go, controller/playground.go, controller/playground_test.go, relay/helper/valid_request.go, relay/constant/relay_mode.go, router/relay-router.go, model/option.go, controller/option.go
- Modify Default: web/default/src/features/playground/api.ts, web/default/src/features/playground/types.ts, web/default/src/features/playground/constants.ts, web/default/src/features/playground/index.tsx, web/default/src/features/playground/hooks/use-stream-request.ts, web/default/src/features/playground/components/input/playground-input-tools.tsx, web/default/src/features/playground/components/image/playground-image-panel.tsx, web/default/src/features/playground/lib/streaming/payload-builder.ts, web/default/src/features/playground/lib/streaming/stream-utils.ts, web/default/src/features/playground/lib/model/model-capability-utils.ts, web/default/src/features/playground/lib/storage/storage-schema.ts
- Modify Classic: web/classic/src/pages/Playground/index.jsx, web/classic/src/components/playground/ChatArea.jsx, web/classic/src/components/playground/MessageContent.jsx, web/classic/src/components/playground/SettingsPanel.jsx, web/classic/src/components/playground/ParameterControl.jsx, web/classic/src/hooks/playground/useApiRequest.jsx, web/classic/src/constants/playground.constants.js
- Test: existing attachment tests, payload/capability/storage tests, new metadata/image/state tests, local fake Tavily/image server tests

- [ ] **Step 1: Add failing backend search and metadata tests**

Assert the user's off/basic/advanced value controls Tavily depth, native search requires both model and route-channel capability, cancellation reaches the provider, response bodies are bounded, errors are redacted, and Playground-only SSE emits a metadata event with at most five actual sources. Ordinary /v1 streaming must not receive that event.

~~~json
{"type":"playground.metadata","search":{"status":"ok","path":"tavily","sources":[{"title":"Source","url":"https://example.test/a","domain":"example.test","snippet":"bounded"}]}}
~~~

- [ ] **Step 2: Finish the search provider and server-only configuration**

Extend PlaygroundSearchRequest with the requested depth and derive it from the request payload. Preserve the existing Tavily provider but add short dial/header/total timeouts, 1 MiB response cap, request cancellation, maximum results/context, and safe URL/title/snippet normalization. Options return configured/masked status rather than the API key.

- [ ] **Step 3: Emit and parse Playground metadata**

controller/playground_metadata_writer.go wraps the Playground response writer and emits one data event before the first chat chunk. It serializes PlaygroundSearchOutcome.Sources from context, never only a count. Default parseStreamMessageUpdates and Classic handleSSE recognize the event, attach sources to the assistant message, and do not render it as model text.

- [ ] **Step 4: Enforce reasoning capability on both sides**

Map off/low/medium/high to no field or reasoning_effort only for supported models/routes. Preserve the user's intent when changing models but strip unsupported fields from the submitted payload and reject manually forged unsupported values with a stable error. Reuse existing vendor conversion paths for Gemini/Claude budgets rather than introducing a second conversion layer.

- [ ] **Step 5: Separate chat and image state and finish Classic image mode**

Use chatModel/chatDraft/chatParams and imageModel/imageDraft/imageParams in both themes. Image candidates are the current user's actually routable image-endpoint models intersected with Root allow/deny settings; an empty result shows a configuration/group message and never falls back to a text model or all models. POST /pg/images/generations sends prompt, model, size, count, format, and supported optional quality/style/background fields. Revoke object URLs when replacing results or unmounting.

- [ ] **Step 6: Preserve document attachments and add failure coverage**

Run existing PDF, DOCX, XLS/XLSX/ODS, image, text, paste, and screen-capture tests in both themes. Add rejection tests for archives/executables, per-file/count/character limits, object URL cleanup, and proof that base64 attachments are absent from persisted Playground storage.

- [ ] **Step 7: Verify with fake services and commit**

~~~powershell
go test ./service ./controller ./relay/... -run 'Playground|Tavily|Image|Reasoning' -count=1
Push-Location web/default; bun test src/features/playground; bun run typecheck; Pop-Location
Push-Location web/classic; bun test src/components/playground src/hooks/playground; Pop-Location
git diff --check
git add -- controller/playground_metadata_writer.go controller/playground_metadata_writer_test.go service/playground_search.go service/playground_search_test.go controller/playground.go controller/playground_test.go relay/helper/valid_request.go relay/constant/relay_mode.go router/relay-router.go model/option.go controller/option.go web/default/src/features/playground web/classic/src/pages/Playground web/classic/src/components/playground web/classic/src/hooks/playground web/classic/src/constants/playground.constants.js
git commit -m "feat(playground): complete search reasoning and image parity"
~~~

## Task 9: Complete Business Records And User Security IP Audit

**Files:**
- Create: service/ip_audit.go, controller/ip_audit.go, service/ip_audit_test.go, controller/ip_audit_test.go
- Modify: model/log.go, model/user.go, controller/log.go, controller/audit.go, controller/user.go, controller/registration.go, controller/oauth.go, controller/wechat.go, middleware/auth.go, router/api-router.go, controller/midjourney.go, controller/task.go, controller/compliance.go, controller/source_attribution.go
- Modify Default: web/default/src/features/usage-logs/api.ts, web/default/src/features/usage-logs/index.tsx, web/default/src/features/usage-logs/types.ts, web/default/src/features/usage-logs/lib/format.ts, web/default/src/features/usage-logs/section-registry.tsx, web/default/src/features/usage-logs/components/ip-audit-table.tsx, web/default/src/features/users/api.ts, web/default/src/features/users/types.ts, web/default/src/features/users/components/data-table-row-actions.tsx, web/default/src/features/users/components/dialogs/user-security-audit-dialog.tsx
- Modify Classic: web/classic/src/pages/Log/index.jsx, web/classic/src/pages/Midjourney/index.jsx, web/classic/src/pages/Task/index.jsx, web/classic/src/components/table/usage-logs/index.jsx, web/classic/src/components/table/usage-logs/UsageLogsActions.jsx, web/classic/src/components/table/usage-logs/UsageLogsColumnDefs.jsx, web/classic/src/components/table/usage-logs/UsageLogsFilters.jsx, web/classic/src/components/table/usage-logs/UsageLogsTable.jsx, web/classic/src/components/table/users/UsersTable.jsx, web/classic/src/components/table/users/UsersColumnDefs.jsx, and new web/classic/src/components/table/users/modals/UserSecurityAuditModal.jsx
- Test: model/log_ip_admin_test.go, model/compliance_test.go, model/source_attribution_test.go, controller record pagination tests, IP authorization tests

- [ ] **Step 1: Add failing exact-count and object-authorization tests**

Test full-filter COUNT results beyond 10,000 rows, query error propagation, IPv4/IPv6 normalization, Admin masked summary for a manageable user, Admin rejection for Admin/Root targets, Root full timeline, ID=1 access, and an audit event whenever full IP/associations are read.

- [ ] **Step 2: Normalize the four record-list contracts**

Drawing, task, compliance, and source-attribution endpoints return items, true total, page, and page_size from the same complete filter. Model functions return errors instead of empty slices on database failure. Keep each record model separate but reuse page/filter parsing helpers.

- [ ] **Step 3: Implement server-side security-detail projection**

Persist users.registration_ip for password, OAuth, WeChat, and administrator-created accounts through the centralized registration service. service.GetUserSecurityAudit(actor, target, range) first applies the current user-management hierarchy, then queries registration, login success/failure, and API event IPs. Admin receives masked addresses, counts, last activity, and risk summary; Root/ID=1 may receive full addresses, related accounts, and timeline. The controller never fetches full rows before deciding the projection.

- [ ] **Step 4: Finish both theme tables and dialogs**

Keep URL state for category, date, scope, filters, page, and page size. Render desktop tables and mobile cards, true total, empty/error/loading states, and a separate Security & IP Audit action on user rows. Show the trusted-proxy warning when status reports unsafe proxy configuration.

- [ ] **Step 5: Verify and commit**

~~~powershell
go test ./model ./service ./controller ./middleware -run 'Log|IPAudit|Compliance|SourceAttribution|Task' -count=1
Push-Location web/default; bun test src/features/usage-logs src/features/users; bun run typecheck; Pop-Location
Push-Location web/classic; bun test src/components/table/usage-logs src/components/table/users; Pop-Location
git diff --check
git add -- service/ip_audit.go controller/ip_audit.go service/ip_audit_test.go controller/ip_audit_test.go model/log.go model/user.go controller/log.go controller/audit.go controller/user.go middleware/auth.go router/api-router.go controller/midjourney.go controller/task.go controller/compliance.go controller/source_attribution.go model/log_ip_admin_test.go model/compliance_test.go model/source_attribution_test.go web/default/src/features/usage-logs web/default/src/features/users web/classic/src/pages/Log web/classic/src/pages/Midjourney web/classic/src/pages/Task web/classic/src/components/table/usage-logs web/classic/src/components/table/users
git commit -m "feat(audit-ip): add exact records and scoped security views"
~~~


## Task 10: Add Wallet-Funded User Redemption Codes

**Files:**
- Create: model/user_redemption.go, service/user_redemption.go, controller/user_redemption.go
- Modify: model/main.go, controller/system_task_handlers.go, router/api-router.go, controller/audit.go
- Create Default: web/default/src/features/user-redemptions/api.ts, types.ts, index.tsx, components/create-dialog.tsx, claim-dialog.tsx, user-redemptions-table.tsx, routes/_authenticated/user-redemptions/index.tsx
- Modify Default: web/default/src/features/wallet/index.tsx, web/default/src/features/redemption-codes/index.tsx, web/default/src/hooks/use-sidebar-data.ts, web/default/src/hooks/use-sidebar-config.ts, and the seven Default locale files listed in the File Map
- Create Classic: web/classic/src/pages/UserRedemption/index.jsx, web/classic/src/components/user-redemptions/UserRedemptionsTable.jsx, web/classic/src/components/user-redemptions/CreateUserRedemptionModal.jsx, web/classic/src/components/user-redemptions/ClaimUserRedemptionModal.jsx
- Modify Classic: web/classic/src/App.jsx, web/classic/src/components/layout/SiderBar.jsx, web/classic/src/pages/Redemption/index.jsx, web/classic/src/pages/Setting/Personal/SettingsSidebarModulesUser.jsx, web/classic/src/pages/Setting/Operation/SettingsSidebarModulesAdmin.jsx, and the eight Classic locale files listed in the File Map
- Test: model/user_redemption_test.go, service/user_redemption_test.go, controller/user_redemption_test.go, scheduled-expiry tests, both frontend state tests

- [ ] **Step 1: Add failing money and concurrency tests**

Cover single-use creation/claim, random packet minimum unit, creator self-claim, duplicate claimer, concurrent final claims, insufficient balance, cancellation refund of only unclaimed quota, expiry refund, and retry idempotency. Run go test ./model ./service ./controller -run UserRedemption -count=1 before implementation.

- [ ] **Step 2: Add separate code and claim models**

UserRedemptionCode stores unique code, creator, kind, total/claimed/refunded integer quota, max/count claims, active/completed/cancelled/expired status, expiry/terminal timestamps, and operation IDs. UserRedemptionClaim stores code ID, claimer ID, quota, claimed time, and credit operation ID, with a unique code-plus-claimer index. Register both models in normal/fast migrations without changing legacy Redemption.

- [ ] **Step 3: Implement creation and cryptographic allocation**

Create validates one quota unit through common.QuotaFromFloat(10_000), one claim for single-use or 2-100 claims for random packets, and an expiry from one hour through 30 days. In one transaction it reserves total quota through WalletLedger operation user-redemption:{id}:reserve and inserts the active code.

For a random packet, reserve one minimum unit for every future claimant and sample uniformly with crypto/rand.Int from the current minimum through a bounded upper value. The last claimant receives all remaining quota.

~~~go
func randomClaimQuota(remaining, remainingClaims, minimum int) (int, error) {
    if remainingClaims == 1 {
        return remaining, nil
    }
    spendable := remaining - minimum*(remainingClaims-1)
    averageBound := (remaining / remainingClaims) * 2
    upper := min(spendable, max(minimum, averageBound))
    return secureIntBetween(minimum, upper)
}
~~~

- [ ] **Step 4: Implement claim, cancel, expiry, and recovery**

Claim locks the code, checks active/expiry/unique claimant, computes quota, inserts the claim, credits WalletLedger with user-redemption:{codeID}:claim:{userID}, and conditionally completes the code. Cancel/expiry refund only total minus claimed minus already refunded using user-redemption:{codeID}:refund. A scheduled system-task handler finalizes expired codes with the existing multi-node lease framework; repeated workers converge on the same terminal state and operation ID.

- [ ] **Step 5: Add user and administrator APIs and both themes**

Authenticated users create, claim, list/detail, and cancel only their own codes. Administrator list/detail uses the user hierarchy; Root sees full claim details, Admin receives the permitted projection. Wallet has Create Code and Redeem actions; My Redemption Codes shows the complete code, type, amounts, claims, expiry, state, refund, copy, details, and cancel. The existing administrator Redemption Codes page gains a separate User Codes tab rather than combining schemas.

- [ ] **Step 6: Verify and commit**

~~~powershell
go test ./model ./service ./controller ./router -run 'UserRedemption|WalletLedger|SystemTask' -count=1
Push-Location web/default; bun test src/features/user-redemptions src/features/wallet src/features/redemption-codes; bun run typecheck; Pop-Location
Push-Location web/classic; bun test src/components/user-redemptions src/pages/UserRedemption; Pop-Location
git diff --check
git add -- model/user_redemption.go service/user_redemption.go controller/user_redemption.go model/main.go controller/system_task_handlers.go router/api-router.go controller/audit.go model/user_redemption_test.go service/user_redemption_test.go controller/user_redemption_test.go web/default/src/features/user-redemptions web/default/src/features/wallet web/default/src/features/redemption-codes web/default/src/routes/_authenticated/user-redemptions web/default/src/hooks/use-sidebar-data.ts web/default/src/hooks/use-sidebar-config.ts web/default/src/i18n/locales web/classic/src/pages/UserRedemption web/classic/src/components/user-redemptions web/classic/src/App.jsx web/classic/src/components/layout/SiderBar.jsx web/classic/src/pages/Redemption web/classic/src/i18n/locales
git commit -m "feat(user-redemptions): add wallet-funded claim codes"
~~~

## Task 11: Replace The Single Email Option With A Template Center

**Files:**
- Create: model/email_template.go, service/email_template.go, controller/email_template.go
- Modify: model/main.go, model/option.go, common/constants.go, common/email.go, controller/misc.go, controller/option.go, router/api-router.go
- Create Default: web/default/src/features/system-settings/integrations/email-templates-section.tsx, email-template-dialog.tsx, email-template-preview.tsx
- Modify Default: web/default/src/features/system-settings/integrations/email-settings-section.tsx, web/default/src/features/system-settings/types.ts, web/default/src/features/system-settings/integrations/section-registry.tsx, and the seven Default locale files listed in the File Map
- Create Classic: web/classic/src/components/settings/EmailTemplateSetting.jsx, EmailTemplateModal.jsx, EmailTemplatePreview.jsx
- Modify Classic: web/classic/src/components/settings/SystemSetting.jsx, web/classic/src/pages/Setting/index.jsx, and the eight Classic locale files listed in the File Map
- Test: model/email_template_test.go, service/email_template_test.go, controller/email_template_test.go, common/email_test.go, controller/misc_test.go, frontend editor/preview tests

- [ ] **Step 1: Add failing selection, validation, migration, and SMTP tests**

Test built-in fallback, one active custom default, unknown variable rejection, code/site/expiration replacement, unsafe tag/attribute rejection, disabled/deleted/corrupt template fallback, valid EmailVerificationTemplate import, invalid legacy preservation/reporting, deterministic concurrent default changes, preview isolation, test-send rate limit, SMTP timeout, and redacted errors.

- [ ] **Step 2: Add the template model and deterministic selector**

EmailTemplate stores an 80-rune name, 200-rune subject, HTML body capped at 64 KiB, enabled/default flags, sort order, creator/updater, and timestamps. Validation parses HTML with golang.org/x/net/html, allow-lists p, br, strong, em, code, ul, ol, li, table, thead, tbody, tr, th, td, and https/mailto links with safe href/title attributes, and accepts only code, site_name, and expiration_minutes variables. Selection uses enabled plus default plus deterministic ID ordering and falls back to the code-owned template on any read/validation failure.

- [ ] **Step 3: Migrate the legacy option without data loss**

After table migration, read EmailVerificationTemplate once. A valid non-empty value becomes the first enabled/default custom template with a migration marker; empty creates none; invalid remains in Option and adds a redacted migration warning. During the compatibility release, the selector may read the legacy value only when no migrated/custom template exists. Repeated startup creates no duplicates.

- [ ] **Step 4: Add Root CRUD, preview, atomic settings save, and test send**

Root-only APIs list, create, copy, update, delete, enable, reorder, set default, preview with fixed sample values, and test-send to an explicit address. Setting a default uses a transaction that clears the previous default and updates the target. SMTP and template changes use one batch request/transaction boundary. Test sends inherit request cancellation and connection/read/total timeouts, are limited by actor and IP, and return no credentials/internal addresses.

- [ ] **Step 5: Build both theme editors safely**

Default and Classic render sanitized preview HTML inside a sandboxed iframe. The built-in row is visible and copyable but not editable/deletable. Forms expose loading, validation, save, delete confirmation, test-send, fallback warning, and retry. Existing SMTP secret fields remain write-only or masked-configured.

- [ ] **Step 6: Verify with the fake SMTP server and commit**

~~~powershell
go test ./model ./service ./controller ./common -run 'EmailTemplate|EmailVerification|SMTP' -count=1
Push-Location web/default; bun test src/features/system-settings; bun run typecheck; Pop-Location
Push-Location web/classic; bun test src/components/settings; Pop-Location
git diff --check
git add -- model/email_template.go service/email_template.go controller/email_template.go model/main.go model/option.go common/constants.go common/email.go controller/misc.go controller/option.go router/api-router.go model/email_template_test.go service/email_template_test.go controller/email_template_test.go common/email_test.go controller/misc_test.go web/default/src/features/system-settings/integrations web/default/src/features/system-settings/types.ts web/default/src/i18n/locales web/classic/src/components/settings web/classic/src/pages/Setting web/classic/src/i18n/locales
git commit -m "feat(email-templates): add safe verification template center"
~~~

## Task 12: Finish Snow Home As Vendored Root-Page Content

**Files:**
- Modify: router/web-router.go, router/web_router_test.go, web/default/src/features/home/index.tsx, web/default/src/features/home/lib/snow-header-scroll.ts, web/default/src/components/layout/components/public-header.tsx, web/classic/src/pages/Home/index.jsx
- Modify vendored assets only as targeted: web/snow/dist/index.html, web/snow/dist/assets/index-2eb69c09.js, web/snow/dist/assets/App3D-f554a111.js
- Create: web/snow/VENDORED.md, scripts/verify-snow-assets.mjs
- Modify: docs/superpowers/specs/2026-07-14-snow-home-delivery.md
- Test: router/web_router_test.go, Default home mode/scroll tests, Classic home mode tests, Playwright desktop/mobile/cycle/long-run checks

- [ ] **Step 1: Pin the existing completed behavior with regression tests**

Keep HomePageMode default/snow, theme-specific SPA history fallback, /snow-assets namespace, New API native header, no top/body/iframe gap, same-origin event validation, scroll-to-centered navigation, full-cycle return to expanded navigation, and cleanup after leaving home. Add cache-header tests: index is no-cache; hashed assets are public immutable.

- [ ] **Step 2: Inventory and verify all vendored files**

scripts/verify-snow-assets.mjs must enumerate 111 files, total bytes, entry references, missing assets, external navigation domains, and the expected status-driven Docs link. It fails when a referenced asset is absent or Website/Console/Playground links leave the current origin. Store the current file manifest/update instructions in web/snow/VENDORED.md; do not require a missing Snow source project.

- [ ] **Step 3: Apply only checked vendored link changes**

Use exact-string/count assertions in the verification script before replacing minified strings. Website maps to /, Console to the current theme's authenticated console/dashboard route, Playground to the current theme's Playground route, and Docs comes from /api/status. Parent navigation messages use a fixed event type, same-origin source check, and an allow-list of relative routes. Do not beautify or broadly rewrite App3D.

- [ ] **Step 4: Verify the 3D scene and application shell**

Start the local app and use Playwright at 1280x800 and 390x844. Capture fresh load, centered-header scroll, full-cycle return, login, console, Playground, and non-home routes. For the Snow canvas, sample pixels and assert nonblank variance after load, after scroll, after one loop, and periodically through a 10-minute run. Record console/page errors, failed requests, canvas dimensions, asset count/bytes, and whether any Three.js recursion error is reproducible.

- [ ] **Step 5: Commit the complete vendored dependency**

~~~powershell
node scripts/verify-snow-assets.mjs
go test ./router ./common -run 'Snow|Web|Embed' -count=1
Push-Location web/default; bun test src/features/home; bun run typecheck; bun run build; Pop-Location
Push-Location web/classic; bun test src/pages/Home; bun run build; Pop-Location
git diff --check
git add -- router/web-router.go router/web_router_test.go web/default/src/features/home web/default/src/components/layout/components/public-header.tsx web/classic/src/pages/Home web/snow/dist web/snow/VENDORED.md scripts/verify-snow-assets.mjs docs/superpowers/specs/2026-07-14-snow-home-delivery.md
git commit -m "fix(snow-home): finalize vendored root page integration"
~~~

Expected: every web/snow/dist file is tracked, no local database/binary/screenshot is staged, and non-root application routes still serve the selected New API theme.

## Task 13: Run Cross-Database, Cross-Theme, And Browser Delivery

**Files:**
- Create: docs/superpowers/verification/2026-07-14-final-delivery.md
- Modify only when a verification failure identifies a defect: the owning source/test file and its focused regression test
- Test: all Go, frontend, migration, fake-service, and browser suites

- [ ] **Step 1: Run full code and frontend verification**

~~~powershell
go test ./...
go build ./...
Push-Location web/default; bun test; bun run typecheck; bun run lint; bun run format:check; bun run build; Pop-Location
Push-Location web/classic; bun test; bun run lint; bun run eslint; bun run build; Pop-Location
~~~

Record exact counts and commands. Fix a discovered regression in its owning slice with a failing test first; make a focused fix commit rather than folding unrelated files into delivery.

- [ ] **Step 2: Run migration matrices**

For SQLite, MySQL, and PostgreSQL, test an empty database, upgrade from a pre-feature fixture, repeated startup, nullable invitation slots, operation/event unique indexes, Token dual-write/backfill, legacy email option import, and rollback on an injected migration failure. For a separate ClickHouse LOG_SQL_DSN, test event_id column creation, duplicate delivery, deduplicated query/export, and TTL preservation.

- [ ] **Step 3: Run fake external dependency matrices**

Use local fake Tavily, SMTP, and image upstream servers. For each, test success, timeout, cancellation, empty result, malformed/oversized response, upstream rejection, and redacted UI/log output. Confirm no paid credentials or external network access are required.

- [ ] **Step 4: Run end-to-end role and money scenarios**

Exercise User, Admin, Root, and ID=1 accounts. Cover profile cooldown/avatar, ban reason, invitation application/approval/direct issue/registration/reward, password reveal window and every Key consumer, wallet ledger, user redemption create/self-claim/concurrent claim/cancel/expiry, email template fallback/test send, IP projection, and audit delivery retry. Assert balances, status rows, audit events, and permission denials directly in the database/API.

- [ ] **Step 5: Run both themes on desktop and mobile**

For Default and Classic at 1280x800 and 390x844, visit all new routes and record loading, empty, success, validation, failure, retry, confirm, and cancellation states. Check no horizontal clipping, incoherent overlap, inaccessible icon, missing translation, or console error. Re-run the Snow canvas/cycle/10-minute checks from Task 12.

- [ ] **Step 6: Verify repository completeness and prepare local acceptance**

~~~powershell
git diff --check
git status --short
git ls-files web/snow/dist | Measure-Object
git ls-files --others --exclude-standard
git grep -n -I -E 'TOKEN_HMAC_KEY=|TOKEN_ENCRYPTION_KEYS=|api_key[[:space:]]*[:=][[:space:]]*[^\" ]+' -- ':!docs/superpowers'
~~~

Build the final executable from tracked sources, start it on the first available localhost port, and verify /api/status, /, /snow-assets/, /sign-in, /dashboard, /playground, and the Classic equivalents. Write the URL, build commit, test matrix, known non-blocking observations, and artifact locations to docs/superpowers/verification/2026-07-14-final-delivery.md.

- [ ] **Step 7: Commit final delivery evidence**

~~~powershell
git add -- docs/superpowers/verification/2026-07-14-final-delivery.md
git diff --cached --check
git diff --cached --name-only
git commit -m "test(delivery): record full feature acceptance"
~~~

Expected staged path is the delivery document only unless a separately tested fix commit was required first.

## Plan Self-Review

### Spec coverage

- Workspace classification and selective commits: Task 1 and Task 13.
- OwnProfile, nickname/avatar, roles, bans, invalidation: Task 2.
- WalletLedger and wallet/Token/subscription semantics: Tasks 3 and 7.
- Critical audit outbox, relational idempotency, ClickHouse deduplication: Task 4.
- Approved single/multi-use invitation codes, QR icon, all registration paths, idempotent rewards: Task 5.
- Custom Key exact matching, phased encryption, 30-minute password window, passwordless setup, every reveal consumer, collision quarantine: Task 6.
- Tavily depth/capability, source metadata, reasoning, document attachment preservation, image state/model parity: Task 8.
- Exact record counts and role-scoped IP security detail: Task 9.
- Wallet-funded single/random user redemption codes: Task 10.
- Built-in/custom email templates and legacy Option migration: Task 11.
- Snow root-only behavior, native navigation, loop, no white gap, relative links, vendored assets, 3D canvas checks: Task 12.
- Three primary databases, ClickHouse logs, fake external services, both themes, desktop/mobile, and local acceptance URL: Task 13.

### Type and contract consistency

- Wallet operation IDs are unique strings used by every new balance mutation.
- Critical audit events use event_id from main-database enqueue through relational/ClickHouse projection.
- Invite rewards use invite_use_id plus reward side; user redemption reserve/claim/refund IDs are deterministic.
- Reveal verification stores only token_reveal_verified_at/verified_until and never a Key.
- All list APIs return items, total, page, and page_size; frontend tables use server totals.
- Default and Classic share API/status/error semantics but keep their existing component libraries.

### Execution order

Tasks 1-4 establish workspace, identity, money, and audit foundations. Task 5 can then issue invitation rewards safely; Task 6 can require durable reveal audits; Tasks 7-11 build user-visible vertical features on those primitives; Task 12 finalizes the already working Snow surface; Task 13 proves the combined delivery. Do not reorder dependent money/audit tasks or combine their commits.
