# Invitation Reward Freeze Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve invitation reward approval history and add reversible, fully audited reward freezes that may drive wallet balance negative.

**Architecture:** Extend the existing payout request state machine and wallet ledger rather than adding a second balance system. Store every freeze/unfreeze as an immutable action row, expose one detail endpoint for reward sources and actions, and render pending/history views with responsive table/card layouts in both frontends.

**Tech Stack:** Go, GORM, SQLite/MySQL/PostgreSQL, Gin, React 19, React 18, Bun, Playwright

---

### Task 1: Finish Display Version Contract

**Files:**
- Modify: `service/system_instance.go`
- Modify: `model/log.go`
- Modify: `controller/log.go`
- Create: `controller/log_version_test.go`
- Modify: `deployment_security_test.go`

- [ ] Write failing tests for system-instance output and exact legacy log-version normalization.
- [ ] Run focused Go tests and confirm they fail because public surfaces still use `common.Version`.
- [ ] Route new public metadata through `common.DisplayVersion`; preserve setup/build machine version.
- [ ] Normalize only the exact legacy `other.admin_info.version` value before API serialization.
- [ ] Run controller, middleware, model, service and deployment guardrail tests.

### Task 2: Freeze State And Ledger Primitive

**Files:**
- Modify: `model/invitation.go`
- Modify: `model/wallet_adjust.go`
- Modify: `model/wallet_adjust_test.go`
- Modify: `model/main.go`

- [ ] Write a failing wallet test proving ordinary debits still reject debt while an explicit debt adjustment records a negative balance.
- [ ] Add `frozen` request/reward states, `freeze_revision`, and immutable `InviteRewardPayoutAction`.
- [ ] Add a narrowly named debt-capable wallet adjustment that shares existing validation and ledger logic.
- [ ] Add the action table to all migration and test fixtures.
- [ ] Run model tests on SQLite and MySQL.

### Task 3: Transactional Freeze And Unfreeze

**Files:**
- Modify: `service/invitation_reward_payout.go`
- Modify: `service/invitation_reward_payout_request_test.go`

- [ ] Write failing tests for full freeze into negative balance, idempotent duplicate rejection, unfreeze restoration, multiple revisions, permissions and audit actions.
- [ ] Implement one transaction that locks actor, target, request, items and rewards before applying the negative ledger entry.
- [ ] Implement the reverse transaction with a distinct idempotent operation ID.
- [ ] Update reward summary so frozen value is separate from paid value.
- [ ] Run focused service tests repeatedly and then the complete service package.

### Task 4: Detail And Mutation API

**Files:**
- Modify: `controller/invitation.go`
- Modify: `controller/invitation_test.go`
- Modify: `router/api-router.go`

- [ ] Write failing handler tests for detail, freeze and unfreeze contracts.
- [ ] Add request detail output with reward source accounts, registration IPs, ledger operation IDs and action history.
- [ ] Add `POST .../:id/freeze` and `POST .../:id/unfreeze`, both requiring a reason.
- [ ] Return stable invitation error codes and mark successful mutations as audit logged.
- [ ] Run controller and router tests.

### Task 5: Default Frontend History And Mobile Cards

**Files:**
- Modify: `web/default/src/features/invitations/types.ts`
- Modify: `web/default/src/features/invitations/api.ts`
- Modify: `web/default/src/features/invitations/components/invitation-reward-approvals-page.tsx`
- Modify: `web/default/src/features/invitations/invitation-rewards-ui.test.ts`
- Modify: `web/default/src/i18n/static-keys.ts`
- Modify: `web/default/src/i18n/locales/*.json`

- [ ] Write failing behavior/source-contract tests for pending/history views, detail, freeze/unfreeze and mobile cards.
- [ ] Add typed API functions and statuses.
- [ ] Render separate pending/history tabs, a detail dialog, and reason-confirmed freeze/unfreeze actions.
- [ ] Use cards below the desktop breakpoint; keep the desktop table.
- [ ] Add all locale keys, then run focused tests, typecheck, lint and i18n checks.

### Task 6: Classic Frontend History And Mobile Cards

**Files:**
- Modify: `web/classic/src/components/invitations/AdminInvitationRewards.jsx`
- Modify: `web/classic/src/components/invitations/api.js`
- Modify: `web/classic/src/i18n/locales/*.json`
- Modify: `web/classic/src/i18n/production-ui-i18n.test.js`

- [ ] Write failing tests for history visibility, detail, freeze/unfreeze and narrow-screen card rendering.
- [ ] Implement the same state machine and API actions using existing Semi controls.
- [ ] Hide pagination total on mobile and keep all actions reachable without horizontal drag.
- [ ] Run Classic tests, ESLint, Prettier and i18n lint.

### Task 7: Shared Responsive Regressions

**Files:**
- Modify: `web/default/src/components/data-table/core/pagination.tsx`
- Modify: `web/default/src/features/keys/components/api-key-group-combobox.tsx`
- Modify: affected Playground send-bar and channel-toolbar components identified by browser reproduction
- Modify: corresponding focused tests

- [ ] Add failing tests for 320px pagination wrapping, popup viewport collision, and 768px toolbar/send-bar containment.
- [ ] Apply mobile-first wrapping and collision-aware popup constraints without changing desktop information density.
- [ ] Run focused tests and Default type/lint/format checks.

### Task 8: Full Verification And Release

**Files:**
- Modify: `docs/superpowers/verification/2026-07-28-final-production-candidate.md`

- [ ] Run `go test ./...`, `go vet ./...`, SQLite and isolated MySQL 8.4 integration tests.
- [ ] Run complete Default and Classic test/lint/format/build gates.
- [ ] Rebuild the Docker candidate after Docker Desktop is healthy, preserving MySQL/Redis volumes.
- [ ] Verify display version, freeze/unfreeze and responsive layouts in a real browser at four viewport sizes.
- [ ] Audit the complete diff, scan for credentials, commit with Conventional Commits, push, label the final image with the commit and frontend hashes, then clean reproducible caches.
