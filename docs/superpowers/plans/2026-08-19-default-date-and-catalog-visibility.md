# Default 日期控件与模型广场可见性 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 统一 Default 主题日期交互，并把模型广场可见性编辑器改成语义明确的“模型分组 x 用户分组”矩阵。

**Architecture:** 在 `web/default/src/components` 增加受控日期范围组件并扩展现有 `DateTimePicker`；业务页面只转换现有值。可见性继续复用 v3 原子配置和状态模块，页面只更换为矩阵编辑器，关闭开关时保留规则并恢复平台原有目录规则。

**Tech Stack:** React 19, TypeScript, react-day-picker, Base UI Popover, Tailwind CSS, i18next, Bun, Rsbuild.

---

### Task 1: Shared Date Controls

**Files:**
- Create: `web/default/src/components/date-range-picker.tsx`
- Create: `web/default/src/components/date-range-picker.test.ts`
- Modify: `web/default/src/components/datetime-picker.tsx`
- Create: `web/default/src/components/datetime-picker.test.ts`

- [ ] **Step 1: Write failing tests**

Test empty/partial/full range labels, local start/end-of-day normalization, Today/7 Days/This month presets, controlled date-time values, clear behavior, bounds, and absence of native date/datetime-local inputs.

- [ ] **Step 2: Verify RED**

Run `bun test src/components/date-range-picker.test.ts src/components/datetime-picker.test.ts`; expect missing exports/components.

- [ ] **Step 3: Implement minimal controls**

Implement `DateRangePicker` with this public contract:

```ts
export type DateRangeValue = { from?: Date; to?: Date }
export type DateRangePickerProps = {
  value: DateRangeValue
  onChange: (value: DateRangeValue) => void
  placeholder?: string
  className?: string
}
```

Use `Calendar mode='range'`, localized presets, Clear, a stable responsive popover, and project buttons. Extend `DateTimePicker` with `minDate`, `maxDate`, `aria-label`, controlled synchronization, and project-styled calendar/time fields.

- [ ] **Step 4: Verify GREEN**

Run the same test command and expect all tests to pass.

### Task 2: Default Business Date Migration

**Files:**
- Modify: `web/default/src/features/user-redemptions/index.tsx`
- Modify: `web/default/src/features/user-redemptions/user-redemptions.state.test.ts`
- Modify: `web/default/src/features/usage-logs/components/compact-date-time-range-picker.tsx`
- Modify: `web/default/src/features/usage-logs/components/compact-date-time-range-picker.test.tsx`
- Modify: `web/default/src/features/invitations/components/admin-invitation-dialogs.tsx`

- [ ] **Step 1: Write failing integration tests**

Assert the user-redemption page exposes two range controls and preserves inclusive Unix bounds; usage logs and invitations use `DateTimePicker`; all three files contain no `type='date'` or `type='datetime-local'`.

- [ ] **Step 2: Verify RED**

Run the three focused test files; expect source and range-bound assertions to fail.

- [ ] **Step 3: Implement minimal migration**

Replace four redemption date fields with Created/Expiry range pickers. Preserve `created_from`, `created_to`, `expires_from`, and `expires_to` API values. Replace usage-log native datetime fields with controlled `DateTimePicker`. Bridge invitation React Hook Form strings to `Date` values while retaining the existing `YYYY-MM-DDTHH:mm` submission contract and 365-day bounds.

- [ ] **Step 4: Verify GREEN**

Run focused tests and `bun run typecheck`; expect no failures or diagnostics.

### Task 3: Matrix-Safe Visibility State

**Files:**
- Modify: `web/default/src/features/system-settings/models/model-catalog-visibility-state.ts`
- Modify: `web/default/src/features/system-settings/models/model-catalog-visibility-state.test.ts`

- [ ] **Step 1: Write failing state tests**

Test that unchecking one cell from an `all` row expands current user groups into `selected` before removal, selected counts ignore stale groups, default mode can change independently, and `enabled: false` retains all saved rules.

- [ ] **Step 2: Verify RED**

Run `bun test src/features/system-settings/models/model-catalog-visibility-state.test.ts`; expect matrix transition/count failures.

- [ ] **Step 3: Implement state helpers**

Add `getEffectiveUserGroupNames`, `getVisibleUserGroupCount`, and `setCatalogDefaultMode`. Update cell mutation so `all -> selected` begins with all known user groups, while preserving explicit stale identifiers when relevant. Keep existing atomic request normalization.

- [ ] **Step 4: Verify GREEN**

Run the same test file and expect all legacy and new cases to pass.

### Task 4: Visibility Matrix UI

**Files:**
- Modify: `web/default/src/features/system-settings/models/model-catalog-visibility-section.tsx`
- Modify: `web/default/src/features/system-settings/models/model-catalog-visibility-section.test.ts`

- [ ] **Step 1: Write failing UI contract tests**

Assert the page includes a semantic matrix, explicit original/custom mode labels, default-rule controls, accessible cell labels, all-current-and-future action, sticky first column, and no collapsible row editor.

- [ ] **Step 2: Verify RED**

Run the section test; expect matrix assertions to fail.

- [ ] **Step 3: Implement matrix UI**

Keep existing retry/save/conflict/query invalidation code. Render a switch status strip, default-rule segmented buttons, a horizontally scrollable table with model-group rows and user-group columns, row all/default actions, selected counts, stale-group notice, disabled matrix styling when custom rules are off, and stable mobile dimensions.

- [ ] **Step 4: Verify GREEN**

Run section and state tests; expect all tests to pass.

### Task 5: I18n and Full Verification

**Files:**
- Modify: `web/default/src/i18n/locales/en.json`
- Modify: `web/default/src/i18n/locales/zh.json`
- Modify generated fallback locale files only through the existing sync script.

- [ ] **Step 1: Add translated UI keys**

Add exact keys used for date ranges, presets, original/custom rules, default behavior, matrix cell labels, counts, and stale groups. Provide Chinese translations and run `bun run i18n:sync`.

- [ ] **Step 2: Run complete checks**

Run `bun test ./src`, `bun run typecheck`, involved-file lint, `bun run format:check`, and `bun run build`.

- [ ] **Step 3: Browser acceptance**

At `http://127.0.0.1:5301`, use Playwright on 1280x800 and 390x844. Verify both redemption ranges, usage/invitation date-time popovers, switch-off original rules, switch-on matrix save/reload, readable long names, no overlap, and no native date panel.

- [ ] **Step 4: Review scoped diff**

Run `git diff --check`, `git status --short`, and scoped `git diff --stat`. Leave all unrelated existing worktree changes untouched.
