# Seven-Area Production Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver the approved production behavior for Playground images, custom API Keys, invitation applications, profile and OAuth binding management, global announcements with managed media, IP security auditing, and centralized system controls in both frontend themes.

**Architecture:** Keep Router -> Controller -> Service -> Model ownership and use dedicated, validated setting DTOs for each business area. Persist announcement acknowledgement and media state on the server, keep all database work portable across SQLite/MySQL/PostgreSQL, and make Default and Classic consume the same API contracts while retaining their native component systems. Every behavior change follows Red-Green-Refactor and is verified first with focused tests, then cross-database, browser, relay, Docker, and Git delivery gates.

**Tech Stack:** Go 1.22+, Gin, GORM v2, SQLite/MySQL/PostgreSQL, Redis, React 19/TypeScript/Bun, React 18/JavaScript/Bun, Docker Compose, Playwright/Chromium.

---

## File Map

- Extend `setting/registration_setting/config.go` with application enablement and rejection cooldown.
- Extend `setting/operation_setting/token_setting.go` with the new-custom-Key switch.
- Create `setting/user_governance_setting/config.go` for avatar and nickname policy.
- Create `setting/announcement_setting/config.go` for popup and managed-image policy.
- Create `setting/ip_security_setting/config.go` for collection, retention, projection interval, and batch size.
- Create dedicated controllers and routes for announcement and security settings; preserve the existing dedicated registration endpoint.
- Modify `controller/token.go` and existing token collision code without changing full-Key or compromise semantics.
- Modify invitation service state derivation and both invitation cards without changing administrator ownership rules.
- Modify binding model/controller return values so self-unbind refreshes only the current session version.
- Modify Default and Classic profile layouts so the top identity card owns the edit-dialog entry.
- Remove the legacy Playground image model-filtering behavior and technical output-format controls from backend and both themes.
- Create announcement acknowledgement model/service/controller code plus managed media upload/read/cleanup code.
- Modify IP event capture, projection worker, retention service, APIs, and both themes.
- Update all supported locale files and focused i18n checks.
- Update Docker mounts and delivery documentation only after runtime verification.

## Task 1: Validated Feature Settings Contracts

**Files:**
- Modify: `setting/registration_setting/config.go`
- Modify: `setting/registration_setting/config_test.go`
- Modify: `setting/operation_setting/token_setting.go`
- Create: `setting/operation_setting/token_setting_test.go`
- Create: `setting/user_governance_setting/config.go`
- Create: `setting/user_governance_setting/config_test.go`
- Create: `setting/announcement_setting/config.go`
- Create: `setting/announcement_setting/config_test.go`
- Create: `setting/ip_security_setting/config.go`
- Create: `setting/ip_security_setting/config_test.go`
- Modify: `model/option.go`

- [ ] **Step 1: Write failing normalization and validation tests**

```go
func TestRegistrationSettingsValidateApplicationPolicy(t *testing.T) {
    settings := Settings{ApplicationsEnabled: true, ReapplyCooldownSeconds: 3600}
    require.NoError(t, ValidateSettings(settings))
    settings.ReapplyCooldownSeconds = -1
    require.Error(t, ValidateSettings(settings))
}

func TestIPSecuritySettingsValidateRuntimeBounds(t *testing.T) {
    require.NoError(t, ValidateSettings(Settings{
        Enabled: true, RetentionDays: 90,
        ProjectionIntervalSeconds: 60, ProjectionBatchSize: 500,
    }))
    require.Error(t, ValidateSettings(Settings{ProjectionIntervalSeconds: 4, ProjectionBatchSize: 500}))
    require.Error(t, ValidateSettings(Settings{ProjectionIntervalSeconds: 60, ProjectionBatchSize: 5001}))
}
```

- [ ] **Step 2: Run setting tests and verify RED**

Run: `go test ./setting/registration_setting ./setting/operation_setting ./setting/user_governance_setting ./setting/announcement_setting ./setting/ip_security_setting -count=1`

Expected: FAIL because the new fields and packages do not exist.

- [ ] **Step 3: Implement defaults, parsing, validation, and option maps**

Use these stable keys and bounds:

```go
const (
    ApplicationsEnabledKey = "registration.invitation_applications_enabled"
    ReapplyCooldownSecondsKey = "registration.invitation_reapply_cooldown_seconds"
    AllowCustomKeysKey = "token_setting.allow_custom_keys"
    PopupEnabledKey = "announcement_setting.popup_enabled"
    ImageMaxBytesKey = "announcement_setting.image_max_bytes"
    EnabledKey = "ip_security.enabled"
    RetentionDaysKey = "ip_security.retention_days"
    ProjectionIntervalSecondsKey = "ip_security.projection_interval_seconds"
    ProjectionBatchSizeKey = "ip_security.projection_batch_size"
)
```

