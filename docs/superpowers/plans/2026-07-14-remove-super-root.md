# Remove Super Root Implementation Plan

> **For agentic workers:** Execute each checkbox in order and keep the existing Root controller contract green.

**Goal:** Remove the `super_root` data and authorization mechanism while retaining all unrelated branch work.

**Architecture:** Delete the capability at its model boundary instead of reverting mixed commits. Keep profile-governance helpers in `model/user_governance.go`, and rely on the established controller checks for ID=1-only Root promotion and demotion.

**Tech Stack:** Go, GORM, SQLite test fixtures, testify

---

### Task 1: Lock the removal contracts

- [ ] Add a failing JSON contract test proving `User` has no `super_root` property.
- [ ] Add a failing schema contract test proving a new users table has no `super_root` column.
- [ ] Run the tests and verify both fail against the current implementation.

### Task 2: Remove the mechanism

- [ ] Remove `SuperRoot` from `User`, update snapshots, and update maps.
- [ ] Remove super-root authorization, transition, and initialization helpers.
- [ ] Remove startup and migration backfills.
- [ ] Delete super-root-only tests and preserve profile-governance tests.
- [ ] Update the combined design document to describe the retained Root model.

### Task 3: Verify behavior

- [ ] Run the new model removal tests.
- [ ] Run all model governance/update tests.
- [ ] Run Root controller behavior tests.
- [ ] Search tracked runtime sources for residual super-root references.
- [ ] Run the relevant backend suite and build.
