# Remove Super Root Design

**Date:** 2026-07-14

## Goal

Remove the unfinished `super_root` authorization mechanism without reverting the independent profile governance, secure verification, wallet, playground, or Snow home-page work that was developed in the same branch.

## Behavior

- Users keep the existing `User`, `Admin`, and `Root` roles.
- The `User` model, JSON responses, and new database schemas no longer contain `super_root`.
- Startup and migrations no longer assign `super_root` to user ID 1.
- Super-root grant, revoke, last-super-root, and governed-transition helpers are removed.
- Existing Root management behavior remains: user ID 1 may promote and demote other Root accounts through the established controller actions; other Root users may not perform those actions; user ID 1 may not demote itself.
- Profile governance such as display names, avatars, usernames, ban reasons, and stale-update protection remains intact.
- Existing databases may retain an unused legacy `super_root` column. Runtime code no longer reads or writes it; destructive cross-database column removal is outside this change.

## Verification

- A model test asserts that `User` JSON has no `super_root` property.
- A model test asserts that `AutoMigrate(&User{})` on a new database creates no `super_root` column.
- Existing controller tests verify the ID=1 Root promotion and demotion rules.
- Repository search verifies there are no runtime references to the removed mechanism.