Defaults are application enabled, 24-hour invitation cooldown, custom Keys enabled, popup enabled, 5 MiB announcement images, IP audit enabled, 90-day retention, 60-second projection interval, and 500-row batches. Validate image bytes from 1 MiB through 5 MiB; retention from 0 through 3650; interval from 5 through 3600; batch size from 10 through 5000; display-name maximum from 1 through 100; nickname cooldown from 0 through 31,536,000 seconds.

- [ ] **Step 4: Register defaults without overwriting existing options**

Add every dedicated module's `DefaultOptions()` output to `model.InitOptionMap` using the existing missing-key-only initialization pattern. Continue reading legacy avatar/nickname options as fallback, but emit only the new canonical keys when administrators save settings.

- [ ] **Step 5: Run setting tests and verify GREEN**

Run: `go test ./setting/... ./model -run 'Test(Registration|TokenSetting|UserGovernance|Announcement|IPSecurity|InitOption)' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit settings contracts**

```bash
git add setting model/option.go
git commit -m "feat(settings): add production feature controls"
```

## Task 2: Atomic Settings APIs and Audit

**Files:**
- Modify: `controller/registration.go`
- Create: `controller/announcement_setting.go`
- Create: `controller/security_setting.go`
- Create: `controller/feature_setting_test.go`
- Modify: `controller/option.go`
- Modify: `router/api-router.go`
- Modify: `router/invitation_routes_test.go`
- Create: `router/feature_setting_routes_test.go`

- [ ] **Step 1: Write failing route and rollback tests**

```go
func TestUpdateSecuritySettingsRejectsInvalidBatchWithoutPartialWrite(t *testing.T) {
    before := operation_setting.GetTokenSetting().AllowCustomKeys
    ctx, recorder := newRootJSONContext(t, http.MethodPut, "/api/security/settings", map[string]any{
        "api_key": map[string]any{"allow_custom_keys": !before},
        "ip_security": map[string]any{"projection_batch_size": 5001},
    })
    UpdateSecuritySettings(ctx)
    response := decodeAPIResponse(t, recorder)
    require.False(t, response.Success)
    require.Equal(t, before, operation_setting.GetTokenSetting().AllowCustomKeys)
}
```

- [ ] **Step 2: Run focused tests and verify RED**

Run: `go test ./controller ./router -run 'Test(UpdateSecuritySettings|FeatureSettingRoutes)' -count=1`

Expected: FAIL because the handlers and routes are absent.

- [ ] **Step 3: Add narrow GET/PUT endpoints**

Expose:

```text
GET,PUT /api/registration/settings
GET,PUT /api/announcement/settings
GET,PUT /api/security/settings
```

Require `RootAuth`, bind typed DTOs, validate the whole request before persistence, flatten all selected values into one map, and call `model.UpdateOptionsBulk` once. Protect the canonical keys from `PUT /api/option/` so a multi-field policy cannot be half-saved.

- [ ] **Step 4: Record one structured audit event per successful batch**

Store changed key names and before/after non-secret scalar values. Never log avatar upload secrets or any API Key. Mark controller audit as already logged so middleware does not create a duplicate generic event.

- [ ] **Step 5: Run API tests and verify GREEN**

Run: `go test ./controller ./router -run 'Test(RegistrationSettings|AnnouncementSettings|SecuritySettings|FeatureSettingRoutes)' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit settings APIs**

```bash
git add controller router
git commit -m "feat(settings): add atomic feature settings APIs"
```

## Task 3: Custom API Key Policy and Complete Status Labels

**Files:**
- Modify: `controller/token.go`
- Modify: `controller/token_test.go`
- Modify: `web/default/src/hooks/use-status.ts`
- Modify: `web/default/src/features/keys/lib/api-key-form.ts`
- Modify: `web/default/src/features/keys/lib/api-key-form.test.ts`
- Modify: `web/default/src/features/keys/components/api-keys-mutate-drawer.tsx`
- Modify: `web/default/src/components/status-badge.tsx`
- Modify: `web/default/src/features/keys/components/api-keys-columns.tsx`
- Modify: `web/default/src/features/keys/components/api-keys-table.tsx`
- Modify: `web/classic/src/components/table/tokens/token-custom-key.js`
- Modify: `web/classic/src/components/table/tokens/token-custom-key.test.js`
- Modify: `web/classic/src/components/table/tokens/modals/EditTokenModal.jsx`

