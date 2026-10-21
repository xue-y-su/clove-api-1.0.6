# Workspace Baseline - 2026-07-14

## Repository State

- Workspace: D:\cloveapi\extracted-source
- Branch: feature/user-governance-wallet-audit-playground
- Baseline commit: ccc40b93 (docs(plan): define full feature delivery sequence)
- Dirty status entries before workspace classification: 148
- Existing modifications were produced by multiple prior tasks and are preserved. This baseline does not reset, overwrite, or classify any business change as disposable.

## File Classification

### Required delivery inputs

- Go source, frontend source, tests, specifications, plans, and verification documents.
- router/web/default/dist, router/web/classic/dist, and router/web/snow/dist test fixtures used by router embedding tests.
- web/snow/dist as a vendored runtime dependency for main.go go:embed.
- Current untracked source/tests for Playground, Snow, Token, wallet presentation, governance, and router behavior.

### Local-only ignored inputs

- .build-cache and .superpowers brainstorming/runtime state.
- SQLite sidecars, local databases, binaries, logs, screenshots, traces, and temporary validation output.
- Root-local plans directory only; docs/superpowers/plans remains a formal tracked documentation location.

### Review before any deletion

- No untracked business source, test fixture, Snow asset, design, or delivery note is deleted by workspace cleanup.
- Repeated/generated files may be removed only after an owning task proves they are unreferenced and already represented by tracked sources.

## Snow Asset Inventory

- Files: 111
- Total bytes: 19,910,364
- Total size: 18.99 MiB
- Delivery mode: vendored static assets
- Required embed root: web/snow/dist
- Acceptance rule: all 111 files must be tracked before the Snow delivery commit; local screenshots and acceptance databases stay outside Git.

## Test Baseline

Executed from the working tree on 2026-07-14:

| Check | Result |
| --- | --- |
| go test ./... | PASS; all Go packages completed, slowest reported package model at 8.689s |
| Default bun test | PASS; 130 tests, 0 failures, 28 files |
| Classic bun test | PASS; 48 tests, 0 failures, 20 files |
| Default bun run typecheck | PASS |
| Default bun run build | PASS; Rsbuild 2.1.4 |
| Classic bun run build | PASS; Rsbuild 2.1.4 |

The frontend build directories are ignored generated output and are not part of this commit. Snow is the exception because its dist is the only available runtime source and is intentionally vendored.

## Commit Discipline

- Never use git add . in this workspace.
- Stage exact files or exact reviewed hunks for one vertical feature only.
- Run git diff --check, inspect git diff --cached --name-only, and scan staged text for credentials and complete Token values before every commit.
- Do not commit .env files, local databases, binaries, server logs, screenshots, Playwright traces, or brainstorming server state.
