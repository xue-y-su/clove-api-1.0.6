# Clove API Display Version Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every page/API-visible application version read `v1.0.0-rc.21-Clove API` while preserving the legal machine version used by builds and packages.

**Architecture:** Add one backend display-version constant and route every public version surface through it. Keep `common.Version` as the linker/env-controlled machine version, and normalize the known legacy value when historical log metadata is returned.

**Tech Stack:** Go, Gin, Testify, React status store, Bun tests, Docker

---

### Task 1: Public Version Contract

**Files:**
- Modify: `common/constants.go`
- Modify: `controller/misc.go`
- Modify: `controller/misc_test.go`
- Modify: `middleware/cors.go`
- Modify: `middleware/cors_test.go`

- [ ] **Step 1: Write failing status and header tests**

Update the status assertion and add a middleware assertion:

```go
require.Contains(t, string(response.Data), `"version":"v1.0.0-rc.21-Clove API"`)
require.NotContains(t, string(response.Data), common.Version)

func TestVersionHeaderUsesDisplayVersion(t *testing.T) {
	router := gin.New()
	router.Use(Version())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	require.Equal(t, common.DisplayVersion, recorder.Header().Get("X-New-Api-Version"))
}
```

- [ ] **Step 2: Run tests and verify RED**

Run:

```powershell
go test ./controller ./middleware -run 'TestGetStatusIncludesVersionAuthor|TestVersionHeaderUsesDisplayVersion' -count=1
```

Expected: FAIL because `common.DisplayVersion` is missing and public surfaces still use `common.Version`.

- [ ] **Step 3: Add the display constant and route public fields**

```go
var Version = "v0.0.0"
const DisplayVersion = "v1.0.0-rc.21-Clove API"
```

Use `common.DisplayVersion` in `buildStatusDataLocked` and the
`X-New-Api-Version` middleware.

- [ ] **Step 4: Run tests and verify GREEN**

```powershell
go test ./controller ./middleware -run 'TestGetStatusIncludesVersionAuthor|TestVersionHeaderUsesDisplayVersion' -count=1
```

Expected: PASS.

### Task 2: Admin Runtime And Log Surfaces

**Files:**
- Modify: `service/system_instance.go`
- Modify: `model/log.go`
- Modify: `controller/log.go`
- Create: `controller/log_version_test.go`

- [ ] **Step 1: Write failing historical log normalization tests**

```go
func TestNormalizeVisibleLogVersionsReplacesLegacyDisplayValue(t *testing.T) {
	logs := []*model.Log{{
		Other: `{"admin_info":{"version":"v1.0.0-rc.21.clove-enhanced"}}`,
	}}

	normalizeVisibleLogVersions(logs)

	require.Contains(t, logs[0].Other, common.DisplayVersion)
	require.NotContains(t, logs[0].Other, "v1.0.0-rc.21.clove-enhanced")
}

func TestNormalizeVisibleLogVersionsPreservesUnknownVersions(t *testing.T) {
	logs := []*model.Log{{Other: `{"admin_info":{"version":"v2.0.0-custom"}}`}}

	normalizeVisibleLogVersions(logs)

	require.Contains(t, logs[0].Other, "v2.0.0-custom")
}
```

- [ ] **Step 2: Run test and verify RED**

```powershell
go test ./controller -run TestNormalizeVisibleLogVersions -count=1
```

Expected: FAIL because the normalizer is missing.

- [ ] **Step 3: Implement public runtime and log mapping**

Use `common.DisplayVersion` for new system-instance responses and newly written
admin log metadata. Add a controller helper that changes only the exact legacy
value under `other.admin_info.version`, then invoke it for admin log list and
token log responses before JSON serialization.

- [ ] **Step 4: Run tests and verify GREEN**

```powershell
go test ./controller ./service ./model -run 'TestNormalizeVisibleLogVersions|SystemInstance|Log' -count=1
```

Expected: PASS.

### Task 3: Frontend And Machine-Version Guardrails

**Files:**
- Modify: `web/default/src/features/system-settings/production-controls.test.ts`
- Modify: `deployment_security_test.go`

- [ ] **Step 1: Add source-contract assertions**

Assert that the maintenance component renders its `currentVersion` input,
the status controller uses `common.DisplayVersion`, and the following machine
surfaces still use `common.Version`:

```go
assert.Contains(t, versionFile, "v1.0.0-rc.21.clove-enhanced")
assert.Contains(t, dockerCompose, "v1.0.0-rc.21.clove-enhanced")
assert.Contains(t, setupController, "Version:       common.Version")
```

- [ ] **Step 2: Run guardrail tests**

```powershell
go test . -run 'DisplayVersion|ProductionDeployment' -count=1
cd web/default
bun test src/features/system-settings/production-controls.test.ts
```

Expected: PASS without changing `VERSION`, Compose tags, Electron SemVer, or
frontend build metadata.

### Task 4: Release Verification

**Files:**
- Modify: `docs/superpowers/verification/2026-07-28-final-production-candidate.md`

- [ ] **Step 1: Run focused and broad checks**

```powershell
go test ./controller ./middleware ./service ./model -count=1
go test . -count=1
go vet ./...
cd web/default
bun test src/features/system-settings/production-controls.test.ts
bun run typecheck
```

Expected: all commands pass.

- [ ] **Step 2: Build and inspect the production artifact**

Build both frontend themes, compile the Linux binary, replace it in a new image
based on the current verified runtime layer, and label the image with the final
commit plus both frontend source fingerprints.

- [ ] **Step 3: Verify API and browser output**

```powershell
(Invoke-RestMethod http://127.0.0.1:18104/api/status).data.version
```

Expected: `v1.0.0-rc.21-Clove API`.

Use the real browser to verify
`/system-settings/operations/update-checker` shows the same value without
overflow or console errors.

- [ ] **Step 4: Commit and push**

```powershell
git add common controller middleware model service web/default/src/features/system-settings/production-controls.test.ts deployment_security_test.go docs/superpowers/verification/2026-07-28-final-production-candidate.md
git commit -m "fix: separate display and machine versions"
git push origin recovery/production-restore-20260719
```

Expected: clean worktree and identical local/remote commit SHA.