- [ ] **Step 1: Write failing backend policy tests**

Test that disabling custom-Key creation rejects a non-empty `custom_key` with stable message `custom_api_key_creation_disabled`, while random creation and editing an existing custom Key remain valid. Re-run the existing collision tests to prove a colliding Key still disables the old owner atomically and never leaks the full value.

- [ ] **Step 2: Run backend token tests and verify RED**

Run: `go test ./controller ./model ./service -run 'Test(CustomKey|TokenCustom|AddToken)' -count=1`

Expected: the disabled-policy test FAILS.

- [ ] **Step 3: Enforce the switch only on new custom values**

Check `operation_setting.GetTokenSetting().AllowCustomKeys` after request validation and before collision handling. Do not prepend `sk-`, do not alter 16-128 character validation, and do not change existing authentication lookup or compromise handling.

- [ ] **Step 4: Write failing frontend form and layout tests**

Assert that create payload omits `custom_key` when the switch is disabled, edit mode preserves an existing value, and `StatusBadge` source no longer combines a shrinking wrapper with a truncating label.

- [ ] **Step 5: Implement both-theme controls and non-truncating status badges**

Pass the server capability through status/settings data, hide or disable only the create control, show an explanatory translated note, and render short statuses with `whitespace-nowrap` and stable minimum width instead of ellipsis.

- [ ] **Step 6: Run focused frontend tests and verify GREEN**

Run: `bun test src/features/keys/lib/api-key-form.test.ts src/components/status-badge.test.tsx` from `web/default`.

Run: `bun test src/components/table/tokens/token-custom-key.test.js` from `web/classic`.

Expected: PASS.

- [ ] **Step 7: Commit custom-Key fixes**

```bash
git add controller/token.go controller/token_test.go web/default/src web/classic/src
git commit -m "fix(tokens): govern custom keys without truncating status"
```

## Task 4: Invitation Policy, Cooldown State, and Compact Layout

**Files:**
- Modify: `service/invitation.go`
- Modify: `service/invitation_registration.go`
- Modify: `service/invitation_query_test.go`
- Modify: `controller/invitation_test.go`
- Modify: `web/default/src/features/invitations/lib/invitation-state.ts`
- Modify: `web/default/src/features/invitations/lib/invitation-state.test.ts`
- Modify: `web/default/src/features/invitations/components/user-invitation-card.tsx`
- Modify: `web/default/src/features/invitations/components/invitation-ui.test.ts`
- Modify: `web/classic/src/components/invitations/invitation-state.js`
- Modify: `web/classic/src/components/invitations/invitation-state.test.js`
- Modify: `web/classic/src/components/invitations/UserInvitationCard.jsx`
- Modify: `web/classic/src/components/invitations/invitation-ui.test.js`
- Modify: Default and Classic registration settings components.

- [ ] **Step 1: Write failing service state tests**

Cover cooldown 0, one second before expiry, exact expiry, disabled application policy, and a live active code. At exact expiry require `can_apply=true`, retain the prior application for history, and return `reapply_at=0` so clients cannot mistake it for an active rejection.

- [ ] **Step 2: Run invitation tests and verify RED**

Run: `go test ./service ./controller -run 'TestInvitation.*(Cooldown|Policy|State)' -count=1`

Expected: FAIL because cooldown is fixed at 24 hours and application enablement is not consulted.

- [ ] **Step 3: Derive policy from registration settings**

Use one helper for both `GetInvitationUserState` and `ApplyForInviteCode`; return stable policy errors with the current retry timestamp. Do not change direct administrator issuance, administrator visibility, revocation, single/multi mode, or reward behavior.

- [ ] **Step 4: Write failing view-state tests**

```ts
expect(getInvitationViewState({ application: rejected, can_apply: true }, false, false)).toBe('available')
expect(getInvitationViewState({ application: rejected, can_apply: false }, false, false)).toBe('rejected')
```

- [ ] **Step 5: Fix both themes and compact reward metrics**

Make `can_apply` authoritative before the rejected branch. Keep the rejection reason visible only while cooling down, put the retry timestamp in that banner, and return to the ordinary apply action after expiry. Use content-height cards and bounded metric cells so the reward area does not stretch to match adjacent panels.

- [ ] **Step 6: Run invitation suites and verify GREEN**

Run: `go test ./model ./service ./controller ./router -run 'Test.*Invitation' -count=1`.

Run: `bun test src/features/invitations` from `web/default`.

Run: `bun test src/components/invitations` from `web/classic`.

Expected: PASS.

- [ ] **Step 7: Commit invitation fixes**

```bash
git add setting/registration_setting service/invitation.go controller web/default/src/features/invitations web/classic/src/components/invitations
git commit -m "fix(invitations): honor policy and expired cooldowns"
```

## Task 5: Profile Dialog and Current-Session-Safe Unbinding

**Files:**
- Modify: `model/user.go`
- Modify: `model/user_oauth_binding.go`
- Modify: `model/user_binding_test.go`
- Modify: `controller/user.go`
- Modify: `controller/custom_oauth.go`
- Modify: `controller/oauth_binding_test.go`
- Modify: `controller/own_profile_test.go`
- Modify: `web/default/src/features/profile/index.tsx`
- Modify: `web/default/src/features/profile/components/profile-header.tsx`
- Refactor: `web/default/src/features/profile/components/profile-identity-card.tsx` into a dialog body owned by the header.
- Modify: `web/default/src/features/profile/lib/profile-identity.test.ts`
- Modify: `web/classic/src/components/settings/personal/cards/ProfileIdentity.jsx`
- Modify: `web/classic/src/components/settings/personal/components/UserInfoHeader.jsx`
- Modify: `web/classic/src/components/settings/personal/profile-identity.test.js`

- [ ] **Step 1: Write failing transaction return-value tests**

Require `ClearBinding` and `DeleteUserOAuthBindingWithLoginProtection` to return the exact post-transaction `security_version`. Add rollback assertions for delete/version-update failures.

- [ ] **Step 2: Run model/controller tests and verify RED**

Run: `go test ./model ./controller -run 'Test.*(ClearBinding|OAuth.*Unbind|SelfBinding)' -count=1`

Expected: FAIL because the model methods return only `error` and the session remains stale.

- [ ] **Step 3: Return and store the exact security version**

Inside the same transaction, delete the selected binding, bump the version, re-read the user version, and return it. For self-unbind handlers only, set `session.Set("security_version", updatedVersion)` and save the session before success. Administrator-initiated removals must not update any target session.

- [ ] **Step 4: Write failing profile source/UI tests**

Assert `profile/index.tsx` no longer renders a second identity card, the header has one edit action, nickname cooldown disables only `display_name`, and avatar selection/removal remains enabled when upload policy allows it.

- [ ] **Step 5: Move editing into a header-owned dialog in both themes**

Keep the upload-to-image-host flow and URL allowlist validation. Show the server-provided next-change timestamp next to the disabled nickname field; never disable the whole submit action solely because nickname is cooling down. Submit only changed fields so avatar-only edits do not resend the nickname.

- [ ] **Step 6: Run profile and binding tests and verify GREEN**

Run: `go test ./model ./controller -run 'Test.*(Profile|Binding|OAuth)' -count=1`.

Run: `bun test src/features/profile` from `web/default`.

Run: `bun test src/components/settings/personal` from `web/classic`.

Expected: PASS.

- [ ] **Step 7: Commit profile and unbinding fixes**

```bash
git add model controller web/default/src/features/profile web/classic/src/components/settings/personal
git commit -m "fix(profile): keep self unbind session and use edit dialogs"
```

## Task 6: Honest Playground Image Model and Parameter Behavior

**Files:**
- Modify: `controller/user.go`
- Modify: `controller/playground.go`
- Modify: `controller/playground_test.go`
- Delete: `service/playground_image_models.go`
- Delete: `service/playground_image_models_test.go`
- Modify: `controller/option.go`
- Modify: Default Playground system-settings files that expose image allow/deny/order fields.
- Modify: Classic Playground system-settings files that expose image allow/deny/order fields.
- Modify: `web/default/src/features/playground/lib/image/image-generation.ts`
- Modify: `web/default/src/features/playground/lib/image/image-generation.test.ts`
- Modify: `web/default/src/features/playground/components/image/playground-image-panel.tsx`
- Modify: `web/default/src/features/playground/components/image/playground-image-panel-i18n.test.ts`
- Modify: `web/classic/src/components/playground/PlaygroundImagePanel.jsx`
- Add or modify Classic image panel tests.

- [ ] **Step 1: Write failing model-list endpoint tests**

Seed models routable by the user's current group and legacy allow/deny options that would previously hide them. Require `/api/user/models?endpoint=image_generation` to return every routable model regardless of those old options.

- [ ] **Step 2: Run backend Playground tests and verify RED**

Run: `go test ./controller ./service -run 'Test.*Playground.*Image' -count=1`

Expected: FAIL because `FilterPlaygroundImageModels` still applies old policy.

- [ ] **Step 3: Remove the old filter from runtime**

Return the current-group model set from the established router/model lookup. Remove old image policy validation cases from `UpdateOption`; leave old Option rows untouched for one release so existing databases remain compatible.

- [ ] **Step 4: Write failing request-shape tests**

Require every generated request to include `response_format: "url"`; require `quality` and `background` only when the user selected a non-auto value; preserve explicit supported values and prove the abort signal is still wired.

- [ ] **Step 5: Remove the output-format UI and retain real parameters**

Delete URL/Base64 segmented controls in both themes. Keep quality and background selectors because they are sent to the upstream `/v1/images/generations` request. Label `auto` as provider-controlled and surface provider errors unchanged enough to diagnose unsupported parameters.

- [ ] **Step 6: Run Playground tests and builds**

Run: `go test ./controller ./service ./relay/helper ./relay/channel/openai -run 'Test.*(Playground|Image)' -count=1`.

Run: `bun test src/features/playground/lib/image src/features/playground/components/image` from `web/default`.

Run: `bun test src/components/playground` from `web/classic`.

Expected: PASS.

- [ ] **Step 7: Commit Playground fixes**

```bash
git add controller service web/default/src web/classic/src
git commit -m "fix(playground): expose routable image models and real controls"
```

## Task 7: Announcement Acknowledgement Persistence

**Files:**
- Create: `model/announcement_acknowledgement.go`
- Create: `model/announcement_acknowledgement_test.go`
- Modify: `model/main.go`
- Create: `service/announcement.go`
- Create: `service/announcement_test.go`
- Create: `controller/announcement.go`
- Create: `controller/announcement_test.go`
- Modify: `router/api-router.go`
- Create: `router/announcement_routes_test.go`

- [ ] **Step 1: Write failing migration and signature tests**

```go
type AnnouncementAcknowledgement struct {
    ID int64 `gorm:"primaryKey"`
    UserID int `gorm:"not null;uniqueIndex:uniq_announcement_ack,priority:1"`
    Signature string `gorm:"type:varchar(64);not null;uniqueIndex:uniq_announcement_ack,priority:2"`
    AcknowledgedAt int64 `gorm:"bigint;not null"`
}
```

Test portable migration, same-user idempotency, cross-user isolation, rejection of unknown signatures, and changed content producing a new signature.

- [ ] **Step 2: Run model/service tests and verify RED**

Run: `go test ./model ./service -run 'TestAnnouncement(Acknowledgement|Signature)' -count=1`

Expected: FAIL because the model and service do not exist.

- [ ] **Step 3: Implement canonical announcement signatures**

Hash canonical fields `id`, `content`, `extra`, `image`, `publishDate`, and `type` in fixed order with SHA-256. Select the latest published announcement only when announcements and popup are enabled. Validate acknowledgement signatures against the current published list before insertion.

- [ ] **Step 4: Add authenticated state and confirmation APIs**

Expose:

```text
GET  /api/announcement/popup-state  (TryUserAuth)
POST /api/announcement/acknowledge  (UserAuth)
```

Return announcement plus signature and `acknowledged`; log successful confirmation through the existing audit service.

- [ ] **Step 5: Run API tests and verify GREEN**

Run: `go test ./model ./service ./controller ./router -run 'TestAnnouncement' -count=1`

Expected: PASS.

- [ ] **Step 6: Commit acknowledgement backend**

```bash
git add model service controller router
git commit -m "feat(announcements): persist user popup acknowledgements"
```

## Task 8: Managed Announcement Media

**Files:**
- Create: `service/announcement_media.go`
- Create: `service/announcement_media_test.go`
- Extend: `controller/announcement.go`
- Extend: `controller/announcement_test.go`
- Modify: `router/api-router.go`
- Modify: `router/web-router.go`
- Modify: `router/web_router_test.go`
- Modify: `.env.example`
- Modify: `.env.production.example`
- Modify: `docker-compose.yml`
- Modify: `docker-compose.dev.yml`

- [ ] **Step 1: Write failing upload/storage tests**

Cover JPEG/PNG/WebP/GIF content sniffing, missing file, configured byte limit, 4096 x 4096 pixel limit, same-content deduplication, random extension spoofing, directory write failure, filename path traversal, immutable cache headers, and referenced-versus-orphan cleanup.

- [ ] **Step 2: Run media tests and verify RED**

Run: `go test ./service ./controller ./router -run 'TestAnnouncementMedia' -count=1`

Expected: FAIL because media storage and routes do not exist.

- [ ] **Step 3: Implement content-addressed storage**

Use `ANNOUNCEMENT_MEDIA_DIR`, defaulting to `/data/announcement-media` in containers and a writable local data directory outside the web bundle in development. Name files as lowercase SHA-256 plus sniffed extension, write to a temporary file in the same directory, fsync/close, then atomically rename. Never use the client filename.

- [ ] **Step 4: Add upload and public read endpoints**

Expose `POST /api/announcement/media` under `RootAuth` and `GET /api/announcement-media/:filename` publicly. Validate the filename against `^[a-f0-9]{64}\.(jpg|png|webp|gif)$`, set `Content-Type`, `Content-Length`, `Cache-Control: public, max-age=31536000, immutable`, and `X-Content-Type-Options: nosniff`.

- [ ] **Step 5: Add bounded orphan cleanup**

After a successful announcement settings save, derive all referenced managed paths, then remove unreferenced files older than a safety window. Limit files inspected per pass and never follow symlinks or leave the configured directory.

- [ ] **Step 6: Mount persistent storage in Docker**

Bind the managed directory under the existing application data volume so container replacement preserves media. Do not create an additional anonymous volume that can drift from the current `/data` lifecycle.

- [ ] **Step 7: Run media tests and verify GREEN**

Run: `go test ./service ./controller ./router -run 'TestAnnouncementMedia' -count=1`.

Expected: PASS.

- [ ] **Step 8: Commit managed media**

```bash
git add service controller router .env.example .env.production.example docker-compose.yml docker-compose.dev.yml
git commit -m "feat(announcements): store managed announcement images"
```

## Task 9: Announcement Management and Global Popup in Both Themes

**Files:**
- Modify: `web/default/src/features/system-settings/content/announcements-section.tsx`
- Modify: `web/default/src/features/system-settings/content/announcements-section.test.ts`
- Modify: `web/default/src/features/system-settings/content/index.tsx`
- Modify: `web/default/src/features/system-settings/types.ts`
- Modify: `web/default/src/features/announcements/global-announcement.ts`
- Modify: `web/default/src/features/announcements/global-announcement.test.ts`
- Modify: `web/default/src/features/announcements/global-announcement-dialog.tsx`
- Modify: `web/default/src/routes/__root.tsx`
- Modify: `web/classic/src/pages/Setting/Dashboard/SettingsAnnouncements.jsx`
- Modify: `web/classic/src/components/settings/DashboardSetting.jsx`
- Modify: `web/classic/src/components/layout/NoticeModal.jsx`
- Create: `web/classic/src/components/layout/GlobalAnnouncementModal.jsx`
- Add focused Classic announcement tests.

- [ ] **Step 1: Write failing frontend API/state tests**

Assert square uploader behavior, upload response insertion as a managed relative URL, image removal, failed upload preserving the dialog and draft, authenticated server acknowledgement, guest local acknowledgement, and content-version reappearance.

- [ ] **Step 2: Run frontend announcement tests and verify RED**

Run: `bun test src/features/announcements src/features/system-settings/content/announcements-section.test.ts` from `web/default`.

Run: `bun test src/components/layout src/pages/Setting/Dashboard` from `web/classic`.

Expected: FAIL because upload and Classic global popup behavior are absent.

- [ ] **Step 3: Build the administrator composer**

Use a compact square add-image control with `ImagePlus`, hidden file input, thumbnail preview, replace/remove actions, progress/disabled state, and translated validation messages. Store only the returned relative media path in announcement JSON. Keep text-only announcements valid.

- [ ] **Step 4: Implement global popup parity**

Mount one global dialog at each application root. Logged-in users fetch server popup state and POST acknowledgement before closing; guests use a local signature key. A failed confirmation keeps the dialog open with retry feedback. Render image without cropping important content and sanitize rich text through the existing Markdown/HTML renderer.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run both commands from Step 2.

Expected: PASS.

- [ ] **Step 6: Commit announcement frontends**

```bash
git add web/default/src web/classic/src
git commit -m "feat(announcements): add managed images and global popup parity"
```

## Task 10: Configurable IP Security Runtime and Clear UI

**Files:**
- Modify: `model/ip_security_event.go`
- Modify: `model/ip_security_event_test.go`
- Modify: `model/ip_security_retention.go`
- Modify: `model/ip_security_retention_test.go`
- Modify: `service/ip_security_projection_worker.go`
- Modify: `service/ip_security_projection_worker_test.go`
- Modify: `service/ip_audit.go`
- Modify: `service/ip_audit_test.go`
- Modify: `controller/ip_audit.go`
- Modify: `controller/ip_audit_outbox_test.go`
- Modify: `web/default/src/features/usage-logs/lib/ip-audit.test.ts`
- Modify: `web/default/src/features/usage-logs/components/ip-audit-table.tsx`
- Modify: `web/default/src/features/usage-logs/components/ip-audit-controls.tsx`
- Modify: `web/default/src/features/users/components/dialogs/user-security-audit-dialog.tsx`
- Modify: `web/classic/src/components/table/usage-logs/ip-audit-state.js`
- Modify: `web/classic/src/components/table/usage-logs/ip-audit-state.test.js`
- Modify: `web/classic/src/components/table/usage-logs/IPAuditPanel.jsx`
- Modify: `web/classic/src/components/table/users/modals/UserSecurityAuditModal.jsx`

- [ ] **Step 1: Write failing collection and disabled-state tests**

Require event enqueue to become a no-op when collection is disabled while normal logs remain written. Require list/summary endpoints to return successful payload `{enabled:false}` with empty data instead of spinning or returning 500.

- [ ] **Step 2: Write failing dynamic worker tests**

Change interval and batch options between projection iterations and require the next iteration to use the new values. Set `IP_SECURITY_PROJECTION_WORKER_ENABLED=false` and require no worker start regardless of database options.

- [ ] **Step 3: Write failing portable retention tests**

For SQLite, MySQL, and PostgreSQL fixtures, delete expired events/projections/quarantine rows in bounded batches, preserve the configured window, and rebuild lifetime rollups from retained data. For ClickHouse sources, use the shorter of configured retention and source TTL.

- [ ] **Step 4: Run backend IP suites and verify RED**

Run: `go test ./model ./service ./controller -run 'Test.*IP(Security|Audit|Retention|Projection)' -count=1`

Expected: new policy tests FAIL.

- [ ] **Step 5: Implement policy-aware capture, projection, and retention**

Read options at event capture and once per worker loop. Clamp legacy environment defaults to settings bounds. Keep Root/ID 1 events; account identity must be the API-Key owner for API traffic and the acting account for authentication events.

- [ ] **Step 6: Write failing frontend state and layout tests**

Assert distinct loading, disabled, empty, error, and data states; assert the related-account label; and assert IPv6 cells contain `min-w-0`, wrapping/breaking classes, and no fixed no-wrap layout.

- [ ] **Step 7: Implement both-theme IP audit UI**

Show explicit labels for total requests, errors, failed logins, and anomalies. Use `overflow-wrap:anywhere` or equivalent for IPv6 and constrain neighboring columns. Do not hide Root; explain the account semantics in the details view.

- [ ] **Step 8: Run IP suites and verify GREEN**

Run: `go test ./model ./service ./controller ./router -run 'Test.*IP(Security|Audit|Retention|Projection)' -count=1`.

Run: `bun test src/features/usage-logs src/features/users src/hooks/sidebar-ip-audit.test.ts` from `web/default`.

Run: `bun test src/components/table/usage-logs src/components/table/users` from `web/classic`.

Expected: PASS.

- [ ] **Step 9: Commit IP security fixes**

```bash
git add setting/ip_security_setting model service controller web/default/src web/classic/src
git commit -m "fix(ip-audit): add runtime policy and explicit UI states"
```

## Task 11: Centralized Settings UI and Full Internationalization

**Files:**
- Modify Default system settings registries, section components, types, API hooks, and tests under `web/default/src/features/system-settings/`.
- Modify Classic settings navigation and components under `web/classic/src/pages/Setting/` and `web/classic/src/components/settings/`.
- Modify: `web/default/src/i18n/static-keys.ts`
- Modify: `web/default/src/i18n/locales/{en,zh,zh-TW,fr,ja,ru,vi}.json`
- Modify: `web/classic/src/i18n/locales/{en,zh,zh-CN,zh-TW,fr,ja,ru,vi}.json`
- Modify: `web/classic/src/i18n/production-ui-i18n.test.js`

- [ ] **Step 1: Write failing section/API tests**

Require Site/Registration to expose required invite code, applications enabled, and cooldown together; Content/Announcements to expose data enabled, popup enabled, image size and composer together; Security to expose custom-Key, user-governance and IP-audit controls; Playground to omit every legacy image allowlist/denylist/order editor while retaining Tavily settings.

- [ ] **Step 2: Run settings tests and verify RED**

Run: `bun test src/features/system-settings` from `web/default`.

Run: `bun test src/checks src/i18n` from `web/classic`.

Expected: FAIL because the new controls and parity are incomplete.

- [ ] **Step 3: Implement settings sections with atomic saves**

Use toggles for booleans and bounded numeric inputs for cooldown, image size, retention, interval, batch size, nickname length, and nickname cooldown. Each section submits one dedicated DTO and displays server validation. Do not restore the removed Playground image policy controls.

- [ ] **Step 4: Add every user-facing string to all locales**

Use English source keys for Default and the existing Classic key convention. Translate all new status, field, validation, disabled, retry, account-semantics, upload, and acknowledgement strings; do not leave English fallback in Chinese locales.

- [ ] **Step 5: Run i18n and focused UI tests**

Run: `bun run i18n:sync && bun run audit:source && bun test src/features/system-settings` from `web/default`.

Run: `bun run i18n:sync && bun test src/checks src/i18n` from `web/classic`.

Expected: PASS with no missing keys.

- [ ] **Step 6: Commit settings UI and translations**

```bash
git add web/default/src/features/system-settings web/default/src/i18n web/classic/src/pages/Setting web/classic/src/components/settings web/classic/src/i18n
git commit -m "feat(settings): expose production controls in both themes"
```

## Task 12: Cross-Database, Browser, Relay, Docker, and Git Delivery

**Files:**
- Modify tests discovered by failures only when they assert an obsolete contract from this approved design.
- Modify: `docs/production-readiness.md` or the existing production delivery document used by this branch.
- Modify: `docs/superpowers/plans/2026-07-21-seven-area-production-hardening.md` to mark completed steps and record actual verification evidence.

- [ ] **Step 1: Run focused backend packages**

Run: `go test ./setting/... ./model ./service ./controller ./router ./middleware -count=1`.

Expected: PASS.

- [ ] **Step 2: Run the complete Go suite**

Run: `go test ./... -count=1`.

Expected: PASS. Report pre-existing unrelated failures separately; do not suppress them.

- [ ] **Step 3: Verify MySQL and PostgreSQL contracts**

Run the repository's existing MySQL integration command against `cloveapi-remote-copy-mysql`, including migration, option batch update, announcement acknowledgement unique index, media metadata-independent behavior, invitation cooldown, and IP retention. Start an ephemeral PostgreSQL container on D: storage and run the same migration/contract subset; remove only that ephemeral container and volume after success.

Expected: PASS on SQLite, MySQL, and PostgreSQL.

- [ ] **Step 4: Run both frontend quality gates**

From `web/default`: `bun run typecheck && bun run lint && bun run audit:source && bun test && bun run build`.

From `web/classic`: `bun run eslint && bun test && bun run build`.

Expected: PASS.

- [ ] **Step 5: Rebuild and start local Docker from current source**

Before build, check `Get-PSDrive C,D`; prune only stopped, unreferenced project build cache if needed. Rebuild the application image from the current Git tree, keep the useful MySQL/Redis/proxy containers and data volumes, restart the application, and verify `/api/status` plus authenticated home load at `http://127.0.0.1:18090`.

- [ ] **Step 6: Run role-based browser regression**

Use Root, administrator, and ordinary-user sessions. Verify desktop and mobile layouts for API Keys, invitations, profile, announcement settings/popup, IP audit, and every relevant system setting in Default; verify feature-equivalent Classic pages. Capture screenshots for evidence and inspect browser console/network for 500s, failed translations, overlap, and infinite loading.

- [ ] **Step 7: Run real rightcode image calls**

Using channel 51, group `Select`, call `gpt-image-2`, `gpt-image-2-vip`, and `nano-banana-2` through the Playground/API. Confirm all group-routable models are selectable, fixed URL output renders, supported quality/background values reach the upstream, unsupported combinations return an actionable provider error, and cancel/retry work.

- [ ] **Step 8: Verify persistence and restart behavior**

Upload an announcement image, publish it, acknowledge as a logged-in user, restart the app container, and verify image access and acknowledgement remain. Confirm guests use local acknowledgement and a changed announcement signature appears again.

- [ ] **Step 9: Audit the final diff against the specification**

Map every acceptance criterion in `docs/superpowers/specs/2026-07-21-seven-area-production-hardening-design.md` to source, automated-test, or runtime evidence. Search for removed image-policy controls, stale output-format selectors, unlocalized strings, `truncate` status labels, and direct reads of superseded option keys.

- [ ] **Step 10: Commit and push the verified delivery**

```bash
git status --short
git diff --check
git add docs
git commit -m "docs: record seven-area production verification"
git push origin recovery/production-restore-20260719
```

Expected: clean worktree, remote branch updated, and all required evidence recorded without secrets, generated caches, screenshots containing credentials, database dumps, or local runtime artifacts.
